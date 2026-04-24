import axios from '../axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { GetArrayCredsList, ArrayCreds } from './model'

export const getArrayCredentialsList = async (namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds`
  const response = await axios.get<GetArrayCredsList>({
    endpoint
  })
  return response?.items || []
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

export const patchArrayCredentials = async (
  name: string,
  data: Partial<ArrayCreds>,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds/${name}`
  const response = await axios.patch<ArrayCreds>({
    endpoint,
    data,
    config: {
      headers: {
        'Content-Type': 'application/merge-patch+json'
      }
    }
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
  netAppConfig?: {
    svm?: string
    flexVol?: string
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

  if (openstackMapping && (openstackMapping.volumeType || openstackMapping.cinderBackendName)) {
    credBody.spec.openstackMapping = {}
    if (openstackMapping.volumeType) {
      credBody.spec.openstackMapping.volumeType = openstackMapping.volumeType
    }
    if (openstackMapping.cinderBackendName) {
      credBody.spec.openstackMapping.cinderBackendName = openstackMapping.cinderBackendName
    }
    if (openstackMapping.cinderBackendPool) {
      credBody.spec.openstackMapping.cinderBackendPool = openstackMapping.cinderBackendPool
    }
    if (openstackMapping.cinderHost) {
      credBody.spec.openstackMapping.cinderHost = openstackMapping.cinderHost
    }
  }

  if (vendorType === 'netapp' && netAppConfig && (netAppConfig.svm || netAppConfig.flexVol)) {
    credBody.spec.netAppConfig = {}
    if (netAppConfig.svm) credBody.spec.netAppConfig.svm = netAppConfig.svm
    if (netAppConfig.flexVol) credBody.spec.netAppConfig.flexVol = netAppConfig.flexVol
  }

  const response = await axios.post<ArrayCreds>({
    endpoint,
    data: credBody
  })

  return response
}

// Patches ArrayCreds.spec.netAppConfig. Used when the user selects SVM/FlexVol
// after the controller has populated discovered backend targets on status.
export const patchNetAppConfig = async (
  name: string,
  netAppConfig: { svm: string; flexVol: string },
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const patchBody: Partial<ArrayCreds> = {
    spec: {
      // vendorType is required by ArrayCredsSpec but merge-patch only
      // applies the fields present; the server preserves the existing value.
      vendorType: 'netapp',
      netAppConfig
    }
  }
  return patchArrayCredentials(name, patchBody, namespace)
}

export const updateArrayCredsWithSecret = async (
  name: string,
  secretName: string,
  vendorType: string,
  openstackMapping?: {
    volumeType?: string
    cinderBackendName?: string
    cinderBackendPool?: string
    cinderHost?: string
  },
  netAppConfig?: {
    svm?: string
    flexVol?: string
  },
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/arraycreds/${name}`

  const patchBody: any = {
    spec: {
      vendorType,
      secretRef: {
        name: secretName
      }
    }
  }

  if (openstackMapping) {
    patchBody.spec.openstackMapping = {}
    if (openstackMapping.volumeType) {
      patchBody.spec.openstackMapping.volumeType = openstackMapping.volumeType
    }
    if (openstackMapping.cinderBackendName) {
      patchBody.spec.openstackMapping.cinderBackendName = openstackMapping.cinderBackendName
    }
    if (openstackMapping.cinderBackendPool) {
      patchBody.spec.openstackMapping.cinderBackendPool = openstackMapping.cinderBackendPool
    }
    if (openstackMapping.cinderHost) {
      patchBody.spec.openstackMapping.cinderHost = openstackMapping.cinderHost
    }
  }

  if (vendorType === 'netapp' && netAppConfig && (netAppConfig.svm || netAppConfig.flexVol)) {
    patchBody.spec.netAppConfig = {}
    if (netAppConfig.svm) patchBody.spec.netAppConfig.svm = netAppConfig.svm
    if (netAppConfig.flexVol) patchBody.spec.netAppConfig.flexVol = netAppConfig.flexVol
  }

  const response = await axios.patch<ArrayCreds>({
    endpoint,
    data: patchBody,
    config: {
      headers: {
        'Content-Type': 'application/merge-patch+json'
      }
    }
  })

  return response
}
