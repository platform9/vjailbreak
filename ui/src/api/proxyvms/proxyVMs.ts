import axios from '../axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { ProxyVM, ProxyVMList } from './model'

const PROXY_VMS_RESOURCE = 'proxyvms'

export const getProxyVMList = async (namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${PROXY_VMS_RESOURCE}`
  const response = await axios.get<ProxyVMList>({ endpoint })
  return response?.items ?? []
}

export const getProxyVM = async (name: string, namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${PROXY_VMS_RESOURCE}/${name}`
  const response = await axios.get<ProxyVM>({ endpoint })
  return response
}

export const postProxyVM = async (body: ProxyVM, namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${PROXY_VMS_RESOURCE}`
  const response = await axios.post<ProxyVM>({ endpoint, data: body })
  return response
}

export const deleteProxyVM = async (name: string, namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${PROXY_VMS_RESOURCE}/${name}`
  const response = await axios.del<ProxyVM>({ endpoint })
  return response
}
