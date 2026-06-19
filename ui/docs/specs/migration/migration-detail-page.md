# Migration Detail Page — Technical Specification

**Route**: `/dashboard/migrations/:migrationName`  
**Entry file**: `src/features/migration/pages/MigrationDetailPage.tsx`  
**Feature branch**: `private/main/sarika/ui-fixes`  
**Created**: 2026-06-12  
**Last updated**: 2026-06-19  
**Status**: Implemented, visual-QA complete

---

## Purpose

Full-page view for a single `Migration` CRD. Shows live status, phase progress, contextual action buttons, error details, spec metadata, activity timeline, and streaming pod logs. Replaces previous table-only approach where users had no drill-down view.

---

## Route & Navigation

- Linked from `MigrationsTable` via **two** click targets:
  - Clicking the migration **name** (Name column) → navigates to `/dashboard/migrations/<name>`
  - Clicking the migration **progress bar** (Progress column) → same navigation
- `MigrationDetailHeader` renders a breadcrumb "Migrations > \<vmName\>" with the "Migrations" link navigating back.
- URL param: `migrationName` (k8s resource name, e.g. `migration-centos7-succeeded`).

---

## Component Tree

```
MigrationDetailPage
├─ MigrationDetailHeader          # breadcrumb, title, phase chip, action buttons, subtitle
├─ MigrationKpiStrip              # 6 KPI cells: Started, Total Elapsed, Remaining, Source, Destination, Agent
├─ MigrationNextActionBanner      # phase-contextual Alert (info/warning/success/error)
├─ Tabs: Overview | Debug logs | Events (disabled) | Resources (disabled)
└─ [overview tab]
   ├─ MigrationPhaseStepper       # 5-step horizontal rail
   ├─ MigrationErrorCard          # shown only when isMigrationFailed()
   │   OR MigrationPhaseDetail    # shown when not failed
   │       ├─ CopyingPhaseDetail        (copying/converting phases)
   │       ├─ AwaitingCutoverDetail     (AwaitingAdminCutOver / AwaitingCutOverStartTime)
   │       ├─ SuccessDetail            (Succeeded)
   │       └─ GenericActiveDetail      (Pending / Validating / fallback)
   └─ [2-col grid]
      ├─ MigrationActivityTimeline # conditions sorted by lastTransitionTime
      └─ MigrationSpecCard         # key/value grid of spec fields
   [logs tab]
   └─ MigrationDetailDebugLogs    # streaming pod logs with filter controls
```

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
- `vmwareCreds` → used for SOURCE KPI cell (`hostName || datacenter`) and header subtitle
- `openstackCreds` → used for DESTINATION KPI cell (`projectName`) and header subtitle
- `migrationTemplate` → fallback for `migrationType`, `networkMapping`, `storageMapping`
- `networkMapping`, `storageMapping` → shown in `MigrationSpecCard`

Returns `MigrationDetailResources | null`. 404s are non-fatal — cells show `—`.

`resources` is passed to both `MigrationDetailHeader` and `MigrationKpiStrip` from `MigrationDetailPage`.

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

---

## Component Details

### `MigrationDetailHeader`
**File**: `src/features/migration/components/detail/MigrationDetailHeader.tsx`

Props: `{ migration, onBack, onCutoverSuccess?, resources? }`

| Phase state | Buttons shown |
|-------------|---------------|
| Failed / ValidationFailed | Retry (disabled, tooltip) + Cancel migration |
| AwaitingAdminCutOver | TriggerAdminCutoverButton + Cancel |
| Active (not terminal/failed/cutover) | Cancel migration |
| Succeeded | No action buttons |

- Retry button is always disabled with tooltip "Retry is not yet available in this version".
- Cancel triggers a confirmation dialog → calls `deleteMigration()` → navigates back to list.
- Title shows `spec.vmName || metadata.name` with `letterSpacing: '-0.015em'` (condensed bold look).
- **Subtitle** (two formats):
  - When resources loaded: `"Migrating {vmName} from {source} to {dest}"` with monospace Fira Code for technical terms
  - Fallback (no resources): `"Migration: {metadata.name} · Plan: {plan}"` (monospace)
  - Source = `vmwareCreds.spec.hostName || datacenter || vmwareCredsRef`
  - Dest = `openstackCreds.spec.projectName || openstackCredsRef`

### `MigrationKpiStrip`
**File**: `src/features/migration/components/detail/MigrationKpiStrip.tsx`

