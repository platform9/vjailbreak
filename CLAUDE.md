# vJailbreak Development Instructions

vJailbreak is a VMware to Platform9 Private Cloud Director (PCD) VM migration tool built on Kubernetes (k3s). It converts VMDK disks to QCOW2 (or raw LUNs) and deploys VMs to OpenStack, supporting hot (live) and cold migrations with Changed Block Tracking (CBT).

## Quick References

- **Project Overview**: See @README.md for user-facing documentation
- **Contributing Guidelines**: See @CONTRIBUTING.md for contribution workflow
- **Architecture Deep-Dive**: https://deepwiki.com/platform9/vjailbreak
- **Platform9 PCD Documentation**: https://docs.platform9.com/

---

## External Documentation

**DIRECTIVE**: When working with vJailbreak components, ALWAYS consult the official documentation of underlying open-source tools before implementing features or debugging issues.

**Core Dependencies**: virt-v2v, libguestfs, nbdkit, k3s, OpenStack, Platform9 PCD, govmomi, controller-runtime, virtio-win

**Documentation Links**:
- **virt-v2v**: https://libguestfs.org/virt-v2v.1.html | https://libguestfs.org/virt-v2v-support.1.html
- **libguestfs**: https://libguestfs.org/ | **nbdkit**: https://libguestfs.org/nbdkit.1.html
- **k3s**: https://docs.k3s.io/ | **OpenStack**: https://docs.openstack.org/
- **Platform9 PCD**: https://docs.platform9.com/
- **govmomi**: https://github.com/vmware/govmomi
- **controller-runtime**: https://pkg.go.dev/sigs.k8s.io/controller-runtime
- **virtio-win**: https://github.com/virtio-win/kvm-guest-drivers-windows

---

## AI Development Behavior

**These rules govern how AI agents must behave in this project:**

### Clarifying Questions (NON-NEGOTIABLE)
- For vague or ambiguous prompts, ask clarifying questions BEFORE taking any action — no silent assumptions on scope, intent, or requirements
- When a description contains an implicit solution ("we want X to do Y"), ask what problem Y solves before committing to Y

### Skills Invocation (NON-NEGOTIABLE)
- Invoke `superpowers:using-superpowers` at session start
- Invoke relevant skills (brainstorming, debugging, TDD, etc.) before any non-trivial task — even 1% chance a skill applies means invoke it

### Security Features
Always build a threat model before writing requirements. Ask:
- "Where exactly can this credential/secret be obtained?" (enumerate all vectors)
- "Can we eliminate the exposure entirely, or only mitigate it?"
- "What is the blast radius if exploited?"

**Eliminate > mitigate**: Propose elimination first. Only fall back to mitigation (rotation, shorter TTL, rate limiting) after confirming elimination is not feasible.

### Test-First Development
All new Go code: write tests first → get approval → confirm they fail → implement (Red-Green-Refactor). No live system contact in tests — mock external dependencies.

### Code Simplicity
Three similar lines is better than a premature abstraction. Logic-preserving refactors reducing complexity are permitted at point of need without dedicated tickets.

### Commit Strategy (Speckit Workflows)
Commit after each completed phase, not in one big commit at the end. Run `git add` + `git commit` scoped to that phase before starting the next.

### Architecture Constraint
All migration state must be represented as Kubernetes Custom Resources within k3s — no external state management.

---

## Core Design Principles

Apply these to every change:

| Principle | Rule |
|-----------|------|
| **Interface-First** | No new abstraction layers when configuration suffices. Prefer config/Lua over a new Go service; prefer a function over a new interface. |
| **Modular Boundaries** | Changes scoped to their directory. No cross-module leakage without full module path imports. |
| **Clean Code** | Minimal, single-responsibility blocks. Each function/component does one thing. |
| **Observability** | All error paths must log. Failed external calls must be visible in logs. |
| **Simplicity First** | Among solutions that satisfy requirements, choose the simplest. |
| **No Premature Abstraction** | Three similar lines is better than a wrapper. Abstract only when the third real case arrives. |

### Refactor-as-You-Go (NON-NEGOTIABLE)

Every feature branch must include **1–2 targeted refactoring changes** to files touched by or adjacent to the feature work. Goal: incrementally improve code simplicity and modularity so pending unit tests can be written. Rules:

- Refactors must be logic-preserving (no behavior change)
- Scope to files already being read/modified — no drive-by rewrites of unrelated code
- Each refactor commit is separate from feature commits (`refactor:` prefix in commit message)
- Priority targets: functions >50 lines, untestable code with tight coupling, duplicate logic across files

---

## Development Rules

**Critical directives — follow these strictly:**

### CRD Changes
- After editing types in `k8s/migration/api/v1alpha1/`, ALWAYS run `make generate` inside `k8s/migration/` to regenerate deepcopy/client code and update CRD YAML
- Test CRD changes with `cd k8s/migration && make test` before committing

### Generated Files
- NEVER hand-edit `deploy/installer.yaml` — it is generated by `make generate-manifests`
- NEVER hand-edit `zz_generated.deepcopy.go` files — they are generated by controller-gen

