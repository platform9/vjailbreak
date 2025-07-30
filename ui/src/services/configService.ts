export interface AnalyticsConfig {
  amplitude?: {
    apiKey: string
  }
  bugsnag?: {
    apiKey: string
    appVersion?: string
  }
  releaseStage?: string
}

export class ConfigService {
  private static readonly CONFIGMAP_NAME = "analytics-config"
  private static readonly CONFIGMAP_NAMESPACE = "default"

  static async fetchAnalyticsConfig(): Promise<AnalyticsConfig | null> {
    try {
      const response = await fetch(
        `/api/v1/namespaces/${this.CONFIGMAP_NAMESPACE}/configmaps/${this.CONFIGMAP_NAME}`,
        {
          headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
          },
        }
      )

      if (!response.ok) {
        throw new Error(
          `ConfigMap fetch failed: ${response.status} ${response.statusText}`
        )
      }

      const configMap = await response.json()
      const data = configMap.data || {}

      const config: AnalyticsConfig = {}

      // Extract shared release stage
      if (data["release-stage"]) {
        config.releaseStage = data["release-stage"]
      }

      // Extract Amplitude config
      if (data["amplitude-api-key"]) {
        config.amplitude = {
          apiKey: data["amplitude-api-key"],
        }
      }

      // Extract Bugsnag config
      if (data["bugsnag-api-key"]) {
        config.bugsnag = {
          apiKey: data["bugsnag-api-key"],
          appVersion: data["app-version"] || undefined,
        }
      }

      return config
    } catch (error) {
      console.error(
        "Failed to fetch analytics configuration from ConfigMap:",
        error
      )
      return null
    }
  }
}
