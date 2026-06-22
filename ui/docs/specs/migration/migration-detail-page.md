# Migration Detail Page — Technical Specification

**Route**: `/dashboard/migrations/:migrationName`  
**Entry file**: `src/features/migration/pages/MigrationDetailPage.tsx`  
**Feature branch**: `1980-enhance-user-experience-with-new-ui`  
**Created**: 2026-06-12  
**Last updated**: 2026-06-22  
**Status**: Implemented, visual-QA complete

---

## Purpose

Full-page view for a single `Migration` CRD. Shows live status, phase progress, contextual action buttons, error details, activity events, migration detail info, and streaming pod logs. Replaces previous table-only approach where users had no drill-down view.

---

## Route & Navigation

- Linked from `MigrationsTable` via **three** click targets:
  - Clicking the migration **name** (Name column) → navigates to `/dashboard/migrations/<name>`
  - Clicking the migration **progress bar** (Progress column) → same navigation
  - Clicking the **"View migration details" icon** (Actions column) → same navigation (changed from opening modal)
- `MigrationDetailHeader` renders a breadcrumb "Migrations > \<vmName\>" with the "Migrations" link navigating back.
- URL param: `migrationName` (k8s resource name, e.g. `migration-centos7-succeeded`).

---

## Component Tree

```
MigrationDetailPage
├─ MigrationDetailHeader          # breadcrumb, title, phase chip, action buttons, subtitle
├─ MigrationKpiStrip              # KPI cells: Started, Total Elapsed, Source Cluster, Destination Cluster, Destination Tenant, Agent
├─ MigrationNextActionBanner      # phase-contextual Alert (info/warning/success/error)
├─ Tabs: Overview | Details | Events | Pod logs | Resources (disabled)
├─ [overview tab]
│  ├─ MigrationPhaseStepper       # 5-step horizontal rail
│  └─ MigrationErrorCard          # shown only when isMigrationFailed()
│      OR MigrationPhaseDetail    # shown when not failed
│          ├─ CopyingPhaseDetail        (copying/converting phases)
│          ├─ AwaitingCutoverDetail     (AwaitingAdminCutOver / AwaitingCutOverStartTime)
│          ├─ SuccessDetail            (Succeeded)
│          └─ GenericActiveDetail      (Pending / Validating / fallback)
├─ [details tab]
│  └─ MigrationDetailsTab         # flat layout: environment, general info, mappings, policies, image profiles
├─ [events tab]
│  └─ MigrationEventsTab          # conditions timeline with search/filter/sort
└─ [logs tab]
   └─ MigrationDetailDebugLogs    # streaming pod logs — dark theme viewer with filter controls
```

**Note**: Activity Timeline and Migration Spec sections were removed from Overview tab. Their content moved to Events tab and Details tab respectively (expanded versions).

---

## Data Fetching

### `useMigrationDetailQuery`
**File**: `src/features/migration/hooks/useMigrationDetailQuery.ts`

```typescript
useMigrationDetailQuery(migrationName: string, namespace?: string): UseQueryResult<Migration>
```

- Calls `getMigration(name, namespace)` → `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/:ns/migrations/:name`
- `refetchInterval`: 5s while active, 30s once terminal (Succeeded/Failed/ValidationFailed)
- `staleTime: 0` — always refetch on window focus
- Query key: `['migration', migrationName]`

### `useMigrationDetailResourcesQuery`
**File**: `src/hooks/api/useMigrationDetailResourcesQuery.ts`

Fetches related resources referenced by the migration:
- `vmwareCreds` → used for KPI fallback values and header subtitle
- `vmwareMachine` → primary source for **Source Cluster** KPI (`spec.vms.clusterName || label vjailbreak.k8s.pf9.io/vmware-cluster`)
- `openstackCreds` → used for DESTINATION TENANT KPI cell (`projectName`) and header subtitle
- `migrationTemplate` → source of **Destination Cluster** KPI (`spec.targetPCDClusterName`); fallback for `migrationType`, `networkMapping`, `storageMapping`
- `networkMapping`, `storageMapping` → shown in `MigrationDetailsTab`
- `pcdClusters` → available for cluster→tenant resolution
- `migrationPlan`, `openstackCredsList`, `rdmDisks`, `arrayCredsMapping` → used by `MigrationDetailsTab`

