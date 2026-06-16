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
} from './helpers/migration.fixtures'

const RETRY_ANNOTATION = 'vjailbreak.k8s.pf9.io/retry-requested'

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

  test('annotates the plan then deletes the migration, with no config writes', async ({
    page,
  }) => {
    const planCalls: RecordedCall[] = []
    const migrationCalls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_PLAN },
        PATCH: { body: MOCK_RETRY_MIGRATION_PLAN },
      },
      planCalls,
      'plan',
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
      migrationCalls,
      'migration',
      order,
    )

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-retry-without-edit')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-retry-without-edit').click()

    await expectDrawerClosed(page)

    // Exactly one PATCH (the retry annotation) and one DELETE, in that order.
    const patches = planCalls.filter((c) => c.method === 'PATCH')
    expect(patches).toHaveLength(1)
    const annotations = (patches[0].body?.metadata as Record<string, unknown> | undefined)
      ?.annotations as Record<string, string> | undefined
    expect(annotations?.[RETRY_ANNOTATION]).toBe('true')
    // No spec edits in retry-without-editing.
    expect(patches[0].body?.spec).toBeUndefined()

    expect(migrationCalls.filter((c) => c.method === 'DELETE')).toHaveLength(1)
    const writeOrder = order.filter((o) => !o.startsWith('GET'))
    expect(writeOrder).toEqual(['PATCH plan', 'DELETE migration'])
  })
})

// ─── RET-003: edit and retry ──────────────────────────────────────────────────

