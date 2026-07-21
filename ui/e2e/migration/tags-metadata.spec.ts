import { test, expect, Page } from '@playwright/test'

import {
  goToMigrations,
  openMigrationDrawer,
  selectVmwareCluster,
  selectPcdCluster,
  mockRoute,
  API,
} from './helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST_EMPTY,
  MOCK_MIGRATION_PLANS_LIST_EMPTY,
  MOCK_VMWARE_CREDS_LIST,
  MOCK_VMWARE_CRED_1,
  MOCK_OPENSTACK_CRED_1,
  MOCK_OPENSTACK_CREDS_LIST,
  MOCK_VMWARE_CLUSTERS_LIST,
  MOCK_PCD_CLUSTERS_LIST,
  MOCK_VMWARE_MACHINES_LIST,
} from './helpers/migration.fixtures'

// ─── Tags & Metadata step (step 5) of the standard migration form ─────────────
//
// Covers: step placement between Security & Placement and Migration Options,
// the preserve-source-tags toggle with its preview accordion (fed by
// VMwareMachine tags/customAttributes on MOCK_VMWARE_MACHINE_1), the custom
// metadata key-value editor, and the section nav marking the step complete.

async function mockFormApis(page: Page) {
  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)
  await mockRoute(page, API.migrationPlans, 'GET', MOCK_MIGRATION_PLANS_LIST_EMPTY)
  await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
  await mockRoute(page, API.vmwareCredByName('vcenter-cred-1'), 'GET', MOCK_VMWARE_CRED_1)
  await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
  await mockRoute(page, API.vmwareMachines, 'GET', MOCK_VMWARE_MACHINES_LIST)
  await mockRoute(page, API.rdmDisks, 'GET', {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'RdmDiskList',
    metadata: { continue: '', resourceVersion: '1' },
    items: [],
  })
  await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
  await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
  await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
}

async function openFormWithClusters(page: Page) {
  await goToMigrations(page)
  await openMigrationDrawer(page)
  await selectVmwareCluster(page, 'DC1-Cluster')
  await selectPcdCluster(page, 'pcd-cluster-1')
  // MUI keeps the cluster Select's Menu modal mounted while it animates closed;
  // its invisible backdrop swallows pointer events. Wait until every menu is hidden.
  await page.waitForFunction(
    () => {
      const menus = document.querySelectorAll('.MuiMenu-root')
      return Array.from(menus).every((m) => m.getAttribute('aria-hidden') === 'true')
    },
    { timeout: 5000 },
  )
}

async function scrollToTagsStep(page: Page) {
  await page.getByTestId('migration-form-step-tags-metadata').scrollIntoViewIfNeeded()
  await expect(page.getByTestId('migration-form-tags-metadata-card')).toBeVisible()
}

// MUI Switch: the hidden input's hitbox extends past the control (left:-100%,
// width:300%), so clicking it directly is unreliable — toggle via the label
// text and assert state on the input.
const TOGGLE_LABEL = 'Preserve VMware tags and custom attributes'

function preserveToggleInput(page: Page) {
  return page.getByRole('checkbox', { name: TOGGLE_LABEL })
}

async function togglePreserve(page: Page) {
  await page.getByText(TOGGLE_LABEL, { exact: true }).click()
}

test.describe('TAGS-001 — Tags & Metadata step placement', () => {
  test.beforeEach(async ({ page }) => {
    await mockFormApis(page)
  })

  test('renders as its own step between Security & Placement and Migration Options', async ({
    page,
  }) => {
    await openFormWithClusters(page)
    await scrollToTagsStep(page)

    // Section nav lists it after security and before options
    await expect(page.getByTestId('section-nav-item-security')).toBeVisible()
    await expect(page.getByTestId('section-nav-item-tags-metadata')).toBeVisible()
    await expect(page.getByTestId('section-nav-item-options')).toBeVisible()

    // DOM order: security step above tags step, tags step above options step
    const order = await page.evaluate(() => {
      const security = document.querySelector('[data-testid="migration-form-step-security"]')
      const tags = document.querySelector('[data-testid="migration-form-step-tags-metadata"]')
      const options = document.querySelector('[data-testid="migration-form-step-options"]')
      if (!security || !tags || !options) return null
      const FOLLOWING = Node.DOCUMENT_POSITION_FOLLOWING
      return {
        securityBeforeTags: Boolean(security.compareDocumentPosition(tags) & FOLLOWING),
        tagsBeforeOptions: Boolean(tags.compareDocumentPosition(options) & FOLLOWING),
      }
    })
    expect(order).not.toBeNull()
    expect(order?.securityBeforeTags).toBe(true)
    expect(order?.tagsBeforeOptions).toBe(true)
  })
})

