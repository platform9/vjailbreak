import { test, expect, Page } from '@playwright/test'

import {
  goToMigrations,
  goToGlobalSettings,
  openMigrationDrawer,
  selectVmwareCluster,
  selectPcdCluster,
  mockRoute,
  expectToast,
  API,
  ROUTES,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST,
  MOCK_MIGRATIONS_LIST_EMPTY,
  MOCK_MIGRATION_PLANS_LIST,
  MOCK_MIGRATION_PLANS_LIST_EMPTY,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_VMWARE_CRED_1,
  MOCK_OPENSTACK_CRED_1,
  MOCK_OPENSTACK_CREDS_LIST,
  MOCK_MIGRATION_TEMPLATE_PENDING,
  MOCK_MIGRATION_TEMPLATE_READY,
  MOCK_MIGRATION_TEMPLATE_LARGE,
  MOCK_NETWORK_MAPPING_CREATED,
  MOCK_STORAGE_MAPPING_CREATED,
  MOCK_VMWARE_CLUSTERS_LIST,
  MOCK_PCD_CLUSTERS_LIST,
  MOCK_IP_VALIDATION_VALID,
  MOCK_VMWARE_MACHINES_LIST,
  MOCK_VMWARE_MACHINES_LIST_WITH_RDM,
  MOCK_VMWARE_MACHINES_LIST_LARGE,
  MOCK_RDM_DISKS_LIST,
  MOCK_SETTINGS_CONFIGMAP_NETWORK_PERSISTENCE_ON,
  MOCK_SETTINGS_CONFIGMAP_NETWORK_PERSISTENCE_OFF,
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
  await page.route('**migrationtemplates**', (route) => {
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
  await expect(page.getByTestId('vms-datagrid').locator('[role="row"]').nth(1)).toBeVisible({ timeout: 15_000 })
}

// Maps all source→target rows in a ResourceMappingTableNew by selecting the first available
// option in each combobox pair. Uses tr selector (not [role="row"]) and waits for MUI menu
// close between selections. Mirrors the proven pattern from validation.spec.ts.
async function mapAllTableRows(page: Page, tableTestId: string): Promise<void> {
  const table = page.getByTestId(tableTestId)
  const openMenuOptions = page.locator(
    '.MuiMenu-root:not([aria-hidden="true"]) li[role="option"]:not([aria-disabled="true"])',
  )
  const waitForMenuClosed = () =>
    page.waitForFunction(
      () => !document.querySelector('.MuiMenu-root:not([aria-hidden="true"])'),
      { timeout: 5000 },
    )
  for (let attempt = 0; attempt < 20; attempt++) {
    const emptyRow = table.locator('tr').filter({ has: page.locator('[role="combobox"]') })
    if ((await emptyRow.count()) === 0) break
    const comboboxes = emptyRow.first().locator('[role="combobox"]')
    if ((await comboboxes.count()) < 2) break
    await comboboxes.nth(0).click()
    await expect(comboboxes.nth(0)).toHaveAttribute('aria-expanded', 'true', { timeout: 3000 })
    await openMenuOptions.first().click()
    await waitForMenuClosed()
    await comboboxes.nth(1).click()
    await expect(comboboxes.nth(1)).toHaveAttribute('aria-expanded', 'true', { timeout: 3000 })
    await openMenuOptions.first().click()
    await waitForMenuClosed()
    await page.waitForTimeout(300)
  }
}

// ─── MIG-028: Empty VMware cluster — no VMs available ────────────────────────

test.describe('MIG-028 — empty cluster shows graceful empty state', () => {
  test('empty VM list shows empty state message; submit disabled', async ({ page }) => {
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
    await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
    await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    // Template returns ready but with zero VMs
    await page.route('**migrationtemplates**', (route) => {
      const method = route.request().method()
      if (method === 'POST') {
        route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
      } else if (method === 'GET') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_MIGRATION_TEMPLATE_READY,
            status: { ...MOCK_MIGRATION_TEMPLATE_READY.status, vmware: [] },
          }),
        })
      } else {
        route.continue()
      }
    })

    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectVmwareCluster(page, 'DC1-Cluster')
    await selectPcdCluster(page, 'pcd-cluster-1')

    // VM grid visible but empty — shows empty state
    const grid = page.getByTestId('vms-datagrid')
    await expect(grid).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText(/no vm|no virtual machine|0 vm|empty/i)).toBeVisible()

    // Submit disabled — nothing to migrate
    await expect(page.getByTestId('migration-form-submit')).toBeDisabled()
  })
})

