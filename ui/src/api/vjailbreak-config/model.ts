export interface VjailbreakConfig {
  apiVersion: string
  kind: string
  metadata: Metadata
  spec: Spec
}

export interface Metadata {
  name: string
  namespace: string
}

export interface Spec {
  debug: boolean
}