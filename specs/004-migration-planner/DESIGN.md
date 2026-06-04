# Feature Design: Inventory Management / Migration Planner

**Feature Branch (proposed)**: `004-migration-planner`
**Created**: 2026-06-04
**Status**: Draft design — companion to `spec.md`
**Author**: Sarika
**Scope (v1)**: Single VMware credential

> This is a design/working document that complements the formal `spec.md`. It translates the
> product idea into concrete vJailbreak components, names the CRDs/UI files involved, states the
> invariants and algorithms, and records the resolved decisions and remaining clarifications.

---

## 1. Overview

Today, an operator migrates VMs by repeatedly walking the multi-step **Migration Form**
(`ui/src/features/migration/pages/MigrationForm.tsx`) and creating one migration at a time.
There is no persistent, at-a-glance view of "what was discovered" and "how I intend to
batch and sequence it."

The **Inventory** page introduces that missing layer. After a VMware credential is added,
vJailbreak discovers its VMs and presents:

1. A **Discovery summary** ("N VMs discovered from credential `<name>`").
2. One or more **Buckets** — named, persistent groups of VMs, each carrying a full migration
   configuration and a lifecycle status (not migrated / scheduled / in progress / migrated). A
   **Default bucket** is auto-created and auto-populated with the safest-to-migrate VMs.
3. A **Trigger** action that lets the operator select multiple buckets and, before launch,
   shows (a) a **recommended migration-agent count** (editable with +/-) and (b) a
   **recommended bucket execution order** — both produced by planning algorithms.

The page sits alongside the existing nav (Migrations, Credentials, Settings) as a new
top-level sidebar item.

### Goal
Turn a flat list of discovered VMs into an organized, schedulable, sequenced migration plan
— with sane auto-defaults so an operator can go from "credential added" to "migration
triggered" with minimal clicks, while still being able to edit everything.

### Hard constraint
The existing `Migration` / `MigrationPlan` objects and the entire migration **execution
workflow must remain untouched**. The planner adds a new CRD and a new page, and at trigger time
*composes* the existing CRDs exactly as the Migration Form does today.

### Non-goals (v1)
- Multiple VMware credentials simultaneously (the page is scoped to one cred for v1).
- Replacing the existing Migration Form. The planner *reuses* the same configuration model.
- Cross-bucket VM sharing (explicitly disallowed — see Invariants).
- Any change to migration execution, `Migration`/`MigrationPlan` schemas, or their controllers.

---

## 2. Where this lives in the codebase

### 2.1 Frontend (`ui/`)
| Concern | Existing file / pattern to follow |
|---|---|
| Sidebar + nav item | `ui/src/config/navigation.tsx` — add a new top-level `NavigationItem` (same shape as `migrations` / `credentials-group`) |
| Routes | `ui/src/App.tsx` — add `<Route>`s under the `/dashboard` `DashboardLayout` block |
| Page feature folder | new `ui/src/features/inventory/` mirroring `ui/src/features/migration/` (pages / steps / components / hooks / types) |
| Box/card layout | `SurfaceCard` (design-system) |
| Side drawer (duplicate/edit/trigger) | `DrawerShell` / `DrawerHeader` / `DrawerBody` / `DrawerFooter` |
| Multi-select VM dropdown | `RHFAutocomplete multiple` |
| Status indicator chip | MUI `<Chip>` with `color` mapped to status |
| Date-time picker (schedule) | `RHFDateField` / `RHFDateTimeField` (supports `shouldDisableTime` for "future only") |
| Data grid (VM lists) | `CommonDataGrid` + `CustomSearchToolbar` |
| Confirm dialog (delete) | `ConfirmationDialog` |
| Migration config form (reuse) | the steps in `ui/src/features/migration/steps/` — `SourceDestinationClusterSelection`, `VmsSelectionStep`, `NetworkAndStorageMappingStep`, `SecurityGroupAndServerGroup`, `MigrationOptionsAlt` |
| VM data hook | `useVMwareMachinesQuery()` (`ui/src/hooks/api/useVMwareMachinesQuery.ts`) |
| Creds hooks | `useVmwareCredentialsQuery()`, `useOpenstackCredentialsQuery()` |
| Plans / rolling plans | `useMigrationsQuery()`, `useRollingMigrationPlansQuery()` |

