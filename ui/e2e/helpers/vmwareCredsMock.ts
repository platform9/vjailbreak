import { Page, Route, expect } from '@playwright/test'
import { installMockRoutes, jsonResponse, emptyResponse, MockRoute } from './mockApi'

type VmwareCred = {
  apiVersion: string
  kind: 'VMwareCreds'
  metadata: { name: string; namespace: string }
  spec?: any
  status?: {
    vmwareValidationStatus?: string
    vmwareValidationMessage?: string
  }
}

type Secret = {
  apiVersion: string
  kind: 'Secret'
  metadata: { name: string; namespace: string }
  data?: Record<string, string>
  type?: string
}

const NS = 'migration-system'
const API_BASE = '/dev-api'
const VJ_API = `${API_BASE}/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/${NS}`
const K8S_API = `${API_BASE}/api/v1/namespaces/${NS}`

export type VmwareMockState = {
  vmwareCreds: VmwareCred[]
  secrets: Secret[]
  // for status transitions during polling
  pollCountByName: Record<string, number>
  // if set, list openstack creds for the post-success prompt logic
  openstackCreds?: any[]
}

export function createDefaultState(): VmwareMockState {
  return {
    vmwareCreds: [],
    secrets: [],
    pollCountByName: {},
    openstackCreds: []
  }
}

export async function mockVmwareCredentialsApi(
  page: Page,
  state: VmwareMockState,
  opts?: {
    createSucceedsAfterPolls?: number
    createFails?: { status: number; message: string }
    secretCreateFails?: { status: number; message: string }
    pollFails?: { status: number; message: string }
    validationFails?: { message: string }
  }
) {
  const createSucceedsAfterPolls = opts?.createSucceedsAfterPolls ?? 1

  const routes: MockRoute[] = [
    // List VMware creds
    {
      method: 'GET',
      url: new RegExp(`.*${escapeRegExp(VJ_API)}/vmwarecreds(?:\\?.*)?$`),
      handler: (route) => jsonResponse(route, 200, { items: state.vmwareCreds })
    },

    // Create secret (VMware)
    {
      method: 'POST',
      url: new RegExp(`.*${escapeRegExp(K8S_API)}/secrets(?:\\?.*)?$`),
      handler: async (route) => {
        if (opts?.secretCreateFails) {
          return jsonResponse(route, opts.secretCreateFails.status, {
            message: opts.secretCreateFails.message
          })
        }

        const body = (await route.request().postDataJSON()) as any
        // basic contract checks so tests catch accidental payload regressions
        expect(body?.kind).toBe('Secret')
        expect(body?.metadata?.namespace).toBe(NS)
        expect(body?.metadata?.name).toContain('-vmware-secret')
        expect(body?.data?.VCENTER_HOST).toBeTruthy()
        expect(body?.data?.VCENTER_USERNAME).toBeTruthy()
        expect(body?.data?.VCENTER_PASSWORD).toBeTruthy()

        const secret: Secret = {
          apiVersion: 'v1',
          kind: 'Secret',
          metadata: { name: body.metadata.name, namespace: NS },
          data: body.data,
          type: body.type ?? 'Opaque'
        }
        state.secrets.push(secret)
        return jsonResponse(route, 201, secret)
      }
    },

    // Create VMwareCreds
    {
      method: 'POST',
      url: new RegExp(`.*${escapeRegExp(VJ_API)}/vmwarecreds(?:\\?.*)?$`),
      handler: async (route) => {
        if (opts?.createFails) {
          return jsonResponse(route, opts.createFails.status, { message: opts.createFails.message })
        }

        const body = (await route.request().postDataJSON()) as any
        expect(body?.kind).toBe('VMwareCreds')
        expect(body?.metadata?.namespace).toBe(NS)
        expect(body?.metadata?.name).toBeTruthy()

        const created: VmwareCred = {
          apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
          kind: 'VMwareCreds',
          metadata: { name: body.metadata.name, namespace: NS },
          spec: body.spec,
          status: {
            vmwareValidationStatus: 'Validating',
            vmwareValidationMessage: ''
          }
        }

        state.vmwareCreds.push(created)
        state.pollCountByName[created.metadata.name] = 0

        return jsonResponse(route, 201, created)
      }
    },

    // Poll get VMwareCreds by name
    {
      method: 'GET',
      url: new RegExp(`.*${escapeRegExp(VJ_API)}/vmwarecreds/([^/?]+)(?:\\?.*)?$`),
      handler: (route) => {
        if (opts?.pollFails) {
          return jsonResponse(route, opts.pollFails.status, { message: opts.pollFails.message })
        }

        const url = route.request().url()
        const name = decodeURIComponent(url.split('/vmwarecreds/')[1].split(/[?#]/)[0])
        const cred = state.vmwareCreds.find((c) => c.metadata.name === name)
        if (!cred) return jsonResponse(route, 404, { message: 'Not Found' })

        state.pollCountByName[name] = (state.pollCountByName[name] ?? 0) + 1
        const pollCount = state.pollCountByName[name]

        if (opts?.validationFails) {
          cred.status = {
            vmwareValidationStatus: 'Failed',
            vmwareValidationMessage: opts.validationFails.message
          }
        } else if (pollCount >= createSucceedsAfterPolls) {
          cred.status = { vmwareValidationStatus: 'Succeeded', vmwareValidationMessage: '' }
        } else {
          cred.status = { vmwareValidationStatus: 'Validating', vmwareValidationMessage: '' }
        }

        return jsonResponse(route, 200, cred)
      }
    },

    // Delete VMwareCreds
    {
      method: 'DELETE',
      url: new RegExp(`.*${escapeRegExp(VJ_API)}/vmwarecreds/([^/?]+)(?:\\?.*)?$`),
      handler: (route) => {
        const url = route.request().url()
        const name = decodeURIComponent(url.split('/vmwarecreds/')[1].split(/[?#]/)[0])
        state.vmwareCreds = state.vmwareCreds.filter((c) => c.metadata.name !== name)
        return emptyResponse(route, 200)
      }
    },

    // Delete Secret
    {
      method: 'DELETE',
      url: new RegExp(`.*${escapeRegExp(K8S_API)}/secrets/([^/?]+)(?:\\?.*)?$`),
      handler: (route) => {
        const url = route.request().url()
        const name = decodeURIComponent(url.split('/secrets/')[1].split(/[?#]/)[0])
        state.secrets = state.secrets.filter((s) => s.metadata.name !== name)
        return emptyResponse(route, 200)
      }
    },

    // Openstack creds list (used to decide whether to show PCD prompt after VMware success)
    {
      method: 'GET',
      url: new RegExp(`.*${escapeRegExp(VJ_API)}/openstackcreds(?:\\?.*)?$`),
      handler: (route) => jsonResponse(route, 200, { items: state.openstackCreds ?? [] })
    },

    // Revalidate endpoint - just accept and let list polling update drive UI.
    {
      method: 'POST',
      url: /.*\/dev-api\/sdk\/vpw\/v1\/revalidate_credentials(?:\?.*)?$/,
      handler: (route) => jsonResponse(route, 200, { message: 'ok' })
    }
  ]

  await installMockRoutes(page, routes)
}

function escapeRegExp(str: string) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
