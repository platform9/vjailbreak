---

description: "Task list for clouds.yaml credentials feature implementation"
---

# Tasks: clouds.yaml credentials for OpenstackCreds

**Input**: Design documents from `/specs/003-clouds-yaml-credentials/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: MANDATORY per constitution principle IV (Test-First Development, NON-NEGOTIABLE) and `CLAUDE.md` ("ALWAYS write unit tests for any new code written by Claude"). All implementation tasks are preceded by a corresponding test task. Tests MUST be written first and observed to FAIL before implementation begins.

**Organization**: Tasks are grouped by user story. Each user story corresponds to one PR:

- **PR #1** = User Story 1 (clouds.yaml backend, foundational)
- **PR #2** = User Story 2 (Application Credentials)
- **PR #3** = User Story 3 (UI)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: `US1`, `US2`, `US3` — user story mapping. Setup, Foundational, and Polish tasks have no story label.
- Each task references exact file paths.

## Path Conventions

Three affected modules in the existing vjailbreak layout:

- `k8s/migration/` — controller (Go module)
- `v2v-helper/` — migration worker (Go module, CGO required)
- `ui/` — React/TypeScript frontend

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: One-time contributor environment preparation. Lives on PR #1's branch.

- [ ] T001 Run `make setup-hooks` from repo root to activate pre-commit validation (constitution requirement, CLAUDE.md Git Workflow rule)
- [ ] T002 Verify local Go toolchain: Go 1.21+, `CGO_ENABLED=1 GOOS=linux GOARCH=amd64` (required for `v2v-helper` tests) — fall back to Docker/Linux VM on macOS per CLAUDE.md
- [ ] T003 [P] Confirm `github.com/gophercloud/utils` is available in `k8s/migration/go.mod`; if missing, add via `cd k8s/migration && go get github.com/gophercloud/utils@latest && go mod tidy`
- [ ] T004 [P] Confirm `github.com/gophercloud/utils` is available in `v2v-helper/go.mod`; `cd v2v-helper && go mod tidy`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: CRD schema changes that block all user stories. Lives on PR #1's branch (these changes ship in PR #1).

**⚠️ CRITICAL**: All user stories depend on these changes. Never hand-edit the generated files; always run `make generate` (constitution principle III, NON-NEGOTIABLE).

- [ ] T005 Add `CloudName string \`json:"cloudName,omitempty"\`` field to `OpenstackCredsSpec` in `k8s/migration/api/v1alpha1/openstackcreds_types.go` with godoc explaining the multi-entry clouds.yaml selection semantics (FR-004, FR-005, FR-006)
- [ ] T006 Replace `OpenStackValidationStatus` and `OpenStackValidationMessage` fields in `OpenstackCredsStatus` with `Conditions []metav1.Condition \`json:"conditions,omitempty"\`` in `k8s/migration/api/v1alpha1/openstackcreds_types.go`; add `+listType=map` and `+listMapKey=type` kubebuilder markers (FR-017)
- [ ] T007 Run `make generate` inside `k8s/migration/` to regenerate `zz_generated.deepcopy.go` and `config/crd/bases/vjailbreak.k8s.pf9.io_openstackcreds.yaml`; DO NOT hand-edit either file (constitution III)
- [ ] T008 Run `make generate-manifests` from repo root to regenerate `deploy/installer.yaml`; verify `cd k8s/migration && make test` still passes on the unchanged baseline behavior (smoke check)

**Checkpoint**: CRD schema in place. User Story 1 implementation can now begin on PR #1's branch.

---

## Phase 3: User Story 1 — Use clouds.yaml as the credential source (Priority: P1) 🎯 MVP

**PR**: #1 (branch `feature/<PR1-issue-id>` once filed, e.g., `feature/1944` if that's the parent issue number)

**Goal**: Operator can place `clouds.yaml` content in the credential Secret and reference it via `OpenstackCreds.cloudName`. Existing OS_* Secrets continue to work unchanged. Microversion config from `clouds.yaml` flows through to service clients as a floor over hardcoded values. Status reported via Conditions; Secret changes observed within seconds via controller watch.

**Independent Test**: Create a Secret with a single-entry `clouds.yaml` and a matching `OpenstackCreds` resource with `cloudName: destination`. Run a migration against the destination cloud and confirm authentication succeeds and Conditions reflect a healthy state. Separately, create an OS_*-only Secret and observe identical behavior to the prior release.

### Tests for User Story 1 (write first; must FAIL before implementation)