> Note: confirm during spec phase whether the current Migration Form already auto-defaults
> network/storage mappings (first-to-first) or whether that logic must be added in the
> planner. The cluster selections are not currently auto-detected/auto-mapped.

### 2.2 Backend (`k8s/migration/`)
| Concern | Existing type (verified in `k8s/migration/api/v1alpha1/`) |
|---|---|
| Discovered VM (one CR per VM) | `VMwareMachine` — `Spec.VMs` (`VMInfo`), `Status.PowerState`, `Status.Migrated` |
| VM attributes for selection/ordering | `VMInfo.VMState` (power), `VMInfo.NetworkInterfaces []NIC` (NIC count), `VMInfo.ClusterName`, `VMInfo.CPU`, `VMInfo.Memory`, `VMInfo.Disks []Disk` (`Disk.CapacityGB`) |
| VM discovery trigger | `VMwareCredsReconciler` → `utils.GetAndCreateAllVMs()` creates `VMwareMachine` CRs |
| One migration group + config | `MigrationPlan` — `Spec.VirtualMachines [][]string`, `Spec.MigrationStrategy` (`DataCopyStart`, `VMCutoverStart`, `VMCutoverEnd`), `Status.MigrationStatus` |
| Multi-group sequenced orchestration | `RollingMigrationPlan` — `Spec.ClusterSequence []ClusterMigrationInfo` (`VMSequence`, `VMMigrationBatchSize`), `Spec.ClusterMapping`, `Spec.VMMigrationPlans []string` |
| Migration template (mappings refs) | `MigrationTemplate` — `NetworkMapping`, `StorageMapping`, `Source`, `Destination`, `TargetPCDClusterName` |
| Network / storage mapping | `NetworkMapping.Spec.Networks []Network{source,target}`, `StorageMapping.Spec.Storages []Storage{source,target}` |
| Destination clusters | `OpenstackCreds.Spec.PCDHostConfig []HostConfig` (first entry = default cluster) |
| Dest networks / volume types | `OpenstackCreds.Status.Openstack.Networks []PCDNetworkInfo`, `.VolumeTypes []string`, `.SecurityGroups`, `.ServerGroups`, `Spec.Flavors` |
| Migration agent / worker | `VjailbreakNode` — `Spec.NodeRole`, `Status.Phase`, `Status.ActiveMigrations []string` |

---

## 3. Terminology mapping

| Product term | Meaning | Maps to |
|---|---|---|
| **Inventory** | All VMs discovered from the (single) VMware credential | set of `VMwareMachine` CRs filtered by cred label |
| **Discovery box** | "N VMs discovered from `<cred>`" summary card | derived count, no new CR |
| **Bucket** | Named, persistent group of VMs + one migration config + status + schedule | **new `MigrationBucket` CRD** — see §8 |
| **Default bucket** | Auto-created bucket of the safest VMs (fallback tiers, §5) | `MigrationBucket` flagged `isDefault: true` |
| **Bucket status** | not migrated / scheduled / in progress / migrated | derived from the bucket's `MigrationPlan`/`Migration` status |
| **Trigger** | Select multiple buckets → suggest agents + order → launch | composes selected buckets into existing `MigrationPlan` + `RollingMigrationPlan` |
| **Agent** | Migration worker that does disk copy/convert | `VjailbreakNode` (role `worker`) |

---

## 4. Page layout & lifecycle

### 4.1 Empty state
Before any VMware credential exists, the Inventory page is empty (a call-to-action pointing
to **Credentials → VMware**).

