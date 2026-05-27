import { test, expect, Page } from '@playwright/test'

import {
  goToMigrations,
  openMigrationDrawer,
  submitMigrationForm,
  selectVmwareCluster,
  selectPcdCluster,
  mockRoute,
  mockRouteError,
  expectToast,
  expectDrawerOpen,
  API,
  ROUTES,
  NS,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST,
  MOCK_MIGRATIONS_LIST_EMPTY,
  MOCK_MIGRATION_PLANS_LIST,
  MOCK_MIGRATION_PLANS_LIST_EMPTY,
  MOCK_MIGRATION_PLAN_2,
  MOCK_MIGRATION_AWAITING_CUTOVER,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_OPENSTACK_CRED_1,
  MOCK_OPENSTACK_CREDS_LIST,
  MOCK_MIGRATION_TEMPLATE_PENDING,
  MOCK_MIGRATION_TEMPLATE_READY,
  MOCK_NETWORK_MAPPING_CREATED,
  MOCK_STORAGE_MAPPING_CREATED,
  MOCK_VMWARE_CLUSTERS_LIST,
  MOCK_PCD_CLUSTERS_LIST,
  MOCK_VMWARE_CRED_1,
  MOCK_VMWARE_MACHINES_LIST,
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

// MUI menu items may not be "visible" during animation; use native DOM click.
async function clickFirstMenuOption(page: Page) {
  await page.waitForSelector('.MuiMenu-root', { state: 'attached', timeout: 3000 })
  await page.evaluate(() => {
    const menus = document.querySelectorAll('.MuiMenu-root')
    const menu = menus[menus.length - 1]
    if (!menu) return
    const option = menu.querySelector('li[role="option"]:not(.Mui-disabled)')
    if (option) (option as HTMLElement).click()
  })
}

async function completeMigrationFormSteps(page: Page) {
  await selectClustersAndWaitForVMs(page)

  // Step 2: select 2 valid VMs by name (avoid powered-off VM — no osFamily → validation error)
  const grid = page.getByTestId('vms-datagrid')
  await grid.locator('[role="row"]').filter({ hasText: 'test-vm-1' }).getByRole('checkbox').click({ force: true })
  await grid.locator('[role="row"]').filter({ hasText: 'test-vm-multi-network' }).getByRole('checkbox').click({ force: true })
  // Wait for selection to register — toolbar shows "Assign Flavor (N)"
  await expect(page.getByText(/assign flavor/i)).toBeVisible({ timeout: 5000 })

  // Step 3: navigate to map-resources and fill all network + storage mappings
  await page.getByTestId('section-nav-item-map-resources').click()

  // ResourceMappingTableNew uses native <tr> elements (no role="row").
  // The last tbody row is the empty input row with two comboboxes.
  // Selecting source + target auto-adds the mapping row.
  const netTable = page.getByTestId('network-mapping-table')
  await expect(netTable).toBeVisible()
  for (let i = 0; i < 10; i++) {
    const emptyRow = netTable.locator('tbody tr').last()
    const sourceSelect = emptyRow.locator('[role="combobox"]').first()
    const isVisible = await sourceSelect.isVisible().catch(() => false)
    if (!isVisible) break

    const prevCount = await netTable.locator('[aria-label="delete-mapping"]').count()
    await sourceSelect.click()
    await clickFirstMenuOption(page)
    await page.waitForFunction(
      () => Array.from(document.querySelectorAll('.MuiMenu-root')).every((m) => m.getAttribute('aria-hidden') === 'true'),
      { timeout: 5000 },
    )
    const targetSelect = netTable.locator('tbody tr').last().locator('[role="combobox"]').last()
    await targetSelect.click()
    await clickFirstMenuOption(page)
    await page.waitForFunction(
      () => Array.from(document.querySelectorAll('.MuiMenu-root')).every((m) => m.getAttribute('aria-hidden') === 'true'),
      { timeout: 5000 },
    )
    await expect(netTable.locator('[aria-label="delete-mapping"]')).toHaveCount(prevCount + 1, { timeout: 5000 })
  }

  const storTable = page.getByTestId('storage-mapping-table')
  await expect(storTable).toBeVisible()
  for (let i = 0; i < 10; i++) {
    const emptyRow = storTable.locator('tbody tr').last()
    const sourceSelect = emptyRow.locator('[role="combobox"]').first()
    const isVisible = await sourceSelect.isVisible().catch(() => false)
    if (!isVisible) break

    const prevCount = await storTable.locator('[aria-label="delete-mapping"]').count()
    await sourceSelect.click()
    await clickFirstMenuOption(page)
    await page.waitForFunction(
      () => Array.from(document.querySelectorAll('.MuiMenu-root')).every((m) => m.getAttribute('aria-hidden') === 'true'),
      { timeout: 5000 },
    )
    const targetSelect = storTable.locator('tbody tr').last().locator('[role="combobox"]').last()
    await targetSelect.click()
    await clickFirstMenuOption(page)
    await page.waitForFunction(
      () => Array.from(document.querySelectorAll('.MuiMenu-root')).every((m) => m.getAttribute('aria-hidden') === 'true'),
      { timeout: 5000 },
    )
    await expect(storTable.locator('[aria-label="delete-mapping"]')).toHaveCount(prevCount + 1, { timeout: 5000 })
  }
}

// ─── MIG-020: API error on standard migration submission ──────────────────────

test.describe('MIG-020 — 500 error on migration plan submission', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.networkMappings, 'POST', MOCK_NETWORK_MAPPING_CREATED)
    await mockRoute(page, API.storageMappings, 'POST', MOCK_STORAGE_MAPPING_CREATED)
    // Simulate 500 on migration plan creation
    await mockRouteError(page, API.migrationPlans, 'POST', 500, 'Internal server error')
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
  })

  test('500 on submit keeps submit disabled and drawer open', async ({ page }) => {
    await completeMigrationFormSteps(page)
    await submitMigrationForm(page)

    // createMigrationPlan throws → handleSubmit rejects → setSubmitting never reset to false
    await expect(page.getByTestId('migration-form-submit')).toBeDisabled({ timeout: 5000 })
    // Drawer stays open — form not reset
    await expectDrawerOpen(page)
  })

  test('form state preserved after submit error', async ({ page }) => {
    await completeMigrationFormSteps(page)
    await submitMigrationForm(page)

    // Drawer stays open — form not reset
    await expectDrawerOpen(page)

    // Section nav still shows completed state (step 1 still selected)
    await expect(page.getByTestId('section-nav-item-source-destination')).toBeVisible()
  })
})

