import { test, expect, Page, Route } from '@playwright/test'

import {
  goToMigrations,
  expectDrawerOpen,
  expectDrawerClosed,
  mockRoute,
  mockRouteError,
  API,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST_WITH_RETRYABLE,
  MOCK_MIGRATIONS_LIST_MULTI_FAILED,
  MOCK_MIGRATION_FAILED_RETRYABLE,
  MOCK_MIGRATION_FAILED_RETRYABLE_2,
  MOCK_RETRY_MIGRATION_PLAN,
  MOCK_RETRY_MIGRATION_PLAN_WITH_IP_OVERRIDE,
  MOCK_RETRY_MIGRATION_PLAN_MULTIVM,
  MOCK_RETRY_MIGRATION_TEMPLATE,
  MOCK_RETRY_NETWORK_MAPPING,
  MOCK_RETRY_STORAGE_MAPPING,
  MOCK_RETRY_VMWARE_MACHINE,
  MOCK_RETRY_NETWORK_MAPPING_CREATED,
  MOCK_RETRY_STORAGE_MAPPING_CREATED,
  MOCK_OPENSTACK_CRED_WITH_FLAVORS,
  MOCK_VMWARE_CRED_1,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_OPENSTACK_CREDS_LIST,
  MOCK_MIGRATION_PLANS_LIST_EMPTY,
  MOCK_RETRY_MIGRATION_NAME,
  MOCK_RETRY_MIGRATION_NAME_2,
  MOCK_RETRY_PLAN_NAME,
  MOCK_RETRY_TEMPLATE_NAME,
  MOCK_RETRY_VM_K8S_NAME,
  MOCK_RETRY_VM_KEY,
  MOCK_RETRY_CLONE_TEMPLATE_CREATED,
  MOCK_RETRY_CLONE_PLAN_CREATED,
  NS,
} from './helpers/migration.fixtures'

// Mounts every route the retry drawer needs to load the failed migration's
// configuration. Individual tests override specific routes for their scenario.
async function mockRetryPrefillRoutes(page: Page): Promise<void> {
  await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
  await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_WITH_RETRYABLE)
  await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
  await mockRoute(
    page,
    API.migrationByName(MOCK_RETRY_MIGRATION_NAME),
    'GET',
    MOCK_MIGRATION_FAILED_RETRYABLE,
  )
  await mockRoute(
    page,
    API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
    'GET',
    MOCK_RETRY_MIGRATION_PLAN,
  )
  await mockRoute(
    page,
    API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
    'GET',
    MOCK_RETRY_MIGRATION_TEMPLATE,
  )
  await mockRoute(page, API.vmwareCredByName('vcenter-cred-1'), 'GET', MOCK_VMWARE_CRED_1)
  await mockRoute(
    page,
    API.openstackCredByName('pcd-cred-1'),
    'GET',
    MOCK_OPENSTACK_CRED_WITH_FLAVORS,
  )
  await mockRoute(
    page,
    API.vmwareMachineByName(MOCK_RETRY_VM_K8S_NAME),
    'GET',
    MOCK_RETRY_VMWARE_MACHINE,
  )
  await mockRoute(
    page,
    API.networkMappingByName('retry-netmap-1'),
    'GET',
    MOCK_RETRY_NETWORK_MAPPING,
  )
  await mockRoute(
    page,
    API.storageMappingByName('retry-stormap-1'),
    'GET',
    MOCK_RETRY_STORAGE_MAPPING,
  )
  // Supporting queries used by the form shell.
  await mockRoute(page, API.rdmDisks, 'GET', { items: [] })
  await mockRoute(page, API.vmwareClusters, 'GET', { items: [] })
  await mockRoute(page, API.pcdClusters, 'GET', { items: [] })
  await mockRoute(page, API.volumeImageProfiles, 'GET', { items: [] })
  // VmsSelectionStep fetches the VM list for the cluster.
  await mockRoute(page, API.vmwareMachines, 'GET', { items: [MOCK_RETRY_VMWARE_MACHINE] })
}

