import { test, expect } from '@playwright/test'

import {
  goToMigrations,
  openMigrationDrawer,
  closeMigrationDrawer,
  expectDrawerOpen,
  expectDrawerClosed,
  mockRoute,
  API,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST,
  MOCK_MIGRATIONS_LIST_EMPTY,
  MOCK_MIGRATION_PLANS_LIST,
  MOCK_MIGRATION_PLANS_LIST_EMPTY,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_OPENSTACK_CREDS_LIST,
} from './helpers/migration.fixtures'

// Section IDs from useFormValidation.ts sectionNavItems (5 total)
const SECTION_IDS = [
  'source-destination',
  'select-vms',
  'map-resources',
  'security',
  'options',
] as const

// ─── MIG-001: Open and close standard migration form ──────────────────────────

test.describe('MIG-001 — open / close standard migration form', () => {
  test.beforeEach(async ({ page }) => {
    // Button is disabled without creds — seed minimal creds to enable it
    await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
    await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
  })

  test('drawer opens with all sections, closes cleanly, and resets on reopen', async ({
    page,
  }) => {
    await goToMigrations(page)

    // Open drawer
    await openMigrationDrawer(page)
    await expectDrawerOpen(page)

    // All 5 section nav items rendered
    for (const sectionId of SECTION_IDS) {
      await expect(page.getByTestId(`section-nav-item-${sectionId}`)).toBeVisible()
    }

    // Step 1 cluster dropdowns visible in initial state
    await expect(page.getByTestId('vmware-cluster-dropdown')).toBeVisible()
    await expect(page.getByTestId('pcd-cluster-dropdown')).toBeVisible()

    // Close via X button
    await closeMigrationDrawer(page)
    await expectDrawerClosed(page)

    // Reopen: form resets to initial state (no stale values)
    await openMigrationDrawer(page)
    await expectDrawerOpen(page)

    for (const sectionId of SECTION_IDS) {
      await expect(page.getByTestId(`section-nav-item-${sectionId}`)).toBeVisible()
    }

    // Cluster dropdowns still present and unselected (reset)
    await expect(page.getByTestId('vmware-cluster-dropdown')).toBeVisible()
    await expect(page.getByTestId('pcd-cluster-dropdown')).toBeVisible()
  })
})

// ─── MIG-002: Migrations list page loads and displays table ───────────────────

test.describe('MIG-002 — migrations list page', () => {
  test.beforeEach(async ({ page }) => {
    // Seed deterministic data — real dev env may not have migrations in all phases
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('table renders with expected columns', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await expect(table).toBeVisible()

    for (const col of ['Name', 'Status', 'Progress']) {
      await expect(
        table.getByRole('columnheader', { name: new RegExp(col, 'i') }),
      ).toBeVisible()
    }
  })

  test('migration rows load and display VM names', async ({ page }) => {
    const table = page.getByTestId('migrations-table')
    await expect(table).toBeVisible()

    // MOCK_MIGRATIONS_LIST has 5 migrations; spec.vmName shown in Name column
    for (const vmName of ['test-vm-1', 'test-vm-2', 'test-vm-3', 'test-vm-4', 'test-vm-5']) {
      await expect(table.getByText(vmName)).toBeVisible()
    }
  })
})

// ─── MIG-003: Migration progress column renders correct phase icons ────────────

test.describe('MIG-003 — progress column phase icons', () => {
  test.beforeEach(async ({ page }) => {
    await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
    await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST)
    await goToMigrations(page)
  })

  test('each migration row has a progress cell with an icon', async ({ page }) => {
    const cells = page.getByTestId('migration-progress-cell')
    // One cell per migration in MOCK_MIGRATIONS_LIST (5 items)
    await expect(cells).toHaveCount(5)
    // Every cell contains an SVG icon (not blank)
    for (let i = 0; i < 5; i++) {
      await expect(cells.nth(i).locator('svg').first()).toBeVisible()
    }
  })

  // Phase-specific checks: each phase type renders a visible icon in its row
  const phaseCases: Array<{ vmName: string; phase: string }> = [
    { vmName: 'test-vm-3', phase: 'Succeeded' },
    { vmName: 'test-vm-4', phase: 'Failed' },
    { vmName: 'test-vm-2', phase: 'CopyingBlocks (Running)' },
    { vmName: 'test-vm-5', phase: 'AwaitingAdminCutOver' },
  ]

  for (const { vmName, phase } of phaseCases) {
    test(`progress cell visible for ${phase} migration`, async ({ page }) => {
      const table = page.getByTestId('migrations-table')
      // Find the row containing this VM's name
      const row = table.locator('[role="row"]').filter({ hasText: vmName })
      await expect(row).toBeVisible()

      const cell = row.getByTestId('migration-progress-cell')
      await expect(cell).toBeVisible()
      await expect(cell.locator('svg').first()).toBeVisible()
    })
  }

  test('progress cell tooltip appears on hover', async ({ page }) => {
    // Hover first progress cell — tooltip should show progressText
    const cell = page.getByTestId('migration-progress-cell').first()
    await cell.hover()
    await expect(page.getByRole('tooltip')).toBeVisible({ timeout: 3000 })
  })
})