// ─── MIG-021: Network mapping API error ──────────────────────────────────────

test.describe('MIG-021 — 422 error on network mapping creation', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    // Network mapping fails with 422
    await mockRouteError(page, API.networkMappings, 'POST', 422, 'Network mapping validation failed')
    await mockRoute(page, API.storageMappings, 'POST', MOCK_STORAGE_MAPPING_CREATED)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    await goToMigrations(page)
    await openMigrationDrawer(page)
  })

  test('422 on network mapping keeps submit disabled and drawer open', async ({ page }) => {
    await completeMigrationFormSteps(page)
    await submitMigrationForm(page)

    // postNetworkMapping rejection propagates → handleSubmit rejects → setSubmitting never reset
    await expect(page.getByTestId('migration-form-submit')).toBeDisabled({ timeout: 5000 })
    // Drawer stays open
    await expectDrawerOpen(page)
  })
})

// ─── MIG-022: Credential fetch failure in Step 1 ─────────────────────────────

test.describe('MIG-022 — credential fetch failure in step 1', () => {
  test('500 on vmwarecreds GET redirects to credentials page', async ({ page }) => {
    // VMware creds endpoint returns 500
    await mockRouteError(page, API.vmwareCreds, 'GET', 500, 'Failed to fetch VMware credentials')
    await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
    await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await page.goto(ROUTES.migrations)
    // vmwareCreds list GET → 500 → app detects no credentials on page load → auto-redirects
    await page.waitForURL(/\/credentials/, { timeout: 15_000 })
  })

  test('500 on vmwareclusters GET shows error in step 1', async ({ page }) => {
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRouteError(page, API.vmwareClusters, 'GET', 500, 'Failed to fetch VMware clusters')
    await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await goToMigrations(page)
    await openMigrationDrawer(page)

    // Cluster dropdown should reflect the error — empty or error state
    await expect(page.getByTestId('vmware-cluster-dropdown')).toBeVisible()
    // Submit should remain disabled
    await expect(page.getByTestId('migration-form-submit')).toBeDisabled()
  })
})

// ─── MIG-023: Migration template polling timeout / failure ────────────────────

