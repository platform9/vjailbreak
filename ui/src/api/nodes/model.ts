import { OpenstackCreds } from "../openstack-creds/model"

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
  status: Status
}

export interface ItemMetadata {
  creationTimestamp: Date
  finalizers: string[]
  generation: number
  name: string
  namespace: string
  resourceVersion: string
  uid: string
}

export interface Spec {
  imageid: string
  noderole: string
  openstackcreds: OpenstackCreds
  openstackflavorid: string
}

export interface Status {
  openstackuuid: string
  phase: string
  vmip: string
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
