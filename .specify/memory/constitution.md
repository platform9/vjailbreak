<!--
Sync Impact Report
==================
Version change: 1.1.0 → 1.2.0
Modified principles: None
Added sections: Principle VII — Code Reuse and Simplicity
Removed sections: None
Templates requiring updates:
  - .specify/templates/plan-template.md ✅ — Constitution Check section is generic; no principle-specific outdated refs
  - .specify/templates/spec-template.md ✅ — Sections align with test-first and requirements principles
  - .specify/templates/tasks-template.md ✅ — Test task structure aligns with Principle IV
Deferred TODOs: None
-->

# vJailbreak Constitution

## Core Principles

### I. Kubernetes-Native Migration Architecture

All migration orchestration MUST run as Kubernetes controllers and worker pods within the k3s cluster.
VM state, progress, and configuration MUST be represented exclusively as Kubernetes Custom Resources (CRDs).
Every new capability MUST be expressed as a CRD type or controller reconciliation loop — no out-of-band
state management is permitted.

**Rationale**: The k8s API provides the single source of truth for migration state. Bypassing it creates
split-brain conditions that break observability, retry logic, and user-facing status reporting.

### II. External Documentation First

Before implementing any feature or debugging any issue involving a core dependency, developers MUST
consult the official documentation of the underlying open-source tool (virt-v2v, libguestfs, nbdkit,
govmomi, controller-runtime, OpenStack, k3s, virtio-win). Implementation MUST NOT begin based solely
on prior knowledge or inference.

**Rationale**: vJailbreak is a thin orchestration layer over complex external systems. Subtle
version-specific behaviors in virt-v2v or libguestfs have historically caused hard-to-reproduce
migration failures. Documentation consultation prevents re-discovering known constraints.

### III. Generated Code Must Never Be Hand-Edited (NON-NEGOTIABLE)

The files `deploy/installer.yaml` and all `zz_generated.deepcopy.go` files MUST NOT be hand-edited
under any circumstances. After modifying CRD types in `k8s/migration/api/v1alpha1/`, developers MUST
run `make generate` inside `k8s/migration/` to regenerate all derived artifacts. CRD changes MUST be
validated with `cd k8s/migration && make test` before committing.

**Rationale**: Hand-editing generated files creates silent divergence between source types and deployed
manifests, causing reconciliation failures in production that are extremely difficult to diagnose.

### IV. Test-First for All New Code (NON-NEGOTIABLE)

All new Go code written in this repository MUST have accompanying unit tests in `_test.go` files
alongside the implementation. Table-driven tests MUST be used wherever multiple input/output cases
apply. External dependencies (VMware, OpenStack, Kubernetes API) MUST be mocked via interfaces —
real external systems MUST NOT be contacted in unit tests. If existing code is hard to test,
developers MUST refactor (extract interfaces, dependency injection, split large functions) without
changing logic before writing tests.

**Rationale**: vJailbreak migrations are irreversible and affect production VMs. Code without tests
cannot be safely changed. Mocking external systems ensures tests are deterministic and fast.

### V. Module Independence

vJailbreak contains four independent Go modules: `k8s/migration/`, `v2v-helper/`, `pkg/vpwned/`,
and `pkg/common/`. All `go` commands MUST be run from the correct module directory. Cross-module
imports MUST reference the full module path. When adding dependencies, `go mod tidy` MUST be run
in the specific module directory only. Modules MUST NOT share a `go.sum`.

**Rationale**: Independent modules allow each component (controller, migration worker, API server)
to have its own dependency graph and build pipeline, preventing version conflicts and enabling
independent image builds.

### VI. AI-Assisted Development — Invoke Skills and Superpowers (NON-NEGOTIABLE)

When using AI agents (Claude Code or equivalent) to develop in this repository, the agent MUST invoke
relevant skills and superpowers before proceeding with any task. If there is even a 1% chance a skill
applies, the agent MUST invoke it before writing code, debugging, planning, or making architectural
decisions. The `superpowers:using-superpowers` skill MUST be invoked at session start. Skills
MUST be invoked in priority order: process skills first (brainstorming, debugging), then implementation
skills. The agent MUST NOT rationalize skipping a skill invocation.

**Rationale**: Skills encode validated workflows and project-specific patterns. Bypassing them causes
the agent to drift toward generic behavior, miss project conventions, and produce work that requires
correction — wasting review cycles on issues the skills would have prevented.

### VII. Code Reuse and Simplicity

Small refactors that reduce complexity, eliminate duplication, or enable reuse MUST NOT be blocked
when they preserve existing logic and behavior. Extracting shared helpers, deduplicating repeated
patterns, and simplifying function signatures are permitted at the point of need without requiring
a dedicated refactor ticket. Logic-preserving refactors MUST be kept in separate commits from
behavioral changes within the same PR. Refactors MUST NOT introduce new abstractions speculatively —
only extract what is actually reused in the current change.

**Rationale**: Complexity accumulates when small, obvious improvements are deferred. Allowing targeted
simplification at the point of need keeps the codebase maintainable without the overhead of large
dedicated refactor PRs that rarely get prioritized.

## Build & Platform Standards

- v2v-helper tests require `CGO_ENABLED=1 GOOS=linux GOARCH=amd64` and MUST NOT be compiled on
  macOS without a Linux cross-compilation toolchain. Use `make test-v2v-helper` (requires Linux or
  Docker).
- Controller tests MUST be run via `cd k8s/migration && make test`.
- Image tags default to `<git-parent-branch>-<short-sha>`. Override with `BUILD_VERSION` and
  `REGISTRY` environment variables.
- `make generate-manifests` MUST only be run after `vjail-controller` and `ui` are built — it
  depends on both artifacts.
- VDDK libraries MUST reside at `/home/ubuntu/vmware-vix-disklib-distrib` on the vJailbreak VM.
- ESXi host DNS resolution is required during the VM copy phase. Missing DNS causes migration
  failures that surface only during disk transfer.

## Development Workflow

- `make setup-hooks` MUST be run once per clone before any commits to activate pre-commit validation.
- Pre-commit hooks validate code formatting and run basic checks. Hooks MUST NOT be bypassed
  (`--no-verify` is prohibited) unless explicitly approved by a maintainer with documented reason.
- All PRs MUST pass `cd k8s/migration && make test` and include unit tests for new code.
- The UI dev server requires `VITE_API_HOST` and `VITE_API_TOKEN` environment variables.
- DNS configuration changes to `/etc/resolv.conf` require restarting the controller deployment:
  `kubectl -n vjailbreak rollout restart deployment migration-controller-manager`.

## Governance

This constitution supersedes all other development practices documented in this repository.
Amendments require: (1) a written rationale documenting the change and motivation, (2) approval
via PR review by at least one maintainer, and (3) a migration plan if the amendment changes an
existing non-negotiable principle.

All PRs and code reviews MUST verify compliance with the principles above, particularly Principles
III and IV. Complexity violations (e.g., additional Go modules, skipped tests) MUST be justified
in the PR description referencing this constitution.

Version semantics follow semver: MAJOR for backward-incompatible governance changes or principle
removal; MINOR for new principles or materially expanded guidance; PATCH for clarifications and
wording fixes.

Runtime development guidance is maintained in `CLAUDE.md` at the repository root.

**Version**: 1.2.0 | **Ratified**: 2026-05-08 | **Last Amended**: 2026-05-08