Returns `MigrationDetailResources | null`. 404s are non-fatal — cells show `—`.

`resources` is passed to `MigrationDetailHeader` and `MigrationKpiStrip` from `MigrationDetailPage`.  
`MigrationDetailsTab` calls the hook internally (React Query cache hit — same query key).

---

## Phase Model

### K8s Phases (from `Phase` enum in `src/features/migration/api/migrations.ts`)

| K8s Phase | Design Step | Notes |
|-----------|-------------|-------|
| `Pending` | 1 — Pending | Queued state |
| `AwaitingDataCopyStart` | 1 — Pending | Pre-copy queue |
| `Validating` | 2 — Validating | Pre-flight checks running |
| `ValidationFailed` | 2 — Validating | Terminal; shows ErrorCard |
| `CopyingBlocks` | 3 — Copying Blocks | Shows disk progress bars |
| `CopyingChangedBlocks` | 3 — Copying Blocks | CBT delta sync |
| `ConvertingDisk` | 3 — Copying Blocks | VMDK→QCOW2 conversion |
| `AwaitingAdminCutOver` | 4 — Cutover | Action required banner + CTA |
| `AwaitingCutOverStartTime` | 4 — Cutover | Scheduled window pending |
| `Succeeded` | 5 — Done | All steps green |
| `Failed` | 2 or 3 (inferred) | Terminal; shows ErrorCard |

### Design Phase Definitions (`DESIGN_PHASE_DEFS`)

```typescript
[
  { key: 'pending',    label: 'Pending',        stepLabel: 'Step 1' },
  { key: 'validating', label: 'Validating',     stepLabel: 'Step 2' },
  { key: 'copying',    label: 'Copying Blocks', stepLabel: 'Step 3' },
  { key: 'cutover',    label: 'Cutover',        stepLabel: 'Step 4' },
  { key: 'done',       label: 'Done',           stepLabel: 'Step 5' },
]
```

### `isMigrationFailed()` — controls ErrorCard vs PhaseDetail

```typescript
// src/features/migration/utils/phaseUtils.ts
export function isMigrationFailed(migration: Migration): boolean {
  const phase = migration.status?.phase
  return !!phase && (phase === Phase.Failed || phase === Phase.ValidationFailed)
}
```

### `failedDetail()` — stepper detail text for failed step (phaseUtils.ts)

Returns short phase-specific text; does NOT return the raw condition message (avoids repeating the full error shown in ErrorCard):

```typescript
function failedDetail(_migration: Migration, designIndex: number): string {
  switch (designIndex) {
    case 1: return 'Validation check failed. See error details below.'
    case 2: return 'Disk copy failed. See error details below.'
    case 3: return 'Cutover failed. See error details below.'
    default: return 'Migration halted. See error details below.'
  }
}
```

### Done step detail text (phaseUtils.ts)

`case 4: return 'Migration completed successfully.'`

---

## Component Details

### `MigrationDetailHeader`
**File**: `src/features/migration/components/detail/MigrationDetailHeader.tsx`

Props: `{ migration, onBack, onCutoverSuccess?, resources? }`

| Phase state | Buttons shown |
|-------------|---------------|
| Failed / ValidationFailed | Retry (disabled, tooltip) + **Delete migration** |
| AwaitingAdminCutOver | TriggerAdminCutoverButton + **Delete migration** |
| Active (not terminal/failed/cutover) | **Delete migration** |
| Succeeded | No action buttons |

- Retry button is always disabled with tooltip "Retry is not yet available in this version".
- **Delete migration** button triggers a confirmation dialog → calls `deleteMigration()` → navigates back to list.
- Dialog title: "Delete migration?" / confirm button: "Delete migration" / progress: "Deleting…"
- Title shows `spec.vmName || metadata.name` with `letterSpacing: '-0.015em'` (condensed bold look).
- **Subtitle** (two formats):
  - When resources loaded: `"Migrating {vmName} from {source} to {dest}"` with monospace Fira Code for technical terms
  - Fallback (no resources): `"Migration: {metadata.name} · Plan: {plan}"` (monospace)
  - Source = `vmwareCreds.spec.hostName || datacenter || vmwareCredsRef`
  - Dest = `openstackCreds.spec.projectName || openstackCredsRef`