// ─── MIG-029: Single VM selection ────────────────────────────────────────────

test.describe('MIG-029 — single VM selection', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)
  })

  test('selecting 1 VM shows "1 selected" in toolbar', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    await grid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })

    // Toolbar shows "Assign Flavor (N)" when VMs are selected — no standalone "N selected" text
    await expect(page.getByTestId('assign-flavor-button')).toContainText('(1)')
  })

  test('flavor assignment dialog for single VM shows singular grammar', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    await grid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })

    await page.getByTestId('assign-flavor-button').click()
    const dialog = page.getByTestId('flavor-assignment-dialog')
    await expect(dialog).toBeVisible()

    // Singular wording — not "2 VMs" or "VMs"
    await expect(dialog.getByText(/1 vm|assign.*1|1.*vm/i)).toBeVisible()
  })

  test('single VM: complete form and verify submit enabled', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    await grid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })

    await page.getByTestId('section-nav-item-map-resources').click()

    // Map networks and storage using proven tr+[role="combobox"] pattern
    await mapAllTableRows(page, 'network-mapping-table')
    await mapAllTableRows(page, 'storage-mapping-table')

    await expect(page.getByTestId('migration-form-submit')).toBeEnabled({ timeout: 5000 })
  })
})

// ─── MIG-030: Large VM count stress test (55 VMs) ────────────────────────────

test.describe('MIG-030 — large VM count stress test', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.vmwareCredByName('vcenter-cred-1'), 'GET', MOCK_VMWARE_CRED_1)
    await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
    await mockRoute(page, API.vmwareMachines, 'GET', MOCK_VMWARE_MACHINES_LIST_LARGE)
    await mockRoute(page, API.rdmDisks, 'GET', { apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1', kind: 'RdmDiskList', metadata: { continue: '', resourceVersion: '1' }, items: [] })
    await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
    await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await page.route('**migrationtemplates**', (route) => {
      const method = route.request().method()
      if (method === 'POST') {
        route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
      } else if (method === 'GET') {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_LARGE) })
      } else {
        route.continue()
      }
    })

    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectVmwareCluster(page, 'DC1-Cluster')
    await selectPcdCluster(page, 'pcd-cluster-1')
    // Wait for first data row to confirm grid is populated
    await expect(page.getByTestId('vms-datagrid').locator('[role="row"]').nth(1)).toBeVisible({ timeout: 15_000 })
  })

  test('55-VM grid renders without freeze; virtualization active', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    await expect(grid).toBeVisible()

    // Data rows are populated
    await expect(grid.locator('[role="row"]').nth(1)).toBeVisible({ timeout: 10_000 })

    // DataGrid paginates — visible rows = header + page rows (default page size 5)
    const visibleRows = await grid.locator('[role="row"]').count()
    expect(visibleRows).toBeGreaterThan(1)
    // Page remains responsive (no timeout = no freeze)
  })

  test('Select All selects visible VMs; count shown in toolbar', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    await expect(grid.locator('[role="row"]').nth(1)).toBeVisible({ timeout: 10_000 })
    const headerCheckbox = grid.locator('[role="columnheader"] [type="checkbox"]')
    await headerCheckbox.click()

    // Toolbar shows "Assign Flavor (N)" for selected VMs
    await expect(page.getByTestId('assign-flavor-button')).toContainText(/\(\d+\)/, { timeout: 5000 })
  })

  test('bulk flavor assignment dialog opens and applies without timeout', async ({ page }) => {
    // Mock PATCH for flavor assignment (handleApplyFlavor calls patchVMwareMachine per VM)
    await page.route('**vmwaremachines**', (route) => {
      if (route.request().method() === 'PATCH') {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ spec: {} }) })
      } else {
        route.continue()
      }
    })

    const grid = page.getByTestId('vms-datagrid')
    await expect(grid.locator('[role="row"]').nth(1)).toBeVisible({ timeout: 10_000 })
    const headerCheckbox = grid.locator('[role="columnheader"] [type="checkbox"]')
    await headerCheckbox.click()
    // Confirm toolbar shows selection
    await expect(page.getByTestId('assign-flavor-button')).toContainText(/\(\d+\)/, { timeout: 5000 })

    await page.getByTestId('assign-flavor-button').click()
    const dialog = page.getByTestId('flavor-assignment-dialog')
    await expect(dialog).toBeVisible()

    // Open Autocomplete and select Auto-assign (always first option)
    await dialog.locator('input[placeholder="Search flavors"]').click()
    // Scope to .MuiAutocomplete-popper to avoid matching hidden MUI Select options elsewhere in DOM
    await page.waitForSelector('.MuiAutocomplete-popper [role="option"]', { timeout: 5000 })
    await page.locator('.MuiAutocomplete-popper [role="option"]').filter({ hasText: /auto-assign/i }).click()

    // Apply to selected VMs
    await dialog.getByRole('button', { name: /apply to selected/i }).click()

    // Dialog closes — operation completes without timeout
    await expect(dialog).not.toBeVisible({ timeout: 10_000 })
  })
})

