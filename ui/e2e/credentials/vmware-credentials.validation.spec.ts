import { expect, test } from '@playwright/test'
import { mockAppPrereqs } from '../helpers/mockApi'
import { createDefaultState, mockVmwareCredentialsApi } from '../helpers/vmwareCredsMock'
import { openAddVmwareDrawer, openVmwareCredentialsPage } from '../helpers/selectors'

test.describe('VMware Credentials - form validation', () => {
  test.beforeEach(async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })
    const state = createDefaultState()
    await mockVmwareCredentialsApi(page, state)
  })

  test('Save is disabled until required fields are valid', async ({ page }) => {
    await openVmwareCredentialsPage(page)
    await openAddVmwareDrawer(page)

    const save = page.getByTestId('vmware-cred-submit')
    await expect(save).toBeDisabled()

    await page.locator('input[name="credentialName"]').fill('Invalid_Name')
    await page.locator('input[name="vcenterHost"]').fill('https://vcenter.example.com')
    await page.locator('input[name="username"]').fill('administrator@vsphere.local')
    await page.locator('input[name="password"]').fill('secret')

    await expect(
      page.getByText(
        'Credential name must start with a lowercase letter/number and use only lowercase letters, numbers, or hyphens.'
      )
    ).toBeVisible()

    await expect(save).toBeDisabled()

    await page.locator('input[name="credentialName"]').fill('production-vcenter')
    await expect(save).toBeEnabled()
  })

  test('Datacenter is optional', async ({ page }) => {
    await openVmwareCredentialsPage(page)
    await openAddVmwareDrawer(page)

    await page.locator('input[name="credentialName"]').fill('prod-vcenter')
    await page.locator('input[name="vcenterHost"]').fill('https://vcenter.example.com')
    await page.locator('input[name="username"]').fill('administrator@vsphere.local')
    await page.locator('input[name="password"]').fill('secret')

    await expect(page.getByTestId('vmware-cred-submit')).toBeEnabled()
  })
})
