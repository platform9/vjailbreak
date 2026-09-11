import { test, expect, Page } from '@playwright/test'

import {
  goToGlobalSettings,
  mockRoute,
  expectToast,
  API,
} from '../migration/helpers/migration.helpers'
import { MOCK_MIGRATIONS_LIST_EMPTY } from '../migration/helpers/migration.fixtures'

const PF9_ENV_CM = '**/api/v1/namespaces/migration-system/configmaps/pf9-env'
const VDDK_STATUS = '**/vpw/v1/vddk/status'
const AI_KEY = '**/vpw/v1/ai/key'
const INJECT_ENV = '**/vpw/v1/inject_env_variables'

type SettingsMockState = {
  putCalls: number
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  lastPutBody: any
}

async function mockGlobalSettingsApis(page: Page): Promise<SettingsMockState> {
  const state: SettingsMockState = { putCalls: 0, lastPutBody: null }

  await page.route(API.settingsConfigMap, (route) => {
    const method = route.request().method()
    if (method === 'PUT') {
      state.putCalls += 1
      state.lastPutBody = route.request().postDataJSON()
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: route.request().postData() ?? '{}',
      })
      return
    }
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        apiVersion: 'v1',
        kind: 'ConfigMap',
        metadata: { name: 'vjailbreak-settings', namespace: 'migration-system' },
        data: {},
      }),
    })
  })

  await mockRoute(page, PF9_ENV_CM, 'GET', {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'pf9-env', namespace: 'migration-system' },
    data: {},
  })
  await mockRoute(page, VDDK_STATUS, 'GET', { uploaded: false })
  await mockRoute(page, AI_KEY, 'GET', { configured: false })
  await mockRoute(page, INJECT_ENV, 'POST', {})
  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)

  return state
}

// LOG_RETENTION_HOURS renders via RHFTextField with a separate FieldLabel (for the
// tooltip icon), so the input has no real <label> association — getByLabel can't find
// it. Every retry-tab field has this same shape, so we go by input[name] instead, same
// as the DEPLOYMENT_NAME lookup in save-regression.spec.ts.
function logRetentionField(page: Page) {
  return page.locator('input[name="LOG_RETENTION_HOURS"]')
}

async function goToIntervalsTab(page: Page): Promise<void> {
  await goToGlobalSettings(page)
  await page.getByTestId('global-settings-tab-retry').click()
  await expect(logRetentionField(page)).toBeVisible()
}

test.describe('GS-LOG-RETENTION-001 — Configurable log retention', () => {
  test('blocks save when retention is below the 24-hour minimum', async ({ page }) => {
    const state = await mockGlobalSettingsApis(page)
    await goToIntervalsTab(page)

    await logRetentionField(page).fill('12')
    await page.getByTestId('global-settings-save').click()

    await expectToast(page, /please fix the validation errors/i)
    expect(state.putCalls).toBe(0)
  })

  test('saves the retention value once it is 24 hours or more', async ({ page }) => {
    const state = await mockGlobalSettingsApis(page)
    await goToIntervalsTab(page)

    await logRetentionField(page).fill('72')
    await page.getByTestId('global-settings-save').click()

    await expectToast(page, /global settings saved successfully/i)
    expect(state.putCalls).toBe(1)
    expect(state.lastPutBody?.data?.LOG_RETENTION_HOURS).toBe('72')
  })
})