- [ ] T009 [P] [US1] Write unit tests for `clouds.yaml` parsing wrapper in `k8s/migration/pkg/utils/clouds_yaml_test.go`: valid single-entry YAML, valid multi-entry with `cloudName` set, ambiguous multi-entry without `cloudName`, invalid YAML, missing required fields, `cacert` path detection
- [ ] T010 [P] [US1] Write unit tests for Conditions helpers (`SetCondition`, Reason mapping) in `k8s/migration/pkg/utils/conditions_test.go` per `contracts/conditions.md`
- [ ] T011 [P] [US1] Write unit tests for credentials parser branching in `k8s/migration/pkg/utils/credutils_test.go`: clouds.yaml-only Secret, OS_*-only Secret, both present (clouds.yaml wins), neither present (error)
- [ ] T012 [P] [US1] Write unit tests for `MicroversionFloor(configValue, hardcodedValue string) string` in `v2v-helper/pkg/utils/microversion_test.go`: config empty → hardcoded; config lower → hardcoded; config higher → config; `latest` handling; non-numeric malformed → error path
- [ ] T013 [P] [US1] Write reconciler tests in `k8s/migration/internal/controller/openstackcreds_controller_test.go` covering:
  - clouds.yaml-mode reconcile populates Conditions per `contracts/conditions.md`
  - OS_*-mode reconcile populates equivalent Conditions (back-compat path)
  - Secret update triggers re-reconciliation via the controller's Secret watch
  - Two `OpenstackCreds` resources referencing the same `clouds.yaml`-backed Secret with different `cloudName` values reconcile independently and both observe a Secret update (FR-016)
  - Flat status fields cleared on first post-upgrade reconcile (R-7)
  - `Reconcile` returns `Result{RequeueAfter: <cadence>}` matching the configured periodic re-validation interval (default 1 hour, R-8)
- [ ] T014 [P] [US1] Write log-redaction tests in `k8s/migration/pkg/utils/credutils_test.go` asserting that `password`, `application_credential_secret`, and `cacert` private key material do NOT appear in any log line emitted during parse and validate paths (R-9)
- [ ] T015 [US1] Confirm all new tests FAIL by running `cd k8s/migration && make test` and `make test-v2v-helper` (TDD red phase per constitution IV)

### Implementation for User Story 1

- [ ] T016 [P] [US1] Implement Condition Type and Reason constants in `k8s/migration/pkg/utils/conditions.go` per `contracts/conditions.md`; expose helper functions for setting common condition patterns (e.g., `SetCredentialsParsed`, `SetCredentialsValidated`)
- [ ] T017 [P] [US1] Implement `MicroversionFloor(configValue, hardcodedValue string) string` in `v2v-helper/pkg/utils/microversion.go` per research R-5
- [ ] T018 [US1] Implement clouds.yaml parsing wrapper in `k8s/migration/pkg/utils/clouds_yaml.go`: read Secret data, write tmpfile if needed for `clientconfig.AuthOptions`, build `*gophercloud.AuthOptions` for selected `cloudName`, return microversion map from per-service `*_api_version` keys
- [ ] T019 [US1] Refactor `k8s/migration/pkg/utils/credutils.go` `ValidateAndGetProviderClient` to branch on Secret content: `clouds.yaml` present → use new wrapper; else → existing OS_* path unchanged (logic-preserving refactor permitted per constitution VII)
- [ ] T020 [US1] Wire microversion floor into `v2v-helper/pkg/utils/openstackopsutils.go`: every service client constructor (`NewComputeClient`, `NewBlockStorageClient`, etc.) sets `Microversion = MicroversionFloor(cfg[svc+"_api_version"], "")`; per-call hot paths (`AttachVolumeToVM` and others previously hardcoded) use `Microversion = MicroversionFloor(cfg["compute_api_version"], hardcodedValue)` per research R-5
- [ ] T021 [US1] Update `k8s/migration/internal/controller/openstackcreds_controller.go` to:
  - Add `Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(...))` mapping Secret events to all `OpenstackCreds` referencing that Secret (FR-018, research R-3)
  - Populate `status.Conditions` per reconcile pass per `contracts/conditions.md`
  - On first reconcile observing legacy `OpenStackValidationStatus`/`Message` populated, clear them after writing equivalent Conditions (R-7)
  - Schedule a periodic 1-hour requeue for time-sensitive Conditions (Expiring/Expired) via `RequeueAfter` (R-8) — relevant for US2 but ship the requeue plumbing here
- [ ] T022 [US1] Run `cd k8s/migration && make test` and `make test-v2v-helper`; verify all tests pass (TDD green phase). Address any failures by adjusting implementation, not test expectations
- [ ] T023 [US1] Update operator-facing documentation:
  - Add a "clouds.yaml credentials" section to `README.md` linking to the new operator guide
  - Create or update `docs/credentials.md` based on `quickstart.md` content (omit App Credential specifics; those land in US2)

