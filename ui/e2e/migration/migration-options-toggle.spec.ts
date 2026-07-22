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

// GH #2176 follow-up — same root cause as the Tags & Metadata toggle (see
// tags-metadata.spec.ts TAGS-005): a click landing right after a cluster Select's
// menu closes also shifts DOM focus onto the click target, which desyncs React's
// controlled-checkbox change tracking and silently reverts the native `checked`
// flip before onChange fires. Whichever Migration Options toggle the user reaches
// first after selecting clusters needed two clicks to enable; every toggle after
// that worked normally. Fixed by driving all Migration Options checkboxes off
// onClick (invert current state) instead of onChange/e.target.checked.
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

test.describe('MIGOPTS-001 — GH-2176 regression: Migration Options toggles respond to a single click', () => {
  test.beforeEach(async ({ page }) => {
    await mockFormApis(page)
  })

  test('first toggle reached after closing cluster selects enables on the first raw click', async ({
    page,
  }) => {
    await goToMigrations(page)
    await openMigrationDrawer(page)
    await selectVmwareCluster(page, 'DC1-Cluster')
    await selectPcdCluster(page, 'pcd-cluster-1')
    await page.waitForFunction(() => {
      const menus = document.querySelectorAll('.MuiMenu-root')
      return Array.from(menus).every((m) => m.getAttribute('aria-hidden') === 'true')
    }, { timeout: 5000 })

    // Jump straight to Migration Options, skipping every other step - this makes
    // "Data copy method" the first control clicked after the selects close.
    const checkbox = page.getByRole('checkbox', { name: /data copy method/i })
    await checkbox.scrollIntoViewIfNeeded()
    await expect(checkbox).toBeVisible()

    const box = await checkbox.boundingBox()
    if (!box) throw new Error('checkbox not visible')
    const cx = box.x + box.width / 2
    const cy = box.y + box.height / 2

    await page.mouse.click(cx, cy)
    await expect(checkbox).toBeChecked()

    await page.mouse.click(cx, cy)
    await expect(checkbox).not.toBeChecked()
  })
})
