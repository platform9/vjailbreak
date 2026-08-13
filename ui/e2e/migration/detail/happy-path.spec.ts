import { test, expect } from '@playwright/test'

import { goToMigrationDetail, mockRoute, API, ROUTES, NS } from '../helpers/migration.helpers'
import {
  MOCK_MIGRATION_PENDING,
  MOCK_MIGRATION_RUNNING,
  MOCK_MIGRATION_SUCCEEDED,
  MOCK_MIGRATION_FAILED,
  MOCK_MIGRATION_AWAITING_CUTOVER,
  MOCK_MIGRATION_CONVERTING_DISK,
  MOCK_MIGRATION_FOR_DETAILS_TAB,
  MOCK_MIGRATION_PLAN_1,
  MOCK_MIGRATION_TEMPLATE_READY,
  MOCK_VMWARE_MACHINE_FOR_DETAILS_TAB,
} from '../helpers/migration.fixtures'

// ─── MDP-003: Breadcrumb back navigation ──────────────────────────────────────

test.describe('MDP-003 — breadcrumb back navigation', () => {
  test('clicking "Migrations" in the breadcrumb navigates back to the list', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_SUCCEEDED)
    await mockRoute(page, API.migrations, 'GET', { items: [] })

    await page.getByTestId('breadcrumb-back-link').click()
    await page.waitForURL(new RegExp(ROUTES.migrations))
    await expect(page.getByTestId('migrations-table')).toBeVisible()
  })
})

// ─── MDP-004: Stepper renders the correct step/status per phase ──────────────

test.describe('MDP-004 — phase stepper reflects migration phase', () => {
  const cases = [
    { migration: MOCK_MIGRATION_PENDING, stepKey: 'pending', status: 'active' },
    { migration: MOCK_MIGRATION_RUNNING, stepKey: 'copying', status: 'active' },
    { migration: MOCK_MIGRATION_AWAITING_CUTOVER, stepKey: 'cutover', status: 'paused' },
    { migration: MOCK_MIGRATION_CONVERTING_DISK, stepKey: 'converting', status: 'active' },
    { migration: MOCK_MIGRATION_SUCCEEDED, stepKey: 'done', status: 'done' },
  ]

  for (const { migration, stepKey, status } of cases) {
    test(`${migration.status.phase} phase highlights the "${stepKey}" step as ${status}`, async ({
      page,
    }) => {
      await goToMigrationDetail(page, migration)
      await expect(page.getByTestId(`stepper-step-${stepKey}`)).toHaveAttribute(
        'data-status',
        status,
      )
    })
  }
})

// ─── MDP-005: Details tab renders all sections ────────────────────────────────

test.describe('MDP-005 — details tab sections', () => {
  test('Environment, General Info, Mappings, and Policies sections all render', async ({
    page,
  }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_FOR_DETAILS_TAB, {
      resources: {
        migrationPlan: MOCK_MIGRATION_PLAN_1,
        migrationTemplate: MOCK_MIGRATION_TEMPLATE_READY,
        vmwareMachine: MOCK_VMWARE_MACHINE_FOR_DETAILS_TAB,
      },
    })

    await page.getByTestId('tab-details').click()
    await expect(page.getByTestId('migration-details-tab')).toBeVisible()

    for (const section of [
      'details-section-environment',
      'details-section-general-info',
      'details-section-mappings',
      'details-section-policies',
    ]) {
      await expect(page.getByTestId(section)).toBeVisible()
    }
    // No image profiles configured on the template — section should be absent
    await expect(page.getByTestId('details-section-image-profiles')).toHaveCount(0)
  })
})

// ─── MDP-006: Events tab — search, filter, sort ───────────────────────────────

test.describe('MDP-006 — events tab', () => {
  test('conditions render and can be searched, filtered, and sorted', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_FAILED)
    await page.getByTestId('tab-events').click()

    await expect(page.getByTestId('migration-events-tab')).toBeVisible()
    await expect(page.getByTestId('events-tab-card')).toHaveCount(2) // synthetic "Created" + the Failed condition

    await page.getByTestId('events-search-input').locator('input').fill('connection timeout')
    await expect(page.getByTestId('events-tab-card')).toHaveCount(1)
    await page.getByTestId('events-search-input').locator('input').fill('')

    // Status ToggleButtonGroup order is All / success / error / pending; the
    // success/error/pending buttons render only an icon + count, no text label.
    const statusFilter = page.getByTestId('events-status-filter')
    await statusFilter.getByRole('button').nth(2).click() // error
    await expect(page.getByTestId('events-tab-card')).toHaveCount(1)
    await statusFilter.getByRole('button').nth(0).click() // all

    await page.getByTestId('events-sort-toggle').getByRole('button', { name: 'Newest first' }).click()
    await expect(page.getByTestId('events-tab-card')).toHaveCount(2)
    // Failed condition (10:30) is newer than the synthetic Created entry (10:00)
    await expect(page.getByTestId('events-tab-card').first()).toContainText('Disk copy failed')
  })
})

