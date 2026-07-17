import { test, expect, Page } from '@playwright/test'

import {
  goToGlobalSettings,
  mockRoute,
  expectToast,
  API,
} from '../migration/helpers/migration.helpers'
import { MOCK_MIGRATIONS_LIST_EMPTY } from '../migration/helpers/migration.fixtures'

// The AI tab added a `required` API-key <input> that lives permanently inside
// the single Global Settings <form>. Inactive tab panels stay MOUNTED (hidden
// via display:none), and the AI key field is always empty, so browser-native
// constraint validation aborted every form submission silently — the browser
// tries to focus the invalid control, cannot (it is display:none), and never
// fires the submit event. Result: Save did nothing on EVERY tab (VDDK,
// General, Network, ...). The fix is `noValidate` on the form; validation is
// fully handled in JS (validateForm/buildErrors in onSave).
//
// These tests run in a real browser, so they reproduce the native-validation
// behavior exactly — they FAIL if `noValidate` is ever removed again.

const PF9_ENV_CM = '**/api/v1/namespaces/migration-system/configmaps/pf9-env'
const VDDK_STATUS = '**/vpw/v1/vddk/status'
const VDDK_UPLOAD = '**/vpw/v1/vddk/upload'
const AI_KEY = '**/vpw/v1/ai/key'
const INJECT_ENV = '**/vpw/v1/inject_env_variables'

type SettingsMockState = {
  putCalls: number
  vddkUploaded: boolean
}

async function mockGlobalSettingsApis(page: Page): Promise<SettingsMockState> {
  const state: SettingsMockState = { putCalls: 0, vddkUploaded: false }

  // Settings ConfigMap — GET returns empty data (form falls back to defaults),
  // PUT records the save and succeeds.
  await page.route(API.settingsConfigMap, (route) => {
    const method = route.request().method()
    if (method === 'PUT') {
      state.putCalls += 1
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

  // VDDK status flips to uploaded after the upload endpoint is hit,
  // mirroring the real backend (page refetches status post-upload).
  await page.route(VDDK_STATUS, (route) => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        state.vddkUploaded
          ? {
              uploaded: true,
              path: '/home/ubuntu/vmware-vix-disklib-distrib',
              version: '8.0.3',
            }
          : { uploaded: false },
      ),
    })
  })

  await page.route(VDDK_UPLOAD, (route) => {
    state.vddkUploaded = true
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        success: true,
        message: 'VDDK file uploaded and extracted successfully!',
        extracted_path: '/home/ubuntu/vmware-vix-disklib-distrib',
      }),
    })
  })

  await mockRoute(page, AI_KEY, 'GET', { configured: false })
  await mockRoute(page, INJECT_ENV, 'POST', {})
  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)

  return state
}

test.describe('GS-SAVE-001 — Global Settings save works with the AI tab mounted', () => {
  test('form opts out of native validation (noValidate) so the hidden required AI key field cannot block submit', async ({
    page,
  }) => {
    await mockGlobalSettingsApis(page)
    await goToGlobalSettings(page)

    const form = page.getByTestId('global-settings-form')
    await expect(form).toHaveAttribute('novalidate', '')

    // The hazardous condition this guards against must still be true for the
    // regression test below to stay meaningful: the AI tab's required key
    // input is mounted (hidden) and empty while another tab is active.
    const aiKeyInput = page.locator('#settings-tabpanel-ai input[required]')
    await expect(aiKeyInput).toHaveCount(1)
    await expect(aiKeyInput).toBeHidden()
    await expect(aiKeyInput).toHaveValue('')
  })

  test('saves settings from the General tab (submit is not silently aborted)', async ({
    page,
  }) => {
    const state = await mockGlobalSettingsApis(page)
    await goToGlobalSettings(page)

    await page.getByTestId('global-settings-save').click()

    await expectToast(page, /global settings saved successfully/i)
    expect(state.putCalls).toBe(1)
  })

  test('uploads VDDK file via Save on the VDDK Upload tab', async ({ page }) => {
    const state = await mockGlobalSettingsApis(page)
    await goToGlobalSettings(page)

    await page.getByTestId('global-settings-tab-vddk').click()

    await page
      .getByTestId('global-settings-form')
      .locator('input[type="file"]')
      .setInputFiles({
        name: 'vddk.tar.gz',
        mimeType: 'application/gzip',
        buffer: Buffer.from('fake vddk archive content'),
      })
    await expect(page.getByText('Ready. Click Save to upload & extract.')).toBeVisible()

    const uploadRequest = page.waitForRequest(
      (req) => req.url().includes('/vpw/v1/vddk/upload') && req.method() === 'POST',
    )
    await page.getByTestId('global-settings-save').click()
    await uploadRequest

    await expect(
      page.getByText(/vddk file uploaded and extracted successfully/i),
    ).toBeVisible({ timeout: 10_000 })
    expect(state.vddkUploaded).toBe(true)
  })

  test('JS validation still blocks saving when a required setting is invalid', async ({
    page,
  }) => {
    const state = await mockGlobalSettingsApis(page)
    await goToGlobalSettings(page)

    // Clear Deployment Name — buildErrors marks it "Required."
    await page.locator('input[name="DEPLOYMENT_NAME"]').fill('')
    await page.getByTestId('global-settings-save').click()

    await expectToast(page, /please fix the validation errors/i)
    expect(state.putCalls).toBe(0)
  })
})