### Git Workflow
- Run `make setup-hooks` once per clone before any commits to activate pre-commit validation
- Pre-commit hooks will validate code formatting and run basic checks

### Unit Test Requirements (NON-NEGOTIABLE)
- ALWAYS write unit tests for any Go file touched by a change — new code AND modified existing code
- Place tests in `_test.go` files alongside the code under test (Go convention)
- If existing code is hard to unit test (e.g., no interfaces, large functions with external deps), refactor up to 1-2 files to make it testable — **refactor must not change logic or behavior**, only restructure for testability (e.g., extract interfaces, dependency injection, split large functions into pure helpers)
- Use table-driven tests for Go code where multiple input/output cases apply
- Mock external dependencies (VMware, OpenStack, Kubernetes API) using interfaces — do not hit real external systems in unit tests
- Use govmomi simulator (`github.com/vmware/govmomi/simulator`) for vCenter logic tests; use controller-runtime fake client for k8s logic tests

### Integration/Build Testing Requirements
- v2v-helper tests require `CGO_ENABLED=1 GOOS=linux GOARCH=amd64`
- v2v-helper tests will NOT compile on macOS without Linux cross-compilation toolchain
- Run `make test-v2v-helper` for v2v-helper tests (requires Linux or Docker)
- Run `cd k8s/migration && make test` for controller tests
- ALWAYS run tests before submitting PRs

### Module Structure
- Four independent Go modules — run `go` commands from the correct directory:
  - Controller: `k8s/migration/`
  - V2V Helper: `v2v-helper/`
  - API Server: `pkg/vpwned/`
  - Common: `pkg/common/`
- When adding dependencies, run `go mod tidy` in the specific module directory
- Cross-module imports must reference the full module path

---

## Repository Layout

| Path | Purpose |
|------|---------|
| `k8s/migration/` | Kubernetes controller manager (Go, controller-runtime) |
| `v2v-helper/` | Migration worker pod — disk copy and conversion (Go, libguestfs) |
| `ui/` | React/TypeScript frontend (MUI, Vite) |
| `pkg/vpwned/` | REST API server (Go) for Cluster Conversion |
| `pkg/common/` | Shared Go utilities |
| `image_builder/` | Builds the vJailbreak appliance QCOW2 image |
| `appliance/` | Vagrant-based k3s cluster for local testing |
| `deploy/` | Generated Kubernetes manifests |
| `docs/` | Astro documentation site |
| `scripts/` | Utility and firstboot scripts |

---

## Quick Commands

```bash
# One-time setup
make setup-hooks

# Build components
make ui v2v-helper vjail-controller build-vpwned
make generate-manifests  # Requires vjail-controller and ui built first
make build-image         # Complete appliance QCOW2

# Testing
make test-v2v-helper     # v2v-helper (requires Linux CGO)
cd k8s/migration && make test  # Controller tests

# Development
make run-local           # Run controller locally
cd ui && yarn dev        # UI dev server (requires VITE_API_HOST, VITE_API_TOKEN)
```

**Image tags**: Default `<git-parent-branch>-<short-sha>`. Override: `BUILD_VERSION=v1.2.3 REGISTRY=myregistry.io make <target>`

---

## Common Pitfalls

- **macOS Development**: v2v-helper tests require Linux CGO, use Docker/Linux VM
- **DNS Resolution**: ESXi host DNS required for VM copy. Add to `/etc/hosts` or restart controller after `/etc/resolv.conf` changes
- **VDDK Libraries**: Must be in `/home/ubuntu/vmware-vix-disklib-distrib` on vJailbreak VM
- **Build Dependencies**: `generate-manifests` requires `vjail-controller` and `ui` built first

---

## Debugging

```bash
# Controller logs
kubectl -n vjailbreak logs -l control-plane=controller-manager -f

# Migration status
kubectl -n migration-system get migration <name> -o yaml

# V2V helper logs
kubectl -n migration-system logs <migration-name>-v2v-helper
```

**Check**: Guest OS support at https://libguestfs.org/virt-v2v-support.1.html

---

## Repository Structure

| Path | Purpose |
|------|---------||
| `k8s/migration/` | Controller (Go module) |
| `v2v-helper/` | Migration worker (Go module, CGO required) |
| `ui/` | React/TypeScript frontend |
| `pkg/vpwned/` | API server (Go module) |
| `pkg/common/` | Shared utilities (Go module) |
| `scripts/` | Utility and firstboot scripts |
| `deploy/` | Generated Kubernetes manifests |

**Key CRDs**: Migration, MigrationPlan, VMwareCreds, OpenstackCreds, NetworkMapping, StorageMapping, MigrationTemplate

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
at specs/003-hot-add-proxy/plan.md
<!-- SPECKIT END -->

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- ALWAYS read graphify-out/GRAPH_REPORT.md before reading any source files, running grep/glob searches, or answering codebase questions. The graph is your primary map of the codebase.
- IF graphify-out/wiki/index.md EXISTS, navigate it instead of reading raw files
- For cross-module "how does X relate to Y" questions, prefer `graphify query "<question>"`, `graphify path "<A>" "<B>"`, or `graphify explain "<concept>"` over grep — these traverse the graph's EXTRACTED + INFERRED edges instead of scanning files
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
