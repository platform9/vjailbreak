import react from "@vitejs/plugin-react"
import path from "path"
import { defineConfig } from "vite"
import runtimeEnv from 'vite-plugin-runtime-env';

// https://vitejs.dev/config/
export default defineConfig({
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
  },
  base: '/',
  resolve: {
    alias: {
      "app-config": path.resolve(__dirname, "config.ts"),
      src: path.resolve(__dirname, "src"), // Alias src directly
    },
  },
})
