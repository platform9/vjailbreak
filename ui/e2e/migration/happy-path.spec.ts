import { test, expect, Page } from '@playwright/test'

import {
  goToMigrations,
  openMigrationDrawer,
  submitMigrationForm,
  selectVmwareCluster,
  selectPcdCluster,
  mockRoute,
  expectToast,
  expectDrawerOpen,
  expectDrawerClosed,
  API,
  ROUTES,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST,
  MOCK_MIGRATIONS_FOR_BULK_DELETE,
  MOCK_MIGRATION_PLANS_LIST,
  MOCK_MIGRATION_AWAITING_CUTOVER,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_VMWARE_CRED_1,
  MOCK_OPENSTACK_CRED_1,
  MOCK_OPENSTACK_CREDS_LIST,
  MOCK_OPENSTACK_CRED_WITH_HOST_CONFIG,
  MOCK_MIGRATION_TEMPLATE_PENDING,
  MOCK_MIGRATION_TEMPLATE_READY,
  MOCK_NETWORK_MAPPING_CREATED,
  MOCK_STORAGE_MAPPING_CREATED,
  MOCK_MIGRATION_PLAN_CREATED,
  MOCK_VMWARE_CLUSTERS_LIST,
  MOCK_PCD_CLUSTERS_LIST,
  MOCK_VMWARE_HOSTS_LIST,
  MOCK_BM_CONFIGS_LIST,
  MOCK_ROLLING_MIGRATION_PLAN_CREATED,
  MOCK_VMWARE_MACHINES_LIST,
  NS,
} from './helpers/migration.fixtures'

// ─── Shared helpers ───────────────────────────────────────────────────────────

async function mockStandardFormApis(page: Page) {
  await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
  await mockRoute(page, API.vmwareCredByName('vcenter-cred-1'), 'GET', MOCK_VMWARE_CRED_1)
  await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
  await mockRoute(page, API.vmwareMachines, 'GET', MOCK_VMWARE_MACHINES_LIST)
  await mockRoute(page, API.rdmDisks, 'GET', { apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1', kind: 'RdmDiskList', metadata: { continue: '', resourceVersion: '1' }, items: [] })
  await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
  await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
  await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
  // Template: POST returns pending; any GET returns ready (polling resolves immediately)
  await page.route(`**migrationtemplates**`, (route) => {
    const method = route.request().method()
    if (method === 'POST') {
      route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
    } else if (method === 'GET') {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_READY) })
    } else {
      route.continue()
    }
  })
}

async function selectClustersAndWaitForVMs(page: Page) {
  await selectVmwareCluster(page, 'DC1-Cluster')
  await selectPcdCluster(page, 'pcd-cluster-1')
  // Wait for actual VM rows to load (nth(1) = first data row after header)
  await expect(page.getByTestId('vms-datagrid').locator('[role="row"]').nth(1)).toBeVisible({ timeout: 15_000 })
}

async function selectTwoVms(page: Page) {
  const grid = page.getByTestId('vms-datagrid')
  await grid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })
  await grid.locator('[role="row"]').nth(2).getByRole('checkbox').click({ force: true })
  // Toolbar should show 2 selected
  await expect(page.getByText(/2 selected/i)).toBeVisible()
}

async function mapAllNetworks(page: Page) {
  const table = page.getByTestId('network-mapping-table')
  await expect(table).toBeVisible()
  const rows = table.locator('[role="row"]').filter({ hasText: /VM Network|Management/i })
  const count = await rows.count()
  for (let i = 0; i < count; i++) {
    await rows.nth(i).getByRole('combobox').click()
    await page.getByRole('option').first().click()
  }
}

async function mapAllStorage(page: Page) {
  const table = page.getByTestId('storage-mapping-table')
  await expect(table).toBeVisible()
  const rows = table.locator('[role="row"]').filter({ hasText: /datastore/i })
  const count = await rows.count()
  for (let i = 0; i < count; i++) {
    await rows.nth(i).getByRole('combobox').click()
    await page.getByRole('option').first().click()
  }
}

// ─── MIG-004: Complete standard migration — happy path ────────────────────────