async function openRetryDrawer(page: Page): Promise<void> {
  await goToMigrations(page)
  const row = page.getByRole('row').filter({ hasText: 'test-vm-retry' })
  await row.locator('[data-testid="ReplayIcon"]').click()
  await expectDrawerOpen(page)
}

// Records calls to a route so the test can assert on method, order and payload.
interface RecordedCall {
  method: string
  body: Record<string, unknown> | undefined
}

// Like recordRoute but returns 404 on GET after a DELETE is received.
// Used for Migration routes so pollUntilGone() terminates once the delete lands.
async function recordRouteGone404AfterDelete(
  page: Page,
  url: string,
  responseByMethod: Record<string, { status?: number; body: Record<string, unknown> }>,
  calls: RecordedCall[],
  label: string,
  order: string[],
): Promise<void> {
  let deleted = false
  await page.route(url, (route: Route) => {
    const method = route.request().method()
    if (method === 'GET' && deleted) {
      route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ message: 'not found' }) })
      return
    }
    const handler = responseByMethod[method]
    if (!handler) return route.continue()
    calls.push({ method, body: route.request().postDataJSON() ?? undefined })
    order.push(`${method} ${label}`)
    if (method === 'DELETE') deleted = true
    route.fulfill({
      status: handler.status ?? 200,
      contentType: 'application/json',
      body: JSON.stringify(handler.body),
    })
  })
}

async function recordRoute(
  page: Page,
  url: string,
  responseByMethod: Record<string, { status?: number; body: Record<string, unknown> }>,
  calls: RecordedCall[],
  label: string,
  order: string[],
): Promise<void> {
  await page.route(url, (route: Route) => {
    const method = route.request().method()
    const handler = responseByMethod[method]
    if (!handler) return route.continue()
    calls.push({ method, body: route.request().postDataJSON() ?? undefined })
    order.push(`${method} ${label}`)
    route.fulfill({
      status: handler.status ?? 200,
      contentType: 'application/json',
      body: JSON.stringify(handler.body),
    })
  })
}

// ─── RET-001: retry opens pre-populated form ──────────────────────────────────

test.describe('RET-001 — retry opens the migration form pre-populated', () => {
  test.beforeEach(async ({ page }) => {
    await mockRetryPrefillRoutes(page)
  })

  test('drawer opens in retry mode with locked VM and prefilled config', async ({ page }) => {
    await openRetryDrawer(page)

    // Retry-mode header
    await expect(page.getByTestId('migration-form-header')).toContainText('Retry Migration')

    // Source/destination locked summary
    const sourceSummary = page.getByTestId('retry-source-destination-summary')
    await expect(sourceSummary).toBeVisible()
    await expect(sourceSummary).toContainText('vcenter-cred-1')
    await expect(sourceSummary).toContainText('DC1')
    await expect(sourceSummary).toContainText('cluster-1')
    await expect(sourceSummary).toContainText('pcd-cred-1')
    await expect(sourceSummary).toContainText('pcd-cluster-1')

    // VM table shows the locked VM (not grayed out) with Assign Flavor / Assign IP available
    await expect(page.getByRole('row', { name: /test-vm-retry/ })).toBeVisible({ timeout: 10_000 })

    // VMware cluster selector is not shown in retry mode; target cluster is editable
    await expect(page.getByTestId('vmware-cluster-dropdown')).not.toBeVisible()
    await expect(page.getByTestId('retry-target-cluster-select')).toBeVisible()

    // Multi-VM warning banner must NOT appear for a single-VM plan
    await expect(page.getByTestId('retry-multivm-warning-banner')).not.toBeVisible()

    // Single retry action
    await expect(page.getByTestId('migration-form-retry')).toBeVisible()
    await expect(page.getByTestId('migration-form-submit')).not.toBeVisible()

    // Retry button becomes enabled once the prefilled config validates
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({
      timeout: 10_000,
    })
  })

  test('retry button is disabled while prefill data is loading', async ({ page }) => {
    // Delay the migration fetch so we can observe query.isLoading → button disabled.
    let resolveMigration!: () => void
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await page.route(API.migrationByName(MOCK_RETRY_MIGRATION_NAME), async (route) => {
      await new Promise<void>((r) => { resolveMigration = r })
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_MIGRATION_FAILED_RETRYABLE),
      })
    })

    await goToMigrations(page)
    const row = page.getByRole('row').filter({ hasText: 'test-vm-retry' })
    await row.locator('[data-testid="ReplayIcon"]').click()
    await expectDrawerOpen(page)

    // While query is loading the retry button must be disabled.
    await expect(page.getByTestId('migration-form-retry')).toBeDisabled()

    // Unblock the fetch — form should populate and button becomes enabled.
    resolveMigration()
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({
      timeout: 10_000,
    })
  })

  test('cancel closes the drawer without any write', async ({ page }) => {
    const writes: string[] = []
    await page.route('**/apis/vjailbreak.k8s.pf9.io/**', (route) => {
      const method = route.request().method()
      if (method !== 'GET') writes.push(`${method} ${route.request().url()}`)
      route.fallback()
    })

    await openRetryDrawer(page)
    await page.getByTestId('migration-form-cancel').click()
    await expectDrawerClosed(page)
    expect(writes).toEqual([])
  })
})