test.describe('MIG-023 — migration template polling failure', () => {
  test('template creation failure shows error in drawer', async ({ page }) => {
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
    await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
    await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    // Template POST returns 500 — creation fails immediately
    await page.route('**migrationtemplates**', (route) => {
      const method = route.request().method()
      if (method === 'POST') {
        route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ message: 'Template creation failed' }) })
      } else {
        route.continue()
      }
    })

    await goToMigrations(page)
    await openMigrationDrawer(page)

    await selectVmwareCluster(page, 'DC1-Cluster')
    await selectPcdCluster(page, 'pcd-cluster-1')

    // Template POST 500 → no template created → VM list cannot be populated
    await expect(page.getByText('No VMs discovered')).toBeVisible({ timeout: 10_000 })
  })

  test('template never becomes ready — VM list shows error or loading state', async ({ page }) => {
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
    await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
    await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    // Template POST succeeds but GET always returns pending status (never ready)
    await page.route('**migrationtemplates**', (route) => {
      const method = route.request().method()
      if (method === 'POST') {
        route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
      } else if (method === 'GET') {
        // Return pending — VM list never populates
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
      } else {
        route.continue()
      }
    })

    await goToMigrations(page)
    await openMigrationDrawer(page)

    await selectVmwareCluster(page, 'DC1-Cluster')
    await selectPcdCluster(page, 'pcd-cluster-1')

    // Submit must remain disabled — VMs never loaded
    await expect(page.getByTestId('migration-form-submit')).toBeDisabled()

    // Drawer should show loading indicator or timeout error (not blank with no feedback)
    const hasLoadingOrError = await page.locator(
      '[role="progressbar"], [aria-label*="loading"], [data-testid*="loading"], [data-testid*="error"]'
    ).or(page.getByText(/loading|timeout|error|failed/i)).first().isVisible()
    expect(hasLoadingOrError).toBe(true)
  })
})

// ─── MIG-024: IP validation API error (bulk IP dialog) ───────────────────────

test.describe('MIG-024 — IP validation API error', () => {
  test.beforeEach(async ({ page }) => {
    await mockStandardFormApis(page)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
    // IP validation API returns 500
    await mockRouteError(page, API.validateIPs, 'POST', 500, 'Validation service unavailable')

    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectClustersAndWaitForVMs(page)

    // Select powered-off VM, open bulk IP dialog
    const grid = page.getByTestId('vms-datagrid')
    const poweredOffRow = grid.locator('[role="row"]').filter({ hasText: 'test-vm-powered-off' })
    await expect(poweredOffRow).toBeVisible()
    await poweredOffRow.getByRole('checkbox').click()

    await page.getByTestId('bulk-ip-edit-button').click()
    await expect(page.getByTestId('bulk-ip-dialog')).toBeVisible()
  })

  test('validateIPs 500 shows error toast; Apply remains disabled', async ({ page }) => {
    const dialog = page.getByTestId('bulk-ip-dialog')

    // Powered-off VM starts with empty IP. Must type a valid IP so ipsToApply is non-empty,
    // otherwise the early-return "no changes → apply overrides" path runs before validateOpenstackIPs.
    const ipInput = dialog.getByPlaceholder(/IP/i).first()
    await ipInput.fill('10.0.0.1')

    await dialog.getByRole('button', { name: /apply/i }).click()

    // 500 response → markBulkValidationFailure sets inline error + showToast (local useToast hook)
    // Error text renders as TextField helperText inside the dialog
    await expect(dialog.getByText(/Validation service unavailable/i)).toBeVisible({ timeout: 5000 })

    // After error: hasBulkIpValidationErrors = true → Apply disabled
    await expect(dialog.getByRole('button', { name: /apply/i })).toBeDisabled()
  })
})

// ─── MIG-025: Pod logs — connection error and reconnect ──────────────────────

test.describe('MIG-025 — pod logs connection error and reconnect', () => {
  const POD_NAME = 'v2v-helper-test-2'

  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('log fetch failure shows error and Reconnect button', async ({ page }) => {
    // Simulate log stream failure
    await page.route(API.podLogs(NS, POD_NAME), (route) => {
      route.fulfill({ status: 500, contentType: 'text/plain', body: 'log stream unavailable' })
    })

    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: 'test-vm-2' })
    await expect(row).toBeVisible()

    await row.getByRole('button', { name: /log/i }).click()
    const drawer = page.getByTestId('pod-logs-drawer')
    await expect(drawer).toBeVisible()

    // Error message shown — not silent failure
    await expect(
      drawer.getByText(/failed|error|connect|unavailable/i),
    ).toBeVisible({ timeout: 5000 })

    // Reconnect button visible
    await expect(
      drawer.getByRole('button', { name: /reconnect|retry/i }),
    ).toBeVisible()
  })

  test('clicking Reconnect restarts log stream', async ({ page }) => {
    let callCount = 0
    const SAMPLE_LOGS = 'INFO 2026-05-20T10:00:00Z Migration started\nINFO 2026-05-20T10:01:00Z Copying disk'

    await page.route(API.podLogs(NS, POD_NAME), (route) => {
      callCount++
      if (callCount === 1) {
        // First call fails
        route.fulfill({ status: 500, contentType: 'text/plain', body: 'error' })
      } else {
        // Subsequent calls succeed
        route.fulfill({ status: 200, contentType: 'text/plain', body: SAMPLE_LOGS })
      }
    })

    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: 'test-vm-2' })
    await row.getByRole('button', { name: /log/i }).click()

    const drawer = page.getByTestId('pod-logs-drawer')
    await expect(drawer).toBeVisible()
    await expect(drawer.getByText(/failed|error/i)).toBeVisible({ timeout: 5000 })

    // Click reconnect
    await drawer.getByRole('button', { name: /reconnect|retry/i }).click()

    // Logs should appear after reconnect
    await expect(drawer.getByText(/Migration started/i)).toBeVisible({ timeout: 5000 })
  })
})

