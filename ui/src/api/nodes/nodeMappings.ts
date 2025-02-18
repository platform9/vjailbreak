import axios from "../axios"
import axiosInstance from "axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import {
  NodeList,
  NodeItem as Node,
  Spec,
  NodeItem,
  OpenstackFlavorsResponse,
  OpenstackProjectsResponse,
  OpenstackCredsRef,
} from "./model"
import { createOpenstackTokenRequestBody } from "../openstack-creds/helpers"
import { OpenstackImagesResponse } from "../openstack-creds/model"
import { nanoid } from "nanoid"

// Private helper function for token generation
const generateOpenstackToken = async (creds) => {
  try {
    const response = await axiosInstance({
      method: "post",
      url: creds?.OS_AUTH_URL + "/auth/tokens",
      data: createOpenstackTokenRequestBody(creds),
      headers: {
        "Content-Type": "application/json",
      },
    })
    return {
      token: response.headers["x-subject-token"],
      response: response.data,
    }
  } catch (error) {
    console.error("Failed to generate OpenStack token:", error)
    throw error
  }
}

export const getNodes = async (namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vjailbreaknodes`
  const response = await axios.get<NodeList>({
    endpoint,
  })
  return response?.items
}

export const deleteNode = async (
  nodeName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vjailbreaknodes/${nodeName}`
  const response = await axios.del<Node>({
    endpoint,
  })
  return response
}

export const getOpenstackImages = async (creds) => {
  try {
    const { token } = await generateOpenstackToken(creds)

    const baseUrl = creds.OS_AUTH_URL.replace("/keystone/v3", "")

    const response = await axiosInstance({
      method: "get",
      url: `${baseUrl}/glance/v2/images`,
      headers: {
        "Content-Type": "application/json",
        "X-Auth-Token": token,
      },
    })

    return response.data as OpenstackImagesResponse
  } catch (error) {
    console.error("Failed to fetch OpenStack images:", error)
    throw error
  }
}

const getProjectId = async (creds) => {
  try {
    const { token } = await generateOpenstackToken(creds)
    const response = await axiosInstance({
      method: "get",
      url: `${creds.OS_AUTH_URL}/auth/projects`,
      headers: {
        "Content-Type": "application/json",
        "X-Auth-Token": token,
      },
    })

    const data = response.data as OpenstackProjectsResponse
    const projectId = data.projects?.[0]?.id

    if (!projectId) {
      throw new Error("No project found")
    }

    return { projectId }
  } catch (error) {
    console.error("Failed to get project ID:", error)
    throw error
  }
}

export const getOpenstackFlavors = async (creds) => {
  try {
    const { token } = await generateOpenstackToken(creds)
    const { projectId } = await getProjectId(creds)
    const baseUrl = creds.OS_AUTH_URL.replace("/keystone/v3", "")

    const response = await axiosInstance({
      method: "get",
      url: `${baseUrl}/nova/v2.1/${projectId}/flavors/detail?is_public=None`,
      headers: {
        "Content-Type": "application/json",
        "X-Auth-Token": token,
      },
    })

    return response.data as OpenstackFlavorsResponse
  } catch (error) {
    console.error("Failed to fetch OpenStack flavors:", error)
    throw error
  }
}

// Helper to create node spec
const createNodeSpec = (params: {
  imageId: string
  openstackCreds: OpenstackCredsRef
  flavorId: string
  role?: string
}): Spec => ({
  imageid: params.imageId,
  noderole: params.role || "worker",
  openstackcreds: params.openstackCreds,
  openstackflavorid: params.flavorId,
})

// Create VjailbreakNode object
const createNodeObject = (params: {
  name?: string
  namespace?: string
  spec: Spec
}): NodeItem => ({
  apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
  kind: "VjailbreakNode",
  metadata: {
    name: params.name || `vjailbreak-agent-${nanoid(5).toLowerCase()}`,
    namespace: params.namespace || "migration-system",
  },
  spec: params.spec,
})

export const createNodes = async (params: {
  imageId: string
  openstackCreds: OpenstackCredsRef
  flavorId: string
  count: number
  namespace?: string
}) => {
  const namespace = params.namespace || VJAILBREAK_DEFAULT_NAMESPACE
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vjailbreaknodes`

  const results: NodeItem[] = []
  const errors: Error[] = []

  for (let i = 0; i < params.count; i++) {
    try {
      const spec = createNodeSpec({
        imageId: params.imageId,
        openstackCreds: params.openstackCreds,
        flavorId: params.flavorId,
      })
      const node = createNodeObject({ spec, namespace })

      const result = await axios.post<NodeItem>({
        endpoint,
        data: node,
      })

      results.push(result)
    } catch (error) {
      errors.push(error as Error)
      console.error(`Failed to create node ${i + 1}:`, error)
    }
  }

  if (errors.length > 0) {
    throw new Error(
      `Failed to create ${errors.length} out of ${params.count} nodes`
    )
  }

  return results
}
