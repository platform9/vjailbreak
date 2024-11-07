import react from "@vitejs/plugin-react"
import path from "path"
import { defineConfig, loadEnv } from "vite"
import runtimeEnv from "vite-plugin-runtime-env"

export default defineConfig(({ mode }) => {
  // Load env variables based on the current mode (development, production, etc.)
  const env = loadEnv(mode, process.cwd())

  return {
    plugins: [
      runtimeEnv(),
      react({
        jsxImportSource: "@emotion/react",
        babel: {
          plugins: ["@emotion/babel-plugin"],
        },
      }),
    ],
    server: {
      port: 3000,
      // Proxy api requests with the /api prefix to the target server. This is useful for development
      // since it can bypass CORS restrictions. Vite intercepts requests that
      // match the path and forwards them to the target. The browser sees the request
      // as being made to the same origin as the Vite server instead of the API server
      // so it doesn't trigger CORS checks.
      proxy: {
        "/dev-api": {
          target: `${env.VITE_API_HOST}`,
          changeOrigin: true,
          headers: {
            Authorization: `Bearer ${env.VITE_API_TOKEN}`,
          },
          rewrite: (path) => path.replace(/^\/dev-api/, ""),
        },
      },
    },
    base: "/",
    resolve: {
      alias: {
        "app-config": path.resolve(__dirname, "config.ts"),
        src: path.resolve(__dirname, "src"),
      },
    },
  }
})
