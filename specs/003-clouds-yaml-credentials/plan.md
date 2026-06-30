# Implementation Plan: clouds.yaml credentials for OpenstackCreds

**Branch**: `1952-clouds-yaml-credentials` | **Date**: 2026-05-18 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-clouds-yaml-credentials/spec.md`

## Summary

Introduce `clouds.yaml` as the credential format for the `OpenstackCreds` CRD with full back-compat for the existing OS_* keys. Wire per-service API version values from `clouds.yaml` (`compute_api_version`, `volume_api_version`, `image_api_version`, `network_api_version`, `identity_api_version`) as an operator-configurable floor over the internal hardcoded microversion values used in `v2v-helper`. Support OpenStack Application Credentials (`auth_type: v3applicationcredential`) for revocable, role-scoped, optionally time-bounded authentication. Replace the flat `OpenStackValidationStatus` / `OpenStackValidationMessage` status fields with a Kubernetes-style `status.conditions` slice. Add a credential Secret watch so credential rotation is observed automatically. Add a `clouds.yaml` paste-or-upload mode to the credential creation form in the web UI as the default credential entry path.

Three PRs decompose from this feature:

1. **PR #1**: Backend parser + CRD changes (new `cloudName` field, Conditions API) + microversion floor wiring + Secret watch
2. **PR #2**: Application Credentials support (`auth_type: v3applicationcredential`) on top of PR #1
3. **PR #3**: UI credential input form (clouds.yaml default tab + legacy fallback)

## Technical Context

**Language/Version**: Go 1.21+ (controller and v2v-helper); TypeScript with React (UI)
**Primary Dependencies**:
- Backend: `github.com/gophercloud/utils/openstack/clientconfig` (clouds.yaml parser + AuthOptions builder), `github.com/gophercloud/gophercloud` (already vendored), `sigs.k8s.io/controller-runtime` (Conditions handling, Secret watch), `k8s.io/apimachinery/pkg/api/meta` (`meta.SetStatusCondition`).
- Frontend: existing React/MUI/Vite stack plus a YAML parser dependency (`js-yaml`, ~30-50 KB minified) for client-side validation and preview.

**Storage**: Kubernetes etcd via CRDs; credential Secret holds the `clouds.yaml` key (Mode A) or legacy OS_* keys (Mode B).
**Testing**: Go `_test.go` table-driven tests with interface-based mocks per CLAUDE.md and constitution principle IV; controller tests via `cd k8s/migration && make test`; v2v-helper tests via `make test-v2v-helper` (requires `CGO_ENABLED=1 GOOS=linux GOARCH=amd64`); UI tests via the existing JS test framework in `ui/`.
**Target Platform**: Linux containers on k3s/Kubernetes (controller and v2v-helper); modern browsers (UI).
**Project Type**: Kubernetes-native controller with web UI (3 affected Go modules + 1 frontend module).
**Performance Goals**: Validation latency negligible (single Keystone token request per reconcile); Secret watch overhead negligible (informer cache hits); UI YAML parse <100 ms for typical clouds.yaml (<10 KB).
**Constraints**: Test-first development mandatory; never hand-edit generated files (`deploy/installer.yaml`, `zz_generated.deepcopy.go`); CGO required for v2v-helper; four independent Go modules with separate `go.sum`.
**Scale/Scope**: 3 PRs, ~5-8 changed/new Go files across `k8s/migration/` and `v2v-helper/`, ~3-5 React component changes in `ui/`. Test code expected to be larger than implementation. No new long-running goroutines beyond the Secret watch already supported by controller-runtime.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Kubernetes-Native Architecture | PASS | CRD-driven; Conditions API for status; Secret watch via controller-runtime informer. No external state management introduced. |
| II. External Documentation First | PASS | Research artifact references `gophercloud-utils/clientconfig`, openstacksdk clouds.yaml spec, controller-runtime Conditions documentation, Keystone Application Credentials documentation. |
| III. Generated Code Protection (NON-NEGOTIABLE) | PASS | After CRD field additions, run `make generate` inside `k8s/migration/`. `zz_generated.deepcopy.go` and `deploy/installer.yaml` are never hand-edited. |
| IV. Test-First Development (NON-NEGOTIABLE) | PASS | Tasks will follow Red-Green-Refactor: failing tests → implementation. External dependencies (gophercloud, Keystone, Kubernetes API) mocked through interfaces. |
| V. Module Independence | PASS | Changes scoped to `k8s/migration/` (controller), `v2v-helper/` (microversion floor wiring), `ui/` (form). Each Go module gets `go mod tidy` separately. |
| VI. AI-Assisted Development (NON-NEGOTIABLE) | PASS | Following the CLAUDE.md AI rules: unit tests for new code, mocks for external systems, no hand-edited generated files. |
| VII. Code Reuse and Simplicity | PASS | Reusing `gophercloud/utils/clientconfig` rather than rolling a custom clouds.yaml parser. Refactors limited to those needed for testability. |

No violations. Complexity Tracking section omitted.

## Project Structure

### Documentation (this feature)

```text
specs/003-clouds-yaml-credentials/
├── plan.md                         # This file
├── spec.md                         # Feature specification (clarified)
├── research.md                     # Phase 0 — design research consolidating decisions
├── data-model.md                   # Phase 1 — CRD and entity model
├── quickstart.md                   # Phase 1 — operator quickstart guide
├── contracts/
│   ├── openstackcreds-crd.md       # CRD schema delta (informative)
│   ├── conditions.md               # Condition Types and Reason codes
│   └── secret-keys.md              # Credential Secret key contract
└── checklists/
    └── requirements.md             # Spec quality checklist (from /speckit-specify)
