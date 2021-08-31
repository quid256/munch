package main

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/aymerick/raymond"
	"github.com/bep/debounce"
	"github.com/rjeczalik/notify"

	"github.com/urfave/cli/v2"
)

const (
	templateFileName = "template.html"
	outputFileName   = "render.html"
)

var recipeBookTemplate *raymond.Template

func init() {
	var err error

	recipeBookTemplate, err = raymond.ParseFile(templateFileName)
	check(err)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func renderRecipes(recipes []recipe) error {
	// Substitute the HTML into the template HTML file
	htmlRes := recipeBookTemplate.MustExec(map[string]interface{}{
		"recipes": recipes,
	})

	f, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer f.Close()

	f.Write([]byte(htmlRes))

	return nil
}

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "render",
				Aliases: []string{"r"},
				Usage:   "render specified files",
				Action: func(c *cli.Context) error {
					var recipes []recipe

					for i := 0; i < c.Args().Len(); i++ {
						f := c.Args().Get(i)

						recipeText, err := os.ReadFile(f)
						if err != nil {
							return err
						}

						fp, err := filepath.Abs(f)
						if err != nil {
							return err
						}

						recipes = append(recipes, processRecipeText(recipeText, fp, f))
					}

					check(renderRecipes(recipes))

					flushNutritionCache()

					return nil
				},
			},
			{
				Name: "watch",
				Action: func(c *cli.Context) error {

					recipeFolder := c.Args().First()
					if recipeFolder == "" {
						recipeFolder = "recipes"
					}

					recipeFolder, err := filepath.Abs(recipeFolder)
					check(err)

					var recipes map[string]recipe //:= make(map[string]recipe)
					var recipeOrder []string

					walkRecipeFolder := func() error {
						recipes = make(map[string]recipe)
						recipeOrder = nil

						return filepath.Walk(recipeFolder, func(path string, info os.FileInfo, err error) error {
							if err != nil {
								return err
							}
							if info.IsDir() {
								return nil
							}

							fp, err := filepath.Abs(path)
							recipeText, err := os.ReadFile(fp)
							if err != nil {
								return err
							}

							shortpath, err := filepath.Rel(recipeFolder, fp)
							check(err)

							recipes[fp] = processRecipeText(recipeText, fp, shortpath)
							recipeOrder = append(recipeOrder, fp)

							return nil
						})
					}
					check(walkRecipeFolder())

					renderBook := func() {
						log.Println("Rendering book...")
						var recipeList []recipe
						for _, r := range recipeOrder {
							recipeList = append(recipeList, recipes[r])
						}
						check(renderRecipes(recipeList))
						flushNutritionCache()
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
							var err error
							recipeBookTemplate, err = raymond.ParseFile(templateFileName)
							check(err)

							debounced(renderBook)

						case ei := <-recipeEvents:
							log.Println("Recipe change event:", ei)

							switch ei.Event() {
							case notify.Write:
								p := ei.Path()
								recipeText, err := os.ReadFile(p)
								if err != nil {
									return err
								}

								rel, err := filepath.Rel(recipeFolder, p)
								check(err)

								recipes[p] = processRecipeText(recipeText, p, rel)

								debounced(renderBook)
							case notify.Create:
								check(walkRecipeFolder())

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
