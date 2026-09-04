import api from 'src/api/axios'

export interface ProxyCredsStatus {
  configured: boolean
  https_override: boolean
}

export interface SaveProxyCredsRequest {
  username: string
  password: string
  https_override: boolean
  https_username?: string
  https_password?: string
}

export async function getProxyCredsStatus(): Promise<ProxyCredsStatus> {
  return api.get<ProxyCredsStatus>({ endpoint: '/dev-api/sdk/vpw/v1/proxy/credentials' })
}

export async function saveProxyCreds(req: SaveProxyCredsRequest): Promise<ProxyCredsStatus> {
  return api.post<ProxyCredsStatus>({ endpoint: '/dev-api/sdk/vpw/v1/proxy/credentials', data: req })
}

export async function deleteProxyCreds(): Promise<ProxyCredsStatus> {
  return api.del<ProxyCredsStatus>({ endpoint: '/dev-api/sdk/vpw/v1/proxy/credentials' })
}
