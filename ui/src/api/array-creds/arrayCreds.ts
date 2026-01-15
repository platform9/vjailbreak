import axios from '../axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { GetArrayCredsList, ArrayCreds } from './model'

export const getArrayCredentialsList = async (namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds`
  const response = await axios.get<GetArrayCredsList>({
    endpoint
  })
  return response?.items
}

export const getArrayCredentials = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds/${name}`
  const response = await axios.get<ArrayCreds>({
    endpoint
  })
  return response
}

export const postArrayCredentials = async (data: any, namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds`
  const response = await axios.post<ArrayCreds>({
    endpoint,
    data
  })
  return response
}

export const deleteArrayCredentials = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds/${name}`
  const response = await axios.del<ArrayCreds>({
    endpoint
  })
  return response
}

export const createArrayCredsWithSecret = async (
  name: string,
  secretName: string,
  vendorType: string,
  openstackMapping?: {
    volumeType?: string
    cinderBackendName?: string
    cinderBackendPool?: string
    cinderHost?: string
  },
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds`

  const credBody: any = {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'ArrayCreds',
    metadata: {
      name,
      namespace
    },
    spec: {
      vendorType,
      secretRef: {
        name: secretName
      }
    }
  }

  if (openstackMapping) {
    credBody.spec.openstackMapping = openstackMapping
  }

  const response = await axios.post<ArrayCreds>({
    endpoint,
    data: credBody
  })

  return response
}