### `MigrationKpiStrip`
**File**: `src/features/migration/components/detail/MigrationKpiStrip.tsx`

5 active cells (Remaining commented out). Technical value cells render in `"Fira Code", monospace` at `0.8rem`. All values use `fontWeight: 600`.

| Cell | Source | Font |
|------|--------|------|
| Started | `metadata.creationTimestamp` formatted to "Jun 12, 02:30" | regular |
| Total Elapsed | `calculateTimeElapsed(creationTs, status)` | regular |
| ~~Remaining~~ | ~~commented out~~ | _commented out_ |
| Source Cluster | `vmwareMachine.spec.vms.clusterName \|\| label[vmware-cluster] \|\| vmwareCreds.datacenter \|\| hostName \|\| '—'` | monospace |
| Destination Cluster | `migrationTemplate.spec.targetPCDClusterName \|\| '—'` | monospace |
| Destination Tenant | `openstackCreds.spec.projectName \|\| openstackCredsRef \|\| '—'` | monospace |
| Agent | `status.agentName \|\| '—'` | monospace |

**Source Cluster priority chain**: `vmwareMachine.spec.vms.clusterName` → `metadata.labels['vjailbreak.k8s.pf9.io/vmware-cluster']` → `vmwareCreds.spec.datacenter` → `vmwareCreds.spec.hostName` → `'—'`. Never falls back to the creds resource name.

**Known issue**: Cells may overflow on tablet/narrow viewports — no responsive fallback.

### `MigrationNextActionBanner`
**File**: `src/features/migration/components/detail/MigrationNextActionBanner.tsx`

Renders MUI `<Alert>` above the tabs:

| Phase | Severity | Message |
|-------|----------|---------|
| CopyingBlocks/ConvertingDisk/etc. | info | "Migration is running. No action required." |
| Pending | info | "Migration is queued. Waiting for an available agent." |
| AwaitingAdminCutOver | warning | "**Action required.** Data copy is complete…" |
| Succeeded | success | "**Migration succeeded.** The target VM is running in PCD." |
| Failed/ValidationFailed | error | "**Migration halted.** Review the error details below before retrying." |

### `MigrationPhaseStepper`
**File**: `src/features/migration/components/detail/MigrationPhaseStepper.tsx`

Horizontal 5-step rail. Each step has:
- **Circle icon** (40×40px):
  - `done` → green filled (`success.main`), white check ✓
  - `active` → blue filled (`primary.main`), white spinner
  - `failed` → red filled (`error.main`), white ✕
  - `pending` → transparent with `2px grey.300` border, small grey.400 center dot
- **Connector line** (2px height): colored to match left step's status
- **Step label** (`STEP N`): uppercase, `0.65rem`, `letterSpacing: 0.8`
  - `active` → `primary.main`, `fontWeight: 700`
  - `failed` → `error.main`, `fontWeight: 700`
  - `done` / `pending` → `text.disabled`, `fontWeight: 400`
- **Phase name** (body2): bold if active/failed, colored by status
- **Meta text**: elapsed time (done), "Xm Ys elapsed" (active), "Pending" (pending), "Halted · Xs" (failed)
- **Detail text**: human-readable status detail from `phaseUtils.ts` — short text for failed (no raw message)

Phase states derived by `derivePhaseStates(migration)` in `phaseUtils.ts`.

### `MigrationPhaseDetail`
**File**: `src/features/migration/components/detail/MigrationPhaseDetail.tsx`

Router — picks sub-component by K8s phase:

**`CopyingPhaseDetail`** (ConvertingDisk / CopyingBlocks / etc.)
- Shows disk rows built from `status.currentDisk` + `status.totalDisks`
- Each row: label, status chip, `LinearProgress` (determinate 100% = done, indeterminate = active, 0% = pending)

**`AwaitingCutoverDetail`** (AwaitingAdminCutOver / AwaitingCutOverStartTime)
- Warning-bordered card
- `TriggerAdminCutoverButton` inline in a callout box
- Cutover checklist (4 steps) with green check icons:
  1. Quiesce and power off the source VM in vCenter
  2. Run a final CBT delta sync to capture changed blocks
  3. Detach volumes from worker, attach to target instance
  4. Boot target VM in PCD and run guest health checks