6 cells in a horizontal flex strip. Technical value cells (Source, Destination, Agent) render in `"Fira Code", monospace` at `0.8rem`. All values use `fontWeight: 600`.

| Cell | Source | Font |
|------|--------|------|
| Started | `metadata.creationTimestamp` formatted to "Jun 12, 02:30" | regular |
| Total Elapsed | `calculateTimeElapsed(creationTs, status)` | regular |
| Remaining | `'Completed'` (Succeeded) / `'Halted'` (Failed/ValidationFailed) / `'—'` | regular |
| Source | `vmwareCreds.spec.hostName \|\| datacenter \|\| vmwareCredsRef \|\| '—'` | monospace |
| Destination | `openstackCreds.spec.projectName \|\| openstackCredsRef \|\| '—'` | monospace |
| Agent | `status.agentName \|\| '—'` | monospace |

**Known issue**: 6 cells may overflow on tablet/narrow viewports — no responsive fallback.

### `MigrationNextActionBanner`
**File**: `src/features/migration/components/detail/MigrationNextActionBanner.tsx`

Renders MUI `<Alert>` above the tabs:

| Phase | Severity | Message |
|-------|----------|---------|
| CopyingBlocks/ConvertingDisk/etc. | info | "Migration is running. No action required." |
| Pending | info | "Migration is queued. Waiting for an available agent." |
| AwaitingAdminCutOver | warning | "**Action required.** Data copy is complete…" |
| Succeeded | success | "**Migration succeeded.** …decommission when ready." |
| Failed/ValidationFailed | error | "**Migration halted.** Review error details…" |

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
- **Detail text**: human-readable status detail from `phaseUtils.ts`

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
- Cutover checklist (5 steps) with green check icons

**`SuccessDetail`** (Succeeded)
- Success-bordered card: "\<vmName\> is running in PCD"

**`GenericActiveDetail`** (Pending / Validating / fallback)
- Plain card showing current phase string

Returns `null` for Failed / ValidationFailed (ErrorCard shown instead).

### `MigrationErrorCard`
**File**: `src/features/migration/components/detail/MigrationErrorCard.tsx`

Shown only when `isMigrationFailed()` returns true.

- **Border**: `1px solid divider` + `4px solid error.main` left accent (not full error border)
- **Header**: error icon, `phase` label + `lastTransitionTime`, error title with `wordBreak: 'break-word'`, copy-diagnostic icon button
- **Error title source**: `errorCondition.message || (phase === 'ValidationFailed' ? 'Validation failed' : 'Migration failed')`
- **Error condition lookup**: `conditions.find((c) => c.type === 'Failed') || conditions.find((c) => c.status === 'False')`
- **"What happened" section**: all conditions where `status === 'False' || type === 'Failed'` — rendered as MUI `<Alert severity="error">`
- **Resolution steps**: 4 generic steps — numbered circles use `primary.main` filled background with white text (not gray)
- **"Raw conditions" accordion**: collapsible, shows all conditions in monospace

**Bug fixed 2026-06-12**: Previously `conditions.find((c) => c.type === 'Failed' || c.reason === 'Migration')` matched `Validated` condition first (all have `reason: 'Migration'`). Fixed to `c.type === 'Failed'` only.

### `MigrationActivityTimeline`
**File**: `src/features/migration/components/detail/MigrationActivityTimeline.tsx`

- Renders `status.conditions` sorted by `lastTransitionTime` ascending
- **Row layout**: `[timestamp col 60px] [icon+line col] [content col]` — timestamp is left of dot (matches design)
- Each entry: time in fixed-width monospace left column, colored icon, `type — message` text
- Icon: green check (`status === 'True'`), red error (`type === 'Failed' || status === 'False' && type !== 'Migrating'`), grey circle (other)
- Vertical connecting line: `width: '2px'` (not `width: 1` which maps to 100% in MUI sx)
- Header row: "Activity timeline" (left) + "View full history" stub link (right, `opacity: 0.5`, no-op)

**Bug fixed 2026-06-19**: `width: 1` on vertical connector rendered as `width: 100%` due to MUI sx fraction mapping. Fixed to `width: '2px'`.

### `MigrationSpecCard`
**File**: `src/features/migration/components/detail/MigrationSpecCard.tsx`

Uses `<KeyValueGrid items={items} labelWidth={180} mdGrids={1} />` — `mdGrids={1}` = 2-column layout, appropriate for half-page context. (`mdGrids={2}` = 4-column, causes overflow in this layout.)