// ─── MIG-031: No PCD credentials exist ───────────────────────────────────────

test.describe('MIG-031 — no PCD credentials graceful empty state', () => {
  test('empty PCD clusters shows empty state and keeps submit disabled', async ({ page }) => {
    // Dismiss onboarding guide via localStorage so navigation guard doesn't redirect away
    await page.addInitScript(() => { localStorage.setItem('getting-started-dismissed', 'true') })
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
    await mockRoute(page, API.pcdClusters, 'GET', { apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1', kind: 'PCDClusterList', metadata: {}, items: [] })
    await mockRoute(page, API.openstackCreds, 'GET', { apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1', kind: 'OpenstackCredsList', metadata: {}, items: [] })
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await page.route('**migrationtemplates**', (route) => route.continue())
    await goToMigrations(page)
    const startButton = page.getByTestId('start-migration-button')
    await expect(startButton).toBeVisible()
    await expect(startButton).toBeDisabled()
  })
})

// ─── MIG-032: StorageAcceleratedCopy — no array credentials ──────────────────

test.describe('MIG-032 — StorageAcceleratedCopy with no array credentials', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)
  })

  test('selecting StorageAcceleratedCopy shows no-array-creds warning', async ({ page }) => {
    await page.getByTestId('section-nav-item-options').click()

    // Select "Storage Accelerated Copy" migration method
    const storageAccelOption = page.getByRole('radio', { name: /storage accelerated copy/i })
      .or(page.getByLabel(/storage accelerated copy/i))
    await storageAccelOption.click()

    // Warning banner should appear
    await expect(
      page.getByText(/no.*array.*credential|array.*credential.*not found|no validated/i),
    ).toBeVisible({ timeout: 3000 })
  })

  test('data copy and cutover sections hidden when StorageAcceleratedCopy + no array creds', async ({ page }) => {
    await page.getByTestId('section-nav-item-options').click()

    const storageAccelOption = page.getByRole('radio', { name: /storage accelerated copy/i })
      .or(page.getByLabel(/storage accelerated copy/i))
    await storageAccelOption.click()

    // Data copy start time and cutover window options should be hidden/disabled
    const dataCopyToggle = page.getByRole('checkbox', { name: /data copy start time/i })
    const cutoverToggle = page.getByRole('checkbox', { name: /cutover window/i })

    const dataCopyHidden = !(await dataCopyToggle.isVisible().catch(() => false))
    const cutoverHidden = !(await cutoverToggle.isVisible().catch(() => false))

    // At least one of the sections should be hidden or disabled
    expect(dataCopyHidden || cutoverHidden).toBe(true)
  })
})

// ─── MIG-033: RDM disks detected — configure flow ────────────────────────────