// ─── RET-003: retry (single-VM plan) ─────────────────────────────────────────

test.describe('RET-003 — retry deletes old plan+template and migration first, then creates fresh resources (single-VM plan)', () => {
  test.beforeEach(async ({ page }) => {
    await mockRetryPrefillRoutes(page)
  })

  // Single-VM Retry:
  //   DELETE old plan → DELETE old template → DELETE old migration (404-tolerant) →
  //   POST new mappings → POST new template (UUID name) → POST new plan (UUID name)
  // No PATCH on the original plan (deleted entirely — only one VM).
  test('DELETEs old plan, old template, and migration first, then POSTs new template and plan', async ({
    page,
  }) => {
    const calls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_PLAN },
        DELETE: { body: MOCK_RETRY_MIGRATION_PLAN },
      },
      calls,
      'original-plan',
      order,
    )
    // Template DELETE happens in step 1b for single-VM plans (frees the name for the new UUID-named template).
    await page.unroute(API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME))
    await recordRoute(
      page,
      API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
        DELETE: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
      },
      calls,
      'original-template',
      order,
    )
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRouteGone404AfterDelete(
      page,
      API.migrationByName(MOCK_RETRY_MIGRATION_NAME),
      {
        GET: { body: MOCK_MIGRATION_FAILED_RETRYABLE },
        DELETE: { body: MOCK_MIGRATION_FAILED_RETRYABLE },
      },
      calls,
      'migration',
      order,
    )
    await recordRoute(
      page,
      API.migrationTemplates,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_TEMPLATE_CREATED } },
      calls,
      'clone-template',
      order,
    )
    await recordRoute(
      page,
      API.migrationPlans,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_PLAN_CREATED } },
      calls,
      'clone-plan',
      order,
    )
    await recordRoute(
      page,
      API.networkMappings,
      { POST: { status: 201, body: MOCK_RETRY_NETWORK_MAPPING_CREATED } },
      calls,
      'networkmapping',
      order,
    )
    await recordRoute(
      page,
      API.storageMappings,
      { POST: { status: 201, body: MOCK_RETRY_STORAGE_MAPPING_CREATED } },
      calls,
      'storagemapping',
      order,
    )

    await openRetryDrawer(page)
    const retryBtn = page.getByTestId('migration-form-retry')
    await expect(retryBtn).toBeEnabled({ timeout: 10_000 })
    await retryBtn.click()
    await expectDrawerClosed(page)

    const writes = order.filter((o) => !o.startsWith('GET'))

    // Old plan deleted first (only VM → delete whole plan).
    expect(writes[0]).toBe('DELETE original-plan')
    // Old template deleted second (frees name; new template gets a fresh UUID).
    expect(writes[1]).toBe('DELETE original-template')
    // Migration deleted third (404-tolerant: GC cascade from plan deletion may beat us).
    expect(writes[2]).toBe('DELETE migration')

    // New mapping resources created after deletions.
    expect(writes).toContain('POST networkmapping')
    expect(writes).toContain('POST storagemapping')

    // New template before new plan.
    expect(writes.indexOf('POST clone-template')).toBeLessThan(writes.indexOf('POST clone-plan'))

    // New plan is the last write.
    expect(writes[writes.length - 1]).toBe('POST clone-plan')

    // No PATCH on original plan.
    expect(writes.filter((w) => w.startsWith('PATCH'))).toHaveLength(0)
  })
})

