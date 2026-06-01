# Migration Feature — E2E Test Case Index

**Feature**: `src/features/migration/`  
**Prefix**: `MIG`  
**Generated**: 2026-05-20

---

## Smoke Tests

These must pass before any release.

---

### MIG-001: Open and close standard migration form without submitting

**Priority**: Critical  
**Tags**: smoke, happy-path

**Preconditions**:
- App is loaded and authenticated
- At least one VMware credential and PCD credential exist

**Steps**:
1. Click "Start Migration" button → **Expected**: `MigrationFormDrawer` opens with step 1 visible
2. Observe section nav on left → **Expected**: 6 sections listed; Source and Destination is active
3. Click close (X) button → **Expected**: Confirmation or immediate close; drawer closes cleanly
4. Reopen drawer → **Expected**: Form resets to initial state (no stale values)

**Pass criteria**: Drawer opens, renders all sections, closes without errors.  
**Fail criteria**: Drawer fails to open; JS errors in console; stale state on reopen.

---

### MIG-002: Migrations list page loads and displays table

**Priority**: Critical  
**Tags**: smoke, happy-path

**Preconditions**: App authenticated; at least one migration exists.

**Steps**:
1. Navigate to `/migrations` → **Expected**: `MigrationsPage` loads
2. Observe table → **Expected**: Columns: Name, Status, Agent, Time Elapsed, Destination, Progress, Actions
3. Observe loading state → **Expected**: Skeleton or spinner shown while fetching
4. Data loads → **Expected**: Migrations listed with correct status chips

**Pass criteria**: Page renders, table populated, no errors.  
**Fail criteria**: Blank page; table empty with no explanation; console errors.

---

### MIG-003: Migration progress column renders correct phase icons

**Priority**: Critical  
**Tags**: smoke, happy-path

**Preconditions**: Migrations in various phases exist (Succeeded, Failed, Running, AwaitingAdminCutOver).

**Steps**:
1. Load `/migrations`
2. Locate Succeeded migration → **Expected**: Green checkmark icon in Progress column
3. Locate Failed migration → **Expected**: Red error icon
4. Locate Running migration → **Expected**: Spinner icon
5. Locate AwaitingAdminCutOver migration → **Expected**: Pause icon
6. Hover over Progress cell → **Expected**: Tooltip shows `progressText`

**Pass criteria**: All phase icons render correctly.  
**Fail criteria**: Wrong icon; no icon; tooltip missing.

---

## Happy Path Tests

---

### MIG-004: Complete standard migration — happy path

**Priority**: Critical  
**Tags**: happy-path

**Preconditions**:
- Valid VMware credential and PCD credential exist
- VMware cluster has at least 2 powered-on VMs
- PCD cluster has networks and storage configured
- No existing migrations blocking resources

**Steps**:
1. Click "Start Migration" → drawer opens
2. Step 1: Select VMware cluster from dropdown → **Expected**: Cluster name appears, credential auto-filled
3. Step 1: Select PCD cluster → **Expected**: PCD credential auto-filled; loading indicator for template creation
4. Wait for template polling → **Expected**: Loading disappears; Step 2 VM list populated
5. Step 2: Select 2 VMs → **Expected**: Toolbar shows "2 selected"; Submit button still disabled (steps incomplete)
6. Step 3: Observe network mappings → **Expected**: Source networks listed; unmapped count shown
7. Step 3: Map all source networks to PCD networks → **Expected**: Unmapped count drops to 0; step 3 nav icon clears error
8. Step 3: Map all source datastores to PCD volume types
9. Scroll to Step 5 (Migration Options) → **Expected**: Section activates in nav
10. Leave options at defaults
11. Scroll to Preview → **Expected**: Summary shows VM names, cluster names, mappings summary
12. Click Submit → **Expected**: Submit button shows spinner; disabled
13. Submit completes → **Expected**: Drawer closes; toast/snackbar "Migration created successfully"
14. MigrationsPage updates → **Expected**: New migration row appears with "Pending" or "Running" status

**Pass criteria**: MigrationPlan CRD created in cluster; migration appears in list.  
**Fail criteria**: Submit fails with error; drawer stays open; migration not created.

---

### MIG-005: Complete rolling migration — happy path

**Priority**: Critical  
**Tags**: happy-path

**Preconditions**:
- VMware cluster with 2+ ESXi hosts
- PCD cluster with pcdHostConfig entries configured
- MAAS bare metal config exists
- All hosts have bare metal configs

