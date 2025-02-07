import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { createOpenstackTokenRequestBody } from "./helpers"
import { GetOpenstackCredsList, OpenstackCreds } from "./model"

export const getOpenstackCredentialsList = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/openstackcreds`
  const response = await axios.get<GetOpenstackCredsList>({
    endpoint,
  })
  return response?.items
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
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/openstackcreds`
  const response = await axios.post<OpenstackCreds>({
    endpoint,
    data,
  })
  return response
}

export const deleteOpenstackCredentials = async (
  name,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/openstackcreds/${name}`
  const response = await axios.del<OpenstackCreds>({
    endpoint,
  })
  return response
}

export const generateOpenstackToken = async (creds: OpenstackCreds) => {
  const tokenEndpoint = creds?.spec?.OS_AUTH_URL + "/auth/tokens"
  const requestBody = createOpenstackTokenRequestBody(creds)

  const response = await axios.post({
    endpoint: tokenEndpoint,
    data: requestBody,
  })

  // Token is in the 'X-Subject-Token' header
  return response
}
