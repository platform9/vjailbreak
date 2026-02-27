# AGENTS.md

Guidance for autonomous coding agents working in this repository.

## Project summary

`vjailbreak` is a multi-component system for migrating VMs from VMware to OpenStack-compatible clouds.

Main parts of the repository:

- `ui/`: React + Vite frontend.
- `k8s/migration/`: Kubernetes controller and migration orchestration logic (Go).
- `v2v-helper/`: Go helper binary used during migration operations.
- `pkg/common/*`: Shared Go modules (`openstack`, `utils`, `validation`).
- `pkg/vpwned/`: Additional Go component with its own module and image build.
- `image_builder/`, `deploy/`, `templates/`: deployment/image assets.

## Baseline tooling

- Go 1.20+ (multiple Go modules are used).
- Node.js 18 + Yarn (UI work).
- Docker (image builds).
- Optional for controller workflows: `kubectl`, `kustomize`, envtest dependencies (installed by `make` targets as needed).

## Repository conventions

- Keep changes scoped to the task; avoid broad refactors unless required.
- Do not commit secrets, kubeconfigs, credentials, or generated artifacts unless explicitly required.
- Follow existing code style and naming in each component.
- Prefer minimal, targeted edits with corresponding validation.

## Common commands

From repository root:

```bash
make setup-hooks
make lint
make test-v2v-helper
```

UI (`ui/`):

```bash
yarn
yarn lint
yarn build
```

Controller (`k8s/migration/`):

```bash
make lint
make test
make build
```

v2v-helper (`v2v-helper/`):

```bash
make test
make build
```

Shared Go modules (run from module directory when touched):

```bash
go test ./...
```

## Validation guidance by change type

- UI-only changes: run `yarn lint` and `yarn build` in `ui/`.
- `k8s/migration` changes: run `make lint` and at least targeted `make test` in `k8s/migration/`.
- `v2v-helper` changes: run `make test` in `v2v-helper/`.
- `pkg/common/*` or `pkg/vpwned` changes: run `go test ./...` in each touched module.
- Cross-cutting changes: run the relevant checks for every impacted component.

## Notes and gotchas

- `make setup-hooks` configures `core.hooksPath` to `.githooks`.
- `v2v-helper` builds/tests use CGO and depend on `libnbd` toolchain availability.
- `k8s/migration make test` may download envtest binaries on first run.
- UI local development uses `yarn dev` (default Vite flow; see `ui/README.md` for env variables).

## Commit guidance

- Use clear, descriptive commit messages.
- Include docs/tests updates whenever behavior changes.
- Before finishing, ensure changed components build/test successfully with the smallest meaningful command set.
