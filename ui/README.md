# vJailbreak UI

## Running the App Locally in Development Mode

Make sure you have the following installed before getting started:

- **Node.js** version 18
- **Yarn** for package management

To run the app locally:

1. Create a .env file and set these env variables:

- **VITE_API_HOST**: Specify the backend server.
- **VITE_API_TOKEN**: This token is added in the Authorization header for API requests.

2. Install dependencies:
   `yarn`

3. Run the dev server:
   `yarn dev`

4. Load the UI in your browser at `http://localhost:3000`

## E2E Tests (Playwright)

1. Install Playwright browsers:
   `yarn e2e:install`

2. Run E2E tests:
   `yarn e2e`

3. Open the Playwright UI runner:
   `yarn e2e:ui`
