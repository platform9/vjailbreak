# E2E Test Suite Handoff — Migration Feature

## Context
- Repo: `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
- Branch: `587-ui-refactor-migration`
- Task: Generate Playwright E2E test suite for `src/features/migration/` feature
- Specs: `docs/specs/migration/migration-e2e-test-index.md` and `docs/specs/migration/migration-feature.md`

---

## What's done

| Step | File | Tests | Status |
|------|------|-------|--------|
| 1 | `playwright.config.ts` | — | ✅ |
| 1 | `e2e/tsconfig.json` | — | ✅ |
| 1 | `e2e/migration/helpers/migration.helpers.ts` | — | ✅ |
| 1 | `e2e/migration/helpers/migration.fixtures.ts` | — | ✅ |
| 2 | `e2e/migration/smoke.spec.ts` | MIG-001, MIG-002, MIG-003 | ✅ |
| 3 | `e2e/migration/happy-path.spec.ts` | MIG-004 to MIG-010 | ✅ |

**`package.json` scripts:** `pw:open`, `pw:run`, `pw:run:migration`
**`@playwright/test@^1.60.0`** in devDependencies (Node 20 required — `nvm use 20`).

---

## What's next (Steps 4–6, implement in order, pause after each)

| Step | File | Tests |
|------|------|-------|
| 4 | `e2e/migration/validation.spec.ts` | MIG-011 to MIG-019 |
| 5 | `e2e/migration/error-handling.spec.ts` | MIG-020 to MIG-027 |
| 6 | `e2e/migration/edge-cases.spec.ts` | MIG-028 to MIG-038 |

---

## Key technical facts

### API pattern
```
**/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/{resource}
```
- Namespace: `migration-system` (`VJAILBREAK_DEFAULT_NAMESPACE` in `src/api/constants.ts`)
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
  - Opened via "Start Migration" button (`data-testid="start-migration-button"`)
- **Rolling migration**: `RollingMigrationFormDrawer` — `data-testid="rolling-migration-form-drawer"`
  - 7 sections: `source-destination`, `baremetal`, `hosts`, `vms`, `map-resources`, `security`, `options`
  - Opened via "Start Conversion" button in `RollingMigrationsTable`

### Cluster selection APIs
- VMware clusters: `GET **/vmwareclusters?labelSelector=vjailbreak.k8s.pf9.io/vmwarecreds={credName}`
  - Dropdown shows `cluster.spec.name` (the `displayName`)
  - Option text for mock: `'DC1-Cluster'`
- PCD clusters: `GET **/pcdclusters`
  - Dropdown shows `spec.clusterName`
  - Option text for mock: `'pcd-cluster-1'`
- VMware creds: `GET **/vmwarecreds` → option shows `spec.hostName` = `'vcenter.example.com'`

### Migration table (MigrationsTable.tsx)
- Name column: `spec.vmName` (e.g. `'test-vm-1'`)
- Status column: `status.phase`

### Pod logs
- URL: `**/sdk/vpw/v1/k8s/api/v1/namespaces/{namespace}/pods/{podName}/log**`
- Streaming via `fetch()` with `ReadableStream`

### Admin cutover
- API: `PATCH **/sdk/vpw/v1/k8s/api/v1/namespaces/{namespace}/pods/{podName}`
- Payload: `{ spec: { initiateCutover: true } }`

### Form validation architecture
- Submit button disabled until all sections valid
- Section nav shows error badge (`data-testid="section-nav-error-badge"`) on incomplete sections
- Validation helpers: `expectSectionNavError(page, sectionId)` / `expectSectionNavClear(page, sectionId)`
- Submit helpers: `expectSubmitDisabled(page)` / `expectSubmitEnabled(page)`

### Fixtures available in `migration.fixtures.ts`
```
MOCK_MIGRATION_{PENDING,RUNNING,SUCCEEDED,FAILED,AWAITING_CUTOVER}
MOCK_MIGRATIONS_LIST           — 5 migrations, all phases
MOCK_MIGRATIONS_LIST_EMPTY
MOCK_MIGRATIONS_FOR_BULK_DELETE — 3 migrations for MIG-007
MOCK_MIGRATION_PLAN_{1,CREATED}
MOCK_MIGRATION_PLANS_LIST / _EMPTY
MOCK_VMWARE_CRED_1 / MOCK_VMWARE_CREDS_LIST
MOCK_OPENSTACK_CRED_1 / MOCK_OPENSTACK_CREDS_LIST
MOCK_OPENSTACK_CRED_WITH_HOST_CONFIG  — includes pcdHostConfig for rolling migration
MOCK_MIGRATION_TEMPLATE_{PENDING,READY,LARGE}
MOCK_VM_LIST_LARGE              — 55 VMs for MIG-030 stress test
MOCK_NETWORK_MAPPING_CREATED / MOCK_STORAGE_MAPPING_CREATED
MOCK_VMWARE_CLUSTER_1 / MOCK_VMWARE_CLUSTERS_LIST
MOCK_PCD_CLUSTER_1 / MOCK_PCD_CLUSTERS_LIST
MOCK_VMWARE_HOST_{1,2} / MOCK_VMWARE_HOSTS_LIST
MOCK_BM_CONFIG_1 / MOCK_BM_CONFIGS_LIST
MOCK_ROLLING_MIGRATION_PLAN_CREATED
MOCK_IP_VALIDATION_{VALID,CONFLICT}
```

### API constants in `migration.helpers.ts`
```
API.migrations / .migrationByName(name)
API.migrationPlans / .migrationPlanByName(name)
API.migrationTemplates / .migrationTemplateByName(name)
API.vmwareCreds / .vmwareCredByName(name)
API.openstackCreds / .openstackCredByName(name)
API.networkMappings / .storageMappings
API.vmwareMachines / .vmwareMachineByName(name)
API.vmwareClusters
API.vmwareHosts
API.pcdClusters
API.bmConfigs
API.rollingMigrationPlans
API.podLogs(namespace, podName)
API.validateIPs
```

### Required `data-testid` attributes (not yet in source — must be added before tests run)

| `data-testid` | Component |
|---------------|-----------|
| `start-migration-button` | toolbar CTA (MigrationsTable) |
| `migration-form-drawer` | MigrationFormDrawer ✅ exists (MigrationForm.tsx:223) |
| `rolling-migration-form-drawer` | RollingMigrationFormDrawer ✅ exists (RollingMigrationForm.tsx:630) |
| `migration-form-close` | X button |
| `migration-form-submit` | Submit button |
| `migrations-table` | MigrationsTable |
| `migration-progress-cell` | Progress column cell |
| `section-nav-item-{sectionId}` | Left section nav items |
| `section-nav-error-badge` | Error badge on nav item |
| `vmware-cluster-dropdown` | cluster picker step 1 |
| `pcd-cluster-dropdown` | cluster picker step 1 |
| `vms-datagrid` | VM selection DataGrid |
| `assign-flavor-button` | toolbar action |
| `assign-host-config-button` | rolling migration hosts toolbar |
| `bulk-ip-edit-button` | toolbar action |
| `network-mapping-table` | mapping step |
| `storage-mapping-table` | mapping step |
| `status-filter` | MigrationsTable filter |
| `date-filter` | MigrationsTable filter |
| `delete-selected-button` | bulk delete toolbar |
| `confirm-delete-button` | delete dialog confirm |
| `pod-logs-drawer` | PodLogsDrawer |
| `logs-search-input` | log search input |
| `logs-follow-toggle` | follow toggle |
| `logs-download-button` | download button |
| `bulk-ip-dialog` | BulkIPEditDialog |
| `flavor-assignment-dialog` | FlavorAssignmentDialog |
| `cutover-confirm-button` | TriggerAdminCutoverButton |
| `maas-config-detail-dialog` | MaasConfigDetailDialog |
| `host-config-assignment-dialog` | HostConfigAssignmentDialog |
| `rdm-config-panel` | RdmDiskConfigurationPanel |
| `hosts-datagrid` | ESXi hosts DataGrid (rolling migration) |

### Design decisions
- MIG-034 (orphan K8s resource cleanup): verify via intercepted DELETE calls, not real K8s
- MIG-030 (50+ VM stress): uses `MOCK_VM_LIST_LARGE` (55 VMs) in fixtures
- Error-handling / edge-case tests: use `page.route()` intercepts
- Smoke + happy-path: use mocked APIs for determinism (real env has non-deterministic cluster configs)
- Template polling mock: intercept `**migrationtemplates**` → POST returns pending, GET returns ready
- Validation tests: use `mockStandardFormApis` pattern + partially complete form states

### Shared setup patterns (used across validation/error/edge specs)

**`mockStandardFormApis(page)`** — mock vmwarecreds, vmwareclusters, pcdclusters, openstackcreds, migrationtemplates

Template route mock pattern (copy this into validation/error/edge specs):
```typescript
async function mockStandardFormApis(page: Page) {
  await mockRoute(page, API.vmwareCreds, 'GET', MOCK_VMWARE_CREDS_LIST)
  await mockRoute(page, API.vmwareClusters, 'GET', MOCK_VMWARE_CLUSTERS_LIST)
  await mockRoute(page, API.pcdClusters, 'GET', MOCK_PCD_CLUSTERS_LIST)
  await mockRoute(page, API.openstackCredByName('pcd-cred-1'), 'GET', MOCK_OPENSTACK_CRED_1)
  await mockRoute(page, API.openstackCreds, 'GET', MOCK_OPENSTACK_CREDS_LIST)
  await page.route('**migrationtemplates**', (route) => {
    const method = route.request().method()
    if (method === 'POST') {
      route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_PENDING) })
    } else if (method === 'GET') {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(MOCK_MIGRATION_TEMPLATE_READY) })
    } else {
      route.continue()
    }
  })
}
```

**`selectClustersAndWaitForVMs(page)`** — selects 'DC1-Cluster' + 'pcd-cluster-1', waits for `vms-datagrid`

**For validation tests** — open form, partially fill, then check error states:
```typescript
// Pattern: open form → fill step 1 only → attempt submit → check errors
await goToMigrations(page)
await openMigrationDrawer(page)
await selectClustersAndWaitForVMs(page)
// Don't select VMs → submit should be blocked
await expectSubmitDisabled(page)
await expectSectionNavError(page, 'select-vms')
```

---

## To resume in new conversation

```
Continue E2E test suite generation for vJailbreak migration feature.

Steps 1–3 are done:
- Step 1: helpers + fixtures (migration.helpers.ts, migration.fixtures.ts)
- Step 2: smoke.spec.ts (MIG-001, MIG-002, MIG-003)
- Step 3: happy-path.spec.ts (MIG-004 to MIG-010)

Write Step 4: e2e/migration/validation.spec.ts (MIG-011 to MIG-019).

See migration-e2e-test-handoff.md for all context, fixtures, API patterns, testid table,
and shared setup patterns needed for validation tests.
```
