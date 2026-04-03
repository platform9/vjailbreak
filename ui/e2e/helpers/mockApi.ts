import { Page, Route } from "@playwright/test"

type Json = Record<string, any>

type Method = "GET" | "POST" | "DELETE" | "PUT" | "PATCH"

export type MockRoute = {
  method: Method
  url: RegExp
  handler: (route: Route) => Promise<void> | void
}

export const jsonResponse = (route: Route, status: number, body: Json) => {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  })
}

export const emptyResponse = (route: Route, status: number) => {
  return route.fulfill({ status })
}

export async function installMockRoutes(page: Page, routes: MockRoute[]) {
  for (const r of routes) {
    await page.route(r.url, async (route) => {
      const req = route.request()
      if (req.method().toUpperCase() !== r.method) {
        return route.fallback()
      }
      return r.handler(route)
    })
  }
}

export async function mockAppPrereqs(page: Page, opts?: { vddkUploaded?: boolean }) {
  const vddkUploaded = opts?.vddkUploaded ?? true
  await installMockRoutes(page, [
    {
      method: "GET",
      url: /\/dev-api\/sdk\/vpw\/v1\/vddk\/status(?:\?.*)?$/,
      handler: (route) => jsonResponse(route, 200, { uploaded: vddkUploaded, version: "test" }),
    },
  ])
}
