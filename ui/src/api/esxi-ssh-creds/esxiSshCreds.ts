import { ESXiSSHCreds, ESXiSSHCredsList } from './model'
import axios from '../axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'

export const getESXiSSHCreds = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ESXiSSHCredsList> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/esxisshcreds`
  return axios.get<ESXiSSHCredsList>({ endpoint })
}

export const getESXiSSHCredsItem = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ESXiSSHCreds> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/esxisshcreds/${name}`
  return axios.get<ESXiSSHCreds>({ endpoint })
}

export const createESXiSSHCreds = async (
  name: string,
  secretName: string,
  username = 'root',
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ESXiSSHCreds> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/esxisshcreds`

  const payload: ESXiSSHCreds = {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'ESXiSSHCreds',
    metadata: {
      name,
      namespace
    },
    spec: {
      secretRef: {
        name: secretName,
        namespace
      },
      username
    }
  }

  return axios.post<ESXiSSHCreds>({
    endpoint,
    data: payload
  })
}

export const deleteESXiSSHCreds = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<void> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/esxisshcreds/${name}`
  return axios.del({ endpoint })
}

export const upsertESXiSSHCreds = async (
  name: string,
  secretName: string,
  username = 'root',
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ESXiSSHCreds> => {
  try {
    return await createESXiSSHCreds(name, secretName, username, namespace)
  } catch (error: any) {
    if (error?.response?.status === 409) {
      // Already exists, just return the existing one
      return await getESXiSSHCredsItem(name, namespace)
    }
    throw error
  }
}
