# E2E Test Suite Handoff — Migration Feature

## Context
- Repo: `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
- Branch: `abhijeet/migration-ui-e2e-test-cases`
- Task: Fix broken Playwright E2E tests for `src/features/migration/`
- Specs: `docs/specs/migration/migration-e2e-test-index.md`, `docs/specs/migration/migration-feature.md`
- Run tests: `source ~/.nvm/nvm.sh && nvm use 20 && npx playwright test e2e/migration/<spec> --grep "MIG-XXX" --reporter=list`

---

## Current Status

| ID | Test | File | Status |
|----|------|------|--------|
| MIG-001 | Page loads with table | smoke.spec.ts | ✅ PASSING |
| MIG-002 | Empty state renders | smoke.spec.ts | ✅ PASSING |
| MIG-003 | Table columns + data | smoke.spec.ts | ✅ PASSING |
| MIG-004 | Standard migration end-to-end | happy-path.spec.ts | 🔧 FIXED (verify in next run) |
| MIG-005 | Rolling migration | happy-path.spec.ts | ❓ NOT YET VERIFIED |
| MIG-006 | Delete single migration | happy-path.spec.ts | ❓ NOT YET VERIFIED |
| MIG-007 | Bulk delete | happy-path.spec.ts | ❓ NOT YET VERIFIED |
| MIG-008 | Pod logs drawer | happy-path.spec.ts | ❓ NOT YET VERIFIED |
| MIG-009 | Admin cutover trigger | happy-path.spec.ts | ✅ PASSING |
| MIG-010 | Status + date filtering | happy-path.spec.ts | ✅ PASSING |
| MIG-011 to MIG-019 | Validation tests | validation.spec.ts | ❓ NOT YET RUN |
| MIG-020 to MIG-027 | Error handling | error-handling.spec.ts | ❓ NOT YET RUN |
| MIG-028 to MIG-038 | Edge cases | edge-cases.spec.ts | ❓ NOT YET RUN |

---

## CRITICAL FIXES (discovered this session — apply before any test work)

### 1. `mockStandardFormApis` — PATCH handler missing

The `migrationtemplates` route in `mockStandardFormApis` MUST handle PATCH.
Submit flow: POST networkMappings → POST storageMappings → **PATCH migrationtemplates/{name}** → POST migrationPlans.
Without PATCH mock, the form hangs at step 3 and drawer never closes.

**Correct pattern (already applied to `happy-path.spec.ts`):**
```typescript
await page.route(`**migrationtemplates**`, (route) => {
  const method = route.request().method()
  if (method === 'POST') {
    route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
  } else if (method === 'GET') {
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_READY) })
  } else if (method === 'PATCH') {
    // updateMigrationTemplate (step 3 of submit) — must succeed for createMigrationPlan to run
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_READY) })
  } else {
    route.continue()
  }
})
```

**Apply this fix to:** `validation.spec.ts`, `error-handling.spec.ts`, `edge-cases.spec.ts` before writing any tests.

### 2. MUI Select (searchable) — option click + menu close

Both `networkMapping` and `storageMapping` RHFSelects use `searchable={true}` (controlled open state).
Standard Playwright `.click()` on menu options fails because:
- MUI marks disabled items with `.Mui-disabled` CSS class (NOT `aria-disabled="true"`)
- MUI keeps `.MuiMenu-root` in DOM forever (aria-hidden="true" when closed, NOT removed)
- Playwright considers menu items "not visible" during open animation

**Correct helper (already in `happy-path.spec.ts`):**
```typescript
async function clickFirstMenuOption(page: Page) {
  await page.waitForSelector('.MuiMenu-root', { state: 'attached', timeout: 3000 })
  await page.evaluate(() => {
    const menus = document.querySelectorAll('.MuiMenu-root')
    const menu = menus[menus.length - 1]
    if (!menu) return
    const option = menu.querySelector('li[role="option"]:not(.Mui-disabled)')
    if (option) (option as HTMLElement).click()
  })
}

// Wait for menu to "close" (aria-hidden set, not DOM removal):
await page.waitForFunction(
  () => {
    const menus = document.querySelectorAll('.MuiMenu-root')
    return Array.from(menus).every((m) => m.getAttribute('aria-hidden') === 'true')
  },
  { timeout: 5000 }
)
```

### 3. ResourceMappingTableNew — row selector

Uses native HTML `<tr>`, NOT explicit `role="row"`. CSS selector `[role="row"]` returns 0 results.
```typescript
// WRONG:
table.locator('[role="row"]')

// CORRECT:
table.locator('tbody tr')
```

### 4. VM selection — avoid powered-off VM

`MOCK_VMWARE_MACHINE_POWERED_OFF` has no `osFamily` → `vmValidation.hasError = true` → submit disabled.
Select by name, not index:
```typescript
// WRONG: nth(2) might hit powered-off VM
await grid.locator('[role="row"]').nth(2).click()

