import { get, put, post } from '../axios'
import { VjailbreakSettings } from './model'

export const VERSION_CONFIG_MAP_NAME = 'vjailbreak-settings'
export const VERSION_NAMESPACE = 'migration-system'

export const getSettingsConfigMap = async (
  namespace: string = VERSION_NAMESPACE
): Promise<VjailbreakSettings> => {
  const endpoint = `/api/v1/namespaces/${namespace}/configmaps/${VERSION_CONFIG_MAP_NAME}`
  return get<VjailbreakSettings>({
    endpoint,
    config: { mock: false } // Force real API call, not mock
  })
}

export const getDeploymentName = async (namespace: string = VERSION_NAMESPACE): Promise<string> => {
  try {
    const configMap = await getSettingsConfigMap(namespace)
    return configMap.data.DEPLOYMENT_NAME
  } catch (error) {
    console.error('Failed to fetch header name', error)
    throw error
  }
}

export interface ApplyTimeSettingsResponse {
  success: boolean
  message: string
}

export const applyTimeSettings = (timezone: string) =>
  post<ApplyTimeSettingsResponse>({
    endpoint: '/dev-api/sdk/vpw/v1/time-settings/apply',
    data: { timezone }
  })

const TIME_SETTINGS_DEPLOYMENTS = [
  'migration-controller-manager',
  'migration-vpwned-sdk',
  'vjailbreak-ui',
]

export const checkDeploymentsReady = async (
  namespace: string = VERSION_NAMESPACE
): Promise<boolean> => {
  const results = await Promise.all(
    TIME_SETTINGS_DEPLOYMENTS.map(async (name) => {
      try {
        const dep = await get<any>({
          endpoint: `/apis/apps/v1/namespaces/${namespace}/deployments/${name}`,
          config: { mock: false },
        })
        const desired: number = dep.spec?.replicas ?? 1
        return (
          (dep.status?.updatedReplicas ?? 0) >= desired &&
          (dep.status?.availableReplicas ?? 0) >= desired &&
          !(dep.status?.unavailableReplicas)
        )
      } catch {
        return false
      }
    })
  )
  return results.every(Boolean)
}

export const updateSettingsConfigMap = async (
  data: VjailbreakSettings,
  namespace: string = VERSION_NAMESPACE
) => {
  const endpoint = `/api/v1/namespaces/${namespace}/configmaps/${VERSION_CONFIG_MAP_NAME}`
  const response = await put({
    endpoint,
    data,
    config: { mock: false }
  })
  return response
}
