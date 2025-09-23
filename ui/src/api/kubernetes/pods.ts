import { KUBERNETES_API_BASE_PATH } from "../constants"

/**
 * Centralized Kubernetes API functions for pod operations
 */

export interface Pod {
  metadata: {
    name: string
    namespace: string
  }
}

export interface PodListResponse {
  items: Pod[]
}

/**
 * Get auth headers for Kubernetes API requests
 */
const getKubernetesHeaders = () => {
  const authToken = import.meta.env.VITE_API_TOKEN
  return {
    "Content-Type": "application/json;charset=UTF-8",
    ...(authToken && { Authorization: `Bearer ${authToken}` }),
  }
}

/**
 * Fetch pods from a namespace with optional label selector
 */
export const fetchPods = async (
  namespace: string,
  labelSelector?: string
): Promise<Pod[]> => {
  const baseUrl = import.meta.env.MODE === "development" ? "/dev-api" : ""
  const endpoint = `${baseUrl}${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/pods`
  
  const params = new URLSearchParams()
  if (labelSelector) {
    params.set("labelSelector", labelSelector)
  }
  
  const url = `${endpoint}?${params.toString()}`

  const response = await fetch(url, {
    headers: getKubernetesHeaders(),
    signal: new AbortController().signal,
  })
      
  if (!response.ok) {
    throw new Error(`Failed to fetch pods: HTTP ${response.status}: ${response.statusText}`)
  }

  const data: PodListResponse = await response.json()
  return data.items
}

/**
 * Stream logs from a specific pod
 */
export const streamPodLogs = async (
  namespace: string,
  podName: string,
  options: {
    follow?: boolean
    tailLines?: string
    signal?: AbortSignal
  } = {}
): Promise<Response> => {
  const { follow = true, tailLines = "100", signal } = options
  
  const baseUrl = import.meta.env.MODE === "development" ? "/dev-api" : ""
  const endpoint = `${baseUrl}${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/pods/${podName}/log`
  
  const params = new URLSearchParams({
    follow: follow.toString(),
    tailLines,
  })
  
  const url = `${endpoint}?${params.toString()}`

  const response = await fetch(url, {
    headers: getKubernetesHeaders(),
    signal,
  })

  if (!response.ok) {
    throw new Error(`Failed to stream logs from pod ${podName}: HTTP ${response.status}: ${response.statusText}`)
  }

  return response
}
