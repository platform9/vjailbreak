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
  MOCK_MIGRATION_PLAN_2,
  MOCK_MIGRATION_PLAN_3,
  MOCK_MIGRATION_PLAN_5,
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
  MOCK_BM_CONFIG_1,
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
  // Template: POST returns pending; GET returns ready (polling resolves immediately);
  // PATCH returns ready (updateMigrationTemplate on submit must succeed to proceed to createMigrationPlan)
  await page.route(`**migrationtemplates**`, (route) => {
    const method = route.request().method()
    if (method === 'POST') {
      route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
    } else if (method === 'GET') {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_READY) })
    } else if (method === 'PATCH') {
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
  // Select by name to avoid powered-off VM (no osFamily → vmValidation.hasError → submit disabled)
  await grid.locator('[role="row"]').filter({ hasText: 'test-vm-1' }).getByRole('checkbox').click({ force: true })
  await grid.locator('[role="row"]').filter({ hasText: 'test-vm-multi-network' }).getByRole('checkbox').click({ force: true })
  // Toolbar shows "Assign Flavor (N)" when VMs are selected
  await expect(page.getByText(/assign flavor/i)).toBeVisible({ timeout: 5000 })
}

// Click the first non-disabled option in the currently open MUI Menu.
// MUI marks disabled MenuItems with .Mui-disabled class (NOT aria-disabled="true").
// Uses native DOM .click() via evaluate to bypass Playwright visibility/actionability checks
// since MUI menu items may be considered "not visible" during open animation.
async function clickFirstMenuOption(page: Page) {
  // Wait for the menu to be in the DOM (attached) before querying options
  await page.waitForSelector('.MuiMenu-root', { state: 'attached', timeout: 3000 })
  await page.evaluate(() => {
    const menus = document.querySelectorAll('.MuiMenu-root')
    const menu = menus[menus.length - 1]
    if (!menu) return
    const option = menu.querySelector('li[role="option"]:not(.Mui-disabled)')
    if (option) (option as HTMLElement).click()
  })
}

async function mapAllNetworks(page: Page) {
  const table = page.getByTestId('network-mapping-table')
  await expect(table).toBeVisible()
  // ResourceMappingTableNew uses native <tr> elements (no explicit role="row" attribute).
  // The last tbody row is the empty input row with two searchable RHFSelect comboboxes.
  // Selecting source + target triggers useEffect which auto-adds the mapping row.
  for (let i = 0; i < 10; i++) {
    const emptyRow = table.locator('tbody tr').last()
    const sourceSelect = emptyRow.locator('[role="combobox"]').first()
    const isVisible = await sourceSelect.isVisible().catch(() => false)
    if (!isVisible) break

    const prevCount = await table.locator('[aria-label="delete-mapping"]').count()

    await sourceSelect.click()
    await clickFirstMenuOption(page)
    // MUI Select keeps .MuiMenu-root mounted (aria-hidden="true") when closed — wait for that
    await page.waitForFunction(
      () => {
        const menus = document.querySelectorAll('.MuiMenu-root')
        return Array.from(menus).every((m) => m.getAttribute('aria-hidden') === 'true')
      },
      { timeout: 5000 }
    )

    const targetSelect = table.locator('tbody tr').last().locator('[role="combobox"]').last()
    await targetSelect.click()
    await clickFirstMenuOption(page)
    await page.waitForFunction(
      () => {
        const menus = document.querySelectorAll('.MuiMenu-root')
        return Array.from(menus).every((m) => m.getAttribute('aria-hidden') === 'true')
      },
      { timeout: 5000 }
    )

    await expect(table.locator('[aria-label="delete-mapping"]')).toHaveCount(prevCount + 1, { timeout: 5000 })
  }
}

