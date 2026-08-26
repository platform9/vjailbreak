import { test, expect, Page } from '@playwright/test'

import { goToGlobalSettings, mockRoute, API } from '../migration/helpers/migration.helpers'
import {
  MOCK_MIGRATIONS_LIST_EMPTY,
  MOCK_MIGRATION_RUNNING,
} from '../migration/helpers/migration.fixtures'

// Reset to Defaults must not fight the appliance's time configuration: TIMEZONE and
// NTP_SERVERS are applied to the host, and changing them mid-migration would disturb a
// running transfer. GlobalSettingsPage therefore keeps those two fields at their current
// values while a non-terminal migration exists, and resets everything else.
//
// Replaces cypress/e2e/global-settings-reset-defaults.cy.ts.

const PF9_ENV_CM = '**/api/v1/namespaces/migration-system/configmaps/pf9-env'
const VDDK_STATUS = '**/vpw/v1/vddk/status'
const AI_KEY = '**/vpw/v1/ai/key'

const TIMEZONE = 'America/New_York'
const NTP_SERVERS = '0.pool.ntp.org, 1.pool.ntp.org'
const DEPLOYMENT_NAME = 'custom-deployment'

// DEFAULTS in GlobalSettingsPage.tsx
const DEFAULT_DEPLOYMENT_NAME = 'vJailbreak'

async function mockSettings(page: Page, migrations: Record<string, unknown>): Promise<void> {
  await mockRoute(page, API.settingsConfigMap, 'GET', {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'vjailbreak-settings', namespace: 'migration-system' },
    data: {
      DEPLOYMENT_NAME,
      TIMEZONE,
      NTP_SERVERS,
    },
  })
  await mockRoute(page, PF9_ENV_CM, 'GET', {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'pf9-env', namespace: 'migration-system' },
    data: {},
  })
  await mockRoute(page, VDDK_STATUS, 'GET', { uploaded: true, version: '8.0.3' })
  await mockRoute(page, AI_KEY, 'GET', { configured: false })
  await mockRoute(page, API.migrations, 'GET', migrations)
}

const timezoneInput = (page: Page) =>
  page.getByTestId('global-settings-field-TIMEZONE').locator('input').first()

test.describe('GS-RESET-001 — Reset to Defaults vs. active migrations', () => {
  test('preserves TIMEZONE and NTP_SERVERS while a migration is running', async ({ page }) => {
    // CopyingBlocks is non-terminal, so time settings are locked.
    await mockSettings(page, { items: [MOCK_MIGRATION_RUNNING] })
    await goToGlobalSettings(page)

    const deploymentName = page.locator('input[name="DEPLOYMENT_NAME"]')
    await expect(deploymentName).toHaveValue(DEPLOYMENT_NAME)
    // The field is an Autocomplete: it renders the option label "(UTC-04:00) America/New_York",
    // not the raw IANA value that is stored in the ConfigMap.
    await expect(timezoneInput(page)).toHaveValue(new RegExp(TIMEZONE))

    await page.getByTestId('global-settings-reset-defaults').click()

    // An unrelated field goes back to its default...
    await expect(deploymentName).toHaveValue(DEFAULT_DEPLOYMENT_NAME)
    // ...while the two time fields keep the values the appliance is running with.
    await expect(timezoneInput(page)).toHaveValue(new RegExp(TIMEZONE))

    await page.getByTestId('global-settings-tab-advanced').click()
    await expect(page.locator('input[name="NTP_SERVERS"]')).toHaveValue(NTP_SERVERS)
  })

  test('resets TIMEZONE and NTP_SERVERS when no migration is running', async ({ page }) => {
    await mockSettings(page, MOCK_MIGRATIONS_LIST_EMPTY)
    await goToGlobalSettings(page)

    const deploymentName = page.locator('input[name="DEPLOYMENT_NAME"]')
    await expect(deploymentName).toHaveValue(DEPLOYMENT_NAME)

    await page.getByTestId('global-settings-reset-defaults').click()

    await expect(deploymentName).toHaveValue(DEFAULT_DEPLOYMENT_NAME)
    await expect(timezoneInput(page)).toHaveValue('')

    await page.getByTestId('global-settings-tab-advanced').click()
    await expect(page.locator('input[name="NTP_SERVERS"]')).toHaveValue('')
  })
})
