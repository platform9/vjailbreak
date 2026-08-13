import { test, expect } from '@playwright/test'

import { goToMigrationDetail } from '../helpers/migration.helpers'
import {
  MOCK_MIGRATION_RUNNING,
  MOCK_MIGRATION_SUCCEEDED,
  MOCK_MIGRATION_FAILED_RDM_NOT_RETRYABLE,
} from '../helpers/migration.fixtures'

// ─── MDP-019: Related-resource 404s fall back gracefully in the KPI strip ─────

test.describe('MDP-019 — KPI strip falls back when related resources 404', () => {
  test('Source Cluster / Destination Cluster / Destination Tenant / Agent show fallback values', async ({
    page,
  }) => {
    // goToMigrationDetail's default (no `resources` option) 404s every related
    // resource lookup — vmwareCreds, openstackCreds, pcdClusters, vmwareMachine, etc.
    await goToMigrationDetail(page, MOCK_MIGRATION_RUNNING)

    await expect(page.getByTestId('migration-detail-page')).toBeVisible()
    await expect(page.getByTestId('kpi-cell-source-cluster')).toContainText('No cluster')
    await expect(page.getByTestId('kpi-cell-destination-cluster')).toContainText('—')
    await expect(page.getByTestId('kpi-cell-destination-tenant')).toContainText('—')
    await expect(page.getByTestId('kpi-cell-agent')).toContainText('—')
  })
})

// ─── MDP-020: Retry disabled with tooltip when status.retryable === false ────

test.describe('MDP-020 — retry disabled for non-retryable (RDM) failures', () => {
  test('Retry button is disabled and shows the RDM tooltip', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_FAILED_RDM_NOT_RETRYABLE)

    const retryButton = page.getByTestId('retry-migration-button')
    await expect(retryButton).toBeDisabled()

    await retryButton.hover({ force: true })
    await expect(
      page.getByRole('tooltip', { name: /cannot be retried because the VM has RDM disks/i }),
    ).toBeVisible()
  })
})

// ─── MDP-021: Events tab true empty state (no conditions, no creationTimestamp)

test.describe('MDP-021 — events tab empty state', () => {
  test('shows the empty-state message when there are no conditions at all', async ({ page }) => {
    const migration = {
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'Migration',
      metadata: { name: 'test-vm-no-events-migration', namespace: 'migration-system' },
      spec: { vmName: 'test-vm-no-events' },
      status: { phase: 'Pending', conditions: [] },
    }
    await goToMigrationDetail(page, migration)

    await page.getByTestId('tab-events').click()
    await expect(page.getByTestId('events-tab-empty')).toBeVisible()
    await expect(page.getByTestId('events-tab-empty')).toContainText('No events recorded')
    await expect(page.getByTestId('events-tab-card')).toHaveCount(0)
  })
})

// ─── MDP-022: Succeeded migrations show no header action buttons ─────────────

test.describe('MDP-022 — no action buttons for a Succeeded migration', () => {
  test('Delete, Retry, and Trigger cutover are all absent', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_SUCCEEDED)

    const header = page.getByTestId('migration-detail-header')
    await expect(header.getByTestId('delete-migration-button')).toHaveCount(0)
    await expect(header.getByTestId('retry-migration-button')).toHaveCount(0)
    await expect(header.getByTestId('cutover-trigger-button')).toHaveCount(0)
  })
})

// ─── MDP-023: SuccessDetail stat boxes reflect the migration's own data ──────

test.describe('MDP-023 — success detail stat boxes', () => {
  test('Target VM, Disks migrated, and Agent stat boxes render correctly', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_SUCCEEDED)

    const success = page.getByTestId('phase-detail-success')
    await expect(success).toBeVisible()
    await expect(success).toContainText('test-vm-3 is running in PCD')
    await expect(success).toContainText('Target VM')
    await expect(success).toContainText('Disks migrated')
    // MOCK_MIGRATION_SUCCEEDED has no status.totalDisks / status.agentName
    await expect(success).toContainText('—')
  })
})
