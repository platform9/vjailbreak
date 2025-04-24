import { VMwareHostList, VMwareHost } from "./model"
import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"

export const getVMwareHosts = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE,
  vmwareCredName?: string,
  clusterName?: string
): Promise<VMwareHostList> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwarehosts`

  let labelSelector = ""

  if (vmwareCredName) {
    labelSelector += `vmwarecreds.k8s.pf9.io=${vmwareCredName}`
  }

  if (clusterName) {
    if (labelSelector) labelSelector += ","
    labelSelector += `vjailbreak.k8s.pf9.io/vmware-cluster=${clusterName}`
  }

  const config = labelSelector
    ? {
        params: {
          labelSelector,
        },
      }
    : undefined

  return axios.get<VMwareHostList>({
    endpoint,
    config,
  })
}

export const getVMwareHost = async (
  hostName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VMwareHost> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwarehosts/${hostName}`

  return axios.get<VMwareHost>({
    endpoint,
  })
}
