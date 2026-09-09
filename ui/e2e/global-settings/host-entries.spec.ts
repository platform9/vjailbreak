import { test, expect, Page } from '@playwright/test'

import { goToGlobalSettings, mockRoute, API } from '../migration/helpers/migration.helpers'
import { MOCK_MIGRATIONS_LIST_EMPTY } from '../migration/helpers/migration.fixtures'

// Host Entries are hostname-to-IP mappings injected into agent node VMs. They are stored as
// a JSON blob in the AGENT_HOST_ENTRIES key of the vjailbreak-settings ConfigMap, so the
// round-trip through the table editor and back into that key is what matters.
//
// Replaces cypress/e2e/global-settings-host-entries.cy.ts.

const PF9_ENV_CM = '**/api/v1/namespaces/migration-system/configmaps/pf9-env'
const VDDK_STATUS = '**/vpw/v1/vddk/status'
const AI_KEY = '**/vpw/v1/ai/key'
const INJECT_ENV = '**/vpw/v1/inject_env_variables'

const EXISTING_ENTRIES = [{ ip: '10.0.0.5', hostnames: ['esxi01.corp.local', 'esxi01'] }]

type SettingsState = { savedData: Record<string, string> | null }

async function mockSettings(page: Page): Promise<SettingsState> {
  const state: SettingsState = { savedData: null }

  await page.route(API.settingsConfigMap, (route) => {
    if (route.request().method() === 'PUT') {
      const body = route.request().postDataJSON() as { data?: Record<string, string> }
      state.savedData = body?.data ?? {}
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
        data: { AGENT_HOST_ENTRIES: JSON.stringify(EXISTING_ENTRIES) },
      }),
    })
  })

  await mockRoute(page, PF9_ENV_CM, 'GET', {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'pf9-env', namespace: 'migration-system' },
    data: {},
  })
  await mockRoute(page, VDDK_STATUS, 'GET', { uploaded: true, version: '8.0.3' })
  await mockRoute(page, AI_KEY, 'GET', { configured: false })
  await mockRoute(page, INJECT_ENV, 'POST', {})
  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST_EMPTY)

  return state
}

async function openHostEntriesTab(page: Page): Promise<void> {
  await goToGlobalSettings(page)
  await page.getByTestId('global-settings-tab-hosts').click()
  await expect(page.getByTestId('host-entries-add-btn')).toBeVisible()
}

test.describe('GS-HOSTS-001 — Global Settings Host Entries tab', () => {
  test('describes the tab once and puts Add Entry in the table header', async ({ page }) => {
    await mockSettings(page)
    await openHostEntriesTab(page)

    // The description is the page-level tab helper; the table must not repeat it.
    const description = page.getByText(/Custom hostname-to-IP mappings/i)
    await expect(description).toHaveCount(1)
    await expect(description).toContainText(
      'Supports ESXi hosts, vCenter, PCD, and OpenStack endpoints'
    )
    await expect(page.locator('table')).not.toContainText('Custom hostname-to-IP mappings')

    // Add Entry lives in the header row, not above/below the table.
    const addButton = page.getByTestId('host-entries-add-btn')
    await expect(addButton).toBeVisible()
    await expect(page.locator('thead').getByTestId('host-entries-add-btn')).toHaveCount(1)
  })

  test('adds an entry and saves it into AGENT_HOST_ENTRIES', async ({ page }) => {
    const state = await mockSettings(page)
    await openHostEntriesTab(page)

    // Entry already in the ConfigMap is listed.
    await expect(page.getByRole('cell', { name: '10.0.0.5' })).toBeVisible()
    await expect(
      page.getByRole('cell', { name: 'esxi01.corp.local, esxi01' })
    ).toBeVisible()

    await page.getByTestId('host-entries-add-btn').click()
    // The inline add row owns the interaction while it is open.
    await expect(page.getByTestId('host-entries-add-btn')).toBeDisabled()

    await page.getByTestId('host-entry-new-ip').fill('192.168.1.100')
    await page.getByTestId('host-entry-new-hostnames').fill('vcenter.corp.local, vcenter')
    await page.getByTestId('host-entry-add-confirm').click()

    await expect(page.getByRole('cell', { name: '192.168.1.100' })).toBeVisible()
    await expect(page.getByTestId('host-entries-add-btn')).toBeEnabled()

    await page.getByTestId('global-settings-save').click()

    // Saved as a JSON array, existing entry first, in the AGENT_HOST_ENTRIES key.
    await expect.poll(() => state.savedData?.AGENT_HOST_ENTRIES ?? null).not.toBeNull()
    expect(JSON.parse(state.savedData!.AGENT_HOST_ENTRIES)).toEqual([
      { ip: '10.0.0.5', hostnames: ['esxi01.corp.local', 'esxi01'] },
      { ip: '192.168.1.100', hostnames: ['vcenter.corp.local', 'vcenter'] },
    ])
  })

  test('rejects an invalid IP and supports edit and delete', async ({ page }) => {
    await mockSettings(page)
    await openHostEntriesTab(page)

    // Invalid IP is refused inline; the row stays open for correction.
    await page.getByTestId('host-entries-add-btn').click()
    await page.getByTestId('host-entry-new-ip').fill('not-an-ip')
    await page.getByTestId('host-entry-new-hostnames').fill('host1')
    await page.getByTestId('host-entry-add-confirm').click()
    await expect(page.getByText('Invalid IP address')).toBeVisible()
    await page.getByTestId('host-entry-add-cancel').click()

    // Edit the existing entry in place.
    await page.getByTestId('host-entry-edit-0').click()
    await page.getByTestId('host-entry-edit-ip').fill('10.0.0.6')
    await page.getByTestId('host-entry-edit-save').click()
    await expect(page.getByRole('cell', { name: '10.0.0.6' })).toBeVisible()

    // Deleting the last row falls back to the empty state.
    await page.getByTestId('host-entry-delete-0').click()
    await expect(page.getByText(/No host entries configured/i)).toBeVisible()
  })
})