test.describe('RET-003 — edit & retry persists edits before retrying', () => {
  test.beforeEach(async ({ page }) => {
    await mockRetryPrefillRoutes(page)
  })

  test('creates mappings, patches template and plan, then annotates and deletes', async ({
    page,
  }) => {
    const planCalls: RecordedCall[] = []
    const migrationCalls: RecordedCall[] = []
    const templateCalls: RecordedCall[] = []
    const networkMappingCalls: RecordedCall[] = []
    const storageMappingCalls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_PLAN },
        PATCH: { body: MOCK_RETRY_MIGRATION_PLAN },
      },
      planCalls,
      'plan',
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
      migrationCalls,
      'migration',
      order,
    )
    await page.unroute(API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME))
    await recordRoute(
      page,
      API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
        PATCH: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
      },
      templateCalls,
      'template',
      order,
    )
    await recordRoute(
      page,
      API.networkMappings,
      { POST: { status: 201, body: MOCK_RETRY_NETWORK_MAPPING_CREATED } },
      networkMappingCalls,
      'networkmapping',
      order,
    )
    await recordRoute(
      page,
      API.storageMappings,
      { POST: { status: 201, body: MOCK_RETRY_STORAGE_MAPPING_CREATED } },
      storageMappingCalls,
      'storagemapping',
      order,
    )

    await openRetryDrawer(page)
    const editAndRetry = page.getByTestId('migration-form-edit-and-retry')
    await expect(editAndRetry).toBeEnabled({ timeout: 10_000 })
    await editAndRetry.click()

    await expectDrawerClosed(page)

    // New mapping resources created.
    expect(networkMappingCalls.filter((c) => c.method === 'POST')).toHaveLength(1)
    expect(storageMappingCalls.filter((c) => c.method === 'POST')).toHaveLength(1)

    // Template re-pointed at the new mappings.
    const templatePatch = templateCalls.find((c) => c.method === 'PATCH')
    expect(templatePatch).toBeDefined()
    const templateSpec = templatePatch?.body?.spec as Record<string, unknown>
    expect(templateSpec.networkMapping).toBe('new-netmap-uuid-1')
    expect(templateSpec.storageMapping).toBe('new-stormap-uuid-1')

    // Plan received a spec patch and the retry annotation.
    const planPatches = planCalls.filter((c) => c.method === 'PATCH')
    expect(planPatches).toHaveLength(2)
    const specPatch = planPatches.find((c) => c.body?.spec)
    const strategy = (specPatch?.body?.spec as Record<string, any>)?.migrationStrategy
    expect(strategy?.type).toBe('cold')
    const annotationPatch = planPatches.find(
      (c) =>
        ((c.body?.metadata as Record<string, unknown> | undefined)?.annotations as
          | Record<string, string>
          | undefined)?.[RETRY_ANNOTATION] === 'true',
    )
    expect(annotationPatch).toBeDefined()

    // The migration is deleted last, after all configuration writes.
    expect(migrationCalls.filter((c) => c.method === 'DELETE')).toHaveLength(1)
    const writeOrder = order.filter((o) => !o.startsWith('GET'))
    expect(writeOrder[writeOrder.length - 1]).toBe('DELETE migration')
    expect(writeOrder.indexOf('PATCH template')).toBeLessThan(
      writeOrder.indexOf('DELETE migration'),
    )
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

  test('plan PATCH includes networkOverridesPerVM when existing override loaded from plan', async ({
    page,
  }) => {
    const planCalls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_PLAN_WITH_IP_OVERRIDE },
        PATCH: { body: MOCK_RETRY_MIGRATION_PLAN_WITH_IP_OVERRIDE },
      },
      planCalls,
      'plan',
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
      [],
      'migration',
      order,
    )
    await page.unroute(API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME))
    await recordRoute(
      page,
      API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
        PATCH: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
      },
      [],
      'template',
      order,
    )
    await mockRoute(page, API.networkMappings, 'POST', MOCK_RETRY_NETWORK_MAPPING_CREATED)
    await mockRoute(page, API.storageMappings, 'POST', MOCK_RETRY_STORAGE_MAPPING_CREATED)

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-edit-and-retry').click()
    await expectDrawerClosed(page)

    // The spec patch on the plan must carry networkOverridesPerVM with the
    // pre-existing override (prefill restored it into form state → buildPlanPatchSpec picked it up).
    const specPatch = planCalls.filter((c) => c.method === 'PATCH').find((c) => c.body?.spec)
    expect(specPatch).toBeDefined()
    const planSpec = specPatch?.body?.spec as Record<string, unknown>
    const overrides = planSpec?.networkOverridesPerVM as Record<string, unknown> | null | undefined
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

  test('plan PATCH sends networkOverridesPerVM: null when plan had no overrides', async ({
    page,
  }) => {
    // Use the standard plan (no networkOverridesPerVM) and verify the patch explicitly
    // clears overrides so stale values from a previous retry are not kept.
    const planCalls: RecordedCall[] = []
    const order: string[] = []

    await page.unroute(API.migrationPlanByName(MOCK_RETRY_PLAN_NAME))
    await recordRoute(
      page,
      API.migrationPlanByName(MOCK_RETRY_PLAN_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_PLAN },
        PATCH: { body: MOCK_RETRY_MIGRATION_PLAN },
      },
      planCalls,
      'plan',
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
      [],
      'migration',
      order,
    )
    await page.unroute(API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME))
    await recordRoute(
      page,
      API.migrationTemplateByName(MOCK_RETRY_TEMPLATE_NAME),
      {
        GET: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
        PATCH: { body: MOCK_RETRY_MIGRATION_TEMPLATE },
      },
      [],
      'template',
      order,
    )
    await mockRoute(page, API.networkMappings, 'POST', MOCK_RETRY_NETWORK_MAPPING_CREATED)
    await mockRoute(page, API.storageMappings, 'POST', MOCK_RETRY_STORAGE_MAPPING_CREATED)

    await openRetryDrawer(page)
    await expect(page.getByTestId('migration-form-edit-and-retry')).toBeEnabled({
      timeout: 10_000,
    })
    await page.getByTestId('migration-form-edit-and-retry').click()
    await expectDrawerClosed(page)

    const specPatch = planCalls.filter((c) => c.method === 'PATCH').find((c) => c.body?.spec)
    expect(specPatch).toBeDefined()
    const planSpec = specPatch?.body?.spec as Record<string, unknown>
    // null explicitly clears any stale overrides on the server via merge-patch semantics.
    expect(planSpec?.networkOverridesPerVM).toBeNull()
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
