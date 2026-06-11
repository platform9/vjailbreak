# Generate Playwright E2E Test Suite

You are a senior QA engineer implementing a Playwright E2E test suite for the **$ARGUMENTS** feature.

## Inputs — read these first before writing any code

1. Read `docs/specs/$ARGUMENTS-e2e-test-index.md` — this is your source of truth for every test case, its steps, pass/fail criteria, priority, and tags.
2. Read `docs/specs/$ARGUMENTS-feature.md` — use this for selectors, API endpoints, validation rules, and state/flow context.
3. Scan the existing test setup (look for `playwright.config.ts`, `e2e/`, `tests/`, or similar) so your new tests match the project's conventions.

Do not write any test code yet. Output a summary of:

- How many test cases you found in the index, grouped by category
- The existing Playwright config and folder structure you'll write into
- Any gaps (missing selectors, unclear preconditions, ambiguous steps) you need me to clarify

Wait for my approval before writing any files.

---

## After approval — implement the test suite

### File structure

Organise tests by category to mirror the index groupings:
e2e/$ARGUMENTS/
smoke.spec.ts
happy-path.spec.ts
validation.spec.ts
error-handling.spec.ts
edge-cases.spec.ts
helpers/
$ARGUMENTS.helpers.ts ← shared setup, teardown, and reusable actions
$ARGUMENTS.fixtures.ts ← test data and mock payloads

### Implementation rules

- Map every test case ID from the index (e.g. FEAT-001) to a `test()` block — use the ID in the test title so failures are traceable back to the index.
- Use `test.describe()` blocks matching the index groupings.
- All selectors must use `data-testid` attributes. If a required `data-testid` is missing from the source code, list it at the end as "Required data-testid additions" rather than using brittle CSS or text selectors.
- API calls: intercept with `page.route()` for error-handling and edge-case tests so they don't depend on a live backend. Happy-path and smoke tests should run against the real dev environment.
- Shared preconditions (login, navigation to the feature page) go into `beforeEach` hooks inside `$ARGUMENTS.helpers.ts`.
- Each test must assert both the UI state and (where applicable) the network request payload using `page.waitForRequest()` or `page.waitForResponse()`.
- Tests tagged `critical` in the index must be in `smoke.spec.ts` and must pass with zero retries.
- Add a comment above each `test()` block with the full test case ID, name, and priority pulled from the index.

### Start order

Implement in this order and pause for review after each file:

1. `$ARGUMENTS.helpers.ts` and `$ARGUMENTS.fixtures.ts`
2. `smoke.spec.ts` (critical tests only)
3. `happy-path.spec.ts`
4. `validation.spec.ts`
5. `error-handling.spec.ts`
6. `edge-cases.spec.ts`

Begin with Step 1 — output the summary and gap analysis. Wait for my go-ahead.
