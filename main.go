package main

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/aymerick/raymond"
	"github.com/bep/debounce"
	"github.com/rjeczalik/notify"

	"github.com/urfave/cli/v2"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// renderRecipes takes a list of recipe objects, renders them into the HTML
// template file, and writes the result to the appropriate output file name.
func renderRecipes(recipes []recipe, template *raymond.Template, outputFile string, password string) error {
	// Marshall of the recipes into JSON
	recipeJSON, err := json.Marshal(recipes)
	if err != nil {
		return err
	}

	// Compress the JSON using gzip
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(recipeJSON)
	check(err)
	err = zw.Close()
	check(err)

	b := buf.Bytes()

	// Use the provided password to encrypt the compressed JSON
	hashedPassword := sha256.Sum256([]byte(password))
	iv, err := base64.StdEncoding.DecodeString("AAAAAAAAAAAAAAAAAAAAAA==")
	check(err)

	// (some random padding necessary for AES, lifted from some random Github gist)
	n := aes.BlockSize - (len(b) % aes.BlockSize)
	pb := make([]byte, len(b)+n)
	copy(pb, b)
	copy(pb[len(b):], bytes.Repeat([]byte{byte(n)}, n))

	c, err := aes.NewCipher(hashedPassword[:])
	cfbdec := cipher.NewCBCEncrypter(c, iv)
	ciphertext := make([]byte, len(pb))
	cfbdec.CryptBlocks(ciphertext, pb)

	// Insert the encrypted recipe data into the appropriate spot in the
	// generated HTML
	htmlRes := template.MustExec(map[string]interface{}{
		"recipeData": base64.RawStdEncoding.EncodeToString(ciphertext),
	})

	// Write the generated HTML to a file
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write([]byte(htmlRes)); err != nil {
		return err
	}

	return nil
}

func readRecipeFile(fp string, recipeFolder string) (recipe, error) {
	recipeText, err := os.ReadFile(fp)
	if err != nil {
		return recipe{}, err
	}

	shortpath, err := filepath.Rel(recipeFolder, fp)
	if err != nil {
		return recipe{}, err
	}

	return processRecipeText(recipeText, fp, shortpath), nil
}

func readRecipeDir(recipeFolder string) ([]recipe, error) {
	var recipes []recipe

	err := filepath.Walk(recipeFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		recipe, err := readRecipeFile(path, recipeFolder)
		check(err)

		recipes = append(recipes, recipe)

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(recipes, func(i, j int) bool {
		return recipes[i].FilePath < recipes[j].FilePath
	})

	return recipes, nil
}

func main() {
	app := &cli.App{
		Name:  "munch",
		Usage: "Render you a cookbook",
		Commands: []*cli.Command{
			{
				Name:    "render",
				Aliases: []string{"r"},
				Usage:   "Render the specified directory into a dynamic cookbook HTML",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Render recipe book to the specified file",
						Value:   "render.html",
					},
					&cli.StringFlag{
						Name:    "template",
						Aliases: []string{"t"},
						Usage:   "File to use for recipe book template",
						Value:   "template.html",
					},
					&cli.StringFlag{
						Name:    "pass",
						Aliases: []string{"p"},
						Usage:   "Password to use for encryption (default is 'password')",
						Value:   "password",
					},
				},
				Action: func(c *cli.Context) error {
					if c.Args().Len() != 1 {
						panic("Only 1 argument allowed")
					}

					fp := c.Args().Get(0)
					fileInfo, err := os.Stat(fp)
					check(err)
					if !fileInfo.IsDir() {
						panic("Specified arg must be a dir")
					}

					template, err := raymond.ParseFile(c.String("template"))
					check(err)

					recipes, err := readRecipeDir(fp)
					check(err)

					check(renderRecipes(recipes, template, c.String("output"), c.String("pass")))
					writeNutritionCache()

					return nil
				},
			},
			{
				Name:    "watch",
				Aliases: []string{"w"},
				Usage:   "Watch the specified directory for recipe changes and update the cookbook HTML",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Render recipe book to the specified file",
						Value:   "render.html",
					},
					&cli.StringFlag{
						Name:    "template",
						Aliases: []string{"t"},
						Usage:   "File to use for recipe book template",
						Value:   "template.html",
					},
					&cli.StringFlag{
						Name:    "pass",
						Aliases: []string{"p"},
						Usage:   "Password to use for encryption (default is 'password')",
						Value:   "password",
					},
				},
				Action: func(c *cli.Context) error {
					if c.Args().Len() != 1 {
						panic("Only 1 argument allowed")
					}

					recipeFolder := c.Args().Get(0)
					fileInfo, err := os.Stat(recipeFolder)
					check(err)
					if !fileInfo.IsDir() {
						panic("Specified arg must be a dir")
					}

					recipeFolder, err = filepath.Abs(recipeFolder)
					check(err)

					recipes, err := readRecipeDir(recipeFolder)
					check(err)

					templateFileName := c.String("template")
					template, err := raymond.ParseFile(templateFileName)
					check(err)

					renderBook := func() {
						log.Println("Rendering book...")
						check(renderRecipes(recipes, template, c.String("output"), c.String("pass")))
						writeNutritionCache()
					}

					renderBook()

					debounced := debounce.New(100 * time.Millisecond)

					watchPath := path.Join(recipeFolder, "...")
					recipeEvents := make(chan notify.EventInfo, 1)
					templateEvents := make(chan notify.EventInfo, 1)

					if err := notify.Watch(watchPath, recipeEvents, notify.All); err != nil {
						log.Fatal(err)
					}
					defer notify.Stop(recipeEvents)

					if err := notify.Watch(templateFileName, templateEvents, notify.All); err != nil {
						log.Fatal(err)
					}
					defer notify.Stop(templateEvents)

					for {
						select {
						case ti := <-templateEvents:
							log.Println("Template change event:", ti)
							template, err = raymond.ParseFile(templateFileName)
							check(err)

							debounced(renderBook)

						case ei := <-recipeEvents:
							p := ei.Path()
							rel, err := filepath.Rel(recipeFolder, p)
							check(err)

							log.Printf("Ev %v: %s", ei.Event(), rel)

							switch ei.Event() {
							case notify.Write:
								recipeText, err := os.ReadFile(p)
								if err != nil {
									return err
								}

								for i, r := range recipes {
									if r.FilePath == rel {
										recipes[i] = processRecipeText(recipeText, p, rel)
										break
									}
								}

								debounced(renderBook)
							case notify.Create:
								r, err := readRecipeFile(p, recipeFolder)
								check(err)

								recipes = append(recipes, r)

								debounced(renderBook)
							case notify.Remove, notify.Rename:
								for i, r := range recipes {
									if r.FilePath == rel {
										recipes = append(recipes[:i], recipes[i+1:]...)
										break
									}
								}

								debounced(renderBook)
							}
						}

					}

				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
