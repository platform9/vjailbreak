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
const PROXY_CREDS = '**/vpw/v1/proxy/credentials'

type ProxyCredsState = {
  configured: boolean
  httpsOverride: boolean
  postCalls: number
  deleteCalls: number
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  lastPostBody: any
}

async function mockGlobalSettingsApis(page: Page): Promise<ProxyCredsState> {
  const state: ProxyCredsState = {
    configured: false,
    httpsOverride: false,
    postCalls: 0,
    deleteCalls: 0,
    lastPostBody: null,
  }

  await mockRoute(page, API.settingsConfigMap, 'GET', {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'vjailbreak-settings', namespace: 'migration-system' },
    data: {},
  })
  await mockRoute(page, API.settingsConfigMap, 'PUT', {})

  await mockRoute(page, PF9_ENV_CM, 'GET', {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'pf9-env', namespace: 'migration-system' },
    data: {
      http_proxy: 'http://proxy.local:3128',
      https_proxy: 'http://proxy.local:3128',
      no_proxy: 'localhost,127.0.0.1',
    },
  })

  await mockRoute(page, VDDK_STATUS, 'GET', { uploaded: false })
  await mockRoute(page, AI_KEY, 'GET', { configured: false })
  await mockRoute(page, INJECT_ENV, 'POST', {})
  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)

  await page.route(PROXY_CREDS, (route) => {
    const method = route.request().method()
    if (method === 'GET') {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ configured: state.configured, https_override: state.httpsOverride }),
      })
      return
    }
    if (method === 'POST') {
      state.postCalls += 1
      state.lastPostBody = route.request().postDataJSON()
      state.configured = true
      state.httpsOverride = Boolean(state.lastPostBody?.https_override)
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ configured: true, https_override: state.httpsOverride }),
      })
      return
    }
    if (method === 'DELETE') {
      state.deleteCalls += 1
      state.configured = false
      state.httpsOverride = false
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ configured: false, https_override: false }),
      })
      return
    }
    route.fallback()
  })

  return state
}

async function goToNetworkTabWithAuthOn(page: Page): Promise<void> {
  await goToGlobalSettings(page)
  await page.getByTestId('global-settings-tab-network').click()
  await page.getByTestId('global-settings-toggle-PROXY_AUTH_ENABLED').click()
  await expect(page.getByLabel('Proxy Username')).toBeVisible()
}

test.describe('GS-PROXY-AUTH-001 — Proxy authentication toggle and save flow', () => {
  test('toggling "Proxy requires authentication" reveals the shared credential fields', async ({
    page,
  }) => {
    await mockGlobalSettingsApis(page)
    await goToGlobalSettings(page)
    await page.getByTestId('global-settings-tab-network').click()

    await expect(page.getByLabel('Proxy Username')).not.toBeVisible()

    await page.getByTestId('global-settings-toggle-PROXY_AUTH_ENABLED').click()

    await expect(page.getByLabel('Proxy Username')).toBeVisible()
    await expect(page.getByRole('textbox', { name: 'Proxy Password' })).toBeVisible()
  })

  test('the HTTPS override toggle reveals a second credential pair', async ({ page }) => {
    await mockGlobalSettingsApis(page)
    await goToNetworkTabWithAuthOn(page)

    await expect(page.getByLabel('HTTPS Proxy Username')).not.toBeVisible()

    await page.getByTestId('global-settings-toggle-PROXY_AUTH_HTTPS_OVERRIDE').click()

    await expect(page.getByLabel('HTTPS Proxy Username')).toBeVisible()
    await expect(page.getByRole('textbox', { name: 'HTTPS Proxy Password' })).toBeVisible()
  })

  test('save is rejected with a validation toast when the password is missing', async ({ page }) => {
    const state = await mockGlobalSettingsApis(page)
    await goToNetworkTabWithAuthOn(page)

    await page.getByLabel('Proxy Username').fill('proxyuser')
    await page.getByTestId('global-settings-save').click()

    await expectToast(page, /both proxy username and password are required/i)
    expect(state.postCalls).toBe(0)
  })

  test('save posts credentials to /vpw/v1/proxy/credentials before injecting env variables', async ({
    page,
  }) => {
    const state = await mockGlobalSettingsApis(page)
    await goToNetworkTabWithAuthOn(page)

    await page.getByLabel('Proxy Username').fill('proxyuser')
    await page.getByRole('textbox', { name: 'Proxy Password' }).fill('proxypass')

    const credsRequest = page.waitForResponse(
      (res) => res.url().includes('/vpw/v1/proxy/credentials') && res.request().method() === 'POST',
    )
    await page.getByTestId('global-settings-save').click()
    await credsRequest

    expect(state.postCalls).toBe(1)
    expect(state.lastPostBody).toMatchObject({
      username: 'proxyuser',
      password: 'proxypass',
      https_override: false,
    })
    await expect(page.getByText('Proxy credentials are configured.')).toBeVisible()
  })

  test('turning authentication off after it was configured deletes the stored credentials', async ({
    page,
  }) => {
    const state = await mockGlobalSettingsApis(page)
    state.configured = true
    await goToGlobalSettings(page)
    await page.getByTestId('global-settings-tab-network').click()
    await expect(page.getByText('Proxy credentials are configured.')).toBeVisible()

    const deleteRequest = page.waitForResponse(
      (res) => res.url().includes('/vpw/v1/proxy/credentials') && res.request().method() === 'DELETE',
    )
    await page.getByTestId('global-settings-toggle-PROXY_AUTH_ENABLED').click()
    await expect(page.getByLabel('Proxy Username')).not.toBeVisible()
    await page.getByTestId('global-settings-save').click()
    await deleteRequest

    expect(state.deleteCalls).toBe(1)
    expect(state.postCalls).toBe(0)
  })
})