// ─── RET-002: retry button disabled while mutation pending ────────────────────

test.describe('RET-002 — retry button is disabled while mutation is in flight', () => {
  test.beforeEach(async ({ page }) => {
    await mockRetryPrefillRoutes(page)
  })

  test('Retry button disabled while plan DELETE is pending', async ({ page }) => {
    // Hold the plan DELETE so mutation.isPending stays true long enough to assert.
    let resolvePlanDelete!: () => void
    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await page.route(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME), async (route) => {
      const method = route.request().method()
      if (method === 'GET') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_RETRY_MIGRATION_PLAN),
        })
      } else if (method === 'DELETE') {
        await new Promise<void>((r) => { resolvePlanDelete = r })
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_RETRY_MIGRATION_PLAN) })
      } else {
        route.continue()
      }
    })

    await openRetryDrawer(page)
    const retryBtn = page.getByTestId('migration-form-retry')
    await expect(retryBtn).toBeEnabled({ timeout: 10_000 })
    await retryBtn.click()

    // While mutation is pending the button must be disabled.
    await expect(retryBtn).toBeDisabled()

    // Unblock — let the test clean up.
    resolvePlanDelete()
  })
})

// ─── RET-004: blocking errors ─────────────────────────────────────────────────

test.describe('RET-004 — missing resources block the retry', () => {
  test('missing migration template shows blocking banner and disables retry', async ({
    page,
  }) => {
    await mockRetryPrefillRoutes(page)
    await page.unroute(API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME))
    await mockRouteError(
      page,
      API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
      'GET',
      404,
      'not found',
    )

    await openRetryDrawer(page)

    await expect(page.getByTestId('retry-blocking-banner')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('retry-blocking-banner')).toContainText(
      MOCK_RETRY_TEMPLATE_NAME,
    )
    await expect(page.getByTestId('migration-form-retry')).toBeDisabled()
  })

  test('missing credentials show blocking banner naming the credential', async ({ page }) => {
    await mockRetryPrefillRoutes(page)
    await page.unroute(API.vmwareCredByName('vcenter-cred-1'))
    await mockRouteError(page, API.vmwareCredByName('vcenter-cred-1'), 'GET', 404, 'not found')

    await openRetryDrawer(page)

    await expect(page.getByTestId('retry-blocking-banner')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('retry-blocking-banner')).toContainText('vcenter-cred-1')
    await expect(page.getByTestId('migration-form-retry')).toBeDisabled()
  })
})

// ─── RET-005: IP overrides round-tripped through the retry form ───────────────

