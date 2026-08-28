import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { open: 'never' }]],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    viewport: { width: 1280, height: 720 },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Every spec stubs its backend calls with page.route(), so the dev server only
  // has to serve the app — no VITE_API_HOST / VITE_API_TOKEN needed in CI.
  // Set PLAYWRIGHT_BASE_URL to point the suite at an already-running appliance
  // instead, in which case Playwright starts nothing.
  webServer: process.env.PLAYWRIGHT_BASE_URL
    ? undefined
    : {
        command: 'yarn dev --port 3000 --strictPort',
        url: 'http://localhost:3000/dashboard/migrations',
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        stdout: 'pipe',
        stderr: 'pipe',
        // The specs stub their own API calls; anything they miss must be refused
        // instantly rather than left to DNS. See the proxy fallback in vite.config.ts.
        env: {
          VITE_API_HOST: process.env.VITE_API_HOST ?? 'http://127.0.0.1:9',
          VITE_API_TOKEN: process.env.VITE_API_TOKEN ?? 'e2e-placeholder-token',
        },
      },
})
