/*
 * This is an example of how to set up config.ts.
 * Copy this file to config.js and edit.
 * Do NOT rename this file, it will show up as
 * a delete in git.
 */

const config = {
  production: {
    // apiHost is intentionally left empty for production to use relative paths,
    // making the domain flexible and ensuring API requests go to the same domain where the app is hosted.
    apiHost: "",
  },
  development: {
    // For development, use the specific backend URL for the local environment.
    apiHost:
      "https://spot.rackspace.com/apis/ngpc.rxt.io/v1/namespaces/org_D6clXT3Lim42oPwu/cloudspaces/vjailbreak/proxy",
  },
}

// Ensure that only two modes exists: production and development
const env = import.meta.env.MODE === "production" ? "production" : "development"
export default config[env]
