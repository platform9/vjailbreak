# Tasks: Agent Node Custom Host Entries

**Input**: Design documents from `specs/002-agent-dns-config/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓

**TDD enforced** (Constitution Principle IV): For every Go and UI component, tests are written before implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story this task belongs to (US1, US2, US3)
- Exact file paths in all descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add shared constant and define `HostEntry` type + function stubs so tests can reference them before implementation exists (TDD red-phase prerequisite).

- [ ] T001 Add `AgentHostEntriesKey = "AGENT_HOST_ENTRIES"` constant to `pkg/common/constants/constants.go`
- [ ] T002 Update vendor copy at `k8s/migration/vendor/github.com/platform9/vjailbreak/pkg/common/constants/constants.go` to match T001
- [ ] T003 Create `pkg/common/utils/hosts.go` with `HostEntry` struct and empty function stubs: `ValidateHostEntry`, `ParseHostEntries`, `SerializeHostEntries`, `BuildUserData` (all return zero values)
- [ ] T004 Copy stub `pkg/common/utils/hosts.go` to vendor at `k8s/migration/vendor/github.com/platform9/vjailbreak/pkg/common/utils/hosts.go`

**Checkpoint**: Stubs compile. Tests can be written against these signatures.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: No additional foundational work beyond Phase 1 — `HostEntry` type and constant are the only shared prereqs. User story phases can begin once Phase 1 is done.

**⚠️ CRITICAL**: Phase 1 must complete before ANY user story work begins.

---

## Phase 3: User Story 1 — Configure Custom Host Entries for Agent Nodes (Priority: P1) 🎯 MVP

**Goal**: Pure host-entry logic in `pkg/common/utils/hosts.go` is fully tested, and new agent VMs receive configured host entries via cloud-init.

**Independent Test**: `cd pkg/common && go test ./utils/... -v` passes; provision a new agent node and verify `/etc/hosts` on the node contains configured entries.

### Tests for User Story 1 (TDD — write before implementation) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T005 [US1] Write table-driven unit tests in `pkg/common/utils/hosts_test.go` covering: `TestValidateHostEntry` (valid/invalid IP, empty IP, no hostnames, bad hostname chars), `TestParseHostEntries` (empty string, `"[]"`, valid JSON, malformed JSON), `TestSerializeParseRoundTrip` (serialize then parse equals input), `TestBuildUserData` (nil entries == current `fmt.Sprintf` output; single entry has correct echo line; multiple entries all present in order)
- [ ] T006 [US1] Write `GetAgentHostEntries` tests in new `k8s/migration/pkg/utils/vjailbreaknodeutils_test.go` using `fake.NewClientBuilder()`: key present + valid JSON, key absent, key present + empty string, key present + malformed JSON (expect non-nil error)

### Implementation for User Story 1

- [ ] T007 [US1] Implement all functions in `pkg/common/utils/hosts.go`: `ValidateHostEntry` (net.ParseIP + hostname regex), `ParseHostEntries`, `SerializeHostEntries`, `BuildUserData` (cloud-init YAML; nil/empty entries → output identical to `fmt.Sprintf(constants.K3sCloudInitScript, ...)`)
- [ ] T008 [US1] Update vendor copy `k8s/migration/vendor/github.com/platform9/vjailbreak/pkg/common/utils/hosts.go` with full implementation from T007
- [ ] T009 [US1] Add `GetAgentHostEntries(ctx context.Context, k8sClient client.Client) ([]pkgutils.HostEntry, error)` to `k8s/migration/pkg/utils/vjailbreaknodeutils.go` (reads `vjailbreak-settings` ConfigMap, returns empty slice when key absent)
- [ ] T010 [US1] Replace `fmt.Sprintf(constants.K3sCloudInitScript, ...)` call at line ~367 in `k8s/migration/pkg/utils/vjailbreaknodeutils.go` with `pkgutils.BuildUserData(...)`, calling `GetAgentHostEntries` first (non-fatal error: log + continue with empty entries)

**Checkpoint**: `cd pkg/common && go test ./utils/... -v` passes. `cd k8s/migration && make test` passes. User Story 1 is independently functional.

---

## Phase 4: User Story 2 — View and Manage Current Host Entries (Priority: P2)

**Goal**: Administrator can view, add, edit, and delete host entries from the vJailbreak settings UI. Changes persist in `vjailbreak-settings` ConfigMap.

**Independent Test**: Open Settings → Host Entries tab; add an entry; save; reload page; verify entry persists. `cd ui && yarn test` passes.

### Tests for User Story 2 (TDD — write before implementation) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T011 [P] [US2] Write `ui/src/features/globalSettings/components/HostEntriesTab.test.tsx` with React Testing Library tests: renders empty state with "Add Entry" button; parses pre-populated JSON prop and shows rows; adds entry with valid IP + hostname → `onChange` called with correct JSON; rejects invalid IP with inline error; rejects duplicate IP with inline error; delete row → row removed, `onChange` called

### Implementation for User Story 2

- [ ] T012 [P] [US2] Add `AGENT_HOST_ENTRIES: string` to `SettingsForm` type in `ui/src/features/globalSettings/helpers.ts`; update `toConfigMapData` and `fromConfigMapData` to include the field; add default `AGENT_HOST_ENTRIES: ''` in `GlobalSettingsPage.tsx` DEFAULTS
- [ ] T013 [US2] Create `ui/src/features/globalSettings/components/HostEntriesTab.tsx` — props: `{value: string, onChange: (v: string) => void, disabled?: boolean}`; renders MUI Table with IP + Hostnames columns + edit/delete actions; "Add Entry" opens inline form with IP and hostname inputs; validates IP (IPv4/IPv6 regex) and hostname (mirrors Go regex) and duplicate IPs; calls `onChange` with updated JSON on every mutation
- [ ] T014 [US2] Add "Host Entries" tab to `ui/src/features/globalSettings/components/GlobalSettingsPage.tsx` using `<LanOutlinedIcon />`; wire `<HostEntriesTab value={watch('AGENT_HOST_ENTRIES')} onChange={v => setValue('AGENT_HOST_ENTRIES', v)} disabled={isSubmitting} />`; existing `updateSettingsConfigMap` save flow handles persistence without changes

**Checkpoint**: UI tab renders, CRUD works, saves to ConfigMap, `yarn test` passes. User Story 2 is independently functional.

---

## Phase 5: User Story 3 — Reprovision an Idle Agent Node (Priority: P2)

**Goal**: Admin can reprovision an idle agent node from the NodesTable UI. Controller handles `vjailbreak.io/reprovision: "requested"` annotation — blocks if active migrations, otherwise tears down and re-creates node with updated host entries.

**Independent Test**: Annotate a VjailbreakNode CR with `vjailbreak.io/reprovision: "requested"` (idle node) → controller clears `OpenstackUUID`, removes annotation, node re-provisions. Same annotation on node with active migrations → annotation becomes `"blocked"`.

### Tests for User Story 3 (TDD — write before implementation) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T015 [US3] Add reprovision unit tests to `k8s/migration/internal/controller/vjailbreaknode_controller_test.go` (Ginkgo): `reprovisionAllowed([]string{})` returns true; `reprovisionAllowed([]string{"m-1"})` returns false; annotation `"requested"` on node with active migrations → annotation becomes `"blocked"`, node not deleted; annotation `"requested"` on idle node → `OpenstackUUID` cleared, annotation removed

### Implementation for User Story 3

- [ ] T016 [US3] Add package-level constants `reprovisionAnnotation`, `reprovisionRequested`, `reprovisionBlocked` and pure helper `func reprovisionAllowed(activeMigrations []string) bool` to `k8s/migration/internal/controller/vjailbreaknode_controller.go`
- [ ] T017 [US3] Implement `reconcileReprovision` method on the reconciler in `k8s/migration/internal/controller/vjailbreaknode_controller.go`: if `!reprovisionAllowed` set annotation to `"blocked"` + requeue 1 min; else call existing `utils.DeleteOpenstackVM` + `utils.DeleteNodeByName`, clear `Status.OpenstackUUID` + `Status.Phase`, remove annotation, update status, requeue 5s; add early-return check in `reconcileNormal` before UUID lookup
- [ ] T018 [P] [US3] Add `reprovisionNode(nodeName: string): Promise<void>` API helper alongside `ui/src/api/nodes/nodeMappings.ts` — PATCHes `vjailbreak.io/reprovision: "requested"` annotation on the VjailbreakNode CR via the k8s API
- [ ] T019 [US3] Add "Reprovision" `IconButton` to `ui/src/features/agents/components/NodesTable.tsx` actions column: disabled when `activeMigrations.length > 0` (tooltip "Node has active migrations") or `isDeleting` or `role === 'master'`; on click calls `reprovisionNode(nodeName)` with snackbar feedback (same pattern as existing delete)

**Checkpoint**: `cd k8s/migration && make test` passes. Reprovision flow works end-to-end. User Story 3 is independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup across all stories.

- [ ] T020 [P] Verify `cd pkg/common && go test ./utils/... -v` passes with full coverage on `hosts.go`
- [ ] T021 [P] Verify `cd k8s/migration && make test` passes (includes `GetAgentHostEntries` and reprovision tests)
- [ ] T022 [P] Verify `cd ui && yarn test` passes (includes `HostEntriesTab.test.tsx`)
- [ ] T023 Verify backward compatibility: `BuildUserData(constants.ENVFileLocation, masterIP, token, nil)` output is byte-for-byte identical to old `fmt.Sprintf(constants.K3sCloudInitScript, ...)` output

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: N/A — absorbed into Phase 1
- **User Story 1 (Phase 3)**: Depends on Phase 1 completion. T005–T006 (tests) before T007–T010 (implementation)
- **User Story 2 (Phase 4)**: Depends on Phase 1 (constant defined). T011 (tests) before T013 (implementation). T012 can run in parallel with T011
- **User Story 3 (Phase 5)**: Depends on Phase 3 completion (needs `GetAgentHostEntries` wired). T015 (tests) before T016–T017 (implementation). T018 can run in parallel with T016
- **Polish (Phase 6)**: Depends on all story phases complete

### User Story Dependencies

- **US1 (P1)**: Start after Phase 1 — no dependency on US2 or US3
- **US2 (P2)**: Start after Phase 1 — no dependency on US1 (pure UI, reads ConfigMap independently)
- **US3 (P2)**: Start after US1 — controller calls `BuildUserData` (from US1) on re-provision

### TDD Within Each Story

1. Write tests against stubs → confirm they FAIL
2. Implement → confirm they PASS
3. Never implement before tests exist

### Parallel Opportunities

- T001, T003 (Phase 1): Same-module different files — can run in parallel after confirming no conflicts
- T005 and T006 (US1 tests): Different test files — parallel
- T011 and T012 (US2): Different files — parallel
- T018 (API helper) and T016 (controller constants): Different files — parallel within US3
- T020, T021, T022 (Polish): All independent — parallel

---

## Parallel Example: User Story 1

```bash
# Step 1: write tests first (parallel — different files)
Task T005: "Write pkg/common/utils/hosts_test.go"
Task T006: "Write k8s/migration/pkg/utils/vjailbreaknodeutils_test.go"

