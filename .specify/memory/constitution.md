# vJailbreak Constitution (v1.4.0)

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

### VII. Code Reuse and Simplicity
Logic-preserving refactors reducing complexity are permitted at the point of need without dedicated tickets. Three similar lines is better than a premature abstraction.

### VIII. Migration Field Parity (NON-NEGOTIABLE)
Any new field, block, section, or toggle added to the Migration Form MUST be propagated in the same PR to **all four** surfaces. A field that exists in only one surface is an incomplete change, not a follow-up ticket.

| # | Surface | Location | Obligation |
|---|---------|----------|------------|
| 1 | Migration Form | `ui/src/features/migration/pages/MigrationForm.tsx` + `ui/src/features/migration/steps/` | Field is captured and validated |
| 2 | Migration Details Page | `ui/src/features/migration/components/detail/MigrationDetailsTab.tsx` + `ui/src/features/migration/utils/migrationDetailFields.ts` | Field's persisted value is displayed |
| 3 | Retry Form | `ui/src/features/migration/components/RetryMigration.tsx`, `hooks/useRetryPrefill.ts`, `hooks/useRetrySubmit.ts`, `utils/retryFormState.ts` | Field is pre-filled from the failed Migration CR **and** re-submitted |
| 4 | Migration Template / Blueprint UI | `ui/src/features/migration/components/templates/` (`SaveAsTemplateDialog.tsx`, `TemplateDetailDrawer.tsx`) + `ui/src/api/migration-blueprints/` | Field is saved into the blueprint and restored when the template is applied |

Persistence is not display: a field that reaches the CR but is not shown on the details page still violates this principle. Prefill is not submit: a field shown in the retry form but dropped by `useRetrySubmit` still violates it.

Each of the four surfaces requires a unit test asserting the new field round-trips (`*.test.ts`/`*.test.tsx` alongside the file). Reviewers must reject PRs touching the migration form that do not show all four.

### IX. UI Kubernetes Access Is Explicit (NON-NEGOTIABLE)
The UI pod runs as `ui-manager-sa`, which holds a deliberately narrow ClusterRole. New resources are never reachable by default — access must be granted explicitly, in the same PR that introduces the read.

- **New vJailbreak CRD read by the UI** → add the plural resource name (and `<plural>/status` where the UI reads status) to the `vjailbreak.k8s.pf9.io` rules of the `ui-manager-role` ClusterRole in `ui/deploy/ui.yaml`, then regenerate manifests with `make generate-manifests`. Per Principle III, never hand-edit `deploy/installer.yaml`, `deploy/00crds.yaml`, or `k8s/migration/dist/install.yaml`.

- **New core/non-CRD resource read by the UI** (pods, pod logs, configmaps, events, …) → route it through the vpwned K8s reverse proxy, not through direct API access. Add an entry to `allowedRoutes` in `pkg/vpwned/server/k8s_proxy_handler.go` (method + path regex) and call it from the UI under `K8S_PROXY_BASE_PATH` (`ui/src/api/constants.ts`). The proxy is a method+path allowlist that also verifies the caller is `system:serviceaccount:migration-system:ui-manager-sa`; an unlisted route returns 403 at the proxy even when RBAC would allow it.

- Every new `allowedRoutes` entry requires a table-driven case in `pkg/vpwned/server/k8s_proxy_handler_test.go` covering both the allowed method and a rejected one.

### X. Upgrade Flow Parity (NON-NEGOTIABLE)
The appliance upgrades itself by replaying the target tag's manifests and images. Anything the upgrade flow does not know about keeps running the **old version forever**, silently, with no warning in the UI or logs.

Trigger: a PR adds a Deployment or pod, a container image, a `deploy/*.yaml` manifest, or a component the upgrade must replace. Such a PR MUST do one of two things — never neither:

1. Wire the component into **both** the upgrade and rollback paths in the same PR, per the table below; or
2. Advise developer to open a GitHub issue naming the sites left unwired and link it in the PR description, so the gap is tracked rather than discovered by a user.

| # | Site | Location | Obligation |
|---|------|----------|------------|
| 1 | Workload list | `pkg/vpwned/upgrade/executor.go` (`DeploymentConfigs`) | Deployment listed; this drives the post-upgrade stability check |
| 2 | Upgrade apply + wait | `pkg/vpwned/upgrade/executor.go` (`runDeploymentPhase`) | Manifest applied for the target tag, deployment added to the readiness wait, and `TotalUpgradeSteps` in `progress.go` incremented — the phase reports one step per manifest applied |
| 3 | Rollback apply + wait | `pkg/vpwned/upgrade/executor.go` (`ExecuteRollback`) | Manifest re-applied for the previous version and deployment added to the readiness wait |
| 4 | Backup + restore | `pkg/vpwned/upgrade/version_validator.go` (`BackupResourcesWithID`, `RestoreResources`) | Snapshotted before the upgrade and restored during rollback — a rollback can only restore what was backed up |
| 5 | Image pre-flight | `pkg/vpwned/upgrade/version_checker.go` (`CheckImagesExist`) | Image verified before the upgrade job starts, so a missing image fails fast instead of surfacing as a readiness timeout followed by a rollback |

A new CRD needs no code change here: cleanup and re-apply discover CRDs dynamically by API group. A new manifest that is **not** part of `deploy/00crds.yaml` does need wiring.

Tests in `pkg/vpwned/upgrade/` MUST iterate `DeploymentConfigs` rather than hardcode workload names, so an unwired workload fails the suite instead of shipping.

## Critical Requirements

- Pre-commit hooks must activate via `make setup-hooks`
- Controller tests run via `cd k8s/migration && make test`
- ESXi DNS resolution is required during VM copy phases
- All PRs must pass tests and include new code coverage
- PRs adding a migration form field must satisfy the four-surface table in Principle VIII
- PRs adding a UI-side Kubernetes read must satisfy Principle IX (RBAC entry or proxy allowlist entry)
- PRs adding a workload, image, or `deploy/` manifest must satisfy Principle X (wire the upgrade and rollback paths, or link a tracking issue)

## Governance

Constitution supersedes all other documentation. Amendments require maintainer approval.


**Version**: 1.4.0 | **Last Amended**: 2026-08-12 | **Source**: branch `private/main/sarika/constitution-update-upgrade`
