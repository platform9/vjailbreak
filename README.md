# vJailbreak UI

# Getting started

**Dependencies**

- Node 18
- yarn 
- [vJailbreak v2v-helper](https://github.com/platform9/vjailbreak)

## Running the app locally for development

***Initializing the App***

`yarn`

***Create a custom config***

`cp config.example.js config.js`

`config.js` is already specified in the .gitignore.

To have the API calls go to a dev server, you can edit the `apiHost` property in `config.js` with the URL like: 

```apiHost: 'https://dev-server.com'```

Unless you override the `MODE` environment variable it will look under the `development` section of `config.js`.

To run the app:

`yarn dev`

Load the UI in your browser at `localhost:3000` as the browser URL.


## Building the Docker Image

To build the Docker image, run:

`yarn docker:build`

The image will be tagged as `vjailbreak:latest`
