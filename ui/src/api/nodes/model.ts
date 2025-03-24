export interface NodeList {
  apiVersion: string
  items: NodeItem[]
  kind: "VjailbreakNodeList"
  metadata: NodeListMetadata
}

export interface NodeItem {
  apiVersion: string
  kind: "VjailbreakNode"
  metadata: ItemMetadata
  spec: Spec
  status?: Status
}

export interface ItemMetadata {
  creationTimestamp?: Date
  finalizers?: string[]
  generation?: number
  name: string
  namespace: string
  resourceVersion?: string
  uid?: string
}

export interface OpenstackCredsRef {
  kind: "openstackcreds"
  name: string
  namespace: string
}

export interface Spec {
  openstackImageID: string
  nodeRole: string
  openstackCreds: OpenstackCredsRef
  openstackFlavorID: string
  availableFlavors?: OpenstackFlavor[]
}

export interface Status {
  activeMigrations?: string[]
  openstackUUID: string
  phase: string
  vmIP: string
}

export interface NodeListMetadata {
  continue: string
  resourceVersion: string
}

export interface OpenstackFlavor {
  id: string
  name: string
  vcpus: number
  ram: number
  disk: number
}

export interface OpenstackFlavorsResponse {
  flavors: OpenstackFlavor[]
}

export interface OpenstackProject {
  id: string
  name: string
  domain_id: string
  description: string
  enabled: boolean
  parent_id: string
  is_domain: boolean
  tags: string[]
  options: Record<string, unknown>
  links: {
    self: string
  }
}

export interface OpenstackProjectsResponse {
  projects: OpenstackProject[]
  links: {
    next: string | null
    self: string
    previous: string | null
  }
}