### 4.2 On first credential add
When the **first** VMware credential is validated and discovery completes:
1. Render the **Discovery box**: *"N VMs discovered from credential `<credName>`."*
2. The backend (a `MigrationBucket` reconciler) auto-creates the **Default bucket**
   (`isDefault: true`), pre-populated per the fallback selection rule (§5). If no VM is
   eligible, no default bucket CR is created and the page shows the discovered VMs with a
   "no default bucket created" message.

### 4.3 Steady state
A vertical stack of cards: Discovery box on top, then one `SurfaceCard` per bucket. Each
bucket card shows its name, VM count, status chip, and an actions menu. A **Trigger** button
(placement consistent with existing UI) opens the multi-bucket trigger drawer.

---

## 5. Default bucket — selection rule (resolved)

The Default bucket is auto-populated using the first **non-empty tier** of this priority chain:

1. **Powered-off VMs with a single NIC** — the safest/simplest (no live state, trivial network
   mapping).
2. else **powered-off VM(s) with the fewest NICs** — still no live state.
3. else **powered-on VM(s) with the fewest NICs** — simplest among running VMs.
4. else **defer** — create no bucket CR; show discovered VMs with a "no default bucket created"
   message.

Signals: powered off = `VMwareMachine.Status.PowerState` (`VMInfo.VMState`); NIC count =
`len(VMInfo.NetworkInterfaces)`. This honors the "no empty bucket" invariant — the default
bucket is only created when a tier yields ≥ 1 VM.

---

## 6. Bucket card: status & actions

### 6.1 Status (chip on the card)
| Status | When | Source |
|---|---|---|
| **Not migrated** (inactive) | Bucket exists, not selected for migration | no active `MigrationPlan`/`Migration` for it |
| **Scheduled** | A future schedule time is set / it's queued in a trigger | bucket schedule field + plan `DataCopyStart` in the future |
| **In progress** | Migration running | `MigrationPlan.Status.MigrationStatus == Running` (or member `Migration.Status.Phase` running) |
| **Migrated** | All VMs migrated | `MigrationPlan.Status.MigrationStatus == Succeeded` / all members `Migrated` |

Render with the existing status-chip pattern (`<Chip color=...>`), reusing the color
conventions already used on the Migrations and Credentials pages.

### 6.2 Per-bucket actions
- **Default bucket:** `Edit`, `Duplicate` only — **no Delete**.
- **All other buckets:** `Edit`, `Duplicate`, `Delete` (with `ConfirmationDialog`).

---

## 7. Bucket configuration, Edit, and Duplicate

### 7.1 Default migration configuration for a bucket
Every bucket carries the same configuration the Migration Form produces, pre-filled with these
**auto-defaults** (operator can override in Edit):

| Setting | Auto-default | Source |
|---|---|---|
| **Selected VMs** | The bucket's VMs (all selected) | bucket membership |
| **VMware source cluster** | Auto-detected from the selected VMs' cluster | `VMInfo.ClusterName` |
| **Destination (PCD) cluster** | **First** entry in the OpenStack creds cluster list | `OpenstackCreds.Spec.PCDHostConfig[0]` |
| **Network mapping** | **First** source network → **first** destination network | dest from `OpenstackCreds.Status.Openstack.Networks[0]` |
| **Storage mapping** | **First** source datastore → **first** destination volume type | dest from `OpenstackCreds.Status.Openstack.VolumeTypes[0]` |
| **Security groups** | None selected | default behavior |
| **Server group** | None selected | default behavior |
| **All other advanced options** | Unselected / defaults | same as current Migration Form defaults |

> Caveat (Q5, open): a bucket may contain VMs spanning multiple source clusters.
> `MigrationPlan`/`RollingMigrationPlan` model sequences **per cluster**. Decide whether a bucket
> is constrained to a single source cluster, or whether one bucket can fan out into multiple
> per-cluster plans at trigger time (recommend: allow multi-cluster, expand at trigger).

### 7.2 Edit bucket
Opens a form **identical to the Migration Form** (reuse the same step components), with the
bucket's VMs pre-selected and config pre-filled. Primary button reads **Save** — it updates the
bucket, it does not launch a migration.