**Steps**:
1. Open Rolling Migration drawer
2. Step 1: Select source VMware cluster + destination PCD cluster
3. Step 2: Bare metal config list shows → Click config name → **Expected**: `MaasConfigDetailDialog` opens with syntax-highlighted config
4. Close details dialog
5. Step 3: ESXi hosts DataGrid shows with hosts listed → **Expected**: Each host shows host config dropdown
6. Select all hosts → Click "Assign Host Config" → **Expected**: `HostConfigAssignmentDialog` opens
7. Select a PCD host config → Click Apply → **Expected**: All hosts updated; validation error clears
8. Step 4: VMs load grouped by ESXi host → Select all VMs
9. Assign OS family to powered-off VMs
10. Step 5: Map networks and storage
11. Step 8: Click Submit → **Expected**: RollingMigrationPlan created
12. MigrationsPage: Rolling migration entries appear

**Pass criteria**: Rolling migration plan created; ESXi host configs updated.  
**Fail criteria**: Host config assignment fails; VMs not associated with hosts; plan creation error.

---

### MIG-006: Delete single migration

**Priority**: High  
**Tags**: happy-path

**Preconditions**: At least one migration exists.

**Steps**:
1. Navigate to `/migrations`
2. Click delete icon on a migration row → **Expected**: Confirmation dialog: "Delete migration?"
3. Click Cancel → **Expected**: Dialog closes; migration still visible
4. Click delete icon again → Confirm delete → **Expected**: Dialog closes; row removed; success snackbar shown
5. Reload page → **Expected**: Migration no longer listed

**Pass criteria**: Migration deleted from cluster.  
**Fail criteria**: Migration remains after delete; error snackbar; page error.

---

### MIG-007: Bulk delete multiple migrations

**Priority**: High  
**Tags**: happy-path

**Preconditions**: At least 3 migrations exist.

**Steps**:
1. Select 3 migrations via checkboxes → **Expected**: "3 selected" count in toolbar; Delete Selected button enabled
2. Click Delete Selected → **Expected**: Confirmation dialog shows count "Delete 3 migration(s)?"
3. Confirm → **Expected**: All 3 rows removed; success snackbar

**Pass criteria**: All selected migrations removed.  
**Fail criteria**: Partial delete; error on any deletion; UI count mismatch.

---

### MIG-008: View pod logs for a migration

**Priority**: High  
**Tags**: happy-path

**Preconditions**: A migration has a running or completed pod.

**Steps**:
1. Click log icon on migration row → **Expected**: `PodLogsDrawer` opens; "Loading logs..." shown briefly
2. Logs appear → **Expected**: Log lines rendered with level/timestamp coloring
3. Type in search box → **Expected**: Logs filtered matching search (fuzzy)
4. Select log level "ERROR" → **Expected**: Only ERROR lines shown
5. Click "Follow" toggle → **Expected**: Drawer scrolls to bottom; new logs auto-append
6. Pause → **Expected**: "Paused — N lines captured" alert
7. Reconnect → **Expected**: Log stream resumes
8. Download → **Expected**: `.txt` file downloaded containing logs

**Pass criteria**: Logs display correctly; all controls functional.  
**Fail criteria**: Empty logs with no error; controls unresponsive; download empty.

---

### MIG-009: Admin cutover trigger — single migration

**Priority**: High  
**Tags**: happy-path

**Preconditions**: A migration is in `AwaitingAdminCutOver` phase.

**Steps**:
1. Locate migration in `AwaitingAdminCutOver` status → **Expected**: Play icon button visible in Actions column
2. Click play icon → **Expected**: Confirmation dialog: "Trigger admin cutover for [name]?"
3. Click Cancel → **Expected**: Dialog closes; migration phase unchanged
4. Click play icon again → Confirm → **Expected**: Loading spinner on button
5. API call completes → **Expected**: Migration phase updates (Succeeded or next phase); success state

**Pass criteria**: Migration phase advances after cutover.  
**Fail criteria**: Error shown; migration stuck in AwaitingAdminCutOver; button unresponsive.

---

### MIG-010: Status and date filtering in migrations table

**Priority**: Medium  
**Tags**: happy-path

**Preconditions**: Migrations in multiple statuses exist.