test.describe('RET-005 — IP overrides are preserved and sent on Retry', () => {
  test.beforeEach(async ({ page }) => {
    await mockRetryPrefillRoutes(page)
    // Replace the single-VM plan with the IP-override variant.
    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await mockRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      'GET',
      MOCK_RETRY_MIGRATION_PLAN_WITH_IP_OVERRIDE,
    )
  })

  test('new plan spec carries networkOverridesPerVM when override loaded from original plan', async ({
    page,
  }) => {
    const calls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_PLAN_WITH_IP_OVERRIDE },
        DELETE: { body: MOCK_RETRY_MIGRATION_PLAN_WITH_IP_OVERRIDE },
      },
      calls,
      'original-plan',
      order,
    )
    await page.unroute(API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME))
    await recordRoute(
      page,
      API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
      { GET: { body: MOCK_RETRY_MIGRATION_TEMPLATE }, DELETE: { body: MOCK_RETRY_MIGRATION_TEMPLATE } },
      calls,
      'original-template',
      order,
    )
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRouteGone404AfterDelete(
      page,
      API.migrationByName(MOCK_RETRY_MIGRATION_NAME),
      { GET: { body: MOCK_MIGRATION_FAILED_RETRYABLE }, DELETE: { body: MOCK_MIGRATION_FAILED_RETRYABLE } },
      calls,
      'migration',
      order,
    )
    await recordRoute(
      page,
      API.migrationTemplates,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_TEMPLATE_CREATED } },
      calls,
      'clone-template',
      order,
    )
    await recordRoute(
      page,
      API.migrationPlans,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_PLAN_CREATED } },
      calls,
      'clone-plan',
      order,
    )
    await mockRoute(page, API.networkMappings, 'POST', MOCK_RETRY_NETWORK_MAPPING_CREATED)
    await mockRoute(page, API.storageMappings, 'POST', MOCK_RETRY_STORAGE_MAPPING_CREATED)

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({ timeout: 10_000 })
    await page.getByTestId('migration-form-retry').click()
    await expectDrawerClosed(page)

    // New plan POST body must carry the IP override from the original plan's prefill.
    const clonePlanIdx = order.findIndex((o) => o === 'POST clone-plan')
    const clonePlanPost = clonePlanIdx >= 0 ? calls[clonePlanIdx] : undefined
    expect(clonePlanPost).toBeDefined()
    const cloneSpec = clonePlanPost?.body?.spec as Record<string, unknown>
    const overrides = cloneSpec?.networkOverridesPerVM as Record<string, unknown> | null | undefined
    expect(overrides).not.toBeNull()
    expect(overrides).toBeDefined()
    const vmOverrides = (overrides as Record<string, unknown>)[MOCK_RETRY_VM_KEY] as
      | Array<Record<string, unknown>>
      | undefined
    expect(vmOverrides).toHaveLength(1)
    expect(vmOverrides?.[0].interfaceIndex).toBe(0)
    expect(vmOverrides?.[0].preserveIP).toBe(false)
    expect(vmOverrides?.[0].UserAssignedIP).toBe('10.0.0.50')
  })

  test('new plan spec sends networkOverridesPerVM: null when original plan had no overrides', async ({
    page,
  }) => {
    const calls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      { GET: { body: MOCK_RETRY_MIGRATION_PLAN }, DELETE: { body: MOCK_RETRY_MIGRATION_PLAN } },
      calls,
      'original-plan',
      order,
    )
    await page.unroute(API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME))
    await recordRoute(
      page,
      API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
      { GET: { body: MOCK_RETRY_MIGRATION_TEMPLATE }, DELETE: { body: MOCK_RETRY_MIGRATION_TEMPLATE } },
      calls,
      'original-template',
      order,
    )
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRouteGone404AfterDelete(
      page,
      API.migrationByName(MOCK_RETRY_MIGRATION_NAME),
      { GET: { body: MOCK_MIGRATION_FAILED_RETRYABLE }, DELETE: { body: MOCK_MIGRATION_FAILED_RETRYABLE } },
      calls,
      'migration',
      order,
    )
    await recordRoute(
      page,
      API.migrationTemplates,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_TEMPLATE_CREATED } },
      calls,
      'clone-template',
      order,
    )
    await recordRoute(
      page,
      API.migrationPlans,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_PLAN_CREATED } },
      calls,
      'clone-plan',
      order,
    )
    await mockRoute(page, API.networkMappings, 'POST', MOCK_RETRY_NETWORK_MAPPING_CREATED)
    await mockRoute(page, API.storageMappings, 'POST', MOCK_RETRY_STORAGE_MAPPING_CREATED)

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({ timeout: 10_000 })
    await page.getByTestId('migration-form-retry').click()
    await expectDrawerClosed(page)

    // null explicitly clears any stale overrides on the new plan.
    const clonePlanIdx = order.findIndex((o) => o === 'POST clone-plan')
    const clonePlanPost = clonePlanIdx >= 0 ? calls[clonePlanIdx] : undefined
    expect(clonePlanPost).toBeDefined()
    const cloneSpec = clonePlanPost?.body?.spec as Record<string, unknown>
    expect(cloneSpec?.networkOverridesPerVM).toBeNull()
  })

  test('"Assign IP" dialog shows user-assigned IP from original plan, not raw VMware IP', async ({
    page,
  }) => {
    // The IP-override plan is already set in beforeEach.
    // VMware machine CR has ipAddress '192.168.1.150'; the plan override has
    // preserveIP=false + UserAssignedIP='10.0.0.50'. The dialog must show 10.0.0.50.
    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({ timeout: 10_000 })

    await page.getByTestId('bulk-ip-edit-button').click()
    const dialog = page.getByTestId('bulk-ip-dialog')
    await expect(dialog).toBeVisible()

    // User-assigned IP must appear; raw VMware IP must not.
    await expect(dialog).toContainText('10.0.0.50')
    await expect(dialog).not.toContainText('192.168.1.150')
  })
})