**`SuccessDetail`** (Succeeded)
- Two-section card with `success.light` border:
  - **Header** (`px: 3, pt: 2.5, pb: 2`):
    - Green uppercase caption: `MIGRATION COMPLETE · {Xh Ym TOTAL}` — elapsed auto-calculated from `metadata.creationTimestamp` to last condition `lastTransitionTime`
    - Bold title: `{vmName} is running in PCD`
    - Subtitle: `"The VM has been migrated to PCD. Verify the target VM status in the destination environment."`
  - **Stat boxes row** (separated by top border, `px: 3, py: 2`, `display: flex, gap: 1.5`):
    - **TARGET VM**: `spec.vmName` / sub: `spec.podRef`
    - **DISKS MIGRATED**: `status.totalDisks` count
    - **AGENT**: `status.agentName`
    - Each box: individual border, uppercase label (`0.8rem`, `letterSpacing: 0.8`), bold `body1` value
- No health checks (not available from API)
- No action buttons (View VM in PCD / Download report / etc.)

**`GenericActiveDetail`** (Pending / Validating / fallback)
- Plain card showing current phase string

Returns `null` for Failed / ValidationFailed (ErrorCard shown instead).

### `MigrationErrorCard`
**File**: `src/features/migration/components/detail/MigrationErrorCard.tsx`

Shown only when `isMigrationFailed()` returns true.

**Layout**:
- **Header** (subtle red-tinted `bgcolor`, separated by bottom border):
  - Left: `WarningAmberIcon` + phase chip (red badge, red text) + `·` + timestamp
  - Right: "Copy diagnostic bundle" outlined button (`ContentCopyIcon`)
  - Below: Error title — `body1` `fontWeight: 700` `color: error.main` `wordBreak: break-word`
- **Body** (`px: 3, py: 2.5, display: grid, gap: 2.5`):
  - **Recommended resolution** (overline) → 4 numbered generic steps (blue filled circles)
  - **"Need more help?"** + `"Troubleshooting guide ↗"` link → `https://platform9.github.io/vjailbreak/guides/troubleshooting/troubleshooting/`
  - **"Show raw log lines from the failure (N)"** → collapsible accordion showing all conditions in monospace; count = failed conditions only

**Error title source**: `errorCondition.message || (phase === 'ValidationFailed' ? 'Validation failed' : 'Migration failed')`

**Error condition lookup**: `conditions.find((c) => c.type === 'Failed') || conditions.find((c) => c.status === 'False')`

**No separate "What happened" / "Why this happens" sections** — these were removed; the error title alone carries the primary message.

**Generic resolution steps**:
1. Review the error message and pod logs — "Go to the Pod logs tab and filter by ERROR or FATAL to find the root cause."
2. Check source VM accessibility — "Verify the vCenter host, ESXi host, and datastore are reachable from the vJailbreak appliance."
3. Verify target capacity — "Confirm the OpenStack Cinder pool has enough free capacity for all disks in the migration."
4. Retry the migration — "After addressing the issue, use the Retry button. vJailbreak will resume from the last checkpoint."

**Bug fixed 2026-06-12**: Previously `conditions.find((c) => c.type === 'Failed' || c.reason === 'Migration')` matched `Validated` condition first (all have `reason: 'Migration'`). Fixed to `c.type === 'Failed'` only.

### `MigrationEventsTab`
**File**: `src/features/migration/components/detail/MigrationEventsTab.tsx`

Enabled tab showing all `status.conditions` in an enhanced interactive timeline. Replaces the previous `MigrationActivityTimeline` component that was in the Overview two-column grid.

**Toolbar**:
- Search field (`minWidth: 240`) — filters by `type`, `message`, `reason`
- Status filter `ToggleButtonGroup`: All (N) | ✓ {success count} | ✗ {error count} | ○ {pending count}
- Sort `ToggleButtonGroup` (right-aligned): "Oldest first" | "Newest first"

**Status classification** (`conditionStatus(c)`):
- `'error'` → `type === 'Failed'` or (`status === 'False'` and `type !== 'Migrating'`)
- `'success'` → `status === 'True'`
- `'pending'` → everything else

