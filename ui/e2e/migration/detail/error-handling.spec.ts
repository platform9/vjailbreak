import { test, expect } from '@playwright/test'

import {
  goToMigrationDetail,
  mockRoute,
  mockRouteError,
  API,
  ROUTES,
} from '../helpers/migration.helpers'
import { MOCK_MIGRATION_RUNNING, MOCK_MIGRATION_PENDING } from '../helpers/migration.fixtures'

// ─── MDP-015: Migration fetch 404s ────────────────────────────────────────────

test.describe('MDP-015 — migration not found', () => {
  test('a 404 on the migration fetch shows the error state instead of crashing', async ({
    page,
  }) => {
    await mockRoute(page, API.migrationByName('missing-migration'), 'GET', {}, 404)
    await page.goto(ROUTES.migrationDetail('missing-migration'))

    // useMigrationDetailQuery doesn't opt out of react-query's default retries
    // (3 attempts, exponential backoff) — the error state only renders once
    // those are exhausted, several seconds after the initial 404.
    await expect(page.getByTestId('migration-detail-error-state')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByTestId('migration-detail-error-state')).toContainText(
      'Failed to load migration',
    )
    await expect(page.getByTestId('migration-detail-page')).toHaveCount(0)
  })
})

// ─── MDP-016: Debug bundle download failure ───────────────────────────────────

test.describe('MDP-016 — debug bundle download failure', () => {
  test('a 500 on the debug-bundle download shows an error toast and re-enables the button', async ({
    page,
  }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_RUNNING)
    await page.route(API.podLogs('migration-system', MOCK_MIGRATION_RUNNING.spec.podRef), (route) =>
      route.fulfill({ status: 200, contentType: 'text/plain', body: '' }),
    )
    await page.getByTestId('tab-pod-logs').click()
    await expect(page.getByTestId('migration-debug-logs')).toBeVisible()

    await mockRouteError(page, '**/dev-api/sdk/vpw/v1/debug-bundle**', 'GET', 500)

    const downloadButton = page.getByTestId('pod-logs-download-button')
    await downloadButton.click()

    await expect(page.getByTestId('pod-logs-download-error-toast')).toContainText(
      'Failed to download debug bundle',
    )
    await expect(downloadButton).toBeEnabled()
  })
})

// ─── MDP-017: Pod log stream disconnect + reconnect ───────────────────────────

test.describe('MDP-017 — pod logs connection error and reconnect', () => {
  test('a failed log stream shows a connection error; Reconnect resumes it', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_RUNNING)

    const podName = MOCK_MIGRATION_RUNNING.spec.podRef
    // A plain boolean, not an attempt counter — React may invoke the connect
    // effect more than once on mount (e.g. StrictMode), so "fail on the very
    // first call only" is flaky. Every request fails until the test itself
    // flips this after asserting the error state.
    let shouldSucceed = false
    await page.route(API.podLogs('migration-system', podName), (route) => {
      if (shouldSucceed) {
        route.fulfill({
          status: 200,
          contentType: 'text/plain',
          body: '10:00:01.000 [v2v-helper] INFO Resumed after reconnect\n',
        })
      } else {
        route.fulfill({ status: 500, body: 'internal error' })
      }
    })

    await page.getByTestId('tab-pod-logs').click()
    await expect(page.getByTestId('pod-logs-connection-error')).toBeVisible()

    shouldSucceed = true
    await page.getByTestId('pod-logs-reconnect-button').click()

    await expect(page.getByTestId('pod-logs-connection-error')).toHaveCount(0)
    await expect(page.getByTestId('pod-logs-stream')).toContainText('Resumed after reconnect')
  })
})

// ─── MDP-018: Delete migration API failure ────────────────────────────────────

test.describe('MDP-018 — delete migration API failure', () => {
  test('a failed migration-plan lookup surfaces inline and keeps the dialog open', async ({
    page,
  }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_PENDING)

    await page.getByTestId('delete-migration-button').click()
    await expect(page.getByTestId('delete-migration-dialog')).toBeVisible()

    // useDeleteMigrations fetches the MigrationPlan before patching it; failing
    // that GET is the simplest way to make the whole delete flow reject.
    await mockRouteError(page, API.migrationPlanByName('test-plan-1'), 'GET', 500)

    await page.getByTestId('confirm-delete-button').click()

    await expect(page.getByTestId('delete-migration-error')).toBeVisible()
    await expect(page.getByTestId('delete-migration-dialog')).toBeVisible()
    // Migration was not removed — still on its detail page
    await expect(page.getByTestId('migration-detail-page')).toBeVisible()
    await expect(page).toHaveURL(new RegExp(MOCK_MIGRATION_PENDING.metadata.name))
  })
})
