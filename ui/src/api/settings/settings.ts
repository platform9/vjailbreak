import { get, post, put } from '../axios'
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

export interface InjectEnvVariablesRequest {
  http_proxy?: string
  https_proxy?: string
  no_proxy?: string
}

export interface InjectEnvVariablesResponse {
  message: string
}

export const injectEnvVariables = (data: InjectEnvVariablesRequest) => {
  return post<InjectEnvVariablesResponse>({
    endpoint: '/dev-api/sdk/vpw/v1/inject_env_variables',
    data
  })
}
