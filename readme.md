# Munch

An app to generate recipe books. Some features:

- Recipes are written in templated markdown
- Nutrition information is pulled automatically from Nutritionix
- Recipes are rendered to a "cookbook" (which is a static html file
  `render.html`) that
  - Has an index and is searchable
  - Is easily printable


## Setup
In order to run munch, you'll need to do the following things:

1. Install golang (LINK)
1. Clone this repo with
   ```
   git clone https://github.com/quid256/munch
   ```
1. Get a Nutritionix API key and put the credentials in a file called
   `credentials.json` of the form
   ```
   {
     "app_key": "APP_KEY_HERE",
     "app_id": "APP_ID_HERE"
   }
   ```
   Put this file in the folder you cloned the repository
1. Navigate to this folder in a terminal and run
   ```
   go build -o bin/munch
   ```
   to build the executable
1. Run munch by calling
   ```
   bin/munch watch recipes
   ```
   This will tell munch to watch the recipes folder and create/update
   `render.html` whenever a recipe is changed.
1. Open `render.html` in a web browser and cook!