// CORRECT: select by name
await grid.locator('[role="row"]').filter({ hasText: 'test-vm-1' }).getByRole('checkbox').click({ force: true })
await grid.locator('[role="row"]').filter({ hasText: 'test-vm-multi-network' }).getByRole('checkbox').click({ force: true })
```

---

## Action Plan (for fresh chat)

### Phase 1 — Baseline (10 min)
Run ALL tests, collect failure list:
```bash
source ~/.nvm/nvm.sh && nvm use 20
npx playwright test e2e/migration/ --reporter=list 2>&1 | grep -E "✓|✘|PASS|FAIL" | tee /tmp/test-baseline.txt
```

### Phase 2 — Fix happy-path.spec.ts (MIG-004 to MIG-008) (30 min)
Run each group individually and fix:
```bash
# MIG-004 (already fixed — verify first)
npx playwright test e2e/migration/happy-path.spec.ts --grep "MIG-004" --reporter=list

# MIG-005
npx playwright test e2e/migration/happy-path.spec.ts --grep "MIG-005" --reporter=list

# MIG-006 + MIG-007
npx playwright test e2e/migration/happy-path.spec.ts --grep "MIG-006|MIG-007" --reporter=list

# MIG-008
npx playwright test e2e/migration/happy-path.spec.ts --grep "MIG-008" --reporter=list
```

### Phase 3 — Validation (MIG-011 to MIG-019) (20 min)
Apply PATCH fix to `mockStandardFormApis` copy in validation.spec.ts, then run:
```bash
npx playwright test e2e/migration/validation.spec.ts --reporter=list
```

### Phase 4 — Error handling + Edge cases (20 min)
Same pattern. Fix mock, run, fix failures.
```bash
npx playwright test e2e/migration/error-handling.spec.ts --reporter=list
npx playwright test e2e/migration/edge-cases.spec.ts --reporter=list
```

### Phase 5 — Full suite green
```bash
npx playwright test e2e/migration/ --reporter=list
```

---

## Key technical facts

### API pattern
```
**/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/{resource}
```
- Namespace: `migration-system` (`VJAILBREAK_DEFAULT_NAMESPACE`)
- Dev API prefix: `/dev-api` (proxied by Vite)
- Auth: `VITE_API_TOKEN` → `Authorization: Bearer` header
- K8s proxy: `/dev-api/sdk/vpw/v1/k8s/api/v1` (for pod logs / cutover PATCH)

### Routes
- Migrations: `/dashboard/migrations`
- Credentials: `/dashboard/credentials`
- Cluster Conversions: `/dashboard/cluster-conversions`

### Form architecture
- **Standard migration**: `MigrationFormDrawer` — `data-testid="migration-form-drawer"`
  - 5 sections: `source-destination`, `select-vms`, `map-resources`, `security`, `options`
  - Opened via `data-testid="start-migration-button"`
- **Rolling migration**: `RollingMigrationFormDrawer` — `data-testid="rolling-migration-form-drawer"`
  - 7 sections: `source-destination`, `baremetal`, `hosts`, `vms`, `map-resources`, `security`, `options`

### Submit flow (standard migration)
1. POST `/networkMappings` → `MOCK_NETWORK_MAPPING_CREATED`
2. POST `/storageMappings` → `MOCK_STORAGE_MAPPING_CREATED`
3. **PATCH `/migrationtemplates/{name}`** → `MOCK_MIGRATION_TEMPLATE_READY` ← must mock!
4. POST `/migrationplans` → `MOCK_MIGRATION_PLAN_CREATED`
5. `onClose()` + navigate → drawer closes

### Cluster selection APIs
- VMware clusters: `GET **/vmwareclusters?labelSelector=...` → option text: `'DC1-Cluster'`
- PCD clusters: `GET **/pcdclusters` → option text: `'pcd-cluster-1'`
- VMware creds: dropdown shows `spec.hostName` = `'vcenter.example.com'`

### Migration table
- Name column: `spec.vmName`
- Status: `status.phase`
- `data-testid="migrations-table"` (MigrationsTable)

### Pod logs
- URL: `**/sdk/vpw/v1/k8s/api/v1/namespaces/{namespace}/pods/{podName}/log**`

### Admin cutover
- API: `PATCH **/sdk/vpw/v1/k8s/api/v1/namespaces/{namespace}/pods/{podName}`
- Payload: `{ spec: { initiateCutover: true } }`

### Form validation
- Submit button: `data-testid="migration-form-submit"` (disabled until all valid)
- Section error badge: `data-testid="section-nav-error-badge"`
- Helpers: `expectSectionNavError(page, sectionId)` / `expectSectionNavClear(page, sectionId)`
- `expectSubmitDisabled(page)` / `expectSubmitEnabled(page)`

### Fixtures available
```
MOCK_MIGRATION_{PENDING,RUNNING,SUCCEEDED,FAILED,AWAITING_CUTOVER}
MOCK_MIGRATIONS_LIST           — 5 migrations, all phases
MOCK_MIGRATIONS_LIST_EMPTY
MOCK_MIGRATIONS_FOR_BULK_DELETE
MOCK_MIGRATION_PLAN_{1,2,3,4,5,CREATED}
MOCK_MIGRATION_PLANS_LIST / _EMPTY
MOCK_VMWARE_CRED_1 / MOCK_VMWARE_CREDS_LIST
MOCK_OPENSTACK_CRED_1 / MOCK_OPENSTACK_CREDS_LIST
MOCK_OPENSTACK_CRED_WITH_HOST_CONFIG  — pcdHostConfig for rolling
MOCK_MIGRATION_TEMPLATE_{PENDING,READY,LARGE}
MOCK_VM_LIST_LARGE              — 55 VMs for MIG-030 stress test
MOCK_NETWORK_MAPPING_CREATED / MOCK_STORAGE_MAPPING_CREATED
MOCK_VMWARE_CLUSTER_1 / MOCK_VMWARE_CLUSTERS_LIST
MOCK_PCD_CLUSTER_1 / MOCK_PCD_CLUSTERS_LIST
MOCK_VMWARE_HOST_{1,2} / MOCK_VMWARE_HOSTS_LIST
MOCK_BM_CONFIG_1 / MOCK_BM_CONFIGS_LIST
MOCK_ROLLING_MIGRATION_PLAN_CREATED
MOCK_IP_VALIDATION_{VALID,CONFLICT}
MOCK_VMWARE_MACHINE_POWERED_OFF  — NO osFamily — do NOT select this VM
```

### API constants
```typescript
API.migrations / .migrationByName(name)
API.migrationPlans / .migrationPlanByName(name)
API.migrationTemplates / .migrationTemplateByName(name)
API.vmwareCreds / .vmwareCredByName(name)
API.openstackCreds / .openstackCredByName(name)
API.networkMappings / .storageMappings
API.vmwareMachines / .vmwareMachineByName(name)
API.vmwareClusters / .vmwareHosts / .pcdClusters
API.bmConfigs / .rdmDisks / .volumeImageProfiles
API.rollingMigrationPlans
API.podLogs(namespace, podName)
API.validateIPs
```

### Required data-testid attributes
Already added to source code (check before running — some may need to be added):

| `data-testid` | Component |
|---------------|-----------|
| `start-migration-button` | toolbar CTA (MigrationsTable) |
| `migration-form-drawer` | MigrationFormDrawer ✅ |
| `rolling-migration-form-drawer` | RollingMigrationFormDrawer ✅ |
| `migration-form-close` | X button |
| `migration-form-submit` | Submit button |
| `migrations-table` | MigrationsTable |
| `section-nav-item-{sectionId}` | Left section nav items |
| `section-nav-error-badge` | Error badge on nav item |
| `vmware-cluster-dropdown` | cluster picker step 1 |
| `pcd-cluster-dropdown` | cluster picker step 1 |
| `vms-datagrid` | VM selection DataGrid |
| `network-mapping-table` | mapping step |
| `storage-mapping-table` | mapping step |
| `status-filter` / `date-filter` | MigrationsTable filters |
| `delete-selected-button` | bulk delete toolbar |
| `confirm-delete-button` | delete dialog confirm |
| `pod-logs-drawer` / `logs-search-input` / `logs-download-button` | PodLogsDrawer |
| `cutover-trigger-button` / `cutover-confirm-button` | TriggerAdminCutoverButton |
| `hosts-datagrid` | ESXi hosts DataGrid (rolling) |

---

## Resume prompt for fresh chat

```
Fix breaking Playwright E2E tests for the vJailbreak migration feature.

Context file: ui/migration-e2e-test-handoff.md — read this FIRST for all patterns and known fixes.

Branch: abhijeet/migration-ui-e2e-test-cases
Test dir: ui/e2e/migration/
Run: source ~/.nvm/nvm.sh && nvm use 20 && npx playwright test ...

CRITICAL: Before running any tests, check that happy-path.spec.ts has the PATCH fix
in mockStandardFormApis (see handoff doc "CRITICAL FIXES" section).

Start with Phase 1: run full suite baseline to see all failures, then work through
phases 2-5 in order (happy-path → validation → error-handling → edge-cases).

Fix strategy: run 1 group at a time with --grep, read screenshot from test-results/,
fix root cause, re-run to verify. Don't fix more than 1-2 tests at once.
```
