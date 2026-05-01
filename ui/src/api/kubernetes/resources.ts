import axios from '../axios'
import { KUBERNETES_API_BASE_PATH, VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'

export interface VjailbreakResourceType {
  name: string
  kind: string
}

interface APIResourceList {
  resources: { name: string; kind: string }[]
}

export const fetchVjailbreakResourceTypes = async (): Promise<VjailbreakResourceType[]> => {
  const response = await axios.get<APIResourceList>({
    endpoint: VJAILBREAK_API_BASE_PATH
  })
  return (response.resources || [])
    .filter((r) => !r.name.includes('/'))
    .map((r) => ({ name: r.name, kind: r.kind }))
}

export interface KubernetesResource {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace?: string
    uid?: string
    resourceVersion?: string
    creationTimestamp?: string
    labels?: Record<string, string>
    annotations?: Record<string, string>
  }
  [key: string]: unknown
}

export interface KubernetesResourceList {
  apiVersion: string
  kind: string
  metadata: Record<string, unknown>
  items: KubernetesResource[]
}

export const fetchCustomResources = async (
  resourceName: string,
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<KubernetesResource[]> => {
  const response = await axios.get<KubernetesResourceList>({
    endpoint: `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${resourceName}`
  })
  return response.items || []
}

export const fetchCustomResource = async (
  resourceName: string,
  name: string,
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<KubernetesResource> => {
  return axios.get<KubernetesResource>({
    endpoint: `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${resourceName}/${name}`
  })
}

export const fetchConfigMaps = async (
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<KubernetesResource[]> => {
  const response = await axios.get<KubernetesResourceList>({
    endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/configmaps`
  })
  return response.items || []
}

export const fetchConfigMap = async (
  name: string,
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<KubernetesResource> => {
  return axios.get<KubernetesResource>({
    endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/configmaps/${name}`
  })
}

export const deleteConfigMap = async (
  name: string,
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<void> => {
  await axios.del<void>({
    endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/configmaps/${name}`
  })
}

export const updateConfigMap = async (
  name: string,
  data: KubernetesResource,
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<KubernetesResource> => {
  return axios.put<KubernetesResource>({
    endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/configmaps/${name}`,
    data
  })
}

export const deleteCustomResource = async (
  resourceName: string,
  name: string,
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<void> => {
  await axios.del<void>({
    endpoint: `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${resourceName}/${name}`
  })
}

export const updateCustomResource = async (
  resourceName: string,
  name: string,
  data: KubernetesResource,
  namespace: string = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<KubernetesResource> => {
  const base = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${resourceName}/${name}`

  // PUT main resource — Kubernetes ignores .status here if the CRD has a status subresource
  const mainResult = await axios.put<KubernetesResource>({ endpoint: base, data })

  // If the payload includes status, also PUT to the /status subresource.
  // Use the resourceVersion from the main result so Kubernetes accepts the update.
  if ('status' in data) {
    try {
      return await axios.put<KubernetesResource>({
        endpoint: `${base}/status`,
        data: { ...mainResult, status: (data as Record<string, unknown>).status }
      })
    } catch {
      // CRD may not have a status subresource — return main result in that case
    }
  }

  return mainResult
}