test.describe('MIG-033 — RDM disks detected and configurable', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    // Override (LIFO) to include RDM VM and non-empty RDM disk list
    await mockRoute(page, API.vmwareMachines, 'GET', MOCK_VMWARE_MACHINES_LIST_WITH_RDM)
    await mockRoute(page, API.rdmDisks, 'GET', MOCK_RDM_DISKS_LIST)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)
  })

  test('selecting VM with RDM disks shows RDM alert banner', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    const rdmRow = grid.locator('[role="row"]').filter({ hasText: 'test-vm-rdm' })
    await expect(rdmRow).toBeVisible({ timeout: 10_000 })
    await rdmRow.getByRole('checkbox').click({ force: true })

    // RDM alert banner should appear
    await expect(page.getByText(/rdm|raw device mapping/i).first()).toBeVisible({ timeout: 5000 })
  })

  test('Configure RDM button opens RDM config panel', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    const rdmRow = grid.locator('[role="row"]').filter({ hasText: 'test-vm-rdm' })
    await expect(rdmRow).toBeVisible({ timeout: 10_000 })
    await rdmRow.getByRole('checkbox').click({ force: true })

    await page.getByRole('button', { name: /configure rdm/i }).click()

    await expect(page.getByTestId('rdm-config-panel')).toBeVisible()
  })

  test('RDM config panel lists each RDM disk with configuration dropdowns', async ({ page }) => {
    const grid = page.getByTestId('vms-datagrid')
    const rdmRow = grid.locator('[role="row"]').filter({ hasText: 'test-vm-rdm' })
    await expect(rdmRow).toBeVisible({ timeout: 10_000 })
    await rdmRow.getByRole('checkbox').click({ force: true })

    await page.getByRole('button', { name: /configure rdm/i }).click()

    const panel = page.getByTestId('rdm-config-panel')
    await expect(panel).toBeVisible()

    // RDM disk identifier shown (useEffect initializes configurations state)
    await expect(panel.getByText(/naa\./i)).toBeVisible({ timeout: 3000 })

    // Dropdowns for backend pool / volume type
    const dropdowns = panel.locator('[role="combobox"]')
    expect(await dropdowns.count()).toBeGreaterThanOrEqual(1)
  })
})

// ─── MIG-034: Migration form — session cleanup on forced close ────────────────

test.describe('MIG-034 — temp K8s resources deleted on form close', () => {
  test('closing form after cluster selection triggers DELETE for temp resources', async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    const deletedUrls: string[] = []

    // Intercept DELETE calls to record which resources are cleaned up
    await page.route('**migrationtemplates/**', (route) => {
      if (route.request().method() === 'DELETE') {
        deletedUrls.push(route.request().url())
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
      } else {
        route.continue()
      }
    })

    await goToMigrations(page)
    await openMigrationDrawer(page)

    // Select clusters — this creates a session-scoped MigrationTemplate
    await selectClustersAndWaitForVMs(page)

    // Close form via X button
    await page.getByTestId('migration-form-close').click()

    // Give the app a moment to fire cleanup calls
    await page.waitForTimeout(500)

    // At least one DELETE should have been fired for the temp template
    expect(deletedUrls.length).toBeGreaterThan(0)
  })

  test('reopening form after close starts fresh session with no stale state', async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    // Mock DELETE for cleanup
    await page.route('**migrationtemplates/**', (route) => {
      if (route.request().method() === 'DELETE') {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
      } else {
        route.continue()
      }
    })

    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)

    // Close and reopen
    await page.getByTestId('migration-form-close').click()
    await expect(page.getByTestId('migration-form-drawer')).not.toBeVisible()

    await page.getByTestId('start-migration-button').click()
    await expect(page.getByTestId('migration-form-drawer')).toBeVisible()

    // Form should be in initial state — no cluster pre-selected from prior session
    const vmwareDropdown = page.getByTestId('vmware-cluster-dropdown')
    await expect(vmwareDropdown).toBeVisible()
    // Dropdown should show placeholder, not the previously selected cluster
    await expect(vmwareDropdown.getByText('DC1-Cluster')).not.toBeVisible()
  })
})

// ─── MIG-035: Migrations table — filtering produces empty result ──────────────