**Checkpoint (PR #1 ready)**: User Story 1 is fully functional and testable. PR #1 can be pushed and opened for review.

---

## Phase 4: User Story 2 — Authenticate via OpenStack Application Credentials (Priority: P2)

**PR**: #2 (branch `feature/<PR2-issue-id>`, opened after PR #1 merges)

**Goal**: Operator can declare `auth_type: v3applicationcredential` in `clouds.yaml` and authenticate vjailbreak via an OpenStack Application Credential. Validation distinguishes invalid/revoked, expired, and insufficient-role conditions. A 30-day expiration warning appears as a Condition before expiry.

**Independent Test**: Create an Application Credential on the destination cloud, embed its ID and secret in `clouds.yaml`, observe `CredentialsValidated=True` on the resource, run a migration. Revoke the credential and observe `CredentialsValidated=False, Reason=CredentialInvalidOrRevoked` on next reconcile. Set an expiration within 30 days and observe `Expiring=True, Reason=Within30Days`.

### Tests for User Story 2 (write first; must FAIL before implementation)

- [ ] T024 [P] [US2] Write unit tests for Application Credential auth flow in `k8s/migration/pkg/utils/clouds_yaml_test.go`: `auth_type: v3applicationcredential` produces `AuthOptions` with `ApplicationCredentialID`/`Secret` set (no username/password); `auth_type: v3password` unchanged
- [ ] T025 [P] [US2] Write unit tests for App Cred expiration evaluation in `k8s/migration/pkg/utils/credutils_test.go`: `expires_at` in past → `Expired=True`; within 7 days → `Expiring=True, Reason=Within7Days`; within 30 days → `Expiring=True, Reason=Within30Days`; >30 days → `Expiring=False`; not an App Cred → both `NotApplicable`
- [ ] T026 [P] [US2] Write unit tests for Keystone error mapping in `k8s/migration/pkg/utils/credutils_test.go`: 401 → `CredentialInvalidOrRevoked`; 403 from a downstream service → `InsufficientRoles` with role list extracted from error body where available
- [ ] T027 [US2] Confirm all new tests FAIL via `cd k8s/migration && make test` (TDD red phase)

### Implementation for User Story 2

- [ ] T028 [US2] Extend `k8s/migration/pkg/utils/credutils.go` to call `applicationcredentials.Get(...)` after successful auth (when auth_type is `v3applicationcredential`) and surface `Expiring`/`Expired` conditions per `contracts/conditions.md`
- [ ] T029 [US2] Extend `k8s/migration/pkg/utils/credutils.go` to map Keystone error responses to specific Condition Reasons: `CredentialInvalidOrRevoked` (401), `InsufficientRoles` (403 + parse missing role names from response body), `KeystoneUnreachable` (network), `TLSVerificationFailed` (TLS cert errors)
- [ ] T030 [US2] Update `k8s/migration/internal/controller/openstackcreds_controller.go` to populate `Expiring`/`Expired`/`RolesSufficient` Conditions during each reconcile based on the new helpers from T028/T029; rely on the periodic 1-hour requeue from T021 for time-based transitions (Expiring → Expired)
- [ ] T031 [US2] Run `cd k8s/migration && make test`; verify all tests pass
- [ ] T032 [US2] Update `docs/credentials.md` (created in T023) with the "Application Credentials (recommended)" section including the operator runbook: App Cred create command, **explicit enumeration of the minimum role set vjailbreak requires on the destination project (`member` plus the `vjailbreak-migrator` role covering Cinder scheduler-stats and Nova hypervisor read)**, rotation workflow, revocation step. Reference `quickstart.md` for the end-to-end operator flow.

**Checkpoint (PR #2 ready)**: User Story 2 is functional and testable. PR #2 can be pushed and opened against `main` after PR #1 has merged.

---

## Phase 5: User Story 3 — Enter clouds.yaml through the web UI (Priority: P3)

**PR**: #3 (branch `feature/<PR3-issue-id>`, opened after PR #1 merges; can be developed in parallel with PR #2)

**Goal**: The credential creation form in the web UI accepts `clouds.yaml` content (paste or upload), parses it client-side, populates a cloud-name selector, shows an auth-method indicator, and submits the new credential via the new CRD schema. The legacy per-field form remains available as a secondary tab.

**Independent Test**: Open the credential creation form, paste a valid `clouds.yaml` with two cloud entries, select one via the dropdown, observe the "Application Credential" badge when applicable, submit, and verify the resulting Kubernetes Secret contains a `clouds.yaml` key and the `OpenstackCreds` resource has `cloudName` set.

### Tests for User Story 3 (write first; must FAIL before implementation)

- [ ] T033 [P] [US3] Write component tests for tab switching in `ui/src/components/credentials/OpenstackCredsForm.test.tsx`: default tab is clouds.yaml; legacy tab remains accessible; switching preserves entered content within each tab
- [ ] T034 [P] [US3] Write component tests for `CloudsYamlForm` parse behavior in `ui/src/components/credentials/CloudsYamlForm.test.tsx`: valid single-entry YAML → cloud-name dropdown shown but disabled; multi-entry → enabled with all entry keys; invalid YAML → inline parse error with line info
- [ ] T035 [P] [US3] Write component tests for auth-method indicator in `ui/src/components/credentials/CloudsYamlForm.test.tsx`: `auth_type: v3password` → "Password" badge; `auth_type: v3applicationcredential` → "Application Credential" badge; missing `auth_type` → default badge per gophercloud-utils interpretation
- [ ] T036 [P] [US3] Write component tests for secret masking in `ui/src/components/credentials/CloudsYamlForm.test.tsx`: after parse, `password` and `application_credential_secret` values are masked in the rendered preview (FR-015)
- [ ] T037 [US3] Confirm UI tests FAIL via the existing UI test runner (`cd ui && yarn test` or equivalent)

### Implementation for User Story 3

- [ ] T038 [US3] Add `js-yaml` dependency in `ui/package.json` (skip if a YAML library is already bundled — verify first with `cd ui && yarn list js-yaml 2>/dev/null || cat package.json | grep -i yaml`)
- [ ] T039 [P] [US3] Implement `ui/src/components/credentials/CloudsYamlForm.tsx`: textarea + file upload, client-side YAML parse, cloud-name dropdown populated from parsed entries, parse error inline display, auth-method badge, secret masking
- [ ] T040 [US3] Refactor existing OpenStack credentials form into a tab container at `ui/src/components/credentials/OpenstackCredsForm.tsx`: default tab `CloudsYamlForm`, secondary tab `LegacyOpenStackForm` (existing form extracted to its own component if needed for cleaner test boundaries)
- [ ] T041 [US3] Wire form submission: on submit from CloudsYamlForm, build a Secret payload with `clouds.yaml` key set to the raw YAML and create an `OpenstackCreds` resource with `cloudName` matching the dropdown selection; on submit from LegacyOpenStackForm, preserve existing OS_*-keyed Secret creation behavior unchanged
- [ ] T042 [US3] Run `cd ui && yarn test`; verify all tests pass
- [ ] T043 [US3] Manually exercise the form in a development server (`cd ui && yarn dev` with `VITE_API_HOST` and `VITE_API_TOKEN` set) against a vjailbreak appliance built from `main` with PR #1 merged: paste clouds.yaml, submit, verify Secret + OpenstackCreds created correctly (constitution-level integration check; not automated)

**Checkpoint (PR #3 ready)**: User Story 3 is functional and testable. PR #3 can be pushed and opened against `main`.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final consolidation across the three PRs. Most of these tasks happen on PR #1's branch before opening; some are per-PR housekeeping.

- [ ] T044 [P] Validate the `quickstart.md` operator walkthrough end-to-end against the destination cloud (PR #1 + PR #2 merged): every command in steps 1-8 executes successfully; verify Conditions populate exactly as documented
- [ ] T045 [P] Confirm pre-commit hooks pass for each PR branch: `make setup-hooks` once, then ensure each commit triggers them without bypass
- [ ] T046 Confirm no `[NEEDS CLARIFICATION]` markers remain in any spec/plan/data-model document
- [ ] T047 Re-run the full test suite locally before each `gh pr push`: `cd k8s/migration && make test`, `make test-v2v-helper`, `cd ui && yarn test`
- [ ] T048 Confirm constitution Test-First (IV) sequence held for every implementation task: tests were written and observed failing before the corresponding implementation task started; no "back-fill" tests (constitution principle IV is NON-NEGOTIABLE)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately on PR #1 branch.
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories. Lives on PR #1 branch and ships in PR #1.
- **User Story 1 (Phase 3)**: Depends on Foundational. Lives on PR #1 branch.
- **User Story 2 (Phase 4)**: Depends on PR #1 merge. New branch off `main` after PR #1.
- **User Story 3 (Phase 5)**: Depends on PR #1 merge. New branch off `main` after PR #1. May proceed in parallel with PR #2.
- **Polish (Phase 6)**: Per-PR housekeeping; some tasks span across PR boundaries.

### User Story Dependencies

- **US1**: Independent (only depends on Setup + Foundational on its own branch).
- **US2**: Depends on PR #1 merged — needs the clouds.yaml parser and Conditions API in place.
- **US3**: Depends on PR #1 merged — needs the CRD schema with `cloudName` and the backend Secret-shape contract.
- **US2 ↔ US3**: Independent of each other; can land in either order or in parallel.

### Within Each User Story (TDD discipline)

- Tests MUST be written FIRST and observed FAILING before implementation (constitution IV NON-NEGOTIABLE; CLAUDE.md "ALWAYS write unit tests").
- Within a single story: helpers/constants → service code → controller / form wiring → integration check.
- Story complete before opening its PR for review.

### Parallel Opportunities

- Setup tasks T003 and T004 in parallel (different go.mod files).
- US1 test tasks T009, T010, T011, T012, T013, T014 in parallel (all in different `_test.go` files).
- US1 implementation T016 and T017 in parallel (different files, no shared symbol writes).
- US2 test tasks T024, T025, T026 in parallel.
- US3 test tasks T033, T034, T035, T036 in parallel (all in `_test.tsx` files).
- Across stories: once PR #1 merges, US2 and US3 can be developed by different contributors in parallel.

---

## Parallel Example: User Story 1

```bash
# Phase 3, after Phase 2 checkpoint, on PR #1's branch:
# Launch all unit-test scaffolding tasks in parallel (different test files):
Task: "Write unit tests for clouds.yaml parsing wrapper in k8s/migration/pkg/utils/clouds_yaml_test.go"
Task: "Write unit tests for Conditions helpers in k8s/migration/pkg/utils/conditions_test.go"
Task: "Write unit tests for credentials parser branching in k8s/migration/pkg/utils/credutils_test.go"
Task: "Write unit tests for MicroversionFloor in v2v-helper/pkg/utils/microversion_test.go"
Task: "Write reconciler tests in k8s/migration/internal/controller/openstackcreds_controller_test.go"
Task: "Write log-redaction tests in k8s/migration/pkg/utils/credutils_test.go"

# After tests fail (T015), launch parallel implementation:
Task: "Implement Condition Type/Reason constants in k8s/migration/pkg/utils/conditions.go"
Task: "Implement MicroversionFloor in v2v-helper/pkg/utils/microversion.go"
```

---

## Implementation Strategy

### MVP First (PR #1 — User Story 1 only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational (CRD schema changes).
3. Complete Phase 3: User Story 1 (clouds.yaml backend + Conditions + Secret watch + microversion floor).
4. **STOP and VALIDATE**: Run the full test suite, exercise quickstart.md steps 1-6 against a real cloud. Open PR #1.
5. Wait for review and merge.

### Incremental Delivery

1. PR #1 → backend foundation ready, operators can already use `clouds.yaml` via kubectl. Demo: a working multi-cloud migration without any UI involvement.
2. PR #2 → Application Credentials available. Demo: a revocable, role-scoped, expiring credential in production.
3. PR #3 → UI catches up. Demo: end-to-end operator flow without leaving the browser.

Each PR delivers operator-visible value independently.

### Parallel Team Strategy

With multiple contributors after PR #1 merges:

1. Single contributor: PR #1 alone (Setup + Foundational + US1).
2. After PR #1 merges:
   - Contributor A: PR #2 (US2 — App Credentials)
   - Contributor B: PR #3 (US3 — UI)
3. PR #2 and PR #3 reviewed and merged independently in any order.

---

## Notes

- **[P]** tasks = different files, no incomplete dependencies. Within the same `_test.go` file, tests are not [P] relative to each other.
- **[Story]** label maps tasks to PRs for traceability and review scope.
- Constitution principle IV (Test-First) is NON-NEGOTIABLE; T015, T027, and T037 are the explicit red-phase confirmation gates.
- Constitution principle III (Generated Code Protection) is NON-NEGOTIABLE; T007 and T008 regenerate; never hand-edit `zz_generated.deepcopy.go` or `deploy/installer.yaml`.
- The current spec branch is `1952-clouds-yaml-credentials` — created by `/speckit-git-feature` and where the spec / plan / research / tasks artifacts live. PR #1 opens from this branch.
- Per-PR branches for sub-issues (PR #2 → `1953-clouds-yaml-app-credentials`, PR #3 → `1954-ui-clouds-yaml-form`) follow the repo convention `<issue-id>-<kebab-description>` (no `feature/` prefix; matches existing branches like `1889-maas-free-bm-provisioning`), branched off `main` after PR #1 merges.
- Commit boundaries should align with task boundaries or small task groups for clean review history.
