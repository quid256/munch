package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math"
	"net/http"
	"reflect"
)

type (
	nutritionixCreds struct {
		AppKey string `json:"app_key"`
		AppID  string `json:"app_id"`
	}

	nutritionixQuery struct {
		Query         string `json:"query"`
		NumServings   int    `json:"num_servings"`
		LineDelimited bool   `json:"line_delimited"`
	}

	nutritionSummary struct {
		Calories     float64 `json:"nf_calories"`
		TotalFat     float64 `json:"nf_total_fat"`
		SaturatedFat float64 `json:"nf_saturated_fat"`
		Protein      float64 `json:"nf_protein"`
		TotalCarbs   float64 `json:"nf_total_carbohydrate"`

		Sugars       float64 `json:"nf_sugars"`
		Cholesterol  float64 `json:"nf_cholesterol"`
		Sodium       float64 `json:"nf_sodium"`
		DietaryFiber float64 `json:"nf_dietary_fiber"`
	}

	foodInfo struct {
		nutritionSummary

		FoodName    string  `json:"food_name"`
		ServingQty  float64 `json:"serving_qty"`
		ServingUnit string  `json:"serving_unit"`

		// FullNutrients []nutrient `json:"full_nutrients"`
	}

	// nutrient struct {
	// 	ID    int     `json:"attr_id"`
	// 	Value float64 `json:"value"`
	// }
)

const nutritionixURL = "https://trackapi.nutritionix.com/v2/natural/nutrients"

var creds nutritionixCreds
var nutritionFields []string

func init() {
	// Load the credentials for Nutritionix into memory when the program is started
	credsData, err := ioutil.ReadFile("credentials.json")
	check(err)

	json.Unmarshal(credsData, &creds)

	nutSummary := reflect.ValueOf(nutritionSummary{}).Type()

	for i := 0; i < nutSummary.NumField(); i++ {
		nutritionFields = append(nutritionFields, nutSummary.Field(i).Name)
	}
}

var client = &http.Client{}

func getNutritionInformation(query nutritionixQuery) nutritionSummary {

	// Marshal the request object
	jsonBytes, err := json.Marshal(query)
	check(err)

	// Construct the request to the API
	req, err := http.NewRequest("POST", nutritionixURL, bytes.NewReader(jsonBytes))
	check(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-app-id", creds.AppID)
	req.Header.Set("x-app-key", creds.AppKey)
	req.Header.Set("x-remote-user-id", "0")

	// Actually send the request
	r, err := client.Do(req)
	check(err)

	// Read and unmarshal the response
	rBody, err := ioutil.ReadAll(r.Body)
	check(err)

	var fullJSON struct {
		Foods []foodInfo `json:"foods"`
	}

	err = json.Unmarshal(rBody, &fullJSON)
	check(err)

	var totalNutrition nutritionSummary

	// Sum all of the nutrient values across ingredients (per serving)
	for _, field := range nutritionFields {
		var total float64
		for _, food := range fullJSON.Foods {
			total += reflect.ValueOf(food.nutritionSummary).FieldByName(field).Float()
		}
		total = math.Round(total*10) / 10
		reflect.ValueOf(&totalNutrition).Elem().FieldByName(field).SetFloat(total)
	}

	return totalNutrition
}
