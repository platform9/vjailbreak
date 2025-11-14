import axios from 'axios'
import { ArrayCreds, ArrayCredsFormData } from './model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'

const getHeaders = () => {
  const authToken = import.meta.env.VITE_API_TOKEN
  return {
    'Content-Type': 'application/json;charset=UTF-8',
    ...(authToken && { Authorization: `Bearer ${authToken}` }),
  }
}

const axiosInstance = axios.create({
  headers: getHeaders(),
})

const NAMESPACE = VJAILBREAK_DEFAULT_NAMESPACE
const ARRAY_CREDS_API_PATH = `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/${NAMESPACE}/arraycreds`

export const getArrayCreds = async (): Promise<ArrayCreds[]> => {
  const response = await axiosInstance.get(ARRAY_CREDS_API_PATH)
  return response.data.items || []
}

export const getArrayCredsById = async (name: string): Promise<ArrayCreds> => {
  const response = await axiosInstance.get(`${ARRAY_CREDS_API_PATH}/${name}`)
  return response.data
}

export const createArrayCreds = async (data: ArrayCredsFormData): Promise<ArrayCreds> => {
  const arrayCreds: ArrayCreds = {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'ArrayCreds',
    metadata: {
      name: data.name,
      namespace: NAMESPACE,
      labels: {
        'vjailbreak.k8s.pf9.io/manually-created': 'true',
      },
    },
    spec: {
      vendorType: data.vendorType,
      autoDiscovered: false,
      openStackMapping: {
        volumeType: data.volumeType,
        cinderBackendName: data.cinderBackendName,
        cinderBackendPool: data.cinderBackendPool || '',
      },
      secretRef: {
        name: `${data.name}-secret`,
        namespace: NAMESPACE,
      },
    },
  }

  // Create the ArrayCreds resource
  const response = await axiosInstance.post(ARRAY_CREDS_API_PATH, arrayCreds)
  
  // Create the secret if credentials are provided
  if (data.managementEndpoint || data.username || data.password || data.apiToken) {
    await createArrayCredsSecret(data.name, {
      managementEndpoint: data.managementEndpoint || '',
      username: data.username || '',
      password: data.password || '',
      apiToken: data.apiToken || '',
    })
  }

  return response.data
}

export const updateArrayCreds = async (
  name: string,
  data: Partial<ArrayCredsFormData>
): Promise<ArrayCreds> => {
  // Get existing resource
  const existing = await getArrayCredsById(name)

  // Update spec fields
  if (data.vendorType) existing.spec.vendorType = data.vendorType
  if (data.volumeType) existing.spec.openStackMapping.volumeType = data.volumeType
  if (data.cinderBackendName) existing.spec.openStackMapping.cinderBackendName = data.cinderBackendName
  if (data.cinderBackendPool !== undefined) {
    existing.spec.openStackMapping.cinderBackendPool = data.cinderBackendPool
  }

  // Update the ArrayCreds resource
  const response = await axiosInstance.put(`${ARRAY_CREDS_API_PATH}/${name}`, existing)

  // Update secret if credentials are provided
  if (data.managementEndpoint || data.username || data.password || data.apiToken) {
    await updateArrayCredsSecret(name, {
      managementEndpoint: data.managementEndpoint || '',
      username: data.username || '',
      password: data.password || '',
      apiToken: data.apiToken || '',
    })
  }

  return response.data
}

export const deleteArrayCreds = async (name: string): Promise<void> => {
  await axiosInstance.delete(`${ARRAY_CREDS_API_PATH}/${name}`)
}

// Secret management
interface SecretData {
  managementEndpoint: string
  username: string
  password: string
  apiToken: string
}

const createArrayCredsSecret = async (arrayCredsName: string, data: SecretData): Promise<void> => {
  const secret = {
    apiVersion: 'v1',
    kind: 'Secret',
    metadata: {
      name: `${arrayCredsName}-secret`,
      namespace: NAMESPACE,
    },
    type: 'Opaque',
    stringData: {
      managementEndpoint: data.managementEndpoint,
      username: data.username,
      password: data.password,
      apiToken: data.apiToken,
    },
  }

  await axiosInstance.post(`/api/v1/namespaces/${NAMESPACE}/secrets`, secret)
}

const updateArrayCredsSecret = async (arrayCredsName: string, data: SecretData): Promise<void> => {
  const secretName = `${arrayCredsName}-secret`
  
  try {
    // Try to get existing secret
    await axiosInstance.get(`/api/v1/namespaces/${NAMESPACE}/secrets/${secretName}`)
    
    // Update existing secret
    const secret = {
      apiVersion: 'v1',
      kind: 'Secret',
      metadata: {
        name: secretName,
        namespace: NAMESPACE,
      },
      type: 'Opaque',
      stringData: {
        managementEndpoint: data.managementEndpoint,
        username: data.username,
        password: data.password,
        apiToken: data.apiToken,
      },
    }
    
    await axiosInstance.put(`/api/v1/namespaces/${NAMESPACE}/secrets/${secretName}`, secret)
  } catch (error) {
    // Secret doesn't exist, create it
    await createArrayCredsSecret(arrayCredsName, data)
  }
}

export const getArrayCredsSecret = async (secretName: string): Promise<SecretData | null> => {
  try {
    const response = await axiosInstance.get(`/api/v1/namespaces/${NAMESPACE}/secrets/${secretName}`)
    const data = response.data.data || {}
    
    return {
      managementEndpoint: data.managementEndpoint ? atob(data.managementEndpoint) : '',
      username: data.username ? atob(data.username) : '',
      password: data.password ? atob(data.password) : '',
      apiToken: data.apiToken ? atob(data.apiToken) : '',
    }
  } catch (error) {
    return null
  }
}