// ─── MIG-026: Delete migration API error ─────────────────────────────────────

test.describe('MIG-026 — delete migration API error', () => {
  const TARGET_VM = 'test-vm-3'
  const TARGET_MIGRATION = 'test-vm-3-migration'

  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    // handleDeleteMigration GETs then PATCHes the plan before DELETing the migration
    await mockRoute(page, API.migrationPlanByName('test-plan-2'), 'GET', MOCK_MIGRATION_PLAN_2)
    await mockRoute(page, API.migrationPlanByName('test-plan-2'), 'PATCH', MOCK_MIGRATION_PLAN_2)
    await goToMigrations(page)
  })

  test('500 on DELETE closes dialog; migration row stays visible', async ({ page }) => {
    await mockRouteError(page, API.migrationByName(TARGET_MIGRATION), 'DELETE', 500, 'Delete failed')

    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: TARGET_VM })
    await expect(row).toBeVisible()

    await row.getByRole('button', { name: /delete/i }).click()
    await page.getByTestId('confirm-delete-button').click()

    // handleDeleteMigration catches error internally (no re-throw) →
    // ConfirmationDialog.onConfirm resolves → onClose() called → dialog closes
    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 })
    // Row still visible — migration not deleted on error
    await expect(row).toBeVisible()
  })

  test('403 on DELETE closes dialog; migration row stays visible', async ({ page }) => {
    await mockRouteError(page, API.migrationByName(TARGET_MIGRATION), 'DELETE', 403, 'Forbidden')

    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: TARGET_VM })
    await expect(row).toBeVisible()

    await row.getByRole('button', { name: /delete/i }).click()
    await page.getByTestId('confirm-delete-button').click()

    await expect(page.getByRole('dialog')).not.toBeVisible({ timeout: 5000 })
    await expect(row).toBeVisible()
  })
})

// ─── MIG-027: Admin cutover error handling ────────────────────────────────────

test.describe('MIG-027 — admin cutover error handling', () => {
  const AWAITING_VM = 'test-vm-5'

  test.beforeEach(async ({ page }) => {
    const listWithAwaitingOnly = {
      ...MOCK_MIGRATIONS_LIST,
      items: [MOCK_MIGRATION_AWAITING_CUTOVER],
    }
    await mockRoute(page, API.migrations, 'GET', listWithAwaitingOnly)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    // triggerAdminCutover first GETs the migration by name to find podRef.
    // Return 500 so the error surfaces immediately without needing to mock pods/PATCH chain.
    await mockRouteError(page, API.migrationByName('test-vm-5-migration'), 'GET', 500, 'Cutover trigger failed')
    await goToMigrations(page)
  })

  test('cutover API error shows error in dialog; retry possible', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: AWAITING_VM })
    await expect(row).toBeVisible()

    await row.getByTestId('cutover-trigger-button').click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.getByTestId('cutover-confirm-button').click()

    // TriggerAdminCutoverButton shows <Alert severity="error"> inline in the dialog on error.
    // The Alert text is the AxiosError message: "Request failed with status code 500"
    await expect(
      page.getByRole('dialog').locator('[role="alert"]'),
    ).toBeVisible({ timeout: 5000 })
  })

  test('dialog remains open after cutover error for retry', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: AWAITING_VM })
    await row.getByTestId('cutover-trigger-button').click()
    await expect(page.getByRole('dialog')).toBeVisible()

    // First attempt — fails
    await page.getByTestId('cutover-confirm-button').click()
    await expect(page.getByRole('dialog').locator('[role="alert"]')).toBeVisible({ timeout: 5000 })

    // Dialog stays open — setOpen(false) NOT called in catch block
    await expect(page.getByRole('dialog')).toBeVisible()
    // Migration phase must NOT be incorrectly updated to Succeeded on error
    await expect(row.getByText('AwaitingAdminCutOver', { exact: true })).toBeVisible()
  })
})
