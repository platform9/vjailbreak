import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { GetVMWareCredsList, VMwareCreds } from "./model"

export const getVmwareCredentialsList = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwarecreds`
  const response = await axios.get<GetVMWareCredsList>({
    endpoint,
  })
  return response
}

export const getVmwareCredentials = async (
  vmwareCredsName,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwarecreds/${vmwareCredsName}`
  const response = await axios.get<VMwareCreds>({
    endpoint,
  })
  return response
}

export const postVmwareCredentials = async (
  body,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwarecreds`
  const response = await axios.post<VMwareCreds>({
    endpoint,
    data: body,
  })
  return response
}

export const deleteVmwareCredentials = async (
  vmwareCredsName,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwarecreds/${vmwareCredsName}`
  const response = await axios.del<VMwareCreds>({
    endpoint,
  })
  return response
}
