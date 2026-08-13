import { Page, expect } from '@playwright/test'

// ─── API route constants ──────────────────────────────────────────────────────

export const NS = 'migration-system'
const V1A1 = `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/${NS}`
const K8S_PROXY_BASE = '/dev-api/sdk/vpw/v1/k8s/api/v1'

const escapeRegExp = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

export const API = {
  settingsConfigMap: `**/api/v1/namespaces/${NS}/configmaps/vjailbreak-settings`,
  migrations: `**${V1A1}/migrations`,
  migrationByName: (name: string) => `**${V1A1}/migrations/${name}`,
  migrationPlans: `**${V1A1}/migrationplans`,
  migrationPlanByName: (name: string) => `**${V1A1}/migrationplans/${name}`,
  migrationTemplates: `**${V1A1}/migrationtemplates`,
  migrationTemplateByName: (name: string) => `**${V1A1}/migrationtemplates/${name}`,
  vmwareCreds: `**${V1A1}/vmwarecreds`,
  vmwareCredByName: (name: string) => `**${V1A1}/vmwarecreds/${name}`,
  // The list endpoint is paginated (?limit=N), which glob patterns can't express
  // without also swallowing the by-name routes — use a regex instead.
  openstackCreds: new RegExp(`${escapeRegExp(V1A1)}/openstackcreds(\\?.*)?$`),
  openstackCredByName: (name: string) => `**${V1A1}/openstackcreds/${name}`,
  networkMappings: `**${V1A1}/networkmappings`,
  networkMappingByName: (name: string) => `**${V1A1}/networkmappings/${name}`,
  storageMappings: `**${V1A1}/storagemappings`,
  storageMappingByName: (name: string) => `**${V1A1}/storagemappings/${name}`,
  vmwareMachines: `**${V1A1}/vmwaremachines**`,
  vmwareMachineByName: (name: string) => `**${V1A1}/vmwaremachines/${name}`,
  vmwareClusters: `**${V1A1}/vmwareclusters**`,
  vmwareHosts: `**${V1A1}/vmwarehosts**`,
  pcdClusters: `**${V1A1}/pcdclusters**`,
  bmConfigs: `**${V1A1}/bmconfigs`,
  bmConfigByName: (name: string) => `**${V1A1}/bmconfigs/${name}`,
  rdmDisks: `**${V1A1}/rdmdisks`,
  volumeImageProfiles: `**${V1A1}/volumeimageprofiles**`,
  validateIPs: `**/validate_openstack_ip`,
  checkSubnetCompatibility: `**/check_network_subnet_compatibility`,
  podLogs: (namespace: string, podName: string) =>
    `**/namespaces/${namespace}/pods/${podName}/log*`,
  rollingMigrationPlans: `**${V1A1}/rollingmigrationplans`,
  k8sPods: (namespace: string) => `**${K8S_PROXY_BASE}/namespaces/${namespace}/pods`,
  k8sPodByName: (namespace: string, podName: string) =>
    `**${K8S_PROXY_BASE}/namespaces/${namespace}/pods/${podName}`,
  migrationConfigMapByName: (namespace: string, vmwareMachineName: string) =>
    `**${K8S_PROXY_BASE}/namespaces/${namespace}/configmaps/migration-config-${vmwareMachineName}`,
}

export const ROUTES = {
  migrations: '/dashboard/migrations',
  credentials: '/dashboard/credentials',
  clusterConversions: '/dashboard/cluster-conversions',
  migrationDetail: (name: string) => `/dashboard/migrations/${name}`,
}

// ─── Navigation ───────────────────────────────────────────────────────────────

export async function goToMigrations(page: Page): Promise<void> {
  await page.goto(ROUTES.migrations)
  await page.waitForURL(/\/dashboard\/migrations/)
  await expect(page.getByTestId('migrations-table')).toBeVisible({ timeout: 10_000 })
}

