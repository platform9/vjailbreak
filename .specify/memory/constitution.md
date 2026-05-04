<!--
  Sync Impact Report
  Version change: N/A (initial) → 1.0.0
  Added sections: Core Principles (I–VII), Technology Standards, Development Workflow, Governance
  Removed sections: N/A — initial authoring
  Templates requiring updates:
    ✅ .specify/templates/plan-template.md — Constitution Check gates align dynamically with Principles I–VII
    ✅ .specify/templates/spec-template.md — Acceptance Scenarios and independent testability align with Principle III
    ✅ .specify/templates/tasks-template.md — Phase structure and parallel task labeling align with Principles II and III
  Follow-up TODOs: None — all placeholders resolved.
-->

# vJailbreak Constitution

## Core Principles

### I. Interface-First Design (NON-NEGOTIABLE)

All external dependencies — VMware/vCenter clients, OpenStack clients, Kubernetes clients, disk I/O
backends, and HTTP transports — MUST be accessed through Go interfaces defined in the consuming package.
Business logic MUST NOT directly instantiate concrete external client types.

- Interfaces belong in the package that *uses* them, not the package that *implements* them.
- Every interface MUST be the minimal surface needed by its consumer (Interface Segregation Principle).
- Fake/mock implementations MUST live in a `fake/` or `testutil/` sub-package alongside the interface.
- TypeScript services (API calls, VMware/OpenStack clients) MUST be injected via props or context,
  never imported directly into components.

**Rationale**: Tight coupling to VMware SDK or OpenStack SDK makes unit testing impossible without live
infrastructure. Interface injection enables fast, deterministic unit tests and safe future swapping of
cloud providers.

### II. Modular Boundaries

The four Go modules (`k8s/migration/`, `v2v-helper/`, `pkg/vpwned/`, `pkg/common/`) are deployment and
compilation boundaries — they MUST remain independent.

- Packages within a module MUST have a single, clearly-named responsibility.
- No circular imports are permitted within or across modules.
- Business logic MUST NOT import `controller-runtime` or Kubernetes API machinery types directly in
  non-controller packages; wrap them in domain types.
- React/TypeScript UI components MUST be organized by feature domain, not by type (avoid flat
  `components/` catch-all directories).
- Shared utilities belong in `pkg/common/` only when used by two or more other modules.

**Rationale**: Independent modules allow separate release cycles, targeted testing, and prevent
accidental coupling between the migration worker (CGO, Linux-only) and the controller (pure Go).

### III. Test-Driven Quality (NON-NEGOTIABLE)

All new features and bug fixes MUST be accompanied by tests before implementation is considered complete.

- **Unit tests**: Mock external dependencies via interfaces (Principle I). Zero network or disk I/O.
- **Controller tests**: Use `envtest` with real CRD schemas; fake Kubernetes client permitted for
  reconciler unit tests. Run via `cd k8s/migration && make test`.
- **v2v-helper integration tests**: Require `CGO_ENABLED=1 GOOS=linux` — no mocking of libguestfs
  or disk operations. Run via `make test-v2v-helper` (Linux or Docker only; macOS excluded).
- **UI tests**: Components tested with Vitest + Testing Library; API calls replaced with mock handlers.
- Test coverage MUST prioritize critical paths: reconcile loops, disk conversion logic, credential
  validation, and migration phase transitions.

**Rationale**: Migration failures cause data loss or VM downtime. Tests that do not exercise real
boundaries (disk I/O, Kubernetes state machines) have historically masked production failures.

### IV. Clean and Explicit Code

Code MUST be readable by a new contributor without inline explanation of *what* it does.

- Functions MUST do one thing; target ≤50 lines per function. Exceptions require a comment explaining
  the structural constraint that prevents decomposition.
- Errors MUST be wrapped with context: `fmt.Errorf("reconcile migration %s: %w", name, err)`.
- Errors MUST be logged exactly once at the handling boundary — never re-logged at each call frame.
- No magic numbers or untyped string literals; use named constants or typed aliases.
- Comments explain *why*, never *what*. Self-describing identifiers make what-comments redundant.
- Go: `gofmt` + `golangci-lint` (project `.golangci.yml`). TypeScript: `eslint` + `prettier`.
  Both are enforced in CI and MUST pass before merge.

**Rationale**: The codebase spans four modules and two languages. Consistent style and explicit error
context are the minimum bar for safe on-call debugging when a migration fails at 2 AM.

### V. Controller-Runtime Discipline

Kubernetes reconcilers MUST follow `controller-runtime` idioms without exception.

- Reconcile functions MUST be idempotent — safe to call any number of times with the same outcome.
- CRD Status MUST use typed conditions following Kubernetes API conventions:
  `Type`, `Status`, `Reason`, `Message`, `LastTransitionTime`.