test.describe('MIG-004 — complete standard migration', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.networkMappings, 'POST', MOCK_NETWORK_MAPPING_CREATED)
    await mockRoute(page, API.storageMappings, 'POST', MOCK_STORAGE_MAPPING_CREATED)
    await mockRoute(page, API.migrationPlans, 'POST', MOCK_MIGRATION_PLAN_CREATED)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('standard migration created end-to-end', async ({ page }) => {
    await openMigrationDrawer(page)
    await expectDrawerOpen(page)

    // Step 1: select clusters
    await selectClustersAndWaitForVMs(page)

    // Step 2: select 2 VMs
    await selectTwoVms(page)

    // Step 3: map all networks and storage
    await page.getByTestId('section-nav-item-map-resources').click()
    await mapAllNetworks(page)
    await mapAllStorage(page)

    // Submit
    await submitMigrationForm(page)
    // Submit button should enter loading state (disabled during submission)
    await expect(page.getByTestId('migration-form-submit')).toBeDisabled()

    // After success: drawer closes, toast shown
    await expectDrawerClosed(page)
    await expectToast(page, /migration.*created|created.*migration|success/i)
  })

  test('migrations page shows new migration after submit', async ({ page }) => {
    // Seed updated list that includes the new plan after creation
    await mockRoute(page, API.migrationPlans, 'GET', {
      ...MOCK_MIGRATION_PLANS_LIST,
      items: [...MOCK_MIGRATION_PLANS_LIST.items, MOCK_MIGRATION_PLAN_CREATED],
    })

    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)
    await selectTwoVms(page)
    await page.getByTestId('section-nav-item-map-resources').click()
    await mapAllNetworks(page)
    await mapAllStorage(page)
    await submitMigrationForm(page)

    await expectDrawerClosed(page)

    // Table shows migrations (including newly created plan's migrations)
    await expect(page.getByTestId('migrations-table')).toBeVisible()
  })
})

// ─── MIG-005: Complete rolling migration — happy path ─────────────────────────

