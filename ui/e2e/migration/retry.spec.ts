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
  MOCK_MIGRATION_FAILED_RETRYABLE,
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
  MOCK_RETRY_PLAN_NAME,
  MOCK_RETRY_TEMPLATE_NAME,
  MOCK_RETRY_VM_K8S_NAME,
  MOCK_RETRY_VM_KEY,
  MOCK_RETRY_CLONE_TEMPLATE_CREATED,
  MOCK_RETRY_CLONE_PLAN_CREATED,
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

    // Dual footer actions
    await expect(page.getByTestId('migration-form-retry-without-edit')).toBeVisible()
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeVisible()
    await expect(page.getByTestId('migration-form-submit')).not.toBeVisible()

    // Edit & Retry becomes enabled once the prefilled config validates
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
  })

  test('both action buttons are disabled while prefill data is loading', async ({ page }) => {
    // Delay the migration fetch so we can observe query.isLoading → buttons disabled.
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

    // While query is loading both action buttons must be disabled.
    await expect(page.getByTestId('migration-form-retry-without-edit')).toBeDisabled()
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeDisabled()

    // Unblock the fetch — form should populate and buttons become enabled.
    resolveMigration()
    await expect(page.getByTestId('migration-form-retry-without-edit')).toBeEnabled({
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

// ─── RET-002: retry without editing ───────────────────────────────────────────

test.describe('RET-002 — retry without editing', () => {
  test.beforeEach(async ({ page }) => {
    await mockRetryPrefillRoutes(page)
  })

  test('deletes the migration only — no writes to plan, template, or mappings', async ({
    page,
  }) => {
    const migrationCalls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRoute(
      page,
      API.migrationByName(MOCK_RETRY_MIGRATION_NAME),
      {
        GET: { body: MOCK_MIGRATION_FAILED_RETRYABLE },
        DELETE: { body: MOCK_MIGRATION_FAILED_RETRYABLE },
      },
      migrationCalls,
      'migration',
      order,
    )

    // Track any write other than DELETE — none should occur.
    const unexpectedWrites: string[] = []
    await page.route('**/apis/vjailbreak.k8s.pf9.io/**', (route) => {
      const method = route.request().method()
      if (method !== 'GET' && method !== 'DELETE') {
        unexpectedWrites.push(`${method} ${route.request().url()}`)
      }
      route.fallback()
    })

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry-without-edit')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-retry-without-edit').click()
    await expectDrawerClosed(page)

    // Exactly one DELETE on the Migration; no PATCHes or POSTs on any resource.
    expect(migrationCalls.filter((c) => c.method === 'DELETE')).toHaveLength(1)
    expect(unexpectedWrites).toHaveLength(0)
  })

  test('both buttons are disabled while retry-without-edit mutation is pending', async ({
    page,
  }) => {
    // Hold the Migration DELETE so mutation.isPending stays true long enough to assert.
    let resolveDelete!: () => void
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await page.route(API.migrationByName(MOCK_RETRY_MIGRATION_NAME), async (route) => {
      const method = route.request().method()
      if (method === 'GET') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_MIGRATION_FAILED_RETRYABLE),
        })
      } else if (method === 'DELETE') {
        await new Promise<void>((r) => { resolveDelete = r })
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_MIGRATION_FAILED_RETRYABLE),
        })
      } else {
        route.continue()
      }
    })

    await openRetryDrawer(page)
    const retryBtn = page.getByTestId('migration-form-retry-without-edit')
    await expect(retryBtn).toBeEnabled({ timeout: 10_000 })
    await retryBtn.click()

    // While mutation isPending, both action buttons must be disabled.
    await expect(retryBtn).toBeDisabled()
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeDisabled()

    // Unblock so the mutation settles and the drawer closes.
    resolveDelete()
    await expectDrawerClosed(page)
  })
})

// ─── RET-003: edit and retry (single-VM plan) ────────────────────────────────

test.describe('RET-003 — edit & retry deletes old plan and migration first, then creates fresh resources (single-VM plan)', () => {
  test.beforeEach(async ({ page }) => {
    await mockRetryPrefillRoutes(page)
  })

  // Single-VM Edit & Retry:
  //   DELETE old plan → DELETE old migration →
  //   POST new mappings → POST new template → POST new plan
  // No PATCH on the original plan (it's deleted entirely since there's only one VM).
  test('DELETEs old plan and migration first, then POSTs new template and plan', async ({
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
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRoute(
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
    const editAndRetry = page.getByTestId('migration-form-edit-and-retry')
    await expect(editAndRetry).toBeEnabled({ timeout: 10_000 })
    await editAndRetry.click()
    await expectDrawerClosed(page)

    const writes = order.filter((o) => !o.startsWith('GET'))

    // Old plan deleted first (only VM → delete whole plan).
    expect(writes[0]).toBe('DELETE original-plan')
    // Migration deleted second.
    expect(writes[1]).toBe('DELETE migration')

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

// ─── RET-004: blocking errors ─────────────────────────────────────────────────

test.describe('RET-004 — missing resources block the retry', () => {
  test('missing migration template shows blocking banner and disables both actions', async ({
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
    await expect(page.getByTestId('migration-form-retry-without-edit')).toBeDisabled()
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeDisabled()
  })

  test('missing credentials show blocking banner naming the credential', async ({ page }) => {
    await mockRetryPrefillRoutes(page)
    await page.unroute(API.vmwareCredByName('vcenter-cred-1'))
    await mockRouteError(page, API.vmwareCredByName('vcenter-cred-1'), 'GET', 404, 'not found')

    await openRetryDrawer(page)

    await expect(page.getByTestId('retry-blocking-banner')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('retry-blocking-banner')).toContainText('vcenter-cred-1')
    await expect(page.getByTestId('migration-form-retry-without-edit')).toBeDisabled()
  })
})

// ─── RET-005: IP overrides round-tripped through the retry form ───────────────

test.describe('RET-005 — IP overrides are preserved and sent on Edit & Retry', () => {
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
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRoute(
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
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({ timeout: 10_000 })
    await page.getByTestId('migration-form-edit-and-retry').click()
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
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRoute(
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
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({ timeout: 10_000 })
    await page.getByTestId('migration-form-edit-and-retry').click()
    await expectDrawerClosed(page)

    // null explicitly clears any stale overrides on the new plan.
    const clonePlanIdx = order.findIndex((o) => o === 'POST clone-plan')
    const clonePlanPost = clonePlanIdx >= 0 ? calls[clonePlanIdx] : undefined
    expect(clonePlanPost).toBeDefined()
    const cloneSpec = clonePlanPost?.body?.spec as Record<string, unknown>
    expect(cloneSpec?.networkOverridesPerVM).toBeNull()
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
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await expect(page.getByTestId('retry-multivm-warning-banner')).not.toBeVisible()
  })
})

// ─── RET-007: multi-VM plan — PATCH first, then create fresh resources ────────

test.describe('RET-007 — Edit & Retry on multi-VM plan patches original plan first', () => {
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

    // Failed Migration DELETE.
    await page.unroute(API.migrationByName(MOCK_RETRY_MIGRATION_NAME))
    await recordRoute(
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
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-edit-and-retry').click()
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
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-edit-and-retry').click()
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
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-edit-and-retry').click()
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
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-edit-and-retry').click()

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