**Event card layout** (per condition):
- Left: icon (20px) + vertical connector line (2px divider, minHeight 20)
- Right card (`border: '1px solid divider'`, `borderLeft: '3px solid {accentColor}'`, `borderRadius: 1.5`):
  - Row: **type** (bold) + status Chip (outlined) + full datetime (monospace, `0.72rem`)
  - Message (body2, text.secondary)
  - Reason (caption, text.disabled, `"Reason: {reason}"`)

**Accent colors**: success → `success.main`, error → `error.main`, pending → `divider`

**Empty states**: "No events recorded yet" / "No events match the current filters."

**Full datetime format**: `toLocaleString` with month/day/year/hour/minute/second

### `MigrationDetailsTab`
**File**: `src/features/migration/components/detail/MigrationDetailsTab.tsx`

Second tab (position 2). Shows same content as `MigrationDetailModal` (General + Advanced tabs) in a flat full-page layout. The modal (`src/components/migrations/MigrationDetailModal.tsx`) is NOT removed — this tab exists for team review/approval before the modal is deprecated.

**Data**: Calls `useMigrationDetailResourcesQuery({ open: true, migration })` internally — hits React Query cache from the page-level call.

**Sections (flat, no inner tabs)**:

1. **Migration Environment** (`SurfaceCard`) — `KeyValueGrid` using `MIGRATION_ENVIRONMENT_FIELDS` constants:
   - Source Datacenter, Source Cluster, ESXi Host, Destination Tenant, Destination Cluster

2. **General Info** (`SurfaceCard`) — `KeyValueGrid` using inline items:
   - VM Name, Migration Type, Created At, Guest OS, CPU, Memory, Total Disks, Network Adapters, vJailbreak Agent, RDM Disks
   - Network Details table (if NIC data present): MAC Handling, MAC Address, IP Handling, IP Addresses
   - RDM Disks table (if `data.rdmDisks` present)

3. **Mappings** (`SurfaceCard`) — two `MappingTable` sub-components:
   - Network Mapping (source → target)
   - Storage Mapping — normal: datastore → volume type; accelerated: datastore → array credentials

4. **Migration Policies** (`SurfaceCard`) — `KeyValueGrid` using `MIGRATION_POLICY_FIELDS` constants:
   - "View only enabled options" toggle (`Switch`) defaults to `true`
   - Post-migration script (shown in `Paper` code block if configured)

5. **Image Profiles** (`SurfaceCard`) — shown only when `planAdvanced.imageProfiles` is configured

**Imports from existing infra** (same as MigrationDetailModal):
- `normalizeMappingRows`, `isDefaultishValue` from `src/components/migrations/helpers`
- `MIGRATION_ENVIRONMENT_FIELDS`, `MIGRATION_POLICY_FIELDS` from `src/components/migrations/migrationDetailConstants`
- `KeyValueGrid`, `SurfaceCard`, `FieldLabel` from `src/components`
- `formatDateTime`, `formatDiskSize` from `src/utils`

### `TriggerAdminCutoverButton`
**File**: `src/features/migration/components/TriggerAdminCutover/TriggerAdminCutoverButton.tsx`

Confirmation dialog spacing: `DialogTitle` `px:3, pt:3, pb:1` / `DialogContent` `px:3, pb:2` / `DialogActions` `px:3, pb:3, gap:1`.

### `MigrationDetailDebugLogs` (tab: "Pod logs")
**File**: `src/features/migration/components/detail/MigrationDetailDebugLogs.tsx`

- Pod name from `spec.podRef`
- Uses `useDirectPodLogs({ podName, namespace, enabled: !isPaused, follow: isLive, sessionKey })`
- `isLive = !isTerminal && !isPaused`

**Toolbar** (single row, `flexWrap: 'nowrap'`, `overflowX: 'auto'`):
- Search: full-width text input with search/clear icon
- `LEVEL` label + Select (ALL/ERROR/WARN/INFO/DEBUG/SUCCESS)
- `SOURCE` label + Select (derived from log lines + "ALL")
- **Live indicator**: clickable `<button>` — pulsing green dot + "Live" text. Click toggles `isPaused`. Disabled (cursor: default) when `isTerminal`.
- **Follow** switch (`Switch` + label) — auto-scrolls to bottom when enabled
- Copy icon (`ContentCopyIcon`) — copies filtered lines to clipboard
- Download icon (`FileDownloadOutlinedIcon`) — saves filtered lines as `.txt`
- Reconnect icon (`SyncIcon`) — increments `sessionKey`, calls `reconnect()`