```

### Source Code (repository root)

```text
k8s/migration/                                          # Controller module (PR #1 + PR #2)
├── api/v1alpha1/openstackcreds_types.go                # CRD field additions: CloudName, status.Conditions
├── api/v1alpha1/zz_generated.deepcopy.go               # Regenerated via `make generate` (DO NOT edit)
├── config/crd/bases/vjailbreak.k8s.pf9.io_openstackcreds.yaml  # Regenerated CRD YAML
├── pkg/utils/credutils.go                              # Parser branching (clouds.yaml vs OS_*)
├── pkg/utils/credutils_test.go                         # NEW: parser branch unit tests
├── pkg/utils/clouds_yaml.go                            # NEW: clouds.yaml + AuthOptions wrapper around clientconfig
├── pkg/utils/clouds_yaml_test.go                       # NEW: clouds.yaml parsing unit tests
├── pkg/utils/conditions.go                             # NEW: Condition Type/Reason constants + helpers
├── pkg/utils/conditions_test.go                        # NEW: condition helpers tests
├── internal/controller/openstackcreds_controller.go    # Secret watch wiring; Conditions reconcile
└── internal/controller/openstackcreds_controller_test.go  # NEW/expanded reconcile + watch tests

v2v-helper/                                             # Migration worker module (PR #1)
├── pkg/utils/openstackopsutils.go                      # Microversion floor wiring; honor config from clouds.yaml
└── pkg/utils/openstackopsutils_test.go                 # NEW/expanded: floor semantics tests

ui/                                                     # React frontend module (PR #3)
├── src/components/credentials/CloudsYamlForm.tsx       # NEW: clouds.yaml paste/upload tab content
├── src/components/credentials/CloudsYamlForm.test.tsx  # NEW: component tests
├── src/components/credentials/LegacyOpenStackForm.tsx  # Existing per-field form (extracted if needed)
└── src/components/credentials/OpenstackCredsForm.tsx   # Tab container (clouds.yaml default + legacy)

docs/                                                   # Astro documentation site
└── credentials.md (NEW or section update)              # Operator runbook for clouds.yaml + App Credentials
```

**Structure Decision**: Three-module Kubernetes-native layout matching existing vjailbreak structure. PR #1 changes the controller (`k8s/migration/`) and migration worker (`v2v-helper/`). PR #2 is controller-only (validation surfacing for Application Credentials). PR #3 is UI-only (`ui/`). Documentation updates accompany whichever PR introduces the user-facing change.
