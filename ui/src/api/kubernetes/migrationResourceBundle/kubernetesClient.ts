import axios from '../../axios'
import { KUBERNETES_API_BASE_PATH, VJAILBREAK_API_BASE_PATH } from '../../constants'
import type { KubernetesList, KubernetesObject } from './types'

export const listVjailbreakCrs = async (
  plural: string,
  namespace: string,
  warnings: string[]
): Promise<KubernetesObject[]> => {
  try {
    const response = await axios.get<KubernetesList>({
      endpoint: `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${plural}`,
      config: { mock: false }
    })
    return Array.isArray(response?.items) ? response.items : []
  } catch (error) {
    warnings.push(
      `Failed to list ${plural}: ${error instanceof Error ? error.message : String(error)}`
    )
    return []
  }
}

export const getCoreObject = async (
  plural: string,
  name: string,
  namespace: string,
  warnings: string[]
): Promise<KubernetesObject | null> => {
  try {
    return await axios.get<KubernetesObject>({
      endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/${plural}/${name}`,
      config: { mock: false }
    })
  } catch (error) {
    warnings.push(
      `Failed to get ${plural}/${name}: ${error instanceof Error ? error.message : String(error)}`
    )
    return null
  }
}

export const listCoreObjects = async (
  plural: string,
  namespace: string,
  warnings: string[]
): Promise<KubernetesObject[]> => {
  try {
    const response = await axios.get<KubernetesList>({
      endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/${plural}`,
      config: { mock: false }
    })
    return Array.isArray(response?.items) ? response.items : []
  } catch (error) {
    warnings.push(
      `Failed to list ${plural}: ${error instanceof Error ? error.message : String(error)}`
    )
    return []
  }
}
