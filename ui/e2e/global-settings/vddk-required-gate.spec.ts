import { test, expect, Page } from '@playwright/test'

import { API, mockRoute } from '../migration/helpers/migration.helpers'

// VDDK_REQUIRED gates whether the app force-redirects/tours a user to the VDDK
// upload page when VDDK isn't present (#2350 made VDDK optional; this configmap
// key controls the UI's enforcement of it). Default is "false" — VDDK optional,
// no forced redirect. Setting it "true" restores the original mandatory flow.

const VDDK_STATUS = '**/vpw/v1/vddk/status'

async function mockBaseRoutes(page: Page, settingsData: Record<string, string>): Promise<void> {
  await mockRoute(page, API.settingsConfigMap, 'GET', {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'vjailbreak-settings', namespace: 'migration-system' },
    data: settingsData,
  })
  await mockRoute(page, VDDK_STATUS, 'GET', { uploaded: false })
  await mockRoute(page, API.vmwareCreds, 'GET', { items: [] })
  await mockRoute(page, API.openstackCreds, 'GET', { items: [] })
  await mockRoute(page, API.migrations, 'GET', { items: [] })
}

test.describe('VDDK-REQ-001 — VDDK_REQUIRED gates the forced VDDK redirect', () => {
  test('VDDK_REQUIRED=false: no VDDK uploaded, no forced redirect to VDDK tab', async ({
    page,
  }) => {
    await mockBaseRoutes(page, { VDDK_REQUIRED: 'false' })
    await page.goto('/')
    await page.waitForURL(/\/dashboard\/credentials\/vm/)
    await expect(page.locator('[data-tour="vddk-dropzone"]')).not.toBeVisible()
  })

  test('VDDK_REQUIRED absent from configmap: defaults to not-required', async ({ page }) => {
    await mockBaseRoutes(page, {})
    await page.goto('/')
    await page.waitForURL(/\/dashboard\/credentials\/vm/)
  })

  test('VDDK_REQUIRED=true: no VDDK uploaded, forced redirect to VDDK upload tab', async ({
    page,
  }) => {
    await mockBaseRoutes(page, { VDDK_REQUIRED: 'true' })
    await page.goto('/')
    await page.waitForURL(/\/dashboard\/global-settings/)
    await expect(page.getByTestId('global-settings-form')).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('[data-tour="vddk-dropzone"]')).toBeVisible({ timeout: 10_000 })
  })
})