// ─── MDP-007: Pod logs tab ─────────────────────────────────────────────────────

test.describe('MDP-007 — pod logs tab', () => {
  test('logs stream in and can be searched and filtered by level', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_RUNNING)

    const podName = MOCK_MIGRATION_RUNNING.spec.podRef
    const logLines = [
      '10:00:01.123 [v2v-helper] INFO Starting migration',
      '10:00:02.456 [v2v-helper] ERROR Disk copy failed',
      '10:00:03.789 [nbdkit] WARN Retry connection',
    ]
    await page.route(API.podLogs(NS, podName), (route) =>
      route.fulfill({ status: 200, contentType: 'text/plain', body: logLines.join('\n') + '\n' }),
    )

    await page.getByTestId('tab-pod-logs').click()
    await expect(page.getByTestId('migration-debug-logs')).toBeVisible()
    await expect(page.getByTestId('pod-logs-stream')).toContainText('Starting migration')
    await expect(page.getByTestId('pod-logs-stream')).toContainText('Disk copy failed')

    await page.getByTestId('pod-logs-search').locator('input').fill('Retry connection')
    await expect(page.getByTestId('pod-logs-stream')).toContainText('Retry connection')
    await expect(page.getByTestId('pod-logs-stream')).not.toContainText('Starting migration')
    await page.getByTestId('pod-logs-search').locator('input').fill('')

    await page.getByTestId('pod-logs-level-select').click()
    await page.getByRole('option', { name: 'ERROR' }).click()
    await expect(page.getByTestId('pod-logs-stream')).toContainText('Disk copy failed')
    await expect(page.getByTestId('pod-logs-stream')).not.toContainText('Starting migration')
  })
})

// ─── MDP-008: Delete migration from header ────────────────────────────────────

test.describe('MDP-008 — delete migration from header', () => {
  test('confirming delete removes the migration and navigates back to the list', async ({
    page,
  }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_PENDING)

    await page.getByTestId('delete-migration-button').click()
    await expect(page.getByTestId('delete-migration-dialog')).toBeVisible()

    // A single handler per URL dispatching on method — registering separate
    // mockRoute() calls for GET vs. PATCH/DELETE on the *same* URL doesn't work:
    // Playwright matches the most-recently-added page.route handler first, and
    // that handler's route.continue() (on a method mismatch) goes straight to
    // the real network instead of falling through to the earlier-registered
    // handler for the other method.
    await page.route(API.migrationPlanByName('test-plan-1'), (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_MIGRATION_PLAN_1),
      }),
    )
    await page.route(API.migrationByName(MOCK_MIGRATION_PENDING.metadata.name), (route) => {
      if (route.request().method() === 'DELETE') {
        route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
      } else {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_PENDING) })
      }
    })
    await mockRoute(page, API.migrations, 'GET', { items: [] })

    await page.getByTestId('confirm-delete-button').click()

    await page.waitForURL(new RegExp(ROUTES.migrations))
    await expect(page.getByTestId('migrations-table')).toBeVisible()
  })
})

// ─── MDP-009: Retry button opens the retry form ───────────────────────────────

test.describe('MDP-009 — retry migration', () => {
  test('Retry button opens the migration form drawer', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_FAILED)

    const retryButton = page.getByTestId('retry-migration-button')
    await expect(retryButton).toBeEnabled()
    await retryButton.click()

    await expect(page.getByTestId('migration-form-drawer')).toBeVisible()
  })
})

// ─── MDP-010: Trigger admin cutover ───────────────────────────────────────────

test.describe('MDP-010 — trigger admin cutover', () => {
  test('confirming cutover flips the Cutover step from paused to active', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_AWAITING_CUTOVER)
    await expect(page.getByTestId('stepper-step-cutover')).toHaveAttribute('data-status', 'paused')

    const podName = MOCK_MIGRATION_AWAITING_CUTOVER.spec.podRef
    const resolvedPodName = `${podName}-abcde`
    await mockRoute(page, API.k8sPods(NS), 'GET', {
      items: [{ metadata: { name: resolvedPodName, namespace: NS } }],
    })
    await mockRoute(page, API.k8sPodByName(NS, resolvedPodName), 'PATCH', {})

    // Both the header and the "Ready for Final Cutover" overview card render their
    // own TriggerAdminCutoverButton — scope to the header to avoid a strict-mode
    // ambiguity between the two.
    await page.getByTestId('migration-detail-header').getByTestId('cutover-trigger-button').click()
    await page.getByTestId('cutover-confirm-button').click()

    await expect(page.getByRole('dialog')).not.toBeVisible()
    await expect(page.getByTestId('stepper-step-cutover')).toHaveAttribute('data-status', 'active')
  })
})
