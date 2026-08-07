# Feature Specification: v2v-helper Module Restructure — Phase 1

**Feature Branch**: `2180-v2v-restructure`
**Created**: 2026-07-30
**Status**: Draft

## Overview

The `v2v-helper` Go module requires a mechanical reorganization to improve navigability and long-term maintainability. The codebase has grown organically, resulting in a 2,723-line god-object file, business logic hidden inside a `utils` package, and an interface defined in a different package from its implementation. This restructure addresses all three problems through file moves and reclassifications only — no logic changes inside any function body.

## User Scenarios & Testing

### User Story 1 — Navigate migration orchestration code (Priority: P1)

A developer opening `migrate/migrate.go` to understand the migration flow should see only orchestration logic — the entry point (`MigrateVM`), the `Migrate` struct, cleanup, and cutover coordination. Today the file is 2,723 lines mixing 10 different concerns, making it hard to find the main flow.

**Why this priority**: This is the most-visited file in the module. Reducing it to orchestration-only enables faster onboarding and easier code review.

**Independent Test**: Open `migrate/migrate.go` after the restructure. It must contain only: `Migrate` struct definition, `MigrateVM()`, `cleanup()`, and cutover helpers (`WaitforCutover`, `CheckIfAdminCutoverSelected`, `CheckCutoverOptions`, `gracefulTerminate`). All other methods must be absent.

**Acceptance Scenarios**:

1. **Given** the restructure is complete, **When** a developer opens `migrate/migrate.go`, **Then** the file is ~550 lines and contains only orchestration and struct definition
2. **Given** the restructure is complete, **When** a developer looks for NBD copy logic, **Then** it is in `migrate/nbd_copy.go`
3. **Given** the restructure is complete, **When** a developer looks for network configuration, **Then** it is in `migrate/network.go`
4. **Given** the restructure is complete, **When** a developer looks for volume lifecycle, **Then** it is in `migrate/vm_ops.go`
5. **Given** the restructure is complete, **When** a developer looks for disk conversion pipeline, **Then** it is in `migrate/conversion.go`

---

### User Story 2 — Find OpenStack client implementation (Priority: P2)

A developer working on OpenStack operations should find the interface and its implementation in the same package (`openstack/`). Today the interface lives in `openstack/openstackops.go` while the 1,392-line implementation lives in `pkg/utils/openstackopsutils.go`, making it non-obvious where OpenStack behavior is defined.

**Why this priority**: Interface/implementation split across packages is a fundamental cohesion violation that confuses new contributors and makes the dependency graph harder to reason about.

**Independent Test**: After the restructure, `pkg/utils/openstackopsutils.go` must not exist. `openstack/clients.go` must exist and contain `OpenStackClients` and all its methods.

**Acceptance Scenarios**:

1. **Given** the restructure is complete, **When** a developer looks for OpenStack client methods, **Then** they are in `openstack/clients.go` (same package as the interface)
2. **Given** the restructure is complete, **When** `pkg/utils/` is listed, **Then** `openstackopsutils.go` is absent
3. **Given** the restructure is complete, **When** `go build ./...` is run, **Then** it passes without errors

---

### User Story 3 — Mock Reporter in tests (Priority: P3)

A developer writing tests for `migrate/` should be able to inject a mock `Reporter` via the `reporter.ReporterOps` interface rather than depending on a concrete `*reporter.Reporter`. Today the `Migrate` struct holds a concrete type, making the Reporter untestable without a real implementation.

**Why this priority**: Smaller impact than the other two changes — the interface already exists and is exported; this is a 1-line struct field type change.

**Independent Test**: After the restructure, `Migrate.Reporter` field type is `reporter.ReporterOps`. Existing callers that assign `*reporter.Reporter` still compile because `*reporter.Reporter` satisfies the interface.

**Acceptance Scenarios**:

1. **Given** the restructure is complete, **When** `Migrate.Reporter` field is inspected, **Then** its type is `reporter.ReporterOps`
2. **Given** existing code assigns `*reporter.Reporter` to `Migrate.Reporter`, **When** compiled, **Then** it compiles without error