- Watch predicates MUST filter events to prevent reconcile storms on unrelated field changes.
- After editing types in `k8s/migration/api/v1alpha1/`, `make generate` MUST be run immediately and
  regenerated files committed in the same PR as the type change.
- NEVER hand-edit `zz_generated.deepcopy.go` or `deploy/installer.yaml`.

**Rationale**: Non-idempotent reconcilers cause split-brain migration state when the controller
restarts mid-operation. Typed conditions are the observable contract between the controller and
every consumer (UI, operator, CLI).

### VI. Observability by Default

Every migration state transition MUST be visible without `kubectl exec` or log tailing.

- All significant state changes MUST emit a Kubernetes Event via `EventRecorder`.
- Structured logging MUST use `zap`; `Info` level for operator-facing events, `Debug` for internals.
- Every log line in a reconciler MUST include migration name, VM name, and current phase.
- Prometheus metrics MUST cover: active migration count, failed migration count, disk copy throughput,
  and per-phase duration histograms.
- The UI MUST surface migration phase, current operation, and error reason without requiring direct
  Kubernetes API access by the end user.

**Rationale**: Migrations run asynchronously for hours. Operators cannot be expected to tail logs;
the system must report its own state proactively and unambiguously.

### VII. Simplicity First (YAGNI)

No abstraction, generalization, or extension point is added until it is concretely needed.

- Apply the rule of three: introduce an abstraction only when the identical pattern appears in three
  independent places.
- Do not add feature flags, backwards-compatibility shims, or speculative extension hooks.
- Prefer stdlib over third-party packages when the stdlib solution is straightforward.
- Remove dead code immediately; do not comment it out or hide it behind a disabled flag.
- Each PR MUST have a stated, bounded scope. Scope creep requires a separate PR.

**Rationale**: vJailbreak has a focused domain. Premature abstractions for hypothetical multi-cloud
generalization have historically introduced complexity without benefit.

## Technology Standards

Canonical technology choices that MUST NOT be changed without a constitution amendment.

- **Go version**: As declared in each module's `go.mod`; upgrade only with full test suite passage
  across all four modules.
- **Kubernetes controller framework**: `controller-runtime` exclusively — no raw `client-go`
  reconciliation loops.
- **Linting**: `golangci-lint` with project `.golangci.yml`; `eslint` + `prettier` for TypeScript.
  CI blocks merges on lint failures.
- **UI stack**: React + TypeScript + MUI + Vite. No additional component libraries without a
  constitution amendment or explicit maintainer approval.
- **CRD code generation**: `controller-gen` via `make generate`. Hand-editing generated files is
  prohibited.
- **Disk conversion toolchain**: `virt-v2v` + `nbdkit`. No direct VMDK parsing in Go application code.
- **Test runners**: `go test` for Go, `vitest` for UI. Framework changes require an amendment.
- **Container registry**: `quay.io/platform9/` for all published images.

## Development Workflow

Quality gates that MUST pass before any PR is merged.

- Run `make setup-hooks` once per clone; pre-commit hooks are mandatory, not advisory.
- Every PR MUST include tests covering changed logic (unit or integration per Principle III).
- Run `go mod tidy` in the affected module after any dependency addition or removal.
- PRs touching CRD types MUST include regenerated `zz_generated.deepcopy.go` and updated CRD YAML
  in the same commit.
- Run `make generate-manifests` after controller or UI changes before cutting a release.
- All tests MUST pass: `cd k8s/migration && make test` (always) and `make test-v2v-helper`
  (required for any v2v-helper change).
- Code review MUST verify: interface usage for external dependencies, idempotent reconcilers,
  structured and contextualized errors, and observability coverage.
- No force-pushes to `main`. Rebase and merge-commit are required for full traceability.

## Governance

This constitution supersedes all other practices and style guides in the vJailbreak repository.
Any contradiction between this document and another guide is resolved in favor of this document.

- **Amendments**: Any addition, removal, or redefinition of a principle requires a PR with a `docs:`
  prefix, reviewed and approved by at least one maintainer, with a migration note describing how
  existing non-compliant code will be brought into compliance.
- **Version policy**: MAJOR for principle removal or redefinition; MINOR for a new principle or
  section; PATCH for clarification or wording refinement.
- **Compliance reviews**: Every PR description MUST assert compliance with each relevant principle
  or provide an explicit written waiver with justification. Unaddressed principles are assumed
  non-compliant and block merge.
- **Guidance file**: See `CLAUDE.md` for runtime development guidance (build commands, debugging,
  module layout). `CLAUDE.md` documents *how*; this constitution documents *why and what*.

**Version**: 1.0.0 | **Ratified**: 2026-04-29 | **Last Amended**: 2026-04-29