**Steps**:
1. Click Status filter → select "Failed" → **Expected**: Only failed migrations shown; count updates
2. Click Status filter → select "Succeeded" → **Expected**: Only succeeded shown
3. Click Status filter → select "All" → **Expected**: All migrations shown
4. Click Date filter → select "Last 24 hours" → **Expected**: Only recent migrations shown
5. Combine status + date filters → **Expected**: Intersection applied correctly

**Pass criteria**: Filters combine correctly; counts accurate.  
**Fail criteria**: Wrong migrations shown; counts wrong; filters not persisted during session.

---

## Validation Tests

---

### MIG-011: Step 1 — Cannot proceed without cluster selection

**Priority**: High  
**Tags**: validation

**Preconditions**: Migration form open.

**Steps**:
1. Attempt to scroll past Step 1 without selecting clusters
2. Observe section nav → **Expected**: Step 1 shows incomplete indicator
3. Click Submit → **Expected**: Submit disabled or error shown at Step 1
4. Select VMware cluster only → **Expected**: Still shows PCD cluster required
5. Select both clusters → **Expected**: Step 1 complete; no error

**Pass criteria**: Cannot submit with incomplete step 1.  
**Fail criteria**: Submit enabled without cluster selection; no feedback.

---

### MIG-012: Step 2 — Cannot submit without VM selection

**Priority**: High  
**Tags**: validation

**Steps**:
1. Complete Step 1
2. Step 2: Do not select any VMs
3. Check Submit state → **Expected**: Submit disabled
4. Check section nav → **Expected**: Step 2 shows error/incomplete
5. Select one VM → **Expected**: Error clears; Submit may be enabled (pending other steps)

**Pass criteria**: Enforced — at least 1 VM required.  
**Fail criteria**: Submit enabled with 0 VMs selected.

---

### MIG-013: OS family required for powered-off VMs

**Priority**: High  
**Tags**: validation

**Preconditions**: At least one powered-off VM available.

**Steps**:
1. Complete Step 1
2. Step 2: Select a powered-off VM
3. OS family column → **Expected**: OS selector shows or warning indicator appears
4. Attempt to submit → **Expected**: Error: "OS family required for powered-off VMs"
5. Assign OS family (Windows or Linux) → **Expected**: Validation error clears

**Pass criteria**: OS assignment enforced for powered-off VMs.  
**Fail criteria**: Can submit without OS assignment; no error shown.

---

### MIG-014: Network mapping validation

**Priority**: High  
**Tags**: validation

**Steps**:
1. Complete Steps 1–2 with VMs that have multiple networks
2. Step 3: Map only some networks (leave at least one unmapped)
3. Check section nav → **Expected**: Step 3 shows error badge + unmapped count
4. Attempt submit → **Expected**: Blocked; scroll to Step 3
5. Map remaining networks → **Expected**: Error badge disappears
6. Submit now enabled

**Pass criteria**: All networks must be mapped to submit.  
**Fail criteria**: Can submit with unmapped networks.

---

### MIG-015: Storage mapping validation

**Priority**: High  
**Tags**: validation

**Steps**:
1. Complete Steps 1–2
2. Step 3: Leave datastores unmapped
3. Attempt submit → **Expected**: Blocked with storage mapping error
4. Map all datastores → **Expected**: Error clears

**Pass criteria**: All storage must be mapped.  
**Fail criteria**: Can submit with unmapped storage.

---

### MIG-016: IP address format validation in bulk IP dialog

**Priority**: High  
**Tags**: validation

**Steps**:
1. Step 2: Select powered-off VMs → open Bulk IP Edit dialog
2. Enter invalid IP "999.999.999.999" → **Expected**: Input shows error; validation status "invalid"
3. Enter invalid IP "not-an-ip" → **Expected**: Validation error
4. Enter partial IP "192.168.1" → **Expected**: Validation error
5. Apply button → **Expected**: Disabled while invalid IPs present
6. Enter valid IP "192.168.1.100" → **Expected**: Validation clears; Apply enabled

**Pass criteria**: Invalid IPs blocked; clear feedback per field.  
**Fail criteria**: Can apply invalid IPs; no per-field feedback.

---

### MIG-017: Migration options — cutover time validation

**Priority**: Medium  
**Tags**: validation

**Steps**:
1. Step 5: Enable "Data Copy Start Time" option
2. Enter past datetime → **Expected**: Error: "Date must be in the future"
3. Enable "Cutover Window" → set end time before start time → **Expected**: Error: "End must be after start"
4. Fix dates to valid future datetimes → **Expected**: Errors clear