**Note**: Toolbar vertical dividers must use `width: '1px'` not `width: 1` — MUI sx treats `1` as `100%` (fraction).

**Meta bar** (dark `#161b22` background, `#30363d` border):
- `X / Y lines · N errors N warnings N info N debug`
- Counts are colored when non-zero (errors=`#f85149`, warnings=`#e3b341`, info=`#79c0ff`, debug=`#8b949e`)
- Right: italic "Logs are a debug aid. Use Overview tab for status."

**Log area** (always dark `#0d1117` regardless of app theme):
- Max height: 540px with scroll
- Log line format: `001 HH:MM:SS.ms [source] LEVEL message`
  - Line numbers: `#484f58` (dim)
  - Timestamps: `#8b949e`
  - Source `[name]`: `#79c0ff` (cyan) — only when structured format matches
  - Level: colored text (ERROR=`#f85149`, WARN=`#e3b341`, INFO=`#79c0ff`, SUCCESS=`#3fb950`, DEBUG=`#8b949e`)
  - Message: `#c9d1d9` (light)
- Unstructured lines (no regex match): line number + raw text
- "streaming…" indicator at bottom when live

**State**:
- `isPaused: boolean` — user-controlled live toggle
- `follow: boolean` — auto-scroll (writable)
- `sessionKey: number` — incremented on reconnect

---

## Layout

**Root container** (`MigrationDetailPage`):
```tsx
<Box sx={{ maxWidth: '100%', px: 3, py: 3 }}>
```

`maxWidth: '100%'` — not `maxWidth: 1280`. The dashboard content area is already constrained by the sidebar layout.

**Tab content**: full-width, no grid wrapper. Each tab renders its own layout internally.

---

## Design System / Theming

### Font Stack

- **Body / UI**: `Fira Sans` (loaded via Google Fonts CDN in `index.html`, weights 300/400/500/700)
- **Monospace / technical values**: `"Fira Code", "SF Mono", "Monaco", "Consolas", "Roboto Mono", monospace`
  - Used in: KPI Source/Destination/Agent cells, header subtitle technical terms, events tab timestamps, raw conditions accordion, pod log viewer
- **Eina04**: Loaded locally via `@font-face` in `ThemeContext.tsx` (not used in current typography definitions)

### Design Tokens (`src/theme/colors.ts`)

```typescript
DESIGN_CODE_BG = '#0d1117'          // dark code block / pod log background
DESIGN_CODE_TEXT = '#e6edf3'         // code block text
DESIGN_BADGE_BG = '#f1f5f9'          // error code badge background (light)
DESIGN_BADGE_TEXT = '#475569'         // error code badge text
DESIGN_KPI_LABEL_LIGHT = '#64748b'   // KPI strip label color in light mode
DESIGN_KPI_LABEL_DARK = '#94a3b8'    // KPI strip label color in dark mode
```

### MUI sx pitfall: `width: 1` = `width: 100%`

In MUI `sx`, numeric values for `width`/`height` are treated as fractions when ≤1. **Always use string `'1px'` for 1-pixel rules**, not `width: 1`. This caused two bugs: ActivityTimeline vertical connector (fixed 2026-06-19), pod log toolbar dividers (fixed 2026-06-22).

---

## K8s API Contract

```
GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/{namespace}/migrations/{name}
```

Returns a `Migration` object. Key fields used by detail page:

```typescript
{
  metadata: {
    name: string
    namespace: string
    creationTimestamp: string | Date
    labels?: Record<string, string>  // may include 'migrationplan'
  }
  spec: {
    vmName?: string
    migrationPlan?: string
    migrationType?: string
    podRef?: string                  // k8s pod name for log streaming
    initiateCutover?: boolean
    disconnectSourceNetwork?: boolean
  }
  status: {
    phase: Phase                     // see Phase enum
    conditions: Condition[]
    agentName?: string
    currentDisk?: string             // "1", "2" — current disk being copied
    totalDisks?: number
  }
}
```

`Condition` shape:
```typescript
{
  type: string            // 'Validated', 'DataCopy', 'Failed', 'Migrating', etc.
  status: 'True' | 'False' | 'Unknown'
  reason?: string
  message?: string
  lastTransitionTime?: string | Date
}
```

---

