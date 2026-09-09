import { test, expect, Page, Route } from '@playwright/test'

import { mockRoute, API } from '../migration/helpers/migration.helpers'

// Scaling up creates a VjailbreakNode. The OpenStack server group is optional: when the
// operator picks one it must reach spec.openstackServerGroup, and when they leave it empty
// the key must be absent entirely rather than sent as ''. An empty string would make Nova
// reject the boot request.
//
// Replaces the "ScaleUp Drawer — Server Group" block of cypress/e2e/vmware-credentials.cy.ts.

const NS = 'migration-system'
const CRED_NAME = 'test-pcd-creds'
const NODES = `**/namespaces/${NS}/vjailbreaknodes`

const MASTER_NODE = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'VjailbreakNode',
  metadata: { name: 'vjailbreak-master', namespace: NS },
  spec: {
    nodeRole: 'master',
    openstackImageID: 'img-abc123',
    openstackFlavorID: 'flavor-001',
    openstackCreds: { kind: 'openstackcreds', name: CRED_NAME, namespace: NS },
  },
  status: { phase: 'NodeReady', vmIP: '10.0.0.1', openstackUUID: 'uuid-master' },
}

const VALIDATED_CRED = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'OpenstackCreds',
  metadata: { name: CRED_NAME, namespace: NS },
  spec: {
    // Flavors are filtered to disk >= 60 in the drawer, so this one must stay above it.
    flavors: [{ id: 'flavor-001', name: 'm1.xlarge', vcpus: 8, ram: 16384, disk: 80 }],
  },
  status: {
    openstackValidationStatus: 'Succeeded',
    openstack: {
      volumeTypes: ['ceph-ssd'],
      securityGroups: [{ id: 'sg-1', name: 'default', requiresIdDisplay: false }],
      serverGroups: [
        { id: 'srv-grp-001', name: 'anti-affinity-agents', policy: 'anti-affinity', members: 0 },
        {
          id: 'srv-grp-002',
          name: 'soft-anti-affinity-agents',
          policy: 'soft-anti-affinity',
          members: 1,
        },
      ],
    },
  },
}

type NodeCapture = { spec: Record<string, unknown> | null }

async function mockAgentsPage(page: Page): Promise<NodeCapture> {
  const capture: NodeCapture = { spec: null }

  await page.route(NODES, (route: Route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON() as { spec?: Record<string, unknown> }
      capture.spec = body?.spec ?? {}
      route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ ...body, status: {} }),
      })
      return
    }
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [MASTER_NODE] }),
    })
  })

  await mockRoute(page, API.openstackCreds, 'GET', { items: [VALIDATED_CRED] })
  await mockRoute(page, API.openstackCredByName(CRED_NAME), 'GET', VALIDATED_CRED)
  await mockRoute(page, API.vmwareCreds, 'GET', { items: [] })

  return capture
}

async function openScaleUpAndSelectCred(page: Page): Promise<void> {
  await page.goto('/dashboard/agents')
  await page.getByTestId('scaleup-open-button').click()
  await expect(page.getByTestId('scaleup-form')).toBeVisible()

  await page.getByPlaceholder('Select PCD Credentials').click()
  await page.getByRole('option', { name: CRED_NAME }).click()
}

async function selectFlavor(page: Page): Promise<void> {
  // Option label is "m1.xlarge — 8 vCPU · 16GB RAM · 80GB disk".
  await page.locator('[name="flavor"]').click({ force: true })
  await page.getByRole('option', { name: /m1\.xlarge/ }).click()
}

test.describe('AGENT-001 — ScaleUp drawer server group', () => {
  test('offers the server group field once a validated credential is selected', async ({
    page,
  }) => {
    await mockAgentsPage(page)
    await openScaleUpAndSelectCred(page)

    const serverGroup = page.getByTestId('scaleup-server-group')
    await expect(serverGroup).toBeVisible()
    await expect(serverGroup.locator('input').first()).toBeEnabled()
  })

  test('sends openstackServerGroup when a server group is chosen', async ({ page }) => {
    const capture = await mockAgentsPage(page)
    await openScaleUpAndSelectCred(page)

    await page.getByTestId('scaleup-server-group').locator('input').first().click()
    await page.getByRole('option', { name: /anti-affinity-agents \(anti-affinity\)/ }).click()

    await selectFlavor(page)
    await page.getByTestId('scaleup-submit').click()

    await expect.poll(() => capture.spec).not.toBeNull()
    expect(capture.spec).toMatchObject({ openstackServerGroup: 'srv-grp-001' })
  })

  test('omits openstackServerGroup when none is chosen', async ({ page }) => {
    const capture = await mockAgentsPage(page)
    await openScaleUpAndSelectCred(page)

    await selectFlavor(page)
    await page.getByTestId('scaleup-submit').click()

    await expect.poll(() => capture.spec).not.toBeNull()
    expect(capture.spec).not.toHaveProperty('openstackServerGroup')
  })
})