**Pass criteria**: Time constraints validated correctly.  
**Fail criteria**: Past dates accepted; inverted cutover window accepted.

---

### MIG-018: Security group profile conflict detection

**Priority**: Medium  
**Tags**: validation

**Steps**:
1. Step 4: Select two image profiles that have conflicting property values
2. **Expected**: Error shown: "Conflicting profile properties detected"
3. Attempt submit → **Expected**: Blocked
4. Deselect conflicting profile → **Expected**: Error clears

**Pass criteria**: Profile conflicts detected and blocked.  
**Fail criteria**: Conflicting profiles accepted silently.

---

### MIG-019: Rolling migration — ESXi hosts require host config

**Priority**: High  
**Tags**: validation

**Steps**:
1. Complete Step 1 of rolling migration
2. Step 3: Leave at least one ESXi host without a host config
3. Observe section nav → **Expected**: Step 3 shows error
4. Attempt submit → **Expected**: Blocked with message "All ESXi hosts must have host config"
5. Assign config to remaining host → **Expected**: Error clears

**Pass criteria**: Cannot submit rolling migration without all hosts configured.  
**Fail criteria**: Submission allowed with unconfigured hosts.

---

## Error Handling Tests

---

### MIG-020: API error on standard migration submission

**Priority**: Critical  
**Tags**: error-handling

**Preconditions**: All steps complete; simulate server error (500) on `/migration-plans` POST.

**Steps**:
1. Complete all steps → click Submit
2. Server returns 500 on migration plan creation → **Expected**: Error toast shown with message
3. Submit button re-enables → **Expected**: Loading state cleared
4. Form state preserved → **Expected**: All selections still present
5. User can retry submit

**Pass criteria**: Error surfaced; form not reset; retry possible.  
**Fail criteria**: Form reset on error; silent failure; spinner stuck.

---

### MIG-021: Network mapping API error

**Priority**: High  
**Tags**: error-handling

**Preconditions**: Simulate 422 error on `/network-mappings` POST.

**Steps**:
1. Complete all steps → click Submit
2. Network mapping creation fails → **Expected**: Error toast; partial cleanup handled; no orphan resources
3. Submit re-enabled for retry

**Pass criteria**: Error shown; idempotent retry possible.  
**Fail criteria**: Orphan NetworkMapping resource created; no error feedback.

---

### MIG-022: Credential fetch failure in Step 1

**Priority**: High  
**Tags**: error-handling

**Steps**:
1. Select VMware cluster → simulate credential fetch 500 error
2. **Expected**: Error shown in Step 1: "Failed to load VMware credentials"
3. Retry mechanism → **Expected**: Retry button or auto-retry visible

**Pass criteria**: Error displayed; user can recover.  
**Fail criteria**: Form silently broken; no indication of credential fetch failure.

---

### MIG-023: Migration template polling timeout / failure

**Priority**: High  
**Tags**: error-handling

**Steps**:
1. Select clusters in Step 1 → template creation starts
2. Simulate template never becoming ready (status stays empty)
3. After timeout → **Expected**: Error shown; VM list not empty (shows loading skeleton or error)
4. Simulate template creation failure → **Expected**: Error alert in drawer

**Pass criteria**: User informed of template failure; form does not hang indefinitely.  
**Fail criteria**: Infinite loading spinner; no timeout; form appears functional but VMs never load.

---

### MIG-024: IP validation API error (bulk IP dialog)

**Priority**: Medium  
**Tags**: error-handling

**Steps**:
1. Open bulk IP dialog → enter IP addresses
2. Simulate 500 from `/validateOpenstackIPs`
3. **Expected**: Per-IP validation status shows error state; message indicates validation failed
4. Apply button remains disabled

**Pass criteria**: Validation API error surfaced per-IP; apply blocked.  
**Fail criteria**: IPs marked valid on API error; apply allowed.

---

### MIG-025: Pod logs — connection error and reconnect

**Priority**: Medium  
**Tags**: error-handling

**Steps**:
1. Open pod logs drawer → logs loading
2. Simulate WebSocket/log stream disconnect
3. **Expected**: Error alert: "Failed to connect to logs"; Reconnect button visible
4. Click Reconnect → **Expected**: Log stream restarts; logs appear again

**Pass criteria**: Reconnect mechanism works; error clearly shown.  
**Fail criteria**: No error shown on disconnect; no reconnect option; stuck spinner.

