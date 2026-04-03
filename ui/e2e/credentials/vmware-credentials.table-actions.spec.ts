import { expect, test } from '@playwright/test'
import { mockAppPrereqs } from '../helpers/mockApi'
import { createDefaultState, mockVmwareCredentialsApi } from '../helpers/vmwareCredsMock'
import { openVmwareCredentialsPage } from '../helpers/selectors'

test.describe('VMware Credentials - table actions', () => {
  test('delete single row removes it from list', async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })
    const state = createDefaultState()
    state.vmwareCreds = [
      {
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'VMwareCreds',
        metadata: { name: 'to-delete', namespace: 'migration-system' },
        status: { vmwareValidationStatus: 'Succeeded', vmwareValidationMessage: '' }
      }
    ]

    await mockVmwareCredentialsApi(page, state)

    await openVmwareCredentialsPage(page)

    const cell = page.locator('div[role="gridcell"]').filter({ hasText: 'to-delete' }).first()
    await expect(cell).toBeVisible({ timeout: 30_000 })

    // Click delete icon
    await page.getByLabel('delete credential').first().click()

    await expect(page.getByRole('heading', { name: 'Confirm Delete' })).toBeVisible()
    await page.getByRole('button', { name: 'Delete' }).click()

    await expect(page.locator('div[role="gridcell"]').filter({ hasText: 'to-delete' })).toHaveCount(
      0
    )
  })

  test('revalidate sets row to validating then back to succeeded', async ({ page }) => {
    await mockAppPrereqs(page, { vddkUploaded: true })
    const state = createDefaultState()
    state.vmwareCreds = [
      {
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'VMwareCreds',
        metadata: { name: 'revalidate-me', namespace: 'migration-system' },
        status: { vmwareValidationStatus: 'Succeeded', vmwareValidationMessage: '' }
      }
    ]

    // route the revalidate endpoint to also toggle our mocked list status
    await page.route(/\/dev-api\/sdk\/vpw\/v1\/revalidate_credentials(?:\?.*)?$/, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback()
      state.vmwareCreds[0].status = {
        vmwareValidationStatus: 'Validating',
        vmwareValidationMessage: ''
      }
      // after a short while, transition to succeeded (so the table polling stops)
      setTimeout(() => {
        state.vmwareCreds[0].status = {
          vmwareValidationStatus: 'Succeeded',
          vmwareValidationMessage: ''
        }
      }, 250)
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'ok' })
      })
    })

    await mockVmwareCredentialsApi(page, state)

    await openVmwareCredentialsPage(page)

    await page.getByLabel('revalidate credential').first().click()

    // Eventually it should return to Succeeded.
    const rowCell = page
      .locator('div[role="gridcell"]')
      .filter({ hasText: 'revalidate-me' })
      .first()
    await expect(rowCell).toBeVisible({ timeout: 30_000 })
    const succeededChip = page
      .locator('.MuiDataGrid-virtualScrollerRenderZone')
      .locator('span')
      .filter({ hasText: 'Succeeded' })
      .first()
    await expect(succeededChip).toBeVisible({ timeout: 30_000 })
  })
})