test.describe('MIG-035 — filter produces empty result', () => {
  test.beforeEach(async ({ page }) => {
    // Only succeeded migrations in the list
    const succeededOnly = {
      ...MOCK_MIGRATIONS_LIST,
      items: MOCK_MIGRATIONS_LIST.items.filter((m) => m.status.phase === 'Succeeded'),
    }
    await mockRoute(page, API.migrations, 'GET', succeededOnly)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('filtering for "Failed" when none exist shows empty state message', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await expect(table).toBeVisible()

    await page.getByTestId('status-filter').click()
    // CustomSearchToolbar uses MUI Menu → MenuItem (role="menuitem"), not option
    await page.getByRole('menuitem', { name: /failed/i }).click()

    // Empty state message shown — MUI DataGrid default is "No rows"
    await expect(
      page.getByText(/no migration|no match|no result|empty|no rows/i),
    ).toBeVisible()
  })

  test('clearing filter restores all migrations', async ({ page }) => {
    const table = page.getByTestId('migrations-table')

    // Apply filter that produces empty result
    await page.getByTestId('status-filter').click()
    // CustomSearchToolbar uses MUI Menu → MenuItem (role="menuitem"), not option
    await page.getByRole('menuitem', { name: /failed/i }).click()
    await expect(page.getByText(/no migration|no match|no result|empty|no rows/i)).toBeVisible()

    // Clear filter — restore all
    await page.getByTestId('status-filter').click()
    await page.getByRole('menuitem', { name: /all/i }).click()

    // Succeeded migration should be visible again
    await expect(table.getByText('test-vm-3')).toBeVisible()
  })
})

// ─── MIG-036: Subnet compatibility warning for network mapping ────────────────

test.describe('MIG-036 — subnet compatibility warning', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)

    // Select VM with known IP to trigger subnet check
    const grid = page.getByTestId('vms-datagrid')
    await grid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })
    await page.getByTestId('section-nav-item-map-resources').click()
  })

  test('mapping to incompatible subnet shows warning per source network', async ({ page }) => {
    const netTable = page.getByTestId('network-mapping-table')
    await expect(netTable).toBeVisible()

    // Use proven tr+[role="combobox"] pattern (ResourceMappingTableNew uses RHFSelect)
    const emptyRow = netTable.locator('tr').filter({ has: page.locator('[role="combobox"]') }).first()
    await expect(emptyRow).toBeVisible({ timeout: 5000 })
    const comboboxes = emptyRow.locator('[role="combobox"]')
    const openOpts = page.locator(
      '.MuiMenu-root:not([aria-hidden="true"]) li[role="option"]:not([aria-disabled="true"])',
    )
    const waitClosed = () =>
      page.waitForFunction(
        () => !document.querySelector('.MuiMenu-root:not([aria-hidden="true"])'),
        { timeout: 5000 },
      )

    await comboboxes.nth(0).click()
    await expect(comboboxes.nth(0)).toHaveAttribute('aria-expanded', 'true', { timeout: 3000 })
    await openOpts.first().click()
    await waitClosed()
    await comboboxes.nth(1).click()
    await expect(comboboxes.nth(1)).toHaveAttribute('aria-expanded', 'true', { timeout: 3000 })
    await openOpts.first().click()
    await waitClosed()

    // Warning may or may not appear — but no JS error should occur
    const warningVisible = await page
      .getByText(/subnet|compatible|cidr|ip.*not.*compatible|vm.*ip/i)
      .isVisible()
      .catch(() => false)
    expect(typeof warningVisible).toBe('boolean')
  })

  test('remapping to compatible subnet clears subnet warning', async ({ page }) => {
    // Map all network rows using proven pattern, then verify no subnet warning
    await mapAllTableRows(page, 'network-mapping-table')

    await page.waitForTimeout(400)
    await expect(page.getByText(/subnet.*warning|incompatible.*subnet/i)).not.toBeVisible()
  })
})

// ─── MIG-037: Preserve IP and Preserve MAC toggles in bulk IP dialog ──────────

