export type SSHKeyPairType = 'generated' | 'manual'

export interface SSHKeyPair {
  name: string
  type: SSHKeyPairType
  publicKey: string
  createdAt: string
}
