import axios from '../axios'
import { K8S_PROXY_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { encodeUtf8ToBase64, decodeBase64ToUtf8 } from '../../utils/base64encoding'
import { SSHKeyPair } from './model'
import { SecretList } from '../secrets/model'

const SSH_KEYPAIR_LABEL = 'vjailbreak.k8s.pf9.io/resource-type=ssh-keypair'
const SSH_KEYPAIR_TYPE_LABEL = 'vjailbreak.k8s.pf9.io/keypair-type'

const GENERATE_ENDPOINT = '/dev-api/sdk/vpw/v1/generate-ssh-keypair'

const secretToSSHKeyPair = (secret: any): SSHKeyPair => ({
  name: secret.metadata.name,
  type: (secret.metadata?.labels?.[SSH_KEYPAIR_TYPE_LABEL] ?? 'manual') as SSHKeyPair['type'],
  publicKey: secret.data?.['ssh-publickey']
    ? decodeBase64ToUtf8(secret.data['ssh-publickey'])
    : '',
  createdAt: secret.metadata?.creationTimestamp ?? ''
})

export const listSSHKeyPairs = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<SSHKeyPair[]> => {
  const endpoint = `${K8S_PROXY_BASE_PATH}/namespaces/${namespace}/secrets?labelSelector=${encodeURIComponent(SSH_KEYPAIR_LABEL)}`
  const response = await axios.get<SecretList>({ endpoint })
  const items = Array.isArray(response?.items) ? response.items : []
  return items.map(secretToSSHKeyPair)
}

export const generateSSHKeyPair = async (name: string): Promise<SSHKeyPair> => {
  const response = await axios.post<{ publicKey: string }>({
    endpoint: GENERATE_ENDPOINT,
    data: { name }
  })
  return {
    name,
    type: 'generated',
    publicKey: response.publicKey,
    createdAt: new Date().toISOString()
  }
}

export const createManualSSHKeyPair = async (
  name: string,
  privateKey: string,
  publicKey: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<SSHKeyPair> => {
  const body = {
    apiVersion: 'v1',
    kind: 'Secret',
    metadata: {
      name,
      namespace,
      labels: {
        'vjailbreak.k8s.pf9.io/resource-type': 'ssh-keypair',
        'vjailbreak.k8s.pf9.io/keypair-type': 'manual'
      }
    },
    type: 'Opaque',
    data: {
      'ssh-privatekey': encodeUtf8ToBase64(privateKey.trim()),
      'ssh-publickey': encodeUtf8ToBase64(publicKey.trim())
    }
  }
  const endpoint = `${K8S_PROXY_BASE_PATH}/namespaces/${namespace}/secrets`
  await axios.post({ endpoint, data: body })
  return {
    name,
    type: 'manual',
    publicKey: publicKey.trim(),
    createdAt: new Date().toISOString()
  }
}

export const deleteSSHKeyPair = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<void> => {
  const endpoint = `${K8S_PROXY_BASE_PATH}/namespaces/${namespace}/secrets/${name}`
  await axios.del({ endpoint })
}
