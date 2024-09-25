# vJailbreak UI

## Getting Started

### Prerequisites

Make sure you have the following installed before getting started:

- **Node.js** version 18
- **Yarn** for package management
- **[vJailbreak v2v-helper](https://github.com/platform9/vjailbreak)** for backend services

## Running the App

### Setting Environment Variables

Before running the app in **any environment** (development or production), the following environment variables must be set:

- **VITE_API_TOKEN**: The API token for authentication.
- **VITE_API_HOST**: For development only. Specify this if you're developing against a backend with a different domain.

These variables are necessary for the app to function correctly.

### Running the App in Development Mode

To start the app locally in development mode:

1. Install dependencies:
   `yarn`

2. Run the dev server:
   `yarn dev`

3. Load the UI in your browser at `http://localhost:3000`

# Dockerizing the App

To build the Docker image, run:

`yarn docker:build`

The resulting image will be tagged as `vjailbreak:latest`