// Related-resource lookups (migration plan, template, creds, mappings, vmware
// machine, rdm disks) are fetched by useMigrationDetailResourcesQuery and are
// non-fatal on failure (KPI cells / Details tab fields fall back to '—' / N/A —
// see "Known mock gap" in migration-detail-page.md). Defaulting them to 404
// keeps detail-page tests deterministic without requiring a full resource mock
// chain for every spec; call mockMigrationDetailResources() before this to
// override with real data for tests that assert on resource-derived fields.
async function mock404DetailResourcesIfUnhandled(page: Page, migration: JsonBody): Promise<void> {
  const migrationPlanName =
    ((migration.spec as JsonBody | undefined)?.migrationPlan as string | undefined) ||
    (((migration.metadata as JsonBody | undefined)?.labels as JsonBody | undefined)
      ?.migrationplan as string | undefined)
  const vmwareMachineName = String((migration.metadata as JsonBody)?.name || '').replace(
    /^migration-/,
    '',
  )

  const routes: Array<[string | RegExp, JsonBody | JsonBody[]]> = [
    [API.vmwareCreds, []],
    [API.openstackCreds, []],
    [API.pcdClusters, { items: [] }],
    [API.rdmDisks, []],
  ]
  if (migrationPlanName) routes.push([API.migrationPlanByName(migrationPlanName), {}])
  if (vmwareMachineName) routes.push([API.vmwareMachineByName(vmwareMachineName), {}])

  for (const [url] of routes) {
    await page.route(url, (route) => route.fulfill({ status: 404, body: '{}' }))
  }
}

export async function goToMigrationDetail(
  page: Page,
  migration: JsonBody,
  { resources }: { resources?: Parameters<typeof mockMigrationDetailResources>[1] } = {},
): Promise<void> {
  const name = String((migration.metadata as JsonBody)?.name || '')
  await mockRoute(page, API.migrationByName(name), 'GET', migration)
  // Full resource chain wins when supplied; otherwise every related-resource
  // lookup 404s (non-fatal — see mock404DetailResourcesIfUnhandled). Only one
  // of the two is registered, since Playwright matches the most-recently-added
  // route first and the two would otherwise race for the same URLs.
  if (resources) {
    await mockMigrationDetailResources(page, resources)
  } else {
    await mock404DetailResourcesIfUnhandled(page, migration)
  }
  // migrationConfigMap is fetched in both paths above (keyed off the same
  // derived VMwareMachine name) but isn't part of either resource chain.
  // Left unmocked, the request falls through to the real (unreachable in
  // tests) backend and hangs rather than 404ing quickly — the resources
  // query never settles and the Details tab spins on "Loading migration
  // details…" forever.
  const namespace = ((migration.metadata as JsonBody | undefined)?.namespace as string) || NS
  const vmwareMachineName = name.replace(/^migration-/, '')
  if (vmwareMachineName) {
    await page.route(API.migrationConfigMapByName(namespace, vmwareMachineName), (route) =>
      route.fulfill({ status: 404, body: '{}' }),
    )
  }
  await page.goto(ROUTES.migrationDetail(name))
  await page.waitForURL(new RegExp(`/dashboard/migrations/${name}`))
  await expect(page.getByTestId('migration-detail-page')).toBeVisible({ timeout: 10_000 })
}

export async function goToGlobalSettings(page: Page): Promise<void> {
  await page.goto('/dashboard/global-settings')
  await page.waitForURL(/\/dashboard\/global-settings/)
  await expect(page.getByTestId('global-settings-form')).toBeVisible({ timeout: 10_000 })
}

// ─── Form interactions ────────────────────────────────────────────────────────

export async function openMigrationDrawer(page: Page): Promise<void> {
  await page.getByTestId('start-migration-button').click()
  await expect(page.getByTestId('migration-form-drawer')).toBeVisible()
}

export async function closeMigrationDrawer(page: Page): Promise<void> {
  await page.getByTestId('migration-form-close').click()
}

export async function submitMigrationForm(page: Page): Promise<void> {
  await page.getByTestId('migration-form-submit').click()
}

export async function selectVmwareCluster(page: Page, clusterValue: string): Promise<void> {
  // Wait for cluster data to load before clicking
  await expect(page.getByTestId('vmware-cluster-dropdown')).not.toBeDisabled({ timeout: 10_000 })
  await page.getByTestId('vmware-cluster-dropdown').click()
  await page.getByRole('option', { name: clusterValue }).click()
}

export async function selectPcdCluster(page: Page, clusterValue: string): Promise<void> {
  await expect(page.getByTestId('pcd-cluster-dropdown')).not.toBeDisabled({ timeout: 10_000 })
  await page.getByTestId('pcd-cluster-dropdown').click()
  await page.getByRole('option', { name: clusterValue }).click()
}

// ─── Route mocking helpers ────────────────────────────────────────────────────

type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
type JsonBody = Record<string, unknown>

