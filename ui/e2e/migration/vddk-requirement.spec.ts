import { test, expect, type Page } from '@playwright/test'
import {
  API,
  ROUTES,
  goToMigrations,
  mockRoute,
  expectSubmitDisabled,
  openMigrationDrawer,
  selectPcdCluster,
  selectVmwareCluster,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST_EMPTY,
  MOCK_MIGRATION_PLANS_LIST_EMPTY,
  MOCK_MIGRATION_TEMPLATE_PENDING,
  MOCK_MIGRATION_TEMPLATE_READY,
  MOCK_OPENSTACK_CREDS_LIST,
  MOCK_OPENSTACK_CRED_1,
  MOCK_PCD_CLUSTERS_LIST,
  MOCK_VMWARE_CLUSTERS_LIST,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_VMWARE_CRED_1,
  MOCK_VMWARE_MACHINES_LIST,
} from './helpers/migration.fixtures'

// Only the "normal" copy path opens VDDK, and the copy method is unknown at dashboard load.

const VDDK_STATUS = '**/vpw/v1/vddk/status'

async function mockVddkMissing(page: Page) {
  await mockRoute(page, VDDK_STATUS, 'GET', { uploaded: false, message: 'VDDK has not been uploaded' })
}

async function mockStandardFormApis(page: Page) {
  await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
  await mockRoute(page, API.vmwareCredByName('vcenter-cred-1'), 'GET', MOCK_VMWARE_CRED_1)
  await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
  await mockRoute(page, API.vmwareMachines, 'GET', MOCK_VMWARE_MACHINES_LIST)
  await mockRoute(page, API.rdmDisks, 'GET', {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'RdmDiskList',
    metadata: { continue: '', resourceVersion: '1' },
    items: [],
  })
  await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
  await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
  await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
  await page.route('**migrationtemplates**', (route) => {
    const method = route.request().method()
    if (method === 'POST') {
      route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING),
      })
    } else if (method === 'GET' || method === 'PATCH') {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_READY),
      })
    } else {
      route.continue()
    }
  })
}

// ─── The app-wide VDDK gate is gone ──────────────────────────────────────────

test.describe('VDDK is not an app-wide prerequisite', () => {
  test('missing VDDK does not trap the dashboard on the VDDK Upload tab', async ({ page }) => {
    await mockVddkMissing(page)
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await page.goto('/dashboard')

    // Credentials exist, so the index redirect lands on Migrations, not the VDDK tab.
    await expect(page).toHaveURL(new RegExp(`${ROUTES.migrations}$`), { timeout: 15_000 })
  })

  test('the Migrations page stays reachable with VDDK missing', async ({ page }) => {
    await mockVddkMissing(page)
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await goToMigrations(page)

    await expect(page).toHaveURL(new RegExp(`${ROUTES.migrations}$`))
    await expect(page.getByTestId('start-migration-button')).toBeVisible({ timeout: 15_000 })
  })
})

// ─── The requirement now follows the selected copy method ────────────────────

test.describe('VDDK requirement follows the storage copy method', () => {
  test.beforeEach(async ({ page }) => {
    await mockVddkMissing(page)
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectVmwareCluster(page, 'DC1-Cluster')
    await selectPcdCluster(page, 'pcd-cluster-1')
    await expect(page.getByTestId('vms-datagrid').locator('[role="row"]').nth(1)).toBeVisible({
      timeout: 15_000,
    })
    await page.getByTestId('section-nav-item-map-resources').click()
  })

  test('Standard Copy with VDDK missing shows the requirement', async ({ page }) => {
    await page.getByRole('radio', { name: /standard copy/i }).click()

    await expect(page.getByTestId('vddk-required-alert')).toBeVisible({ timeout: 5_000 })
    await expectSubmitDisabled(page)
  })

  test('vJailbreak Accelerated Copy does not require VDDK', async ({ page }) => {
    await page.getByRole('radio', { name: /vjailbreak accelerated copy/i }).click()

    await expect(page.getByTestId('vddk-required-alert')).toBeHidden()
  })

  test('Storage Accelerated Copy does not require VDDK', async ({ page }) => {
    await page.getByRole('radio', { name: /storage accelerated copy/i }).click()

    await expect(page.getByTestId('vddk-required-alert')).toBeHidden()
  })

  test('switching from an exempt method back to Standard Copy re-raises it', async ({ page }) => {
    await page.getByRole('radio', { name: /vjailbreak accelerated copy/i }).click()
    await expect(page.getByTestId('vddk-required-alert')).toBeHidden()

    await page.getByRole('radio', { name: /standard copy/i }).click()
    await expect(page.getByTestId('vddk-required-alert')).toBeVisible({ timeout: 5_000 })
  })
})
