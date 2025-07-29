import { EventProperties } from "../types/amplitude"

// Pure Amplitude configuration
export interface AmplitudeConfig {
  apiKey: string
  serverUrl?: string
  disabled?: boolean
  trackingOptions?: {
    ipAddress?: boolean
    deviceId?: boolean
    platform?: boolean
  }
}

// Tracking behavior configuration
export interface TrackingBehavior {
  enabled: boolean
  defaultComponent?: string
  logToConsole?: boolean
}

// Create Amplitude config from environment
export const createAmplitudeConfig = (): AmplitudeConfig => {
  const isDevelopment = import.meta.env.MODE === "development"
  const hasApiKey = Boolean(import.meta.env.VITE_AMPLITUDE_API_KEY)

  return {
    apiKey: import.meta.env.VITE_AMPLITUDE_API_KEY || "dev-api-key",
    serverUrl: import.meta.env.VITE_AMPLITUDE_SERVER_URL,
    disabled: isDevelopment && !hasApiKey,
    trackingOptions: {
      ipAddress: true,
      deviceId: true,
      platform: true,
    },
  }
}

// Global tracking behavior (simplified)
let trackingBehavior: TrackingBehavior = {
  enabled: true,
  logToConsole: import.meta.env.MODE === "development",
}

// Functional configuration management
export const getTrackingBehavior = (): TrackingBehavior => trackingBehavior

export const updateTrackingBehavior = (
  updates: Partial<TrackingBehavior>
): void => {
  trackingBehavior = { ...trackingBehavior, ...updates }
}

// Simplified control functions
export const enableTracking = () => updateTrackingBehavior({ enabled: true })
export const disableTracking = () => updateTrackingBehavior({ enabled: false })

// Helper to enrich properties with defaults
export const enrichEventProperties = (
  properties: EventProperties = {},
  defaults: Partial<EventProperties> = {}
): EventProperties => {
  const merged = { ...defaults, ...properties }
  // Filter out undefined values
  return Object.fromEntries(
    Object.entries(merged).filter(([, value]) => value !== undefined)
  )
}