## Mock Server (local QA)

**Toggle**: `VITE_USE_MOCK_API=true` in `ui/.env`

**Mock data**: `ui/mock-json-server/mock-data/mock-migrations.json` — 5 migrations covering all phase states:

| Name | Phase | Use case |
|------|-------|---------|
| `migration-u22-converting-disk` | ConvertingDisk | Active copy with disk progress bars |
| `migration-win2019-cutover` | AwaitingAdminCutOver | Cutover CTA, warning banner |
| `migration-centos7-succeeded` | Succeeded | All steps green, success card with stat boxes |
| `migration-rhel8-failed-copy` | Failed (copy step) | Error card, failed condition |
| `migration-debian11-validation-failed` | ValidationFailed | Error card at step 2 |

**GET by name route** in `ui/mock-json-server/routers/mock-router.ts`:
```typescript
router.get(
  "/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/:namespace/migrations/:name",
  (req, res) => { /* find from mock-migrations.json by name */ }
)
```

**CORS fix** in `ui/mock-json-server/server.ts`:
```typescript
server.use(cors({ origin: true, credentials: true }))
// NOT: server.use(cors())  — wildcard origin breaks withCredentials: true on axios
```

**Known mock gap**: `vmwarecreds`, `openstackcreds`, `vmwaremachines` endpoints not mocked → Source Cluster / Destination Tenant KPIs show `—` in mock; header subtitle falls back to k8s resource name. Not a production issue.

---

## Open Issues / Deferred Work

| Item | Impact | Notes |
|------|--------|-------|
| KpiStrip responsive | low | Cells may overflow on tablet; no breakpoint fallback |
| Retry button wired to backend | blocked | No PATCH endpoint yet; button disabled with tooltip |
| No "View VM in PCD" link for Succeeded | deferred | OpenStack instance ID not exposed in Migration API |
| Events tab: "View full history" stub | deferred | Link renders but has no onClick |
| Resources tab | deferred | Disabled stub |
| Pod logs: source filter only works for structured lines | low | Unstructured log lines won't appear in any source bucket except ALL |
| MigrationDetailsTab replaces drawer | deferred | Drawer (`MigrationDetailModal`) kept until team approves detail page; remove drawer after approval |

---

## Change Log

| Date | Change |
|------|--------|
| 2026-06-12 | Initial implementation |
| 2026-06-19 | ActivityTimeline `width: '2px'` fix; ErrorCard condition lookup fix |
| 2026-06-22 | "View migration details" icon → navigates to detail page (was: open modal) |
| 2026-06-22 | KpiStrip: Remaining commented out; Source→Source Cluster (vmwareMachine source); new Destination Cluster cell; Destination→Destination Tenant |
| 2026-06-22 | Success banner: removed decommission text |
| 2026-06-22 | Done step detail: "Migration completed successfully." (was: "Target VM is healthy.") |
| 2026-06-22 | Cutover checklist: removed "Disconnect source network on the original VM" |
| 2026-06-22 | "Cancel migration" → "Delete migration" on all buttons/dialogs |
| 2026-06-22 | Tab "Debug logs" → "Pod logs" |
| 2026-06-22 | MigrationDetailDebugLogs: full dark-theme redesign; Live toggle; Follow auto-scroll; toolbar divider `'1px'` fix |
| 2026-06-22 | TriggerAdminCutoverButton: dialog padding improved |
| 2026-06-22 | **Events tab enabled** — `MigrationEventsTab` component with search/filter/sort; conditions moved from Overview timeline |
| 2026-06-22 | **Details tab added** (position 2) — `MigrationDetailsTab` flat layout matching drawer content; drawer kept for review |
| 2026-06-22 | Overview tab: removed ActivityTimeline + SpecCard + two-column grid |
| 2026-06-22 | Tab order: Overview → Details → Events → Pod logs → Resources |
| 2026-06-22 | ErrorCard redesign: header with phase chip + Copy bundle button; `body1` error title; troubleshooting link; "Show raw log lines" accordion |
| 2026-06-22 | `failedDetail()` in phaseUtils: no longer returns raw condition message — returns short phase-specific text |
| 2026-06-22 | SuccessDetail redesign: "MIGRATION COMPLETE · elapsed" header + stat boxes (Target VM, Disks, Agent); no health checks; no action buttons |