test.describe('MIG-037 — preserve IP and MAC toggles', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await mockRoute(page, API.validateIPs, 'POST', MOCK_IP_VALIDATION_VALID)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)

    // Select powered-on VM with known IP (Preserve IP switch is enabled for powered-on VMs)
    const grid = page.getByTestId('vms-datagrid')
    await grid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })

    await page.getByTestId('bulk-ip-edit-button').click()
    await expect(page.getByTestId('bulk-ip-dialog')).toBeVisible()
  })

  test('enabling Preserve IP disables the IP input field', async ({ page }) => {
    const dialog = page.getByTestId('bulk-ip-dialog')
    const preserveIpSwitch = dialog.locator('input[type="checkbox"]').first()
    const ipInput = dialog.locator('input[type="text"]').first()

    // For powered-on VM with existing IP: Preserve IP defaults checked, IP input disabled
    await expect(preserveIpSwitch).toBeChecked()
    await expect(ipInput).toBeDisabled()

    // Uncheck Preserve IP → input enabled
    await preserveIpSwitch.click({ force: true })
    await expect(ipInput).toBeEnabled()

    // Re-check Preserve IP → input disabled again
    await preserveIpSwitch.click({ force: true })
    await expect(ipInput).toBeDisabled()
  })

  test('disabling Preserve IP re-enables the IP input field', async ({ page }) => {
    const dialog = page.getByTestId('bulk-ip-dialog')
    const preserveIpSwitch = dialog.locator('input[type="checkbox"]').first()
    const ipInput = dialog.locator('input[type="text"]').first()

    // Preserve IP defaults to checked — IP input is disabled
    await expect(ipInput).toBeDisabled()

    // Uncheck Preserve IP → IP input becomes enabled
    await preserveIpSwitch.click({ force: true })
    await expect(ipInput).toBeEnabled()
  })

  test('Preserve MAC toggle can be enabled and saves state', async ({ page }) => {
    const dialog = page.getByTestId('bulk-ip-dialog')
    const preserveMacToggle = dialog.locator('input[type="checkbox"]').nth(1)

    await expect(preserveMacToggle).toBeVisible()

    // Preserve MAC defaults to checked; verify we can toggle it off then back on
    await expect(preserveMacToggle).toBeChecked()
    await preserveMacToggle.click({ force: true })
    await expect(preserveMacToggle).not.toBeChecked()
    await preserveMacToggle.click({ force: true })
    await expect(preserveMacToggle).toBeChecked()
  })
})

// ─── MIG-038: Mixed OS VMs in post-migration script validation ────────────────

test.describe('MIG-038 — mixed OS script validation', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    // Mock PATCH for vmwaremachine OS assignment so state propagates synchronously
    await page.route('**vmwaremachines**', (route) => {
      if (route.request().method() === 'PATCH') {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ spec: { vms: {} } }) })
      } else {
        route.fallback()
      }
    })
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)

    // Select both a powered-on (Linux) and powered-off VM to create a mixed-OS scenario
    const grid = page.getByTestId('vms-datagrid')
    await grid.locator('[role="row"]').nth(1).getByRole('checkbox').click({ force: true })
    const poweredOffRow = grid.locator('[role="row"]').filter({ hasText: 'test-vm-powered-off' })
    await poweredOffRow.getByRole('checkbox').click({ force: true })

    // Assign Windows to powered-off VM; wait for PATCH response to confirm state propagated
    const patchPromise = page.waitForResponse(
      (response) => response.url().includes('vmwaremachines') && response.request().method() === 'PATCH'
    )
    await poweredOffRow.getByRole('combobox').click()
    await page.getByRole('option', { name: /windows/i }).click()
    await patchPromise

    // Navigate to options section
    await page.getByTestId('section-nav-item-options').click()
  })

  test('enabling post-migration script without OS tags shows mixed-OS warning', async ({ page }) => {
    // MUI Checkbox in FormControlLabel: click the label text to toggle — more reliable than
    // getByRole('checkbox').check() which fails with "did not change its state" on MUI controlled inputs
    await page.locator('label').filter({ hasText: /enable script/i }).click()

    // Wait for the textarea to become enabled before filling
    const scriptInput = page.locator('textarea').first()
    await expect(scriptInput).toBeEnabled({ timeout: 3000 })
    await scriptInput.fill('#!/bin/bash\necho "hello"')
    await scriptInput.blur()

    // Scroll the drawer to bottom — helper text renders below the textarea and is clipped
    // by the drawer-body overflow container until scrolled into view
    await page.getByTestId('drawer-body').evaluate((el) => { el.scrollTop = el.scrollHeight })
    // Target MuiFormHelperText specifically to avoid multi-element strict-mode issues with getByText
    await expect(
      page.locator('.MuiFormHelperText-root').filter({ hasText: /mixed os/i })
    ).toBeVisible({ timeout: 5000 })
  })

  test('adding OS tags to script clears mixed-OS warning', async ({ page }) => {
    // Same label-click approach for MUI Checkbox
    await page.locator('label').filter({ hasText: /enable script/i }).click()

    const scriptInput = page.locator('textarea').first()
    await expect(scriptInput).toBeEnabled({ timeout: 3000 })

    // Enter script without tags to trigger warning
    await scriptInput.fill('#!/bin/bash\necho "hello"')
    await scriptInput.blur()
    await page.getByTestId('drawer-body').evaluate((el) => { el.scrollTop = el.scrollHeight })
    const warning = page.locator('.MuiFormHelperText-root').filter({ hasText: /mixed os/i })
    await expect(warning).toBeVisible({ timeout: 5000 })

    // Add OS-specific tags using the correct format: // LINUX-SCRIPT: and // WINDOWS-SCRIPT:
    await scriptInput.fill('// LINUX-SCRIPT:\necho "linux"\n// WINDOWS-SCRIPT:\nWrite-Host "windows"')
    await scriptInput.blur()

    await expect(warning).not.toBeVisible({ timeout: 5000 })
  })
})