Fields shown:

| Label | Source |
|-------|--------|
| Migration name | `metadata.name` |
| VM name | `spec.vmName` |
| Plan | `spec.migrationPlan || metadata.labels.migrationplan` |
| Migration type | `spec.migrationType || migrationTemplate.spec.migrationType` |
| Cutover | `spec.initiateCutover ? 'Admin initiated' : 'Automatic'` |
| Disconnect source network | `spec.disconnectSourceNetwork ? 'Yes' : 'No'` |
| Total disks | `status.totalDisks` |
| Pod ref | `spec.podRef` |
| Network mapping | from `resources.networkMapping` or template |
| Storage mapping | from `resources.storageMapping` or template |

### `MigrationDetailDebugLogs`
**File**: `src/features/migration/components/detail/MigrationDetailDebugLogs.tsx`

- Pod name from `spec.podRef`
- Uses `useDirectPodLogs({ podName, namespace, follow, sessionKey })`
- `follow = true` when phase is not terminal (live streaming), `false` once terminal
- **Known issue**: `follow` state variable setter removed to fix unused-var TS error. No UI toggle exists. `[follow] = useState(true)` — always true while active, disabled by `isTerminal` check.
- Log line format parsed: `HH:MM:SS.ms [SOURCE] LEVEL message`
- Filter controls: text search, log level dropdown (ALL/ERROR/WARN/INFO/DEBUG/SUCCESS), source dropdown (derived from log lines)
- Meta bar: "X / Y lines", error/warn count chips, live indicator (pulsing green dot)
- Actions: copy visible, download .txt, reconnect (increments `sessionKey`)
- Max height: 480px with scroll

---

## Layout

**Root container** (`MigrationDetailPage`):
```tsx
<Box sx={{ maxWidth: '100%', px: 3, py: 3 }}>
```

`maxWidth: '100%'` — not `maxWidth: 1280`. The dashboard content area is already constrained by the sidebar layout. Using a fixed pixel value extends the container beyond the visible viewport.

**Two-column grid** (Activity Timeline + Spec Card):
```tsx
<Box sx={{
  display: 'grid',
  gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
  gap: 2,
  mt: 2,
}}>
```

---

## Design System / Theming

### Font Stack

- **Body / UI**: `Fira Sans` (loaded via Google Fonts CDN in `index.html`, weights 300/400/500/700)
- **Monospace / technical values**: `"Fira Code", "SF Mono", "Monaco", "Consolas", "Roboto Mono", monospace`
  - Used in: KPI Source/Destination/Agent cells, header subtitle technical terms, activity timeline timestamps, raw conditions accordion
- **Eina04**: Loaded locally via `@font-face` in `ThemeContext.tsx` (not used in current typography definitions)

### Design Tokens (`src/theme/colors.ts`)

In addition to palette colors, the following design tokens are exported for use in detail/complex components:

```typescript
DESIGN_CODE_BG = '#0d1117'          // dark code block background
DESIGN_CODE_TEXT = '#e6edf3'         // code block text
DESIGN_BADGE_BG = '#f1f5f9'          // error code badge background (light)
DESIGN_BADGE_TEXT = '#475569'         // error code badge text
DESIGN_KPI_LABEL_LIGHT = '#64748b'   // KPI strip label color in light mode
DESIGN_KPI_LABEL_DARK = '#94a3b8'    // KPI strip label color in dark mode
```

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
| `migration-centos7-succeeded` | Succeeded | All steps green, success card |
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

---

## Open Issues / Deferred Work

| Item | Impact | Notes |
|------|--------|-------|
| Debug logs follow toggle | low | `follow` hardcoded `true`, no UI to pause live stream |
| KpiStrip 6-col responsive | low | Cells may overflow on tablet; no breakpoint fallback |
| SOURCE/DESTINATION KPIs show `—` in mock | non-blocking | vmwarecreds/openstackcreds endpoints not mocked; header subtitle also falls back to k8s name |
| Retry button wired to backend | blocked | No PATCH endpoint yet; button disabled with tooltip |
| No "View VM in PCD" link for Succeeded | deferred | OpenStack instance ID not exposed in Migration API |
| "View full history" link in ActivityTimeline | deferred | Events tab disabled; link stub renders but has no onClick |
| Events tab | deferred | Disabled stub |
| Resources tab | deferred | Disabled stub |