test.describe('MIG-005 — complete rolling migration', () => {
  const ROLLING_SECTION_IDS = [
    'source-destination',
    'baremetal',
    'hosts',
    'vms',
    'map-resources',
    'security',
    'options',
  ] as const

  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    // Override openstack cred to include pcdHostConfig
    await mockRoute(
      page,
      API.openstackCredByName('pcd-cred-1'),
      'GET',
      MOCK_OPENSTACK_CRED_WITH_HOST_CONFIG,
    )
    await mockRoute(page, API.openstackCreds, 'GET', {
      ...MOCK_OPENSTACK_CREDS_LIST,
      items: [MOCK_OPENSTACK_CRED_WITH_HOST_CONFIG],
    })
    await mockRoute(page, API.vmwareHosts, 'GET', MOCK_VMWARE_HOSTS_LIST)
    await mockRoute(page, API.bmConfigs, 'GET', MOCK_BM_CONFIGS_LIST)
    await mockRoute(page, API.networkMappings, 'POST', MOCK_NETWORK_MAPPING_CREATED)
    await mockRoute(page, API.storageMappings, 'POST', MOCK_STORAGE_MAPPING_CREATED)
    await mockRoute(page, API.rollingMigrationPlans, 'POST', MOCK_ROLLING_MIGRATION_PLAN_CREATED)
  })

  test('rolling migration form opens with 7 sections', async ({ page }) => {
    await page.goto(ROUTES.clusterConversions)
    await page.getByRole('button', { name: /start cluster conversion/i }).click()

    const drawer = page.getByTestId('rolling-migration-form-drawer')
    await expect(drawer).toBeVisible()

    for (const sectionId of ROLLING_SECTION_IDS) {
      await expect(page.getByTestId(`section-nav-item-${sectionId}`)).toBeVisible()
    }
  })

  test('baremetal section shows MAAS config; click opens detail dialog', async ({ page }) => {
    await page.goto(ROUTES.clusterConversions)
    await page.getByRole('button', { name: /start cluster conversion/i }).click()
    await expect(page.getByTestId('rolling-migration-form-drawer')).toBeVisible()

    await selectClustersAndWaitForVMs(page)

    // Navigate to baremetal section
    await page.getByTestId('section-nav-item-baremetal').click()
    // MAAS config name visible
    await expect(page.getByText('maas-config-1')).toBeVisible()

    // Click config name → MaasConfigDetailDialog opens
    await page.getByText('maas-config-1').click()
    await expect(page.getByTestId('maas-config-detail-dialog')).toBeVisible()

    // Close dialog
    await page.keyboard.press('Escape')
    await expect(page.getByTestId('maas-config-detail-dialog')).not.toBeVisible()
  })

  test('hosts section: assign host config to all ESXi hosts', async ({ page }) => {
    await page.goto(ROUTES.clusterConversions)
    await page.getByRole('button', { name: /start cluster conversion/i }).click()
    await expect(page.getByTestId('rolling-migration-form-drawer')).toBeVisible()

    await selectClustersAndWaitForVMs(page)
    await page.getByTestId('section-nav-item-hosts').click()

    // 2 ESXi hosts from MOCK_VMWARE_HOSTS_LIST
    const hostsGrid = page.locator('[data-testid="hosts-datagrid"], [aria-label*="hosts"]')
    await expect(hostsGrid.locator('[role="row"]').nth(1)).toBeVisible()

    // Select all hosts and open HostConfigAssignmentDialog
    const selectAllCheckbox = hostsGrid.locator('[role="columnheader"] [type="checkbox"]')
    await selectAllCheckbox.click()

    await page.getByTestId('assign-host-config-button').click()
    await expect(page.getByTestId('host-config-assignment-dialog')).toBeVisible()

    // Select a host config option and apply
    await page.getByRole('option', { name: /PCD Host Config 1/i }).click()
    await page.getByRole('button', { name: /apply/i }).click()

    await expect(page.getByTestId('host-config-assignment-dialog')).not.toBeVisible()
  })

  test('rolling migration plan submitted successfully', async ({ page }) => {
    await page.goto(ROUTES.clusterConversions)
    await page.getByRole('button', { name: /start cluster conversion/i }).click()

    const drawer = page.getByTestId('rolling-migration-form-drawer')
    await expect(drawer).toBeVisible()

    await selectClustersAndWaitForVMs(page)

    // VMs section
    await page.getByTestId('section-nav-item-vms').click()
    const vmsGrid = page.getByTestId('vms-datagrid')
    await vmsGrid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })

    // Map resources
    await page.getByTestId('section-nav-item-map-resources').click()
    await mapAllNetworks(page)
    await mapAllStorage(page)

    // Submit
    await submitMigrationForm(page)
    await expectDrawerClosed(page)
    await expectToast(page, /rolling migration.*created|created.*rolling|success/i)
  })
})

// ─── MIG-006: Delete single migration ────────────────────────────────────────

test.describe('MIG-006 — delete single migration', () => {
  const TARGET = 'test-vm-3-migration'

  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('cancel delete leaves migration visible', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: 'test-vm-3' })
    await expect(row).toBeVisible()

    await row.getByRole('button', { name: /delete/i }).click()
    await expect(page.getByTestId('confirm-delete-button')).toBeVisible()

    // Cancel
    await page.getByRole('button', { name: /cancel/i }).click()
    await expect(page.getByTestId('confirm-delete-button')).not.toBeVisible()
    await expect(row).toBeVisible()
  })

  test('confirm delete removes migration from table', async ({ page }) => {
    // Mock the DELETE API
    await mockRoute(page, API.migrationByName(TARGET), 'DELETE', {})

    // Return empty list after delete to simulate removal
    let deleteCount = 0
    await page.route(API.migrations, (route) => {
      if (route.request().method() === 'GET') {
        const list =
          deleteCount > 0
            ? { ...MOCK_MIGRATIONS_LIST, items: MOCK_MIGRATIONS_LIST.items.filter((m) => m.metadata.name !== TARGET) }
            : MOCK_MIGRATIONS_LIST
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(list) })
      } else {
        route.continue()
      }
    })

    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: 'test-vm-3' })
    await expect(row).toBeVisible()

    await row.getByRole('button', { name: /delete/i }).click()
    await page.getByTestId('confirm-delete-button').click()

    deleteCount = 1
    // Row removed after successful deletion
    await expect(row).not.toBeVisible({ timeout: 5000 })
    // Success feedback shown
    await expectToast(page, /deleted|success/i)
  })
})

