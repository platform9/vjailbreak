import "@fontsource/roboto/300.css"
import "@fontsource/roboto/400.css"
import "@fontsource/roboto/500.css"
import "@fontsource/roboto/700.css"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { ReactQueryDevtools } from "@tanstack/react-query-devtools"
import React, { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { BrowserRouter } from "react-router-dom"
import Bugsnag from '@bugsnag/js'
import BugsnagPluginReact from '@bugsnag/plugin-react'
import BugsnagPerformance from '@bugsnag/browser-performance'
import App from "./App.tsx"
import { ThemeProvider } from "./theme/ThemeContext.tsx"
import { getBugsnagConfig, getBugsnagPerformanceConfig } from "./config/bugsnag"
import { createAmplitudeConfig } from "./config/amplitude"
import { errorReportingService } from "./services/errorReporting"
import { initializeAmplitude } from "./services/amplitudeService"
import { ConfigService } from "./services/configService"

const queryClient = new QueryClient()

async function initializeApp() {
  // Fetch analytics configuration from ConfigMap
  const configMapData = await ConfigService.fetchAnalyticsConfig()

  if (!configMapData) {
    console.warn('Failed to load analytics configuration from ConfigMap')
  }

  // Initialize configs with ConfigMap data or fallback to environment variables
  const bugsnagConfig = getBugsnagConfig(configMapData || undefined)
  const bugsnagPerformanceConfig = getBugsnagPerformanceConfig(configMapData || undefined)
  const amplitudeConfig = createAmplitudeConfig(configMapData || undefined)

  if (bugsnagConfig.apiKey) {
    try {
      Bugsnag.start({
        ...bugsnagConfig,
        plugins: [new BugsnagPluginReact()],
      })

      BugsnagPerformance.start(bugsnagPerformanceConfig)

      errorReportingService.initialize(Bugsnag)

      errorReportingService.addMetadata('app', 'name', 'vjailbreak')
      errorReportingService.addMetadata('app', 'component', 'ui')
    } catch (error) {
      console.error('Failed to initialize Bugsnag:', error)
    }
  } else {
    console.warn('Bugsnag not initialized: API key not found in ConfigMap or environment variables')
  }

  // Initialize Amplitude
  try {
    initializeAmplitude(amplitudeConfig)
  } catch (error) {
    console.error('Failed to initialize Amplitude:', error)
  }

  const ErrorBoundary = Bugsnag.getPlugin('react')?.createErrorBoundary(React) || React.Fragment

  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <ErrorBoundary>
        <BrowserRouter>
          <ThemeProvider>
            <QueryClientProvider client={queryClient}>
              <App />
              <ReactQueryDevtools initialIsOpen={false} />
            </QueryClientProvider>
          </ThemeProvider>
        </BrowserRouter>
      </ErrorBoundary>
    </StrictMode>
  )
}

// Initialize the app
initializeApp().catch(error => {
  console.error('Failed to initialize application:', error)

  // Fallback initialization without analytics if ConfigMap fetch fails completely
  const ErrorBoundary = React.Fragment

  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <ErrorBoundary>
        <BrowserRouter>
          <ThemeProvider>
            <QueryClientProvider client={queryClient}>
              <App />
              <ReactQueryDevtools initialIsOpen={false} />
            </QueryClientProvider>
          </ThemeProvider>
        </BrowserRouter>
      </ErrorBoundary>
    </StrictMode>
  )
})
