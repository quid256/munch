package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aymerick/raymond"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

const nutritionCacheFileName = ".cache.json"

var fractions = map[string]string{
	"1/2": "&frac12;",
	"1/3": "&frac13;",
	"2/3": "&frac23;",
	"1/4": "&frac14;",
	"3/4": "&frac34;",
	"1/8": "&frac18;",
	"3/8": "&frac38;",
	"5/8": "&frac58;",
	"7/8": "&frac78;",
}

type recipe struct {
	Instructions string
	Nutrition    nutritionSummary

	ProteinPercent float64
	CarbsPercent   float64
	FatPercent     float64

	FilePath string
}

type nutritionCacheEntry struct {
	Hash             string
	NutritionSummary nutritionSummary
}

var nutritionCache map[string]nutritionCacheEntry

func init() {
	if _, err := os.Stat(nutritionCacheFileName); os.IsNotExist(err) {
		nutritionCache = make(map[string]nutritionCacheEntry)
	} else {
		b, err := ioutil.ReadFile(nutritionCacheFileName)
		check(err)

		err = json.Unmarshal(b, &nutritionCache)
		check(err)
	}
}

var curNutrition nutritionSummary

func createMaterialsHelper(filePath string) func(options *raymond.Options) raymond.SafeString {
	return func(options *raymond.Options) raymond.SafeString {
		servingsStr := options.HashStr("servings")
		if servingsStr == "" {
			panic("No `servings` provided for " + filePath)
		}
		numServings, err := strconv.Atoi(servingsStr)
		check(err)

		ingr := strings.TrimSpace(options.Fn())
		lines := strings.Split(ingr, "\n")
		var ingrProcessed strings.Builder
		var ingrList strings.Builder

		for i, l := range lines {
			var nonComment string

			commentStart := strings.Index(l, "//")
			if commentStart >= 0 {
				nonComment = l[:commentStart]
			} else {
				nonComment = l
			}

			if strings.TrimSpace(nonComment) == "" {
				continue
			}

			if i > 0 {
				ingrProcessed.WriteRune('\n')
				ingrList.WriteRune('\n')
			}

			ingrProcessed.WriteString(nonComment)
			ingrList.WriteString("- " + nonComment)

			if commentStart >= 0 {
				ingrList.WriteString("<span class=\"deemph\">(" + strings.TrimSpace(l[commentStart+2:]) + ")</span>")
			}
		}

		ingr = ingrProcessed.String()

		sum := md5.Sum([]byte(ingr + "|||||SERVINGS=" + servingsStr))
		hash := base64.StdEncoding.EncodeToString(sum[:])

		// recompute the nutrition information, if necessary
		fileName := filepath.Base(filePath)
		cached, ok := nutritionCache[fileName]
		if !ok || cached.Hash != hash {
			log.Printf("Pulling Nutritionix data for %s\n", fileName)

			curNutrition = getNutritionInformation(nutritionixQuery{
				Query:         ingr,
				NumServings:   numServings,
				LineDelimited: true,
			})

			nutritionCache[fileName] = nutritionCacheEntry{
				Hash:             hash,
				NutritionSummary: curNutrition,
			}
		} else {
			curNutrition = cached.NutritionSummary
		}

		return raymond.SafeString(ingrList.String())
	}
}

func processRecipeText(recipeText []byte, fp string, shortpath string) recipe {
	tmpl := raymond.MustParse(string(recipeText))

	tmpl.RegisterHelper("MAT", createMaterialsHelper(fp))

	res := tmpl.MustExec(map[string]string{})

	// Substitute fractions for their pretty counterparts
	re := regexp.MustCompile(`([^0-9])([1-9]/[1-9])([^0-9])`)
	res = re.ReplaceAllStringFunc(res, func(s string) string {
		matches := re.FindStringSubmatch(s)

		if fr, ok := fractions[matches[2]]; ok {
			return matches[1] + fr + matches[3]
		}

		return s
	})

	// Convert the Markdown to HTML, and panic if failure occurs
	var buf bytes.Buffer
	md := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))
	check(md.Convert([]byte(res), &buf))

	proteinCals := 4 * curNutrition.Protein
	carbCals := 4 * curNutrition.TotalCarbs
	fatCals := 9 * curNutrition.TotalFat
	totalCals := proteinCals + carbCals + fatCals

	return recipe{
		Instructions:   buf.String(),
		Nutrition:      curNutrition,
		ProteinPercent: math.Round(100 * proteinCals / totalCals),
		CarbsPercent:   math.Round(100 * carbCals / totalCals),
		FatPercent:     math.Round(100 * fatCals / totalCals),
		FilePath:       shortpath,
	}
}

func writeNutritionCache() {
	b, err := json.Marshal(nutritionCache)
	check(err)

	cacheFile, err := os.Create(nutritionCacheFileName)
	check(err)
	defer cacheFile.Close()

	cacheFile.Write(b)
}