---

### MIG-026: Delete migration API error

**Priority**: Medium  
**Tags**: error-handling

**Steps**:
1. Attempt to delete a migration
2. Simulate 403 or 500 on `DELETE /migrations/{name}`
3. **Expected**: Error snackbar/toast with message
4. Migration still visible in table → **Expected**: Not removed on error

**Pass criteria**: Error surfaced; migration not removed on failure.  
**Fail criteria**: Migration removed from UI despite API error; silent failure.

---

### MIG-027: Admin cutover error handling

**Priority**: Medium  
**Tags**: error-handling

**Steps**:
1. Open cutover confirmation dialog → confirm
2. Simulate 500 from cutover API
3. **Expected**: Error shown in dialog: "Failed to trigger cutover: [message]"
4. Dialog stays open → User can retry or cancel

**Pass criteria**: Error shown inline in dialog; retry possible.  
**Fail criteria**: Dialog closes on error; no feedback; migration phase incorrectly updated.

---

## Edge Case Tests

---

### MIG-028: Empty VMware cluster — no VMs available

**Priority**: Medium  
**Tags**: edge-case

**Steps**:
1. Select a VMware cluster with zero VMs
2. Step 2: VM list loads → **Expected**: Empty state "No VMs available in this cluster"
3. Submit button → **Expected**: Disabled

**Pass criteria**: Empty state shown; graceful handling.  
**Fail criteria**: Spinner stuck; error thrown; empty table with no explanation.

---

### MIG-029: Single VM selection

**Priority**: Medium  
**Tags**: edge-case

**Steps**:
1. Select exactly 1 VM
2. Toolbar → **Expected**: "1 selected" shown; bulk action buttons enabled
3. Flavor assignment → **Expected**: Dialog says "Assign flavor to 1 VM"
4. Complete form and submit → **Expected**: Migration created for single VM

**Pass criteria**: Single VM handled correctly throughout all steps.  
**Fail criteria**: Plural grammar errors; single VM causes validation false-positive.

---

### MIG-030: Large VM count (stress test — 50+ VMs)

**Priority**: Low  
**Tags**: edge-case

**Steps**:
1. Select a cluster with 50+ VMs
2. Step 2: Select all VMs → **Expected**: DataGrid virtualizes; no performance degradation
3. Toolbar "Select All" → **Expected**: All rows selected; count accurate
4. Bulk flavor assignment → **Expected**: Applies to all; completes without timeout

**Pass criteria**: No UI freeze; all operations complete.  
**Fail criteria**: Browser freeze; incorrect selection count; flavor apply times out.

---

### MIG-031: No PCD credentials exist

**Priority**: Medium  
**Tags**: edge-case

**Steps**:
1. Open standard migration form
2. Step 1: PCD cluster dropdown → **Expected**: Empty state or "No PCD clusters found"
3. **Expected**: Submit disabled; helpful message to configure credentials

**Pass criteria**: Graceful empty state; clear CTA.  
**Fail criteria**: Dropdown throws error; form broken state.

---

### MIG-032: StorageAcceleratedCopy — no validated array credentials

**Priority**: Medium  
**Tags**: edge-case

**Steps**:
1. Complete Steps 1–2
2. Step 5: Select "Storage Accelerated Copy" method
3. Step 3: Storage section → **Expected**: Shows array credentials mapping section
4. No validated array creds available → **Expected**: Warning banner: "No validated array credentials found"
5. Data copy + cutover sections in Step 5 → **Expected**: Hidden/disabled

**Pass criteria**: Warning shown; correct sections hidden.  
**Fail criteria**: StorageAcceleratedCopy allows proceeding without array creds; wrong sections shown.

---

### MIG-033: RDM disks detected — configure flow

**Priority**: Medium  
**Tags**: edge-case

**Steps**:
1. Select VMs containing RDM disks
2. Step 2: **Expected**: RDM alert banner appears: "Selected VMs contain RDM disks"
3. Toolbar: "Configure RDM" button visible
4. Click Configure RDM → panel opens → **Expected**: Each RDM disk listed with dropdowns
5. Configure backend pool and volume type → **Expected**: Volume type compatibility warning if mismatch
6. Apply → **Expected**: `patchRdmDisk` called; success toast

**Pass criteria**: RDM disks detected and configurable.  
**Fail criteria**: RDM alert not shown; Configure button missing; apply fails silently.

---