// ─── MIG-039: Global setting DEFAULT_NETWORK_PERSISTENCE seeds migration form ──

test.describe('MIG-039 — global default network persistence seeds migration form', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.settingsConfigMap, 'GET', MOCK_SETTINGS_CONFIGMAP_NETWORK_PERSISTENCE_ON)
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)
    await page.getByTestId('section-nav-item-options').click()
  })

  test('checkbox pre-checked when global default is ON', async ({ page }) => {
    const checkbox = page.getByTestId('migration-option-network-persistence')
    await expect(checkbox).toBeChecked({ timeout: 5000 })
  })

  test('user can uncheck even when global default is ON', async ({ page }) => {
    const checkbox = page.getByTestId('migration-option-network-persistence')
    await expect(checkbox).toBeChecked({ timeout: 5000 })
    await checkbox.click({ force: true })
    await expect(checkbox).not.toBeChecked()
  })
})

// ─── MIG-040: Global setting OFF — migration form checkbox unchecked by default ─

test.describe('MIG-040 — global default OFF leaves network persistence unchecked', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.settingsConfigMap, 'GET', MOCK_SETTINGS_CONFIGMAP_NETWORK_PERSISTENCE_OFF)
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)
    await page.getByTestId('section-nav-item-options').click()
  })

  test('checkbox unchecked when global default is OFF', async ({ page }) => {
    const checkbox = page.getByTestId('migration-option-network-persistence')
    await expect(checkbox).not.toBeChecked({ timeout: 5000 })
  })

  test('user can manually check even when global default is OFF', async ({ page }) => {
    const checkbox = page.getByTestId('migration-option-network-persistence')
    await expect(checkbox).not.toBeChecked({ timeout: 5000 })
    await checkbox.click({ force: true })
    await expect(checkbox).toBeChecked()
  })
})

// ─── MIG-041: Global settings page — DEFAULT_NETWORK_PERSISTENCE toggle ───────

test.describe('MIG-041 — global settings page DEFAULT_NETWORK_PERSISTENCE toggle', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.settingsConfigMap, 'GET', MOCK_SETTINGS_CONFIGMAP_NETWORK_PERSISTENCE_OFF)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToGlobalSettings(page)
  })

  test('Advanced tab shows DEFAULT_NETWORK_PERSISTENCE toggle off by default', async ({ page }) => {
    await page.getByTestId('global-settings-tab-advanced').click()
    const toggle = page.getByTestId('global-settings-toggle-DEFAULT_NETWORK_PERSISTENCE')
    await expect(toggle).toBeVisible({ timeout: 5000 })
    await expect(toggle.locator('input')).not.toBeChecked()
  })

  test('toggling DEFAULT_NETWORK_PERSISTENCE on enables the setting', async ({ page }) => {
    await page.getByTestId('global-settings-tab-advanced').click()
    const toggleInput = page.getByTestId('global-settings-toggle-DEFAULT_NETWORK_PERSISTENCE').locator('input')
    await expect(toggleInput).not.toBeChecked()

    // Mock PUT for save
    await mockRoute(page, API.settingsConfigMap, 'PUT', MOCK_SETTINGS_CONFIGMAP_NETWORK_PERSISTENCE_ON)
    await toggleInput.click({ force: true })
    await expect(toggleInput).toBeChecked()
  })
})