// ─── RET-006: multi-VM plan warning banner ────────────────────────────────────

test.describe('RET-006 — multi-VM plan shows shared-plan warning banner', () => {
  test('banner visible when plan contains more than one VM', async ({ page }) => {
    await mockRetryPrefillRoutes(page)
    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await mockRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      'GET',
      MOCK_RETRY_MIGRATION_PLAN_MULTIVM,
    )

    await openRetryDrawer(page)

    const banner = page.getByTestId('retry-multivm-warning-banner')
    await expect(banner).toBeVisible({ timeout: 10_000 })
    await expect(banner).toContainText('2 VMs')
    await expect(banner).toContainText('Shared plan')
  })

  test('banner not shown when plan contains exactly one VM', async ({ page }) => {
    await mockRetryPrefillRoutes(page)
    // Standard single-VM plan — banner must be absent.
    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await expect(page.getByTestId('retry-multivm-warning-banner')).not.toBeVisible()
  })
})

// ─── RET-007: multi-VM plan — PATCH first, then create fresh resources ────────

test.describe('RET-007 — Retry on multi-VM plan patches original plan first', () => {
  // Mounts a multi-VM plan and records all mutating API calls in order.
  async function setupClonePlanRoutes(
    page: Page,
    calls: RecordedCall[],
    order: string[],
  ): Promise<void> {
    await mockRetryPrefillRoutes(page)

    // Override the plan route with the multi-VM variant.
    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_PLAN_MULTIVM },
        PATCH: { body: MOCK_RETRY_MIGRATION_PLAN_MULTIVM },
      },
      calls,
      'original-plan',
      order,
    )

    // Clone MigrationTemplate POST.
    await recordRoute(
      page,
      API.migrationTemplates,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_TEMPLATE_CREATED } },
      calls,
      'clone-template',
      order,
    )

    // Clone MigrationPlan POST.
    await recordRoute(
      page,
      API.migrationPlans,
      { POST: { status: 201, body: MOCK_RETRY_CLONE_PLAN_CREATED } },
      calls,
      'clone-plan',
      order,
    )

    // New mapping resources.
    await recordRoute(
      page,
      API.networkMappings,
      { POST: { status: 201, body: MOCK_RETRY_NETWORK_MAPPING_CREATED } },
      calls,
      'networkmapping',
      order,
    )
    await recordRoute(
      page,
      API.storageMappings,
      { POST: { status: 201, body: MOCK_RETRY_STORAGE_MAPPING_CREATED } },
      calls,
      'storagemapping',
      order,
    )

    // Failed Migration DELETE — returns 404 on subsequent GETs so pollUntilGone() terminates.
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRouteGone404AfterDelete(
      page,
      API.migrationByName(MOCK_RETRY_MIGRATION_NAME),
      {
        GET: { body: MOCK_MIGRATION_FAILED_RETRYABLE },
        DELETE: { body: MOCK_MIGRATION_FAILED_RETRYABLE },
      },
      calls,
      'migration',
      order,
    )
  }

  test('PATCHes original plan first, DELETEs migration, then POSTs new template and plan', async ({
    page,
  }) => {
    const calls: RecordedCall[] = []
    const order: string[] = []

    await setupClonePlanRoutes(page, calls, order)

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-retry').click()
    await expectDrawerClosed(page)

    const writes = order.filter((o) => !o.startsWith('GET'))

    // Original plan PATCHed first to remove the retrying VM.
    expect(writes[0]).toBe('PATCH original-plan')
    // Migration deleted second.
    expect(writes[1]).toBe('DELETE migration')
    // New template must be POSTed before new plan.
    expect(writes.indexOf('POST clone-template')).toBeLessThan(writes.indexOf('POST clone-plan'))
    // New plan is the last write.
    expect(writes[writes.length - 1]).toBe('POST clone-plan')
  })

  test('new plan spec has a single VM (the retrying VM)', async ({ page }) => {
    const calls: RecordedCall[] = []
    const order: string[] = []

    await setupClonePlanRoutes(page, calls, order)

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-retry').click()
    await expectDrawerClosed(page)

    const clonePlanIdx = order.findIndex((o) => o === 'POST clone-plan')
    const clonePlanPost = clonePlanIdx >= 0 ? calls[clonePlanIdx] : undefined
    expect(clonePlanPost).toBeDefined()

    const cloneSpec = clonePlanPost?.body?.spec as Record<string, unknown> | undefined
    const vms = cloneSpec?.virtualMachines as string[][] | undefined
    expect(vms?.flat()).toHaveLength(1)
    expect(vms?.flat()[0]).toBe(MOCK_RETRY_VM_KEY)
  })

  test('original plan PATCH removes the retrying VM from virtualMachines', async ({ page }) => {
    const calls: RecordedCall[] = []
    const order: string[] = []

    await setupClonePlanRoutes(page, calls, order)

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-retry').click()
    await expectDrawerClosed(page)

    const patchCall = calls.find(
      (c) => c.method === 'PATCH' && (c.body?.spec as Record<string, unknown> | undefined)?.virtualMachines,
    )
    expect(patchCall).toBeDefined()

    const updatedVMs = (
      (patchCall?.body?.spec as Record<string, unknown> | undefined)
        ?.virtualMachines as string[][] | undefined
    )?.flat() ?? []
    // Retrying VM must be removed; other VM must remain.
    expect(updatedVMs).not.toContain(MOCK_RETRY_VM_KEY)
    expect(updatedVMs).toContain('other-vm-9999')
  })

  test('PATCH failure shows error and does not POST new resources', async ({ page }) => {
    const calls: RecordedCall[] = []
    const order: string[] = []

    await setupClonePlanRoutes(page, calls, order)

    // Override the plan route so PATCH fails with 409.
    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await page.route(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME), (route: Route) => {
      const method = route.request().method()
      if (method === 'GET') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_RETRY_MIGRATION_PLAN_MULTIVM),
        })
      } else if (method === 'PATCH') {
        calls.push({ method, body: route.request().postDataJSON() ?? undefined })
        order.push('PATCH original-plan')
        route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ message: 'Conflict' }) })
      } else {
        route.continue()
      }
    })

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-retry').click()

    // Drawer stays open on error.
    await expect(page.getByTestId('migration-form-drawer')).toBeVisible()
    await expect(page.getByTestId('retry-error-message')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('retry-error-message')).toContainText(
      'Failed to apply edits and retry',
    )

    const writes = order.filter((o) => !o.startsWith('GET'))
    // PATCH was attempted.
    expect(writes).toContain('PATCH original-plan')
    // No new resources POSTed — PATCH failed before step 2 (migration delete) and beyond.
    expect(writes.filter((w) => w.startsWith('POST'))).toHaveLength(0)
    // Migration NOT deleted.
    expect(writes.filter((w) => w === 'DELETE migration')).toHaveLength(0)
  })
})

