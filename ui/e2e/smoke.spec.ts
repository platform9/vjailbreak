import { expect, test } from "@playwright/test"

test("app loads", async ({ page }) => {
  await page.goto("/")

  await expect(page).toHaveTitle(/vJailbreak/i)
  await expect(page.locator("#root")).toBeVisible()
})
