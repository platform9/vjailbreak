import { Locator, Page } from '@playwright/test'

export const byTestId = (page: Page, id: string): Locator => page.getByTestId(id)

export const openVmwareCredentialsPage = async (page: Page) => {
  // Disable Joyride overlays in E2E (they can intercept pointer events)
  await page.addInitScript(() => {
    window.localStorage.setItem('getting-started-dismissed', 'true')
  })
  await page.goto('/dashboard/credentials/vm')
  await page.locator('[data-tour="add-vmware-creds"]').waitFor()
  await page.getByRole('grid').waitFor()
}

export const openAddVmwareDrawer = async (page: Page) => {
  await page.locator('[data-tour="add-vmware-creds"]').click()
  await byTestId(page, 'vmware-cred-form').waitFor()
}
