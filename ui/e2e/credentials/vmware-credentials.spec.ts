import { test, expect, Page, Route } from '@playwright/test'

import { mockRoute, API } from '../migration/helpers/migration.helpers'

// Adding a VMware credential is a two-write flow: the password goes into a Secret, the
// credential CR references it, and the drawer then polls the CR until the controller
// reports a validation status. Both outcomes matter — a success closes the drawer, a
// failure keeps it open and surfaces the controller's message.
//
// Replaces the "Add VMware Credentials" block of cypress/e2e/vmware-credentials.cy.ts.

const NS = 'migration-system'
const SECRETS = `**/namespaces/${NS}/secrets`

type SecretCapture = { data: Record<string, string> | null }

async function mockCredentialsPage(page: Page): Promise<SecretCapture> {
  const capture: SecretCapture = { data: null }

  await mockRoute(page, API.vmwareCreds, 'GET', { items: [] })
  await mockRoute(page, API.openstackCreds, 'GET', { items: [] })

  await page.route(SECRETS, (route: Route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    const body = route.request().postDataJSON() as { data?: Record<string, string> }
    capture.data = body?.data ?? {}
    route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ metadata: { name: 'vmware-secret', namespace: NS } }),
    })
  })

  return capture
}

async function openDrawerAndFill(page: Page, credName: string, host: string): Promise<void> {
  await page.goto('/dashboard/credentials/vm')
  await page.getByRole('button', { name: /add vmware credentials/i }).click()
  await expect(page.getByTestId('vmware-cred-form')).toBeVisible()

  await page.locator('input[name="credentialName"]').fill(credName)
  await page.locator('input[name="vcenterHost"]').fill(host)
  await page.locator('input[name="datacenter"]').fill('Datacenter-1')
  await page.locator('input[name="username"]').fill('admin@vsphere.local')
  await page.locator('input[name="password"]').fill('securepassword')
}

test.describe('CRED-001 — add VMware credentials', () => {
  test('creates the secret and credential, then reports successful validation', async ({
    page,
  }) => {
    const credName = 'test-vcenter-creds'
    const capture = await mockCredentialsPage(page)

    await page.route(API.vmwareCreds, (route: Route) => {
      if (route.request().method() !== 'POST') return route.fallback()
      route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          metadata: { name: credName, namespace: NS },
          spec: { secretRef: { name: 'vmware-secret' }, datacenter: 'Datacenter-1' },
          status: { vmwareValidationStatus: 'Succeeded' },
        }),
      })
    })
    // The drawer polls the CR by name after creating it.
    await mockRoute(page, API.vmwareCredByName(credName), 'GET', {
      metadata: { name: credName, namespace: NS },
      status: { vmwareValidationStatus: 'Succeeded' },
    })

    await openDrawerAndFill(page, credName, 'vcenter.example.com')

    // The switch sits below the fold in the drawer. force:true skips the actionability
    // checks but the click still needs viewport coordinates, so scroll it in first —
    // without this the test fails intermittently with "Element is outside of the viewport".
    const insecure = page.locator('input[name="insecure"]')
    await insecure.scrollIntoViewIfNeeded()
    await insecure.click({ force: true })
    await expect(insecure).toBeChecked()

    await page.getByTestId('vmware-cred-submit').click()

    // The password is never written to the CR — it goes into the Secret.
    await expect.poll(() => capture.data).not.toBeNull()
    expect(Object.keys(capture.data!)).toContain('VCENTER_PASSWORD')

    // The drawer reports the validated state and stays open — dismissing is the operator's
    // call, because closing with a created-but-unconfirmed credential deletes it again
    // (see closeDrawer in VMwareCredentialsDrawer.tsx).
    await expect(page.getByText(/VMware credentials created and validated/i)).toBeVisible({
      timeout: 15_000,
    })
    await expect(page.getByTestId('vmware-cred-form')).toBeVisible()
  })

  test('keeps the drawer open and shows the controller message when validation fails', async ({
    page,
  }) => {
    const credName = 'fail-creds'
    await mockCredentialsPage(page)

    await page.route(API.vmwareCreds, (route: Route) => {
      if (route.request().method() !== 'POST') return route.fallback()
      route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ metadata: { name: credName, namespace: NS } }),
      })
    })
    await mockRoute(page, API.vmwareCredByName(credName), 'GET', {
      metadata: { name: credName, namespace: NS },
      status: {
        vmwareValidationStatus: 'Failed',
        vmwareValidationMessage: 'Invalid credentials',
      },
    })

    await openDrawerAndFill(page, credName, 'fail.com')
    await page.getByTestId('vmware-cred-submit').click()

    await expect(page.getByText(/Invalid credentials/i).first()).toBeVisible({ timeout: 15_000 })
    // Nothing is dismissed on failure: the operator can correct and retry in place.
    await expect(page.getByTestId('vmware-cred-form')).toBeVisible()
  })
})
