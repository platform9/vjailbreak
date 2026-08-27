import { test, expect, Page, Route } from '@playwright/test'

import {
  goToMigrations,
  openMigrationDrawer,
  selectVmwareCluster,
  selectPcdCluster,
  mockRoute,
  API,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST_EMPTY,
  MOCK_MIGRATION_PLANS_LIST_EMPTY,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_VMWARE_CRED_1,
  MOCK_OPENSTACK_CRED_1,
  MOCK_OPENSTACK_CREDS_LIST,
  MOCK_VMWARE_CLUSTERS_LIST,
  MOCK_PCD_CLUSTERS_LIST,
  MOCK_VMWARE_MACHINES_LIST,
  MOCK_MIGRATION_TEMPLATE_PENDING,
  MOCK_MIGRATION_TEMPLATE_READY,
} from './helpers/migration.fixtures'

// The VM list's refresh button does not simply refetch: it asks the backend to revalidate the
// VMware credential, polls that single credential until the status settles, and only then
// reloads the VM list. The button must stay disabled for the duration and recover on error.
//
// Replaces cypress/e2e/vm-selection-refresh-revalidate.cy.ts.

const REVALIDATE = '**/vpw/v1/revalidate_credentials'
const VMWARE_CRED_NAME = 'vcenter-cred-1'

const REVALIDATING_CRED = {
  ...MOCK_VMWARE_CRED_1,
  status: { ...(MOCK_VMWARE_CRED_1.status ?? {}), vmwareValidationStatus: 'Revalidating' },
}

// Counts VM list fetches so a refetch can be distinguished from the initial load.
type MachinesCounter = { fetches: number }

async function mockFormApis(page: Page): Promise<MachinesCounter> {
  const counter: MachinesCounter = { fetches: 0 }

  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
  await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
  await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
  await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
  await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
  await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
  await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
  await mockRoute(page, API.rdmDisks, 'GET', { items: [] })
  await mockRoute(page, API.volumeImageProfiles, 'GET', { items: [] })

  await page.route(API.vmwareMachines, (route: Route) => {
    counter.fetches += 1
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MOCK_VMWARE_MACHINES_LIST),
    })
  })

  await page.route('**migrationtemplates**', (route: Route) => {
    const method = route.request().method()
    const body =
      method === 'POST' ? MOCK_MIGRATION_TEMPLATE_PENDING : MOCK_MIGRATION_TEMPLATE_READY
    if (method === 'DELETE') return route.fulfill({ status: 200, body: '{}' })
    route.fulfill({
      status: method === 'POST' ? 201 : 200,
      contentType: 'application/json',
      body: JSON.stringify(body),
    })
  })

  return counter
}

async function openFormAndSelectClusters(page: Page): Promise<void> {
  await goToMigrations(page)
  await openMigrationDrawer(page)
  await selectVmwareCluster(page, 'DC1-Cluster')
  await selectPcdCluster(page, 'pcd-cluster-1')
  await expect(
    page.getByTestId('vms-datagrid').locator('[role="row"]').nth(1)
  ).toBeVisible({ timeout: 15_000 })
}

// The migrations page behind the drawer renders its own refresh button with the same
// testid, so scope to the form drawer.
const refreshButton = (page: Page) =>
  page.getByTestId('migration-form-drawer').getByTestId('vm-list-refresh-button')

test.describe('MIG-043 — VM list refresh revalidates the VMware credential', () => {
  test('refresh posts revalidate_credentials for the VMware credential', async ({ page }) => {
    await mockFormApis(page)
    await mockRoute(page, API.vmwareCredByName(VMWARE_CRED_NAME), 'GET', MOCK_VMWARE_CRED_1)
    await mockRoute(page, REVALIDATE, 'POST', { message: 'revalidation triggered' })

    await openFormAndSelectClusters(page)

    const revalidateRequest = page.waitForRequest(
      (req) => req.url().includes('/revalidate_credentials') && req.method() === 'POST'
    )
    await expect(refreshButton(page)).toBeEnabled()
    await refreshButton(page).click()

    const payload = (await revalidateRequest).postDataJSON()
    expect(payload).toMatchObject({
      name: VMWARE_CRED_NAME,
      namespace: 'migration-system',
      kind: 'VmwareCreds',
    })
  })

  test('refresh stays disabled while the backend reports Revalidating', async ({ page }) => {
    await mockFormApis(page)

    // The credential must report Succeeded while the form loads — an unvalidated credential
    // means no VM list at all, and the refresh button is disabled for that reason instead.
    // Flipping to Revalidating only after the POST reproduces the real sequence: the hook
    // polls the single credential (not the list) and keeps the button busy until it settles.
    let revalidating = false
    await page.route(API.vmwareCredByName(VMWARE_CRED_NAME), (route: Route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(revalidating ? REVALIDATING_CRED : MOCK_VMWARE_CRED_1),
      })
    })
    await page.route(REVALIDATE, (route) => {
      revalidating = true
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'revalidation triggered' }),
      })
    })

    await openFormAndSelectClusters(page)
    await refreshButton(page).click()

    await expect(refreshButton(page)).toBeDisabled()
    // The spinner is rendered inside the button while it is busy.
    await expect(refreshButton(page).locator('svg')).toBeVisible()
  })

  test('VM list is refetched once revalidation completes', async ({ page }) => {
    const counter = await mockFormApis(page)
    // Status is already Succeeded, so the hook completes as soon as the credential
    // query refetches, which is what triggers the VM list reload.
    await mockRoute(page, API.vmwareCredByName(VMWARE_CRED_NAME), 'GET', MOCK_VMWARE_CRED_1)
    await mockRoute(page, REVALIDATE, 'POST', { message: 'revalidation triggered' })

    await openFormAndSelectClusters(page)
    const fetchesBeforeRefresh = counter.fetches

    await refreshButton(page).click()

    await expect
      .poll(() => counter.fetches, { timeout: 20_000 })
      .toBeGreaterThan(fetchesBeforeRefresh)
  })

  test('refresh re-enables after a revalidation API error', async ({ page }) => {
    await mockFormApis(page)
    await mockRoute(page, API.vmwareCredByName(VMWARE_CRED_NAME), 'GET', MOCK_VMWARE_CRED_1)
    await page.route(REVALIDATE, (route) =>
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Internal server error' }),
      })
    )

    await openFormAndSelectClusters(page)
    await expect(refreshButton(page)).toBeEnabled()
    await refreshButton(page).click()

    // clearActiveRevalidation() runs in the mutation's onError, so the operator can retry.
    await expect(refreshButton(page)).toBeEnabled({ timeout: 15_000 })
  })
})
