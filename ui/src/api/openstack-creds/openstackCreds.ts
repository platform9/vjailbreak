import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { GetOpenstackCredsList, OpenstackCreds } from "./model"

export const getOpenstackCredentialsList = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/openstackcreds`
  const response = await axios.get<GetOpenstackCredsList>({
    endpoint,
  })
  return response
}

export const getOpenstackCredentials = async (
  name,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/openstackcreds/${name}`
  const response = await axios.get<OpenstackCreds>({
    endpoint,
  })
  return response
}

export const postOpenstackCredentials = async (
  data,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  console.log(data)
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/openstackcreds`
  console.log(endpoint)
  const response = await axios.post<OpenstackCreds>({
    endpoint,
    data,
  })
  return response
}