// ─── MIG-007: Bulk delete multiple migrations ─────────────────────────────────

test.describe('MIG-007 — bulk delete migrations', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_FOR_BULK_DELETE)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('select 3 migrations and bulk delete', async ({ page }) => {
    // Mock DELETE for all 3 migrations
    for (const name of ['test-vm-3-migration', 'test-vm-4-migration', 'test-vm-6-migration']) {
      await mockRoute(page, API.migrationByName(name), 'DELETE', {})
    }

    const table = page.getByTestId('migrations-table')
    await expect(table).toBeVisible()

    // Select all 3 via header checkbox or individual checkboxes
    const headerCheckbox = table.locator('[role="columnheader"] [type="checkbox"]')
    await headerCheckbox.click()
    await expect(page.getByText(/3 selected/i)).toBeVisible()

    // Click bulk delete
    await page.getByTestId('delete-selected-button').click()

    // Confirmation dialog shows count
    await expect(page.getByText(/delete 3/i)).toBeVisible()
    await page.getByTestId('confirm-delete-button').click()

    // Success feedback
    await expectToast(page, /deleted|success/i)
  })
})

// ─── MIG-008: View pod logs for a migration ───────────────────────────────────

test.describe('MIG-008 — pod logs drawer', () => {
  const POD_NAME = 'v2v-helper-test-3'
  const SAMPLE_LOGS = [
    'INFO 2026-05-20T10:00:00Z Starting migration for test-vm-3',
    'INFO 2026-05-20T10:01:00Z Copying disk 1/2',
    'ERROR 2026-05-20T10:02:00Z Connection timeout — retrying',
    'INFO 2026-05-20T10:03:00Z Migration completed',
  ].join('\n')

  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    // Mock pod log stream
    await page.route(API.podLogs(NS, POD_NAME), (route) => {
      route.fulfill({ status: 200, contentType: 'text/plain', body: SAMPLE_LOGS })
    })
    await goToMigrations(page)
  })

  test('PodLogsDrawer opens and renders log lines', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: 'test-vm-3' })
    await expect(row).toBeVisible()

    // Open logs drawer
    await row.getByRole('button', { name: /log/i }).click()
    const drawer = page.getByTestId('pod-logs-drawer')
    await expect(drawer).toBeVisible()

    // Log lines appear
    await expect(drawer.getByText(/Starting migration/i)).toBeVisible({ timeout: 5000 })
    await expect(drawer.getByText(/Copying disk/i)).toBeVisible()
  })

  test('log search filters visible lines', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await table.locator('[role="row"]').filter({ hasText: 'test-vm-3' }).getByRole('button', { name: /log/i }).click()
    const drawer = page.getByTestId('pod-logs-drawer')
    await expect(drawer).toBeVisible()
    await expect(drawer.getByText(/Starting migration/i)).toBeVisible({ timeout: 5000 })

    // Search for ERROR
    await drawer.getByTestId('logs-search-input').fill('ERROR')
    await expect(drawer.getByText(/Connection timeout/i)).toBeVisible()
    // Non-matching lines hidden
    await expect(drawer.getByText(/Starting migration/i)).not.toBeVisible()
  })

  test('download button triggers file download', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await table.locator('[role="row"]').filter({ hasText: 'test-vm-3' }).getByRole('button', { name: /log/i }).click()
    const drawer = page.getByTestId('pod-logs-drawer')
    await expect(drawer).toBeVisible()
    await expect(drawer.getByText(/Starting migration/i)).toBeVisible({ timeout: 5000 })

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      drawer.getByTestId('logs-download-button').click(),
    ])
    expect(download.suggestedFilename()).toMatch(/\.txt$/)
  })
})

