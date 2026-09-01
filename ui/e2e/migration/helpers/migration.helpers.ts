import { Page, expect } from '@playwright/test'

// ─── API route constants ──────────────────────────────────────────────────────

export const NS = 'migration-system'
const V1A1 = `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/${NS}`

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
  // List endpoints carry query strings (?labelSelector=, ?limit=). A trailing `**`
  // glob would also match `/vmwaremachines/<name>`, and Playwright resolves routes
  // most-recently-registered first — so the list stub would swallow the by-name
  // fetch and hand the caller a `{items: []}` body. Anchor with a regex instead.
  vmwareMachines: new RegExp(`${escapeRegExp(V1A1)}/vmwaremachines(\\?.*)?$`),
  vmwareMachineByName: (name: string) => `**${V1A1}/vmwaremachines/${name}`,
  vmwareClusters: new RegExp(`${escapeRegExp(V1A1)}/vmwareclusters(\\?.*)?$`),
  vmwareHosts: new RegExp(`${escapeRegExp(V1A1)}/vmwarehosts(\\?.*)?$`),
  // Rolling submit writes the chosen host config back onto each ESXi host.
  vmwareHostByName: (name: string) => `**${V1A1}/vmwarehosts/${name}`,
  esxiMigrations: new RegExp(`${escapeRegExp(V1A1)}/esximigrations(\\?.*)?$`),
  pcdClusters: new RegExp(`${escapeRegExp(V1A1)}/pcdclusters(\\?.*)?$`),
  bmConfigs: `**${V1A1}/bmconfigs`,
  bmConfigByName: (name: string) => `**${V1A1}/bmconfigs/${name}`,
  rdmDisks: `**${V1A1}/rdmdisks`,
  volumeImageProfiles: `**${V1A1}/volumeimageprofiles**`,
  validateIPs: `**/validate_openstack_ip`,
  checkSubnetCompatibility: `**/check_network_subnet_compatibility`,
  podLogs: (namespace: string, podName: string) =>
    `**/namespaces/${namespace}/pods/${podName}/log*`,
  rollingMigrationPlans: `**${V1A1}/rollingmigrationplans`,
  proxyVMs: new RegExp(`${escapeRegExp(V1A1)}/proxyvms(\\?.*)?$`),
}

export const ROUTES = {
  migrations: '/dashboard/migrations',
  credentials: '/dashboard/credentials',
  clusterConversions: '/dashboard/cluster-conversions',
}

// ─── Navigation ───────────────────────────────────────────────────────────────

export async function goToMigrations(page: Page): Promise<void> {
  await page.goto(ROUTES.migrations)
  await page.waitForURL(/\/dashboard\/migrations/)
  await expect(page.getByTestId('migrations-table')).toBeVisible({ timeout: 10_000 })
}

// Pod logs live on the Migration Detail page's "Pod logs" tab. The per-row logs
// button that used to open a drawer was removed with the new migrations UI (#2040).
export async function goToMigrationPodLogs(page: Page, migrationName: string): Promise<void> {
  await page.goto(`${ROUTES.migrations}/${migrationName}`)
  const logsTab = page.getByRole('tab', { name: /pod logs/i })
  await expect(logsTab).toBeVisible({ timeout: 10_000 })
  await logsTab.click()
}

// The logs toolbar renders its search box without a testid — locate it by placeholder.
export function podLogsSearchInput(page: Page) {
  return page.getByPlaceholder('Search logs…')
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

// storageCopyMethod defaults to 'HotAdd' (vJailbreak Accelerated Copy), which requires
// picking a Ready Proxy VM before submit is enabled.
export async function selectProxyVM(page: Page, vmName: string): Promise<void> {
  await expect(page.getByTestId('proxy-vm-dropdown')).not.toBeDisabled({ timeout: 10_000 })
  await page.getByTestId('proxy-vm-dropdown').click()
  await page.getByRole('option', { name: new RegExp(vmName) }).click()
}

// ─── Route mocking helpers ────────────────────────────────────────────────────

type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
type JsonBody = Record<string, unknown>

// Method mismatches must fall back, not continue: tests stack one mock per method on
// the same URL (GET + PATCH + DELETE on a plan, say), and Playwright runs handlers
// most-recently-registered first. route.continue() would send the request to the dev
// server — skipping the earlier handler for that method — and the app would see a 500.
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
      route.fallback()
    }
  })
}

export async function mockRouteError(
  page: Page,
  url: string | RegExp,
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
      route.fallback()
    }
  })
}

// ─── Assertion helpers ────────────────────────────────────────────────────────

export async function expectToast(page: Page, text: string | RegExp): Promise<void> {
  await expect(page.getByRole('alert').filter({ hasText: text })).toBeVisible({ timeout: 5000 })
}

export async function expectDrawerOpen(page: Page): Promise<void> {
  await expect(page.getByTestId('migration-form-drawer')).toBeVisible()
}

// Submitting closes the drawer only after the whole create chain (mappings → template →
// plan) resolves, which is slower than the default expect timeout on a cold dev server.
export async function expectDrawerClosed(page: Page, timeout = 15_000): Promise<void> {
  await expect(page.getByTestId('migration-form-drawer')).not.toBeVisible({ timeout })
}

// Resolves once the MigrationPlan POST comes back — the last write of a standard submit.
export function waitForMigrationPlanCreated(page: Page) {
  return page.waitForResponse(
    (res) =>
      res.request().method() === 'POST' &&
      new URL(res.url()).pathname.endsWith('/migrationplans'),
    { timeout: 20_000 }
  )
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

// A section with work still to do renders neither the attention badge nor the
// completion check — just its step number. Unmapped networks/datastores land here:
// they block submit without being flagged as an error (the mapping fieldErrors are
// only set when the mapping POST itself fails).
export async function expectSectionNavIncomplete(page: Page, sectionId: string): Promise<void> {
  const item = page.getByTestId(`section-nav-item-${sectionId}`)
  await expect(item).toBeVisible()
  await expect(item.getByTestId('section-nav-error-badge')).toHaveCount(0)
  // The completion check is the only svg rendered inside the step chip.
  await expect(item.locator('svg')).toHaveCount(0)
}

export async function expectSubmitDisabled(page: Page): Promise<void> {
  await expect(page.getByTestId('migration-form-submit')).toBeDisabled()
}

export async function expectSubmitEnabled(page: Page): Promise<void> {
  await expect(page.getByTestId('migration-form-submit')).toBeEnabled()
}