export async function mockRoute(
  page: Page,
  url: string | RegExp,
  method: HttpMethod,
  body: JsonBody | JsonBody[],
  status = 200,
): Promise<void> {
  await page.route(url, (route) => {
    if (route.request().method() === method) {
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(body),
      })
    } else {
      route.continue()
    }
  })
}

export async function mockRouteError(
  page: Page,
  url: string,
  method: HttpMethod,
  status: 400 | 403 | 404 | 422 | 500,
  message = `Simulated ${status} error`,
): Promise<void> {
  await page.route(url, (route) => {
    if (route.request().method() === method) {
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify({ message }),
      })
    } else {
      route.continue()
    }
  })
}

// Full resource chain for tests that assert on values derived from the related
// resources (Migration Environment / General Info / Mappings sections, KPI
// Source/Destination cells). Mirrors the fetch order in
// useMigrationDetailResourcesQuery: migrationPlan -> migrationTemplate ->
// (vmwareCreds/openstackCreds/pcdClusters/networkMapping/storageMapping) ->
// vmwareMachine -> rdmDisks.
export async function mockMigrationDetailResources(
  page: Page,
  {
    migrationPlan,
    migrationTemplate,
    vmwareCredsList = [],
    openstackCredsList = [],
    pcdClusters = [],
    networkMapping,
    storageMapping,
    vmwareMachine,
    rdmDisks = [],
  }: {
    migrationPlan: JsonBody
    migrationTemplate: JsonBody
    vmwareCredsList?: JsonBody[]
    openstackCredsList?: JsonBody[]
    pcdClusters?: JsonBody[]
    networkMapping?: JsonBody
    storageMapping?: JsonBody
    vmwareMachine?: JsonBody
    rdmDisks?: JsonBody[]
  },
): Promise<void> {
  const planName = String((migrationPlan.metadata as JsonBody)?.name || '')
  const templateName = String((migrationTemplate.metadata as JsonBody)?.name || '')
  await mockRoute(page, API.migrationPlanByName(planName), 'GET', migrationPlan)
  await mockRoute(page, API.migrationTemplateByName(templateName), 'GET', migrationTemplate)
  await mockRoute(page, API.vmwareCreds, 'GET', vmwareCredsList)
  await mockRoute(page, API.openstackCreds, 'GET', openstackCredsList)
  await mockRoute(page, API.pcdClusters, 'GET', { items: pcdClusters })
  await mockRoute(page, API.rdmDisks, 'GET', rdmDisks)
  if (networkMapping) {
    const name = String((networkMapping.metadata as JsonBody)?.name || '')
    await mockRoute(page, API.networkMappingByName(name), 'GET', networkMapping)
  }
  if (storageMapping) {
    const name = String((storageMapping.metadata as JsonBody)?.name || '')
    await mockRoute(page, API.storageMappingByName(name), 'GET', storageMapping)
  }
  if (vmwareMachine) {
    const name = String((vmwareMachine.metadata as JsonBody)?.name || '')
    await mockRoute(page, API.vmwareMachineByName(name), 'GET', vmwareMachine)
  }
}

// ─── Assertion helpers ────────────────────────────────────────────────────────

export async function expectToast(page: Page, text: string | RegExp): Promise<void> {
  await expect(page.getByRole('alert').filter({ hasText: text })).toBeVisible({ timeout: 5000 })
}

export async function expectDrawerOpen(page: Page): Promise<void> {
  await expect(page.getByTestId('migration-form-drawer')).toBeVisible()
}

export async function expectDrawerClosed(page: Page): Promise<void> {
  await expect(page.getByTestId('migration-form-drawer')).not.toBeVisible()
}

export async function expectSectionNavError(page: Page, sectionId: string): Promise<void> {
  await expect(
    page.getByTestId(`section-nav-item-${sectionId}`).getByTestId('section-nav-error-badge'),
  ).toBeVisible()
}

export async function expectSectionNavClear(page: Page, sectionId: string): Promise<void> {
  await expect(
    page.getByTestId(`section-nav-item-${sectionId}`).getByTestId('section-nav-error-badge'),
  ).not.toBeVisible()
}

export async function expectSubmitDisabled(page: Page): Promise<void> {
  await expect(page.getByTestId('migration-form-submit')).toBeDisabled()
}

export async function expectSubmitEnabled(page: Page): Promise<void> {
  await expect(page.getByTestId('migration-form-submit')).toBeEnabled()
}