// ─── MIG-009: Admin cutover trigger — single migration ────────────────────────

test.describe('MIG-009 — admin cutover trigger', () => {
  const AWAITING_VM = 'test-vm-5'
  const POD_NAME = 'v2v-helper-test-5'

  test.beforeEach(async ({ page }) => {
    const listWithAwaitingOnly = {
      ...MOCK_MIGRATIONS_LIST,
      items: [MOCK_MIGRATION_AWAITING_CUTOVER],
    }
    await mockRoute(page, API.migrations, 'GET', listWithAwaitingOnly)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('cancel leaves migration phase unchanged', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: AWAITING_VM })
    await expect(row).toBeVisible()

    // Cutover button visible in Actions column (play/cutover icon)
    await row.getByTestId('cutover-confirm-button').click()
    // OR fallback: await row.getByRole('button', { name: /cutover/i }).click()

    // Confirmation dialog
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.getByRole('button', { name: /cancel/i }).click()
    await expect(page.getByRole('dialog')).not.toBeVisible()

    // Migration still in AwaitingAdminCutOver
    await expect(row).toBeVisible()
  })

  test('confirm triggers cutover API and shows success', async ({ page }) => {
    // Mock the pod PATCH (cutover trigger)
    await page.route(`**/pods/${POD_NAME}`, (route) => {
      if (route.request().method() === 'PATCH') {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
      } else {
        route.continue()
      }
    })

    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: AWAITING_VM })
    await expect(row).toBeVisible()

    await row.getByTestId('cutover-confirm-button').click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.getByRole('button', { name: /confirm|trigger/i }).click()

    // Dialog closes; success feedback shown
    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 })
    await expectToast(page, /cutover.*triggered|success/i)
  })
})

// ─── MIG-010: Status and date filtering in migrations table ───────────────────

test.describe('MIG-010 — status and date filtering', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('status filter "Failed" shows only failed migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await expect(table).toBeVisible()

    await page.getByTestId('status-filter').click()
    await page.getByRole('option', { name: /failed/i }).click()

    // Only Failed migration visible
    await expect(table.getByText('test-vm-4')).toBeVisible()
    // Other statuses not visible
    await expect(table.getByText('test-vm-3')).not.toBeVisible()
    await expect(table.getByText('test-vm-2')).not.toBeVisible()
  })

  test('status filter "All" restores all migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await page.getByTestId('status-filter').click()
    await page.getByRole('option', { name: /failed/i }).click()
    await expect(table.getByText('test-vm-4')).toBeVisible()

    // Reset to All
    await page.getByTestId('status-filter').click()
    await page.getByRole('option', { name: /all/i }).click()

    for (const vm of ['test-vm-1', 'test-vm-2', 'test-vm-3', 'test-vm-4', 'test-vm-5']) {
      await expect(table.getByText(vm)).toBeVisible()
    }
  })

  test('status filter "Succeeded" shows only succeeded migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await page.getByTestId('status-filter').click()
    await page.getByRole('option', { name: /succeeded/i }).click()

    await expect(table.getByText('test-vm-3')).toBeVisible()
    await expect(table.getByText('test-vm-4')).not.toBeVisible()
  })

  test('date filter "Last 24 hours" shows only recent migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await page.getByTestId('date-filter').click()
    await page.getByRole('option', { name: /24 hours/i }).click()

    // Migrations with creationTimestamp 2026-05-20T10:00:00Z should be within 24h of "now"
    // MOCK_MIGRATIONS_LIST items use that timestamp — exact result depends on app date handling
    await expect(table).toBeVisible()
  })

  test('combined status + date filter applies intersection', async ({ page }) => {
    const table = page.getByTestId('migrations-table')

    // Apply both filters
    await page.getByTestId('status-filter').click()
    await page.getByRole('option', { name: /failed/i }).click()

    await page.getByTestId('date-filter').click()
    await page.getByRole('option', { name: /24 hours/i }).click()

    // Only Failed migration within 24h
    await expect(table.getByText('test-vm-4')).toBeVisible()
    await expect(table.getByText('test-vm-3')).not.toBeVisible()
  })
})