### MIG-034: Migration form — session cleanup on forced close

**Priority**: High  
**Tags**: edge-case

**Steps**:
1. Open migration form → select clusters (creates temp K8s resources: VMwareCreds, OpenstackCreds, MigrationTemplate)
2. Close form via X button
3. Check Kubernetes cluster → **Expected**: Session-scoped resources deleted (template + temp creds)
4. Reopen form → **Expected**: Fresh session; new sessionId; no stale state

**Pass criteria**: Temp resources cleaned up; no orphans.  
**Fail criteria**: Orphan K8s resources remain; stale state on reopen.

---

### MIG-035: Migrations table — filtering produces empty result

**Priority**: Low  
**Tags**: edge-case

**Steps**:
1. Apply Status filter = "Failed" when no failed migrations exist
2. **Expected**: Empty state message: "No migrations match the current filters"
3. Clear filter → **Expected**: All migrations visible again

**Pass criteria**: Empty state handled; filters clearable.  
**Fail criteria**: Blank table with no message; clear filter button missing.

---

### MIG-036: Subnet compatibility warning for network mapping

**Priority**: Medium  
**Tags**: edge-case

**Steps**:
1. Complete Steps 1–2 with VMs having known IP addresses
2. Step 3: Map source network to a PCD network whose subnet CIDR does not include the VM IPs
3. **Expected**: Subnet warning appears per source network: "VM IPs may not be compatible with target subnet"
4. Remap to correct subnet → **Expected**: Warning disappears (after 350ms debounce)

**Pass criteria**: Subnet warnings shown and cleared correctly.  
**Fail criteria**: No warnings shown; warnings not cleared on remap.

---

### MIG-037: Preserve IP and Preserve MAC toggles in bulk IP dialog

**Priority**: Medium  
**Tags**: edge-case

**Steps**:
1. Open BulkIPEditDialog with powered-off VMs
2. Enable "Preserve IP" for a network interface → **Expected**: IP field disabled; existing IP shown as locked
3. Enable "Preserve MAC" → **Expected**: MAC preserve state saved
4. Disable "Preserve IP" → **Expected**: IP field becomes editable
5. Apply → **Expected**: VM `networkInterfaces` updated with preserveIp/preserveMac flags

**Pass criteria**: Preserve flags applied correctly; field disabled when preserve active.  
**Fail criteria**: Field still editable when preserve active; flags not persisted.

---

### MIG-038: Mixed OS VMs in post-migration script validation

**Priority**: Medium  
**Tags**: edge-case, validation

**Preconditions**: Select both Windows and Linux VMs.

**Steps**:
1. Step 5: Enable post-migration script option
2. Enter bash script without OS tags
3. **Expected**: Warning: "Script must include OS-specific tags for mixed OS environments"
4. Add OS tags (e.g., `# [Windows]` section and `# [Linux]` section)
5. **Expected**: Warning clears

**Pass criteria**: Mixed OS script validation enforced.  
**Fail criteria**: Mixed OS validation not triggered; script applies to wrong OS.

---

## Test Summary

| Category | Count | Priority Distribution |
|----------|-------|----------------------|
| Smoke | 3 | 3× Critical |
| Happy Path | 7 | 4× Critical, 2× High, 1× Medium |
| Validation | 8 | 2× Critical, 5× High, 1× Medium |
| Error Handling | 8 | 1× Critical, 4× High, 3× Medium |
| Edge Cases | 10 | 1× High, 6× Medium, 3× Low |
| **Total** | **36** | |

---

## Test Tags Index

| Tag | Test IDs |
|-----|---------|
| `smoke` | MIG-001, MIG-002, MIG-003 |
| `happy-path` | MIG-004, MIG-005, MIG-006, MIG-007, MIG-008, MIG-009, MIG-010 |
| `validation` | MIG-011, MIG-012, MIG-013, MIG-014, MIG-015, MIG-016, MIG-017, MIG-018, MIG-019, MIG-038 |
| `error-handling` | MIG-020, MIG-021, MIG-022, MIG-023, MIG-024, MIG-025, MIG-026, MIG-027 |
| `edge-case` | MIG-028, MIG-029, MIG-030, MIG-031, MIG-032, MIG-033, MIG-034, MIG-035, MIG-036, MIG-037, MIG-038 |
| `critical` | MIG-001, MIG-002, MIG-003, MIG-004, MIG-005, MIG-011, MIG-020 |
