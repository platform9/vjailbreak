# vJailbreak Constitution (v1.2.0)

This document establishes governance principles for the vJailbreak migration orchestration project.

## Core Principles

### I. Kubernetes-Native Architecture
All migration state must be represented as Kubernetes Custom Resources within k3s — no external state management allowed.

### II. External Documentation First
Developers must consult official documentation for dependencies (virt-v2v, libguestfs, controller-runtime, etc.) before implementing features.

### III. Generated Code Protection (NON-NEGOTIABLE)
Files like `deploy/installer.yaml` and `zz_generated.deepcopy.go` must never be hand-edited. Regenerate via `make generate` inside `k8s/migration/` after any CRD type changes.

### IV. Test-First Development (NON-NEGOTIABLE)
All new Go code requires unit tests with mocked external dependencies. No live system contact in tests. TDD sequence: tests written → approved → fail → implement (Red-Green-Refactor).

### V. Module Independence
Four independent Go modules must maintain separate dependency graphs. Commands run from module directories only. No shared `go.sum` files. Cross-module imports use full module paths.

### VI. AI-Assisted Development (NON-NEGOTIABLE)
AI agents must invoke relevant skills before coding. `superpowers:using-superpowers` invoked at session start.

### VII. Code Reuse and Simplicity (NEW)
Logic-preserving refactors reducing complexity are permitted at the point of need without dedicated tickets. Three similar lines is better than a premature abstraction.

## Critical Requirements

- Pre-commit hooks must activate via `make setup-hooks`
- Controller tests run via `cd k8s/migration && make test`
- ESXi DNS resolution is required during VM copy phases
- All PRs must pass tests and include new code coverage

## Governance

Constitution supersedes all other documentation. Amendments require maintainer approval.

**Version**: 1.2.0 | **Source**: branch `1889-maas-free-bm-provisioning`
