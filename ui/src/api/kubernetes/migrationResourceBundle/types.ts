export type Metadata = {
  name?: string
  namespace?: string
  uid?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  ownerReferences?: Array<{
    apiVersion?: string
    kind?: string
    name?: string
    uid?: string
  }>
}

export type UnknownRecord = Record<string, unknown>

export type KubernetesObject = {
  apiVersion?: string
  kind?: string
  metadata?: Metadata
  spec?: UnknownRecord
  status?: UnknownRecord
  data?: Record<string, string>
}

export type KubernetesList = {
  items?: KubernetesObject[]
}

export type BundleEntry = {
  path: string
  object: KubernetesObject
}