### 7.3 Duplicate bucket (resolved)
Clicking **Duplicate** opens a popup/drawer titled *"Select VMs to keep in the duplicate
bucket."* It presents a multi-select of **all VMs in the inventory** (`RHFAutocomplete multiple`
fed by `useVMwareMachinesQuery`). VMs already assigned to another bucket are **greyed out and
labelled "already in a bucket" and cannot be selected** (block, not move). The new bucket
inherits the source bucket's config as a starting point.

### 7.4 Invariants (axioms — enforced UI + backend)
1. **No empty buckets.** A bucket must contain ≥ 1 VM.
2. **VM uniqueness across buckets.** A VM belongs to at most one bucket at a time.

Enforced in the UI (validation, disabled controls) and on the backend (validation in the
`MigrationBucket` reconciler/webhook) so the rules can't be violated via the API.

---

## 8. Architecture: `MigrationBucket` CRD (resolved — Option A)

A bucket is its own custom resource, **`MigrationBucket`**, with (proposed) shape:
- `Spec.VMwareCredsRef` — source credential (keeps multi-cred a clean future extension).
- `Spec.VMs []string` — member VMs.
- `Spec.IsDefault bool` — default-bucket flag (non-deletable).
- `Spec.Schedule *metav1.Time` — optional future schedule.
- Embedded migration configuration mirroring the Migration Form output (`MigrationPlanStrategy`
  + network/storage mappings + security/server groups + advanced options).
- `Status.Phase` — NotMigrated / Scheduled / InProgress / Migrated (derived).

At **trigger** time the selected buckets are compiled into the existing CRDs:

```
selected MigrationBuckets  ──compile──▶  N × MigrationPlan (one per bucket, or per cluster)
                                         + 1 × RollingMigrationPlan (ordering + batch sizing)
                                         + scale VjailbreakNode worker count to suggestion
```

This keeps the execution CRDs (`MigrationPlan`, `RollingMigrationPlan`) and the migration
workflow **untouched** (hard constraint / FR-024): the planner only creates them, the same way
the Migration Form does. New work is limited to the `MigrationBucket` CRD + reconciler + RBAC +
UI types (requires `make generate`).

---

## 9. Trigger flow (multi-bucket launch)

1. Operator clicks **Trigger**. A **drawer** lists all buckets with **checkboxes**; operator
   selects any subset (1..N).
2. On **Submit/Trigger**, a **popup** appears with two planning outputs:
   - **(a) Recommended agent count** — `VjailbreakNode` workers to scale up, with **+/-**
     controls (floor 0; ceiling `A_max`, Q8).
   - **(b) Recommended bucket order** — the migration sequence, editable (Q9).
3. The popup offers **Trigger now** vs **Schedule**:
   - **Trigger now takes priority.** If triggered now, per-bucket schedules are ignored for this
     run.
   - Otherwise each bucket honors its **per-bucket schedule time** (§9.3).
4. On confirm: scale workers, create `MigrationPlan`(s) + a `RollingMigrationPlan` encoding the
   chosen order/batch sizes, and update each bucket's status.

### 9.1 Agent-count recommendation algorithm

This is the "intelligence" the trigger popup shows: given the VMs the operator wants to
migrate and the capacity that already exists, recommend how many **new** agent nodes to scale
up. It is a CPU-resource model. The recommendation is always **explainable** — the popup
states the numbers it was derived from — and **editable** (the operator can +/- the value).

#### Notation