---

### Edge Cases

- What if a method is accidentally omitted from the new split files? → `go build ./...` will fail with "undefined: MethodName" — caught immediately
- What if moving `openstackopsutils.go` introduces a circular import? → `openstack/` is already imported by `migrate/`; adding the implementation does not change the import graph
- What if a test file directly references a method that moved files? → Tests are in the same package (`package migrate`), so within-package file splits are invisible to tests
- What if `*reporter.Reporter` does not fully implement `reporter.ReporterOps`? → `go build ./...` will fail with interface satisfaction error — caught immediately

## Requirements

### Functional Requirements

- **FR-001**: `migrate/migrate.go` MUST contain only: `Migrate` struct, `MigrateVM()`, `cleanup()`, `WaitforCutover`, `CheckIfAdminCutoverSelected`, `CheckCutoverOptions`, `gracefulTerminate`
- **FR-002**: NBD copy logic (`LiveReplicateDisks`, `EnableCBTWrapper`, `SyncCBT`, `WaitForAdminCutover`) MUST reside in `migrate/nbd_copy.go`
- **FR-003**: Network logic (port reservation + OS-specific network config) MUST reside in `migrate/network.go`
- **FR-004**: Conversion pipeline (`ConvertVolumes`, boot detection, virt-v2v dispatch) MUST reside in `migrate/conversion.go`
- **FR-005**: Volume lifecycle, storage provider setup, instance creation, and health checks MUST reside in `migrate/vm_ops.go`
- **FR-006**: All new `migrate/` files MUST declare `package migrate` (same package — no import changes required)
- **FR-007**: `pkg/utils/openstackopsutils.go` MUST be removed; its content MUST move to `openstack/clients.go` with package declaration changed to `package openstack`
- **FR-008**: All callers of `OpenStackClients` MUST import `openstack` package (not `pkg/utils`)
- **FR-009**: `Migrate.Reporter` field type MUST be changed from `*reporter.Reporter` to `reporter.ReporterOps`
- **FR-010**: No function body content MAY be modified — functions move verbatim
- **FR-011**: `hotadd_copy.go` and `vaai_copy.go` MUST remain unchanged

### Non-Functional Requirements

- **NFR-001**: `go build ./...` from `v2v-helper/` MUST pass after each sub-step
- **NFR-002**: `go vet ./...` from `v2v-helper/` MUST pass after restructure
- **NFR-003**: All existing tests MUST pass (verified via `make test-v2v-helper` on Linux/CGO)
- **NFR-004**: CGO build constraints (`CGO_ENABLED=1 GOOS=linux GOARCH=amd64`) MUST remain unchanged

## Success Criteria

### Measurable Outcomes

- **SC-001**: `migrate/migrate.go` line count is ≤600 lines after restructure (down from 2,723)
- **SC-002**: `pkg/utils/openstackopsutils.go` does not exist after restructure
- **SC-003**: `openstack/clients.go` exists and contains `OpenStackClients` type and all its methods
- **SC-004**: `go build ./...` exits with code 0 from `v2v-helper/`
- **SC-005**: `go vet ./...` exits with code 0 from `v2v-helper/`
- **SC-006**: All 10 existing test files continue to pass
- **SC-007**: No file in `migrate/` exceeds 800 lines

## Assumptions

- All moved functions are moved verbatim — no renaming, no signature changes, no body edits
- `*reporter.Reporter` already satisfies `reporter.ReporterOps` (verified before implementation)
- The `openstack/` package does not currently import `pkg/utils/` (no circular import risk from the move)
- macOS build validation covers import correctness; full CGO test run requires Linux environment
- `hotadd_copy.go` and `vaai_copy.go` standalone helpers (`getDiskKeys`, `readGuestWWIDs`, `sanitizeVolumeName`) remain with their only callers — moving single-caller helpers adds churn without gain
- **Test-First exception**: Constitution principle IV (Test-First) is satisfied by the existing test suite. This restructure moves functions verbatim with zero new logic; no new tests are required. `go build ./...` and `make test-v2v-helper` (all 10 existing test files) are the correctness gates.