// ─── RET-008: bulk retry ───────────────────────────────────────────────────────

test.describe('RET-008 — bulk retry from the migrations table', () => {
  // Sets up the page with two Failed+retryable migrations and wires up DELETE mocks.
  async function setupBulkRetry(
    page: Page,
    migration1Calls: string[],
    migration2Calls: string[],
  ): Promise<void> {
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_MULTI_FAILED)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await page.route(
      API.migrationByName(MOCK_RETRY_MIGRATION_NAME),
      (route) => {
        const method = route.request().method()
        if (method === 'DELETE') {
          migration1Calls.push('DELETE migration-1')
          route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
        } else {
          route.continue()
        }
      },
    )

    await page.route(
      API.migrationByName(MOCK_RETRY_MIGRATION_NAME_2),
      (route) => {
        const method = route.request().method()
        if (method === 'DELETE') {
          migration2Calls.push('DELETE migration-2')
          route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
        } else {
          route.continue()
        }
      },
    )
  }

  test('"Retry Selected" button appears only when all selected rows are Failed+retryable', async ({
    page,
  }) => {
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_MULTI_FAILED)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await goToMigrations(page)

    // Select first row — it's Failed+retryable → button should appear
    const row1 = page.getByRole('row').filter({ hasText: 'test-vm-retry' }).first()
    await row1.locator('input[type="checkbox"]').click()
    await expect(page.getByTestId('bulk-retry-button')).toBeVisible()
    await expect(page.getByTestId('bulk-retry-button')).toContainText('Retry Selected (1)')
  })

  test('bulk retry DELETEs each migration and makes no plan PATCH or POST calls', async ({
    page,
  }) => {
    const migration1Calls: string[] = []
    const migration2Calls: string[] = []
    const writes: string[] = []

    await setupBulkRetry(page, migration1Calls, migration2Calls)

    // Record any write to plans/templates (must not happen in bulk retry).
    await page.route(`**/${NS}/migrationplans**`, (route) => {
      const method = route.request().method()
      if (method !== 'GET') writes.push(`${method} plan-endpoint`)
      route.continue()
    })
    await page.route(`**/${NS}/migrationtemplates**`, (route) => {
      const method = route.request().method()
      if (method !== 'GET') writes.push(`${method} template-endpoint`)
      route.continue()
    })

    await goToMigrations(page)

    // Select both rows.
    const rows = page.getByRole('row').filter({ hasText: /test-vm-retry/ })
    await rows.first().locator('input[type="checkbox"]').click()
    await rows.last().locator('input[type="checkbox"]').click()

    await expect(page.getByTestId('bulk-retry-button')).toBeVisible()
    await page.getByTestId('bulk-retry-button').click()

    // Confirm in dialog.
    await expect(page.getByTestId('confirm-bulk-retry-button')).toBeVisible()
    await page.getByTestId('confirm-bulk-retry-button').click()

    // Both migration DELETEs must fire.
    await expect(async () => {
      expect(migration1Calls).toContain('DELETE migration-1')
      expect(migration2Calls).toContain('DELETE migration-2')
    }).toPass({ timeout: 10_000 })

    // No plan or template writes of any kind.
    expect(writes.filter((w) => !w.startsWith('GET'))).toHaveLength(0)
  })

  test('"Retry Selected" button is absent when selection includes a non-Failed migration', async ({
    page,
  }) => {
    // Mix: one Failed retryable + one Running migration.
    const mixedList = {
      ...MOCK_MIGRATIONS_LIST_MULTI_FAILED,
      items: [MOCK_MIGRATION_FAILED_RETRYABLE, { ...MOCK_MIGRATION_FAILED_RETRYABLE_2, status: { phase: 'CopyingBlocks', conditions: [] } }],
    }
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', mixedList)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)

    await goToMigrations(page)

    // Select both rows (one Failed, one Running).
    const rows = page.getByRole('row').filter({ hasText: /test-vm-retry/ })
    await rows.first().locator('input[type="checkbox"]').click()
    await rows.last().locator('input[type="checkbox"]').click()

    // Button must not appear because not ALL selected are retryable Failed.
    await expect(page.getByTestId('bulk-retry-button')).not.toBeVisible()
  })
})
