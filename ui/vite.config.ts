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
      // For dev mode: proxy api requests with the /dev-api prefix to the target server.
      proxy: {
        "/dev-api": {
          // Without a fallback an unset VITE_API_HOST becomes the literal string
          // "undefined", and the proxy then tries to resolve a host by that name. On a
          // developer machine that fails instantly; on CI it goes through the search
          // domain and stalls for seconds on every request the specs do not stub, which
          // made the E2E suite ~12x slower there. A dead local port refuses immediately.
          target: env.VITE_API_HOST || "http://127.0.0.1:9",
          changeOrigin: true,
          secure: false, // Allow self-signed certificates for HTTPS
          ws: true, // Enable WebSocket support for Kubernetes exec API
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
