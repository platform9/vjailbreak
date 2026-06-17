import { Page, expect } from '@playwright/test'

// ─── API route constants ──────────────────────────────────────────────────────

export const NS = 'migration-system'
const V1A1 = `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/${NS}`

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
  openstackCreds: `**${V1A1}/openstackcreds`,
  openstackCredByName: (name: string) => `**${V1A1}/openstackcreds/${name}`,
  networkMappings: `**${V1A1}/networkmappings`,
  storageMappings: `**${V1A1}/storagemappings`,
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
  podLogs: (namespace: string, podName: string) =>
    `**/namespaces/${namespace}/pods/${podName}/log*`,
  rollingMigrationPlans: `**${V1A1}/rollingmigrationplans`,
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
  url: string,
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
