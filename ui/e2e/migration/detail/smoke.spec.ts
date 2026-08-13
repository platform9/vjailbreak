import { test, expect } from '@playwright/test'

import { goToMigrations, goToMigrationDetail, mockRoute, API } from '../helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST,
  MOCK_MIGRATION_PLANS_LIST,
  MOCK_MIGRATION_SUCCEEDED,
} from '../helpers/migration.fixtures'

// ─── MDP-001: Detail page loads ────────────────────────────────────────────────

test.describe('MDP-001 — migration detail page loads', () => {
  test.beforeEach(async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_SUCCEEDED)
  })

  test('header, KPI strip, and tabs render with no console errors', async ({ page }) => {
    // Related-resource lookups (vmwareCreds, migrationPlan, …) 404 by default —
    // see mock404DetailResourcesIfUnhandled — and useMigrationDetailResourcesQuery's
    // safeGet() logs them via console.error before falling back to null. That noise
    // is expected here; only flag errors outside that known, non-fatal pattern.
    const unexpectedConsoleErrors: string[] = []
    page.on('console', (msg) => {
      if (msg.type() !== 'error') return
      const text = msg.text()
      if (text.includes('404') || text.includes('Error in safeGet')) return
      unexpectedConsoleErrors.push(text)
    })

    await expect(page.getByTestId('migration-detail-header')).toBeVisible()
    await expect(page.getByTestId('migration-kpi-strip')).toBeVisible()
    await expect(page.getByTestId('migration-detail-tabs')).toBeVisible()

    for (const tabId of ['tab-overview', 'tab-details', 'tab-events', 'tab-pod-logs']) {
      await expect(page.getByTestId(tabId)).toBeVisible()
    }
    // AI Analysis tab only shows for failed/validation-failed migrations
    await expect(page.getByTestId('tab-ai-analysis')).toHaveCount(0)

    await expect(page.getByTestId('migration-phase-stepper')).toBeVisible()

    expect(unexpectedConsoleErrors).toEqual([])
  })
})

// ─── MDP-002: Navigate from MigrationsTable row to detail page ────────────────

test.describe('MDP-002 — navigate to detail page from migrations table', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await mockRoute(page, API.migrationByName('test-vm-3-migration'), 'GET', MOCK_MIGRATION_SUCCEEDED)
    await goToMigrations(page)
  })

  test('clicking a row navigates to the migration detail page', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    const row = table.locator('[role="row"]').filter({ hasText: 'test-vm-3' })
    await row.click()

    await page.waitForURL(/\/dashboard\/migrations\/test-vm-3-migration/)
    await expect(page.getByTestId('migration-detail-page')).toBeVisible()
    await expect(page.getByTestId('migration-detail-header')).toContainText('test-vm-3')
  })
})
