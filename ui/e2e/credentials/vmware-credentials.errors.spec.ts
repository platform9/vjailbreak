import { expect, test } from '@playwright/test'
import { mockAppPrereqs } from '../helpers/mockApi'
import { createDefaultState, mockVmwareCredentialsApi } from '../helpers/vmwareCredsMock'
import { openAddVmwareDrawer, openVmwareCredentialsPage } from '../helpers/selectors'

test.describe('VMware Credentials - error handling', () => {
  test('secret create failure shows error', async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })
    const state = createDefaultState()
    await mockVmwareCredentialsApi(page, state, {
      secretCreateFails: { status: 500, message: 'boom' }
    })

    await openVmwareCredentialsPage(page)
    await openAddVmwareDrawer(page)

    await page.locator('input[name="credentialName"]').fill('prod-vcenter')
    await page.locator('input[name="vcenterHost"]').fill('https://vcenter.example.com')
    await page.locator('input[name="username"]').fill('administrator@vsphere.local')
    await page.locator('input[name="password"]').fill('secret')

    await page.getByTestId('vmware-cred-submit').click()

    await expect(page.getByText(/Error creating VMware credentials/i)).toBeVisible({
      timeout: 30_000
    })
  })

  test('validation failure triggers cleanup delete calls', async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })
    const state = createDefaultState()

    // Track that cleanup endpoints were hit.
    let deleteCredCalled = false
    let deleteSecretCalled = false

    page.on('request', (req) => {
      if (req.method() !== 'DELETE') return
      const url = req.url()
      if (url.includes('/namespaces/migration-system/vmwarecreds/')) deleteCredCalled = true
      if (url.includes('/namespaces/migration-system/secrets/')) deleteSecretCalled = true
    })

    await mockVmwareCredentialsApi(page, state, {
      validationFails: { message: 'Invalid vCenter credentials' },
      createSucceedsAfterPolls: 1
    })

    await openVmwareCredentialsPage(page)
    await openAddVmwareDrawer(page)

    await page.locator('input[name="credentialName"]').fill('prod-vcenter')
    await page.locator('input[name="vcenterHost"]').fill('https://vcenter.example.com')
    await page.locator('input[name="username"]').fill('administrator@vsphere.local')
    await page.locator('input[name="password"]').fill('secret')

    await page.getByTestId('vmware-cred-submit').click()

    await expect(page.getByText('Validating VMware credentials…')).toBeVisible({ timeout: 30_000 })

    await expect(page.getByText(/Validation failed|Invalid vCenter credentials/i)).toBeVisible({
      timeout: 60_000
    })

    await expect.poll(() => deleteCredCalled).toBe(true)
    await expect.poll(() => deleteSecretCalled).toBe(true)
  })

  test('cancel after create triggers cleanup', async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })
    const state = createDefaultState()

    let deleteCredCalled = false
    let deleteSecretCalled = false

    page.on('request', (req) => {
      if (req.method() !== 'DELETE') return
      const url = req.url()
      if (url.includes('/namespaces/migration-system/vmwarecreds/')) deleteCredCalled = true
      if (url.includes('/namespaces/migration-system/secrets/')) deleteSecretCalled = true
    })

    // Make validation take longer so we can cancel mid-flight.
    await mockVmwareCredentialsApi(page, state, { createSucceedsAfterPolls: 100 })

    await openVmwareCredentialsPage(page)
    await openAddVmwareDrawer(page)

    await page.locator('input[name="credentialName"]').fill('prod-vcenter')
    await page.locator('input[name="vcenterHost"]').fill('https://vcenter.example.com')
    await page.locator('input[name="username"]').fill('administrator@vsphere.local')
    await page.locator('input[name="password"]').fill('secret')

    await page.getByTestId('vmware-cred-submit').click()
    await expect(page.getByText('Validating VMware credentials…')).toBeVisible()

    await page.getByTestId('vmware-cred-cancel').click()

    await expect.poll(() => deleteCredCalled).toBe(true)
    await expect.poll(() => deleteSecretCalled).toBe(true)
  })
})
