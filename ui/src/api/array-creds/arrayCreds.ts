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
  // Check if credentials are provided
  const hasCredentials = data.managementEndpoint || data.username || data.password
  
  // Build the spec object
  const spec: any = {
    vendorType: data.vendorType,
    openstackMapping: {
      volumeType: data.volumeType,
      cinderBackendName: data.cinderBackendName,
      cinderBackendPool: data.cinderBackendPool || '',
    },
  }
  
  // Only add secretRef if credentials are provided
  if (hasCredentials) {
    spec.secretRef = {
      name: `${data.name}-secret`,
      namespace: NAMESPACE,
    }
  }
  
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
    spec,
  }

  // Create the secret first if credentials are provided
  if (hasCredentials) {
    await createArrayCredsSecret(data.name, {
      managementEndpoint: data.managementEndpoint || '',
      username: data.username || '',
      password: data.password || '',
      skipSSLVerification: data.skipSSLVerification || false,
    })
  }

  // Create the ArrayCreds resource after secret is created
  const response = await axiosInstance.post(ARRAY_CREDS_API_PATH, arrayCreds)

  return response.data
}

export const updateArrayCreds = async (
  name: string,
  data: Partial<ArrayCredsFormData>
): Promise<ArrayCreds> => {
  // Get existing resource
  const existing = await getArrayCredsById(name)

  // Ensure openstackMapping exists
  if (!existing.spec.openstackMapping) {
    existing.spec.openstackMapping = {
      volumeType: '',
      cinderBackendName: '',
    }
  }

  // Update spec fields
  if (data.vendorType) existing.spec.vendorType = data.vendorType
  if (data.volumeType) existing.spec.openstackMapping.volumeType = data.volumeType
  if (data.cinderBackendName) existing.spec.openstackMapping.cinderBackendName = data.cinderBackendName
  if (data.cinderBackendPool !== undefined) {
    existing.spec.openstackMapping.cinderBackendPool = data.cinderBackendPool
  }

  // Update secret if credentials are provided
  const shouldUpdateSecret = data.managementEndpoint || data.username || data.password || data.skipSSLVerification !== undefined
  if (shouldUpdateSecret) {
    // Ensure secretRef is set
    if (!existing.spec.secretRef) {
      existing.spec.secretRef = {}
    }
    if (!existing.spec.secretRef.name) {
      existing.spec.secretRef.name = `${name}-secret`
      existing.spec.secretRef.namespace = NAMESPACE
    }

    await updateArrayCredsSecret(name, {
      managementEndpoint: data.managementEndpoint || '',
      username: data.username || '',
      password: data.password || '',
      skipSSLVerification: data.skipSSLVerification || false,
    })
    
    // Add annotation to trigger reconciliation when secret is updated
    if (!existing.metadata.annotations) {
      existing.metadata.annotations = {}
    }
    existing.metadata.annotations['vjailbreak.k8s.pf9.io/credentials-updated'] = new Date().toISOString()
  }

  // Update the ArrayCreds resource
  const response = await axiosInstance.put(`${ARRAY_CREDS_API_PATH}/${name}`, existing)

  return response.data
}

export const deleteArrayCreds = async (name: string): Promise<void> => {
  await axiosInstance.delete(`${ARRAY_CREDS_API_PATH}/${name}`)
}

export const deleteArrayCredsSecret = async (secretName: string): Promise<void> => {
  try {
    await axiosInstance.delete(`/api/v1/namespaces/${NAMESPACE}/secrets/${secretName}`)
    console.log(`Successfully deleted secret: ${secretName}`)
  } catch (error: any) {
    // Only log if it's not a 404 (secret doesn't exist)
    if (error?.response?.status !== 404) {
      console.error('Error deleting secret:', error)
    }
    // Don't throw - secret might not exist
  }
}

// Secret management
interface SecretData {
  managementEndpoint: string
  username: string
  password: string
  skipSSLVerification: boolean
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
      ARRAY_HOSTNAME: data.managementEndpoint,
      ARRAY_USERNAME: data.username,
      ARRAY_PASSWORD: data.password,
      ARRAY_INSECURE: data.skipSSLVerification.toString(),
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
        ARRAY_HOSTNAME: data.managementEndpoint,
        ARRAY_USERNAME: data.username,
        ARRAY_PASSWORD: data.password,
        ARRAY_INSECURE: data.skipSSLVerification.toString(),
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
      managementEndpoint: data.ARRAY_HOSTNAME ? atob(data.ARRAY_HOSTNAME) : '',
      username: data.ARRAY_USERNAME ? atob(data.ARRAY_USERNAME) : '',
      password: data.ARRAY_PASSWORD ? atob(data.ARRAY_PASSWORD) : '',
      skipSSLVerification: data.ARRAY_INSECURE ? atob(data.ARRAY_INSECURE) === 'true' : false,
    }
  } catch (error) {
    return null
  }
}
