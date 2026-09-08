import { test, expect, type Page } from '@playwright/test'

const DEBUG_PROMPT_API = '**/vpw/v1/ai/debug-prompt'

const MOCK_DEBUG_PROMPT = {
  version: '1.0.0',
  last_updated: '2026-09-07',
  prompt: 'test system prompt for vJailbreak debugging',
}

async function stubDebugPromptApi(page: Page) {
  await page.route(DEBUG_PROMPT_API, (route) => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(MOCK_DEBUG_PROMPT),
    })
  })
}

// ── User Story 1: Claude Code tab ─────────────────────────────────────────────

test('US1: /help/debug-ai route renders', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await expect(page).toHaveURL(/\/help\/debug-ai/)
})

test('US1: Claude Code tab is active by default', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await expect(page.getByRole('tab', { name: /claude code/i })).toHaveAttribute(
    'aria-selected',
    'true',
  )
})

test('US1: install instructions are visible on Claude Code tab', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  // Claude Code tab should show install instructions
  await expect(page.getByText(/install/i).first()).toBeVisible()
})

test('US1: repository link is present on Claude Code tab', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await expect(page.getByRole('link', { name: /vjailbreak/i }).first()).toBeVisible()
})

test('US1: version badge shows version from API', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await expect(page.getByText(/v1\.0\.0/).first()).toBeVisible()
})

test('US1: page loads without vjailbreak-ai (API unavailable)', async ({ page }) => {
  // No stub — API will fail; page must still load with graceful degradation
  await page.route(DEBUG_PROMPT_API, (route) => route.fulfill({ status: 500 }))
  await page.goto('/help/debug-ai')
  await expect(page.getByRole('tab', { name: /claude code/i })).toBeVisible()
  await expect(page.getByText(/version unavailable/i)).toBeVisible()
})

test('US1: Other Agents tab exists', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await expect(page.getByRole('tab', { name: /other agents/i })).toBeVisible()
})

// ── User Story 2: Other Agents tab ────────────────────────────────────────────

test('US2: clicking Other Agents tab shows prompt text', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await page.getByRole('tab', { name: /other agents/i }).click()
  await expect(page.getByText(MOCK_DEBUG_PROMPT.prompt)).toBeVisible()
})

test('US2: Copy System Prompt button exists on Other Agents tab', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await page.getByRole('tab', { name: /other agents/i }).click()
  await expect(page.getByRole('button', { name: /copy system prompt/i })).toBeVisible()
})

test('US2: clicking Copy button changes label to Copied! then reverts', async ({ page }) => {
  await stubDebugPromptApi(page)
  await page.goto('/help/debug-ai')
  await page.getByRole('tab', { name: /other agents/i }).click()

  // Wait for the prompt content to load before interacting with the copy button
  await expect(page.getByText(MOCK_DEBUG_PROMPT.prompt)).toBeVisible()

  await page.getByRole('button', { name: /copy system prompt/i }).click()

  // After clicking the button text changes to "Copied!" — use a fresh locator that matches
  // the new accessible name; the original locator becomes stale as its text changed.
  await expect(page.getByRole('button', { name: /copied!/i })).toBeVisible({ timeout: 5000 })

  // Reverts after 2 seconds
  await expect(page.getByRole('button', { name: /copy system prompt/i })).toBeVisible({
    timeout: 4000,
  })
})