| Symbol | Meaning | Source in code |
|---|---|---|
| `t` | Total VMs to migrate across the selected buckets | bucket membership |
| `C` | CPU **request** of one v2v migration pod (cores) | `vjailbreakSettings.V2VHelperPodCPURequest`, applied at `migrationplan_controller.go:1234-1243` |
| `CR` | Total cores to run all `t` migrations at once = `t · C` | derived |
| `alloc(n)` | Schedulable CPU of node `n` = `corev1.Node.Status.Allocatable["cpu"]` | already net of kube/system-reserved |
| `used(n)` | Cores already consumed on node `n` | Σ CPU requests of pods on `n`, or `activeMigrations(n) · C` |
| `m` | Free cores on the **master** = `alloc(master) − used(master)` | master `VjailbreakNode` = `constants.VjailbreakMasterNodeName` |
| `Δᵢ` | Free cores on existing agent `i` = `alloc(i) − used(i)` | per ready worker `VjailbreakNode` |
| `ΣΔ` | Total free cores across existing agents = `Σ Δᵢ` | derived |
| `F` | Schedulable CPU a **fresh** agent adds = `alloc` of the agent flavor | from flavor / a reference agent node |
| `A_max` | Ceiling on new agents (OpenStack quota / configured max) | Q8 |

> Why `alloc()` and not the raw flavor `F` minus a reserve: Kubernetes `Allocatable` is already
> `capacity − kube-reserved − system-reserved`. The master therefore reports *less* than its
> flavor (control plane reserved) while agents report the *full* schedulable amount (no control
> plane). The master/agent asymmetry falls out automatically — there is no manual `−1`.
>
> Why the **request** and not the limit for `C`: the Kubernetes scheduler reserves cores based
> on a pod's `Requests`, not its `Limits`. The limit is only the burst ceiling. Sizing capacity
> must use the request.

#### Steps

1. **Compute demand.** `t` = count of VMs across the selected buckets. Read `C` from the
   vjailbreak-settings ConfigMap (`V2VHelperPodCPURequest`). Then `CR = t · C`.

2. **Compute master free cores `m`.** Fetch the master `corev1.Node`
   (`GetMasterK8sNode`). `m = alloc(master) − used(master)`, where `used` is computed by the
   method chosen in step 3. Floor at 0.

3. **Compute existing-agent free cores `ΣΔ`.** List `VjailbreakNode`s; keep those with
   `Spec.NodeRole == "worker"` and `Status.Phase == NodeReady`. For each such agent `i`, map
   it to its `corev1.Node` by name (`GetNodeByName(vj.Name)`) and compute `Δᵢ = alloc(i) − used(i)`
   (floor at 0), using one of:
   - **Method A — scheduler-accurate (recommended).** `used(i)` = sum of the CPU `Requests`
     of all pods scheduled on node `i` (list pods in `migration-system` where
     `pod.Spec.NodeName == vj.Name`; sum `container.Resources.Requests.Cpu()`). Captures every
     CPU consumer, not only migrations.
   - **Method B — vJailbreak-native (simpler, matches the original sketch).**
     `used(i) = len(GetActiveMigrations(vj.Name)) · C`. Uses only data vJailbreak already
     tracks; assumes each running migration consumes exactly one request.

   Sum: `ΣΔ = Σ Δᵢ`.

4. **Compute the deficit.** `Deficit = max(0, CR − (m + ΣΔ))`. This is the additional cores
   needed beyond what the master and existing agents can already absorb.

