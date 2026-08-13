import { test, expect } from '@playwright/test'

import { goToMigrationDetail } from '../helpers/migration.helpers'
import {
  MOCK_MIGRATION_FAILED,
  MOCK_MIGRATION_VALIDATION_FAILED,
  MOCK_MIGRATION_CONVERTING_DISK,
  MOCK_MIGRATION_RUNNING,
  MOCK_MIGRATION_FAILED_WITH_VALIDATED_CONDITION,
  MOCK_MIGRATION_FOR_DETAILS_TAB,
  MOCK_MIGRATION_PLAN_1,
  MOCK_MIGRATION_TEMPLATE_READY,
  MOCK_VMWARE_MACHINE_FOR_DETAILS_TAB,
} from '../helpers/migration.fixtures'

const PHASE_DETAIL_TESTIDS = [
  'phase-detail-copying',
  'phase-detail-converting',
  'phase-detail-awaiting-cutover',
  'phase-detail-awaiting-ldm-boot',
  'phase-detail-success',
  'phase-detail-generic',
]

// ─── MDP-011: ErrorCard shown instead of PhaseDetail for Failed/ValidationFailed

test.describe('MDP-011 — ErrorCard replaces PhaseDetail for failed phases', () => {
  for (const migration of [MOCK_MIGRATION_FAILED, MOCK_MIGRATION_VALIDATION_FAILED]) {
    test(`${migration.status.phase} shows the error card and no phase-detail variant`, async ({
      page,
    }) => {
      await goToMigrationDetail(page, migration)

      await expect(page.getByTestId('migration-error-card')).toBeVisible()
      for (const testId of PHASE_DETAIL_TESTIDS) {
        await expect(page.getByTestId(testId)).toHaveCount(0)
      }
    })
  }
})

// ─── MDP-012: Failed condition wins over a co-present Validated condition ────

test.describe('MDP-012 — error card picks the Failed condition, not Validated', () => {
  test('error title comes from the Failed condition; raw-log count excludes the Validated one', async ({
    page,
  }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_FAILED_WITH_VALIDATED_CONDITION)

    const card = page.getByTestId('migration-error-card')
    await expect(card.getByTestId('error-card-title')).toHaveText(
      'Disk copy failed: destination volume full',
    )
    await expect(card.getByTestId('error-card-title')).not.toContainText('Validated successfully')

    await expect(card.getByTestId('error-card-logs-toggle')).toContainText(
      'Show raw log lines from the failure (1)',
    )
    await card.getByTestId('error-card-logs-toggle').click()
    await expect(card.getByTestId('error-card-raw-logs')).toContainText('[Failed]')
  })
})

// ─── MDP-013: ConvertingDisk routes to ConvertingDiskDetail, not CopyingPhaseDetail

test.describe('MDP-013 — ConvertingDisk phase routing', () => {
  test('ConvertingDisk renders "Converting Disk Format", not "Copying Disk Blocks"', async ({
    page,
  }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_CONVERTING_DISK)

    await expect(page.getByTestId('phase-detail-converting')).toBeVisible()
    await expect(page.getByTestId('phase-detail-converting')).toContainText(
      'Converting Disk Format',
    )
    await expect(page.getByTestId('phase-detail-copying')).toHaveCount(0)
  })

  test('CopyingBlocks still renders the CopyingPhaseDetail variant', async ({ page }) => {
    await goToMigrationDetail(page, MOCK_MIGRATION_RUNNING)

    await expect(page.getByTestId('phase-detail-copying')).toBeVisible()
    await expect(page.getByTestId('phase-detail-converting')).toHaveCount(0)
  })
})

// ─── MDP-014: Migration Policies split-view — configured vs. defaults ────────

test.describe('MDP-014 — migration policies split view', () => {
  test('badge shows 0 configured / 12 default, and the defaults accordion expands', async ({
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

    const policies = page.getByTestId('details-section-policies')
    await expect(policies.getByTestId('policies-badge')).toHaveText('0 configured · 12 default')

    const toggle = policies.getByTestId('policies-defaults-toggle')
    await expect(toggle).toContainText('Show 12 policies using defaults')
    await toggle.click()
    await expect(toggle).toContainText('Policies using defaults (12)')
  })
})
