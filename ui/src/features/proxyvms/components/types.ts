export type SSHKeySource = 'generated' | 'manual'
export type FormMode = 'select' | 'create'

export interface VMOption {
  name: string
  ipAddress?: string
  cpu: number
  powerState: string
  osFamily?: string
}

export interface SelectFormData {
  vmwareCredsRef: string
  vmName: string
  sshPrivateKey: string
  authorizedKeysConfirmed: boolean
}

export interface CreateFormData {
  vmwareCredsRef: string
  vmName: string
  datacenter: string
  datastore: string
  network: string
  cluster: string
}

export interface GeneratedKey {
  secretName: string
  publicKey: string
}