5. **Convert the deficit to agents.** `F` = schedulable CPU a fresh agent adds (the agent
   flavor's allocatable). `A_raw = ceil(Deficit / F)`.

6. **Clamp the result.** `A = clamp(A_raw, 0, A_max) = max(0, min(A_raw, A_max))`.
   - Lower bound `0`: if capacity is already sufficient (`Deficit == 0`), recommend scaling up
     **nothing** instead of a negative number.
   - Upper bound `A_max`: never recommend more agents than the environment can create
     (OpenStack vCPU/instance quota, or a configured maximum). If `A_raw > A_max`, surface a
     note that the workload exceeds one-shot capacity and will run in waves.

7. **Present and allow override.** Show `A` with +/- controls and the derivation, e.g.
   *"30 VMs × 2 cores = 60 needed; 12 free now (master 4 + agents 8); each agent adds 8 →
   suggest 7 new agents."*

#### Reference formula

```
C   = V2VHelperPodCPURequest                  # cores per migration (scheduler request)
CR  = t * C                                   # cores to run all t VMs concurrently
m   = max(0, alloc(master) - used(master))    # free cores on master
ΣΔ  = Σ over ready workers of max(0, alloc(i) - used(i))
A   = clamp( ceil( max(0, CR - (m + ΣΔ)) / F ), 0, A_max )
```

#### Worked example
`C = 2`, `t = 30` ⇒ `CR = 60`. Master free `m = 4`; two ready agents free `Δ = [3, 5]` ⇒
`ΣΔ = 8`. `Deficit = max(0, 60 − 12) = 48`. Fresh agent `F = 8` ⇒ `A_raw = ceil(48/8) = 6`.
With `A_max = 10`, `A = clamp(6, 0, 10) = 6` new agents.

#### Optional capped-concurrency variant
`CR = t · C` sizes to run **every** selected VM simultaneously (an aggressive upper bound). To
respect a concurrency cap (e.g. `RollingMigrationPlan.ClusterMigrationInfo.VMMigrationBatchSize`),
replace `t` with `min(t, max_concurrency)` in step 1; everything else is unchanged.

#### HotAdd disk refinement (uses `d_b`)
For the HotAdd copy method, concurrency per agent is **also** bounded by the vSphere 60-disks-
per-VM attach limit. Per-agent concurrency `k = min( floor(F / C), floor(60 / avg_disks_per_vm) )`.
If migrations are HotAdd, also require `A_disk = ceil( max(0, t − k · (a + 1)) / k )` (where `a`
is the number of existing agents and `+1` accounts for the master) and take
`A = clamp(max(A_cpu, A_disk), 0, A_max)`. For cold/normal copy, ignore the disk term. See
Q8 for sourcing `F`, `C`, and `A_max`.

### 9.2 Bucket-ordering algorithm

Goal: pick the order in which the selected buckets migrate. The v1 default is
**success-first** — order so the earliest buckets have the highest probability of a clean
migration. This surfaces easy wins first, de-risks the run, and builds operator confidence
before the harder buckets start.

#### Rationale — what makes a bucket "more likely to succeed"
Two per-VM signals, both already in the discovery data:
- **Power state.** Powered-off VMs migrate more reliably than powered-on ones — there is no
  live guest state to track and no cutover window to coordinate. A bucket dominated by
  powered-off VMs is lower-risk.
- **NIC count.** Fewer NICs per VM means simpler network mapping and fewer failure modes. A
  bucket whose VMs are mostly single-NIC is simpler to migrate.

The auto-created default bucket (all powered-off, single-NIC) maximizes both signals, so it
naturally sorts to the front — no special-casing required.

#### Notation (per selected bucket `b`)

| Symbol | Meaning | Source in code |
|---|---|---|
| `PO(b)` | Count of powered-off VMs in `b` | `VMwareMachine.Status.PowerState` (`VMInfo.VMState`) |
| `PON(b)` | Count of powered-on VMs in `b` | same |
| `f(b)` | Success score = `(PO(b) + 1) / (PON(b) + 1)` (Laplace-smoothed powered-off ratio) | derived |
| `modeNIC(b)` | Most frequent NIC-count per VM among `b`'s VMs (lower = simpler) | mode of `len(VMInfo.NetworkInterfaces)` |
| `size(b)` | Total disk size = `Σ Disk.CapacityGB` over `b`'s VMs | `VMInfo.Disks[].CapacityGB` |

> The `+1` smoothing on `f(b)` does two things: it avoids divide-by-zero when a bucket has no
> powered-on VMs (the all-powered-off default bucket would otherwise be `÷0`), and it keeps the
> score monotonic — more powered-off and/or fewer powered-on always raises it. An all-powered-off
> bucket gets the largest score and sorts first.

#### Ordering key (success-first, applied as a stable multi-key sort, highest success first)
1. **`f(b)` descending** — more powered-off relative to powered-on ⇒ higher success.
2. **`modeNIC(b)` ascending** — fewer NICs per VM ⇒ simpler ⇒ higher success.
3. **`size(b)` ascending** — among equals, smaller buckets first (quick wins / fail fast).
4. **bucket name ascending** — deterministic final tie-break.

In short: `ordering = sortDesc(buckets, by = f, then NIC-simplicity, then size, then name)`.

#### Why radix (counting) sort for the NIC signal
NIC-count-per-VM is a small bounded integer (typically 0–~10). Computing `modeNIC(b)` and
ranking VMs by NIC simplicity is therefore a **counting/LSD-radix sort** over that integer key:
`O(V + K)` (V = VMs, K = max NIC count), and **stable**, which is cheaper and cleaner than a
comparison sort. Concretely, a single pass tallies each VM's NIC count into a small array — that
tally yields `modeNIC(b)` directly (the index with the highest count) and, if a global VM-level
ranking is wanted, radix-sorting all VMs by the key `(poweredOn ? 1 : 0, nicCount)` ascending
puts the highest-success VMs at the front (which is exactly what the default bucket captures).
Bucket-level ordering itself is over a small number of buckets, so a comparator sort on the
composite key above is sufficient.

#### Steps
1. **Per-bucket single pass.** For each selected bucket, iterate its VMs once (from the
   `VMwareMachine` list) and: count powered-off vs powered-on (`Status.PowerState`); tally NIC
   counts (`len(VMInfo.NetworkInterfaces)`) into a small counting array; accumulate `size(b)`.
2. **Derive metrics.** `f(b) = (PO(b)+1)/(PON(b)+1)`; `modeNIC(b)` = index of the max in the
   NIC tally (radix/counting result); `size(b)` from the accumulator.
3. **Sort buckets** by the composite key (f desc, modeNIC asc, size asc, name asc).
4. **Emit the order** into `RollingMigrationPlan.Spec.VMMigrationPlans` (and
   `Spec.ClusterSequence` where buckets expand per cluster). The order remains
   **operator-editable** (drag-reorder); a manual order overrides the computed one.

#### Worked example
| Bucket | PO | PON | mode NIC | `f = (PO+1)/(PON+1)` |
|---|---|---|---|---|
| Default | 10 | 0 | 1 | 11.0 |
| C | 6 | 2 | 1 | 2.33 |
| B | 3 | 5 | 2 | 0.67 |

Order → **Default → C → B** (11.0 > 2.33 > 0.67). The default bucket leads; C beats B on both
powered-off ratio and NIC simplicity.

#### Selectable alternatives (expose later, not the v1 default)
- **Smallest-first** (quick wins): buckets ascending by `size(b)` / VM count.
- **Largest-first** (throughput): descending by `size(b)` to start long-poles earliest.

Document the chosen default weighting and the alternatives in `research.md` during the spec
phase (Q9 covers whether reorder editing ships in v1).

### 9.3 Per-bucket scheduling
- Each bucket has an optional **schedule time** (future only). UI: date-time picker with
  `shouldDisableTime`/min-date set to *now* so past times are disabled.
- Backend: maps to `MigrationPlanStrategy.DataCopyStart` (and/or a `MigrationBucket.Spec.Schedule`
  field). The controller already supports "await start time" phases
  (`AwaitingDataCopyStart`, `AwaitingCutOverStartTime`).
- **Precedence:** Trigger-now overrides schedule for the buckets included in that trigger.

---

## 10. Decisions (resolved) and remaining clarifications

**Resolved**
- **Naming:** the nav item / page is **"Inventory."**
- **Default-bucket creation:** **backend** — new `MigrationBucket` CRD; the controller creates
  the default bucket CR after discovery.
- **Trigger button placement:** keep **consistent with the current UI design**.
- **Default-bucket population / zero-match:** fallback tiers (§5); if no tier matches, **defer**
  (no bucket CR, show a message).
- **Duplicate conflict:** **block** — already-bucketed VMs are greyed and labelled "already in a
  bucket".
- **CRD strategy:** **Option A** (new `MigrationBucket` CRD).
- **Multiple credentials:** **single credential for v1**.
- **Migration workflow:** **untouched** — `Migration`/`MigrationPlan` and execution unchanged
  (FR-024).

**Remaining clarifications (carried into `/plan` → `research.md`)**
- **Q5 — Multi-cluster buckets:** constrain a bucket to one source cluster, or allow
  multi-cluster and expand per cluster at trigger time? (recommend allow + expand)
- **Q8 — Agent algorithm constants:** sources/values for `C` (per-migration CPU request), `F`
  (fresh-agent allocatable), and `A_max` (max new agents).
- **Q9 — Order editability:** does operator drag-reordering ship in v1, or read-only first?
- **VM-removed-from-inventory:** for an existing bucket member that disappears from discovery,
  drop silently vs flag the bucket.

---

## 11. Suggested phasing & next steps

**Phase 1 — Inventory + buckets (no trigger):**
- Nav item + route + empty/loaded page states.
- Discovery box; backend default-bucket auto-creation + fallback selection rule.
- `MigrationBucket` CRD + reconciler + RBAC; `make generate`; controller unit tests.
- Bucket cards: status chip, Edit (reuse Migration Form, Save semantics), Duplicate (block
  already-bucketed), Delete. Invariants enforced UI + backend.

**Phase 2 — Trigger + planning:**
- Multi-bucket trigger drawer.
- Agent-count recommendation algorithm (+/- override) — explainable.
- Bucket-ordering algorithm (success-first; alternatives later).
- Compilation to existing `MigrationPlan` + `RollingMigrationPlan`; worker scale-up.
- Trigger-now vs schedule precedence.

**Phase 3 — Scheduling polish & multi-cred readiness.**

**Then:** complete the spec-kit artifacts under `specs/004-migration-planner/`: `plan.md`
(Constitution Check), `research.md` (algorithm constants & throughput data), `data-model.md`
(the `MigrationBucket` CRD and compilation mapping), `contracts/crds.md`, and `tasks.md`.

---

## 12. Verified codebase references

CRD fields confirmed by reading `k8s/migration/api/v1alpha1/*_types.go`:
- `MigrationPlan`: `Spec.VirtualMachines [][]string`, `Spec.MigrationStrategy` with
  `DataCopyStart`/`VMCutoverStart`/`VMCutoverEnd metav1.Time`, `Status.MigrationStatus`.
- `VMwareMachine`: `Spec.VMs` → `VMInfo` (`VMState`, `NetworkInterfaces []NIC`, `ClusterName`,
  `CPU`, `Memory`, `Disks []Disk` with `CapacityGB`); `Status.PowerState`, `Status.Migrated`.
- `OpenstackCreds`: `Spec.PCDHostConfig []HostConfig`, `Spec.Flavors`,
  `Status.Openstack.{Networks,VolumeTypes,SecurityGroups,ServerGroups}`.
- `RollingMigrationPlan`: `Spec.ClusterSequence []ClusterMigrationInfo`
  (`VMSequence`, `VMMigrationBatchSize`), `Spec.ClusterMapping`, `Spec.VMMigrationPlans []string`.
- `VjailbreakNode`: `Spec.NodeRole`, `Status.Phase`, `Status.ActiveMigrations []string`.
- v2v pod CPU request/limit set in `migrationplan_controller.go:1234-1243` from
  `vjailbreakSettings.V2VHelperPodCPURequest` / `V2VHelperPodCPULimit`.
- Agent helpers in `pkg/utils/vjailbreaknodeutils.go`: `GetMasterK8sNode`, `GetNodeByName`,
  `GetActiveMigrations`, `IsNodeReady`.

UI references confirmed: `ui/src/config/navigation.tsx`, `ui/src/App.tsx`,
`ui/src/features/migration/` (form + steps), design-system components (`SurfaceCard`,
`DrawerShell`, `CommonDataGrid`, RHF form controls), API hooks in `ui/src/hooks/api/`.
