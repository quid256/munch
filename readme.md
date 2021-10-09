# Munch

An app to generate recipe books. Some features:

- Recipes are written in templated markdown
- Nutrition information is pulled automatically from Nutritionix
- Recipes are rendered to a "cookbook" (which is a static html file
  `render.html` by default) that
  - Has an index and is searchable
  - Is easily printable
  - Is encrypted (so you can host your recipe book in a public location even if
    it contains proprietary recipes)
  - Is mobile-friendly (WIP)

## Setup
In order to run munch, you'll need to do the following things:

1. Install golang (https://golang.org/)
1. Clone this repo by running
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
1. Navigate to the folder of the repo in a terminal and run
   ```
   go build -o bin/munch
   ```
   to build the executable
1. Run munch by calling
   ```
   bin/munch watch recipes
   ```
   This will tell `munch` to watch the recipes folder and create/update
   `render.html` whenever a recipe is changed.
1. Open `render.html` in a web browser and enter 'password', the default
   password, to log in to the web interface.
1. Cook!

Take a look at the example recipes in the `recipes/` folder to see how to format
your own recipe files. They're pretty simple to make and render into
nice-ish HTML with automatically pulled nutrition information!