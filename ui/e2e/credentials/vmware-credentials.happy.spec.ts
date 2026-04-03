import { expect, test } from '@playwright/test'
import { mockAppPrereqs } from '../helpers/mockApi'
import { createDefaultState, mockVmwareCredentialsApi } from '../helpers/vmwareCredsMock'
import { openAddVmwareDrawer, openVmwareCredentialsPage } from '../helpers/selectors'

test.describe('VMware Credentials - happy path', () => {
  test('create + validate succeeded shows PCD prompt when missing PCD creds', async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })

    const state = createDefaultState()
    // No PCD creds present => should show prompt after success
    state.openstackCreds = []

    await mockVmwareCredentialsApi(page, state, { createSucceedsAfterPolls: 1 })

    await openVmwareCredentialsPage(page)
    await openAddVmwareDrawer(page)

    await page.locator('input[name="credentialName"]').fill('production-vcenter')
    await page.locator('input[name="vcenterHost"]').fill('https://vcenter.example.com')
    await page.locator('input[name="datacenter"]').fill('Primary-DC')
    await page.locator('input[name="username"]').fill('administrator@vsphere.local')
    await page.locator('input[name="password"]').fill('secret')

    await page.getByTestId('vmware-cred-submit').click()

    await expect(page.getByText('Validating VMware credentials…')).toBeVisible()

    await expect(page.getByRole('heading', { name: 'Add PCD Credentials' })).toBeVisible({
      timeout: 60_000
    })
    await expect(
      page.getByText(
        'Your VMware credentials are ready. Add your PCD credentials next to start migrations.'
      )
    ).toBeVisible()

    // choose Later -> should close prompt and stay on VMware creds page
    await page.getByRole('button', { name: 'Later' }).click()

    await expect(page.locator('[data-tour="add-vmware-creds"]')).toBeVisible()

    const row = page.locator('.MuiDataGrid-row').filter({ hasText: 'production-vcenter' }).first()
    await expect(row).toBeVisible()
    await expect(row.getByText('Succeeded')).toBeVisible()
  })

  test('insecure toggle shows warning', async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })
    const state = createDefaultState()
    await mockVmwareCredentialsApi(page, state)

    await openVmwareCredentialsPage(page)
    await openAddVmwareDrawer(page)

    const insecure = page.locator('input[name="insecure"]')
    await insecure.scrollIntoViewIfNeeded()
    await insecure.click({ force: true })
    await expect(
      page.getByText(
        'Use this option only when you fully trust the network between Platform9 and the vCenter host.'
      )
    ).toBeVisible()
  })
})