# Step 2: implement (sequential — T007 before T008 before T009/T010)
Task T007: "Implement pkg/common/utils/hosts.go"
Task T008: "Update vendor copy"
Task T009+T010: "Add GetAgentHostEntries + replace fmt.Sprintf in vjailbreaknodeutils.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T004)
2. Write tests T005–T006 → confirm FAIL
3. Complete US1 implementation T007–T010
4. **STOP and VALIDATE**: `cd pkg/common && go test ./utils/... -v` + `cd k8s/migration && make test`
5. New agent nodes now receive custom host entries — core value delivered

### Incremental Delivery

1. Phase 1 + US1 → **MVP**: host entries injected into new agent VMs
2. Add US2 → **Operational**: admin can manage entries without kubectl
3. Add US3 → **Complete**: existing nodes can be remediated without SSH

### Task Count Summary

| Phase | Tasks | Notes |
|-------|-------|-------|
| Phase 1: Setup | 4 (T001–T004) | Stubs + constants |
| Phase 3: US1 | 6 (T005–T010) | 2 test + 4 impl |
| Phase 4: US2 | 4 (T011–T014) | 1 test + 3 impl |
| Phase 5: US3 | 5 (T015–T019) | 1 test + 4 impl |
| Phase 6: Polish | 4 (T020–T023) | Validation |
| **Total** | **23** | |

---

## Notes

- All [P] tasks operate on different files with no shared state — safe to parallelize
- TDD order is non-negotiable (Constitution IV): write failing tests, then implement
- Vendor copies (T002, T004, T008) must be updated immediately after source changes — same commit
- T023 (backward compatibility check) guards against regressions in existing node provisioning