test.describe('TAGS-002 — preserve source tags toggle and preview', () => {
  test.beforeEach(async ({ page }) => {
    await mockFormApis(page)
  })

  test('toggle reveals the preview accordion and hides it when turned off', async ({ page }) => {
    await openFormWithClusters(page)
    await scrollToTagsStep(page)

    // Off by default — no preview
    await expect(page.getByTestId('source-tags-preview')).not.toBeVisible()

    await togglePreserve(page)
    await expect(preserveToggleInput(page)).toBeChecked()
    await expect(page.getByTestId('source-tags-preview')).toBeVisible()

    await togglePreserve(page)
    await expect(preserveToggleInput(page)).not.toBeChecked()
    await expect(page.getByTestId('source-tags-preview')).not.toBeVisible()
  })

  test('preview shows the selected VM tags and custom attributes', async ({ page }) => {
    await openFormWithClusters(page)

    // Select the VM carrying tag data so the preview has a populated row
    const grid = page.getByTestId('vms-datagrid')
    await expect(grid.locator('[role="row"]').nth(1)).toBeVisible({ timeout: 15_000 })
    await grid
      .locator('[role="row"]')
      .filter({ hasText: 'test-vm-1' })
      .getByRole('checkbox')
      .click({ force: true })

    await scrollToTagsStep(page)
    await togglePreserve(page)

    // Expand the accordion and verify tag/attribute chips
    const preview = page.getByTestId('source-tags-preview')
    await preview.click()
    await expect(preview.getByText('test-vm-1')).toBeVisible()
    await expect(preview.getByText('env: production')).toBeVisible()
    await expect(preview.getByText('Owner: alice@corp.com')).toBeVisible()
  })
})

test.describe('TAGS-003 — custom metadata key-value editor', () => {
  test.beforeEach(async ({ page }) => {
    await mockFormApis(page)
  })

  test('rows can be added, edited, and removed', async ({ page }) => {
    await openFormWithClusters(page)
    await scrollToTagsStep(page)

    await expect(page.getByTestId('custom-metadata-row-0')).not.toBeVisible()

    await page.getByTestId('add-custom-metadata').click()
    await expect(page.getByTestId('custom-metadata-row-0')).toBeVisible()

    const row0 = page.getByTestId('custom-metadata-row-0')
    await row0.locator('input').nth(0).fill('migrated_by')
    await row0.locator('input').nth(1).fill('vjailbreak')

    await page.getByTestId('add-custom-metadata').click()
    await expect(page.getByTestId('custom-metadata-row-1')).toBeVisible()

    // Remove the second row
    await page
      .getByTestId('custom-metadata-row-1')
      .getByRole('button', { name: 'Remove metadata row' })
      .click()
    await expect(page.getByTestId('custom-metadata-row-1')).not.toBeVisible()

    // First row keeps its values
    await expect(row0.locator('input').nth(0)).toHaveValue('migrated_by')
    await expect(row0.locator('input').nth(1)).toHaveValue('vjailbreak')
  })
})

test.describe('TAGS-004 — section nav completion marking', () => {
  test.beforeEach(async ({ page }) => {
    await mockFormApis(page)
  })

  test('nav chip turns into a check once the toggle is enabled', async ({ page }) => {
    await openFormWithClusters(page)
    await scrollToTagsStep(page)

    const navItem = page.getByTestId('section-nav-item-tags-metadata')

    // Incomplete before any interaction: chip shows the step number, no check icon
    await expect(navItem.locator('svg[data-testid="CheckIcon"]')).not.toBeVisible()

    await togglePreserve(page)

    await expect(navItem.locator('svg[data-testid="CheckIcon"]')).toBeVisible()
  })
})
