import { ArrayCredsMappingFormData } from './model'

const NAMESPACE = 'migration-system'

export const createArrayCredsMappingJson = (data: ArrayCredsMappingFormData) => {
  const timestamp = Date.now()
  const name = `arraycreds-mapping-${timestamp}`

  return {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'ArrayCredsMapping',
    metadata: {
      name,
      namespace: NAMESPACE,
    },
    spec: {
      mappings: data.mappings,
    },
  }
}