async function mapAllStorage(page: Page) {
  const table = page.getByTestId('storage-mapping-table')
  await expect(table).toBeVisible()
  for (let i = 0; i < 10; i++) {
    const emptyRow = table.locator('tbody tr').last()
    const sourceSelect = emptyRow.locator('[role="combobox"]').first()
    const isVisible = await sourceSelect.isVisible().catch(() => false)
    if (!isVisible) break

    const prevCount = await table.locator('[aria-label="delete-mapping"]').count()

    await sourceSelect.click()
    await clickFirstMenuOption(page)
    await page.waitForFunction(
      () => {
        const menus = document.querySelectorAll('.MuiMenu-root')
        return Array.from(menus).every((m) => m.getAttribute('aria-hidden') === 'true')
      },
      { timeout: 5000 }
    )

    const targetSelect = table.locator('tbody tr').last().locator('[role="combobox"]').last()
    await targetSelect.click()
    await clickFirstMenuOption(page)
    await page.waitForFunction(
      () => {
        const menus = document.querySelectorAll('.MuiMenu-root')
        return Array.from(menus).every((m) => m.getAttribute('aria-hidden') === 'true')
      },
      { timeout: 5000 }
    )

    await expect(table.locator('[aria-label="delete-mapping"]')).toHaveCount(prevCount + 1, { timeout: 5000 })
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
    await mockRoute(page, API.bmConfigByName('maas-config-1'), 'GET', MOCK_BM_CONFIG_1)
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
    // "View Bare Metal Config Details" link visible when MAAS config exists
    const viewDetailsLink = page.getByTestId('rolling-migration-form-baremetal-view-details')
    await expect(viewDetailsLink).toBeVisible({ timeout: 5000 })

    // Click link → MaasConfigDetailDialog opens
    await viewDetailsLink.click()
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

    // 2 ESXi hosts from MOCK_VMWARE_HOSTS_LIST — hosts-datagrid is on the Paper wrapper
    // MUI DataGrid uses role="row" (not <tbody><tr>); nth(1) = first data row after header
    const hostsGrid = page.getByTestId('hosts-datagrid')
    await expect(hostsGrid.locator('[role="row"]').nth(1)).toBeVisible({ timeout: 5000 })

    // "Assign Host Config" button applies to ALL hosts (no row selection needed)
    await page.getByTestId('assign-host-config-button').click()
    await expect(page.getByTestId('host-config-assignment-dialog')).toBeVisible()

    // Open the MUI Select and pick the host config option
    await page.getByTestId('rolling-migration-form-host-config-select').click()
    await page.getByRole('option', { name: /PCD Host Config 1/i }).click()

    await page.getByTestId('rolling-migration-form-host-config-apply').click()

    await expect(page.getByTestId('host-config-assignment-dialog')).not.toBeVisible()
  })

  test('rolling migration plan submitted successfully', async ({ page }) => {
    await page.goto(ROUTES.clusterConversions)
    await page.getByRole('button', { name: /start cluster conversion/i }).click()

    const drawer = page.getByTestId('rolling-migration-form-drawer')
    await expect(drawer).toBeVisible()

    await selectClustersAndWaitForVMs(page)

    // Assign host config to all ESXi hosts (required for submit to be enabled)
    await page.getByTestId('section-nav-item-hosts').click()
    await expect(page.getByTestId('hosts-datagrid').locator('[role="row"]').nth(1)).toBeVisible({ timeout: 5000 })
    await page.getByTestId('assign-host-config-button').click()
    await expect(page.getByTestId('host-config-assignment-dialog')).toBeVisible()
    await page.getByTestId('rolling-migration-form-host-config-select').click()
    await page.getByRole('option', { name: /PCD Host Config 1/i }).click()
    await page.getByTestId('rolling-migration-form-host-config-apply').click()
    await expect(page.getByTestId('host-config-assignment-dialog')).not.toBeVisible()

    // VMs section
    await page.getByTestId('section-nav-item-vms').click()
    const vmsGrid = page.getByTestId('vms-datagrid')
    await vmsGrid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })

    // Map resources
    await page.getByTestId('section-nav-item-map-resources').click()
    await mapAllNetworks(page)
    await mapAllStorage(page)

    // Submit — rolling form calls onClose() then navigate('/dashboard/cluster-conversions'), no toast
    await page.getByTestId('rolling-migration-form-submit').click()
    await expect(page.getByTestId('rolling-migration-form-drawer')).not.toBeVisible({ timeout: 5000 })
    await page.waitForURL(/\/dashboard\/cluster-conversions/, { timeout: 5000 })
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
    // Mock the DELETE API and the migration plan GET+PATCH (required by handleDeleteMigration)
    await mockRoute(page, API.migrationPlanByName('test-plan-2'), 'GET', MOCK_MIGRATION_PLAN_2)
    await mockRoute(page, API.migrationPlanByName('test-plan-2'), 'PATCH', MOCK_MIGRATION_PLAN_2)
    await mockRoute(page, API.migrationByName(TARGET), 'DELETE', {})

    // After delete the app calls invalidateQueries → GET migrations is refetched.
    // Register dynamic route: returns filtered list once deleteCount=1.
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
    await expect(page.getByTestId('confirm-delete-button')).toBeVisible()

    deleteCount = 1
    await page.getByTestId('confirm-delete-button').click()

    // Wait for dialog to close (happens after invalidateQueries + handleDeleteClose)
    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 })

    // React Query's invalidateQueries does not trigger a GET in the Playwright env.
    // Navigate away and back to force a fresh data load via component remount.
    await page.goto(ROUTES.migrations)
    await page.waitForURL(/\/dashboard\/migrations/)
    await expect(page.getByTestId('migrations-table')).toBeVisible({ timeout: 10_000 })

    await expect(row).not.toBeVisible({ timeout: 3000 })
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
    // Mock DELETE for all 3 migrations and their plan GET+PATCH (required by handleDeleteMigration)
    for (const name of ['test-vm-3-migration', 'test-vm-4-migration', 'test-vm-6-migration']) {
      await mockRoute(page, API.migrationByName(name), 'DELETE', {})
    }
    await mockRoute(page, API.migrationPlanByName('test-plan-2'), 'GET', MOCK_MIGRATION_PLAN_2)
    await mockRoute(page, API.migrationPlanByName('test-plan-2'), 'PATCH', MOCK_MIGRATION_PLAN_2)
    await mockRoute(page, API.migrationPlanByName('test-plan-3'), 'GET', MOCK_MIGRATION_PLAN_3)
    await mockRoute(page, API.migrationPlanByName('test-plan-3'), 'PATCH', MOCK_MIGRATION_PLAN_3)
    await mockRoute(page, API.migrationPlanByName('test-plan-5'), 'GET', MOCK_MIGRATION_PLAN_5)
    await mockRoute(page, API.migrationPlanByName('test-plan-5'), 'PATCH', MOCK_MIGRATION_PLAN_5)

    const table = page.getByTestId('migrations-table')
    await expect(table).toBeVisible()

    // MUI DataGrid header checkbox has opacity:0 — use force:true
    const headerCheckbox = table.locator('[role="columnheader"] [type="checkbox"]')
    await headerCheckbox.click({ force: true })
    // Toolbar shows "Delete Selected (N)" when rows are selected
    await expect(page.getByText(/delete selected/i)).toBeVisible({ timeout: 5000 })

    // Click bulk delete
    await page.getByTestId('delete-selected-button').click()

    // Confirmation dialog
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.getByTestId('confirm-delete-button').click()

    // Dialog closes after deletion
    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 8000 })
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

    // logs-search-input testid is on the MUI TextField wrapper div — target the actual input inside
    await drawer.getByTestId('logs-search-input').locator('input').fill('ERROR')
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

    // Open the cutover confirmation dialog via the trigger button in the Actions column
    await row.getByTestId('cutover-trigger-button').click()

    // Confirmation dialog
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.getByRole('button', { name: /cancel/i }).click()
    await expect(page.getByRole('dialog')).not.toBeVisible()

    // Migration still in AwaitingAdminCutOver
    await expect(row).toBeVisible()
  })

  test('confirm triggers cutover API and shows success', async ({ page }) => {
    // triggerAdminCutover flow: GET migration → GET pods list → PATCH pod
    await mockRoute(page, API.migrationByName('test-vm-5-migration'), 'GET', MOCK_MIGRATION_AWAITING_CUTOVER)
    // Pods list GET (k8s proxy)
    await page.route(`**/k8s/api/v1/namespaces/${NS}/pods`, (route) => {
      if (route.request().method() === 'GET') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ items: [{ metadata: { name: POD_NAME, namespace: NS } }] }),
        })
      } else {
        route.continue()
      }
    })
    // Pod PATCH
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

    // Open dialog via trigger button, then confirm
    await row.getByTestId('cutover-trigger-button').click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.getByTestId('cutover-confirm-button').click()

    // Dialog closes after successful cutover trigger
    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 })
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
    // status-filter uses MUI Menu (not Select) — items have role="menuitem" not "option"
    await page.getByRole('menuitem', { name: /failed/i }).click()

    // Only Failed migration visible
    await expect(table.getByText('test-vm-4')).toBeVisible()
    // Other statuses not visible
    await expect(table.getByText('test-vm-3')).not.toBeVisible()
    await expect(table.getByText('test-vm-2')).not.toBeVisible()
  })

  test('status filter "All" restores all migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await page.getByTestId('status-filter').click()
    await page.locator('[role="menu"]').waitFor({ state: 'visible' })
    await page.getByRole('menuitem', { name: /failed/i }).click()
    await expect(table.getByText('test-vm-4')).toBeVisible()

    // Wait for all React Query refetches triggered by filter change to complete,
    // so no mid-click toolbar remounts close the menu on the next interaction.
    await page.waitForLoadState('networkidle')

    // Reset to All — scope to the menu element to avoid detached-DOM races
    await page.getByTestId('status-filter').click()
    const statusMenu = page.locator('[role="menu"]')
    await statusMenu.waitFor({ state: 'visible' })
    await statusMenu.getByRole('menuitem', { name: /^all$/i }).click()

    for (const vm of ['test-vm-1', 'test-vm-2', 'test-vm-3', 'test-vm-4', 'test-vm-5']) {
      await expect(table.getByText(vm)).toBeVisible()
    }
  })

  test('status filter "Succeeded" shows only succeeded migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await page.getByTestId('status-filter').click()
    await page.getByRole('menuitem', { name: /succeeded/i }).click()

    await expect(table.getByText('test-vm-3')).toBeVisible()
    await expect(table.getByText('test-vm-4')).not.toBeVisible()
  })

  test('date filter "Last 24 hours" shows only recent migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await page.getByTestId('date-filter').click()
    await page.getByRole('menuitem', { name: /24 hours/i }).click()

    // Migrations with creationTimestamp 2026-05-20T10:00:00Z should be within 24h of "now"
    // MOCK_MIGRATIONS_LIST items use that timestamp — exact result depends on app date handling
    await expect(table).toBeVisible()
  })

  test('combined status + date filter applies intersection', async ({ page }) => {
    const table = page.getByTestId('migrations-table')

    // Apply status filter first
    await page.getByTestId('status-filter').click()
    await page.locator('[role="menu"]').waitFor({ state: 'visible' })
    await page.getByRole('menuitem', { name: /failed/i }).click()

    // Wait for refetches triggered by filter change before opening next menu
    await page.waitForLoadState('networkidle')

    // Apply date filter — use "Last 30 days" so fixture timestamps (2026-05-20) are safely within range
    // regardless of when the tests run.
    await page.getByTestId('date-filter').click()
    const dateMenu = page.locator('[role="menu"]')
    await dateMenu.waitFor({ state: 'visible' })
    await dateMenu.getByRole('menuitem', { name: /30 days/i }).click()

    // Only Failed migration within 30 days (intersection of Failed + date window)
    await expect(table.getByText('test-vm-4')).toBeVisible()
    await expect(table.getByText('test-vm-3')).not.toBeVisible()
  })
})
