# vAssessment

Agentless, read-only discovery and migration-readiness assessment for a VMware
estate — the pre-flight tool for vJailbreak. See the [product doc](https://platform9.atlassian.net/wiki/spaces/vJailbreak/pages/6262587411)
for the full design.

This module is an independent Go module (own `go.mod`), following the same
convention as `k8s/migration`, `v2v-helper`, `pkg/vpwned`, and `pkg/common`.

## Status

Scaffolding ([#2386](https://github.com/platform9/vjailbreak/issues/2386)) plus
the pieces of discovery that don't need a live vCenter connection at all:

| Package | What it does |
|---|---|
| `preflight/` | Validates connection-input shape (`validate connection`) and an exported vCenter role's privileges against a required-privileges file (`validate role`) — both pure/offline, no vCenter call |
| `catalog/` | The tier-1 ("vCenter creds only") pre-check rules from the PRD §6.1 catalog, as pure functions of a `VM` struct. Each rule registers itself in its own file — add/change one without touching the rest |

Actual discovery (govmomi collectors populating `catalog.VM` from a live
vCenter, and SQLite snapshots) lands in
[#2387](https://github.com/platform9/vjailbreak/issues/2387);
snapshot show/diff in [#2388](https://github.com/platform9/vjailbreak/issues/2388).

```bash
vassessment validate connection --host=vcenter01.acme.internal --username=svc@vsphere.local --password=***
vassessment validate role --granted=granted-role.json --required=required-role.json
```

## Build & run

```bash
cd vassessment
make build   # -> bin/vassessment
make run     # go run ./cmd/vassessment
make test
```

Or from the repo root: `make vassessment` / `make test-vassessment`.
