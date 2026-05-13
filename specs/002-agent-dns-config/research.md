# Research: Agent Node Custom Host Entries (v2)

## Decision 1: Host Entries Storage Location

**Decision**: Store in existing `vjailbreak-settings` ConfigMap under key `AGENT_HOST_ENTRIES` (JSON-encoded).

**Rationale**: Zero new infrastructure. All controllers and UI already read/write this ConfigMap. New key follows the exact same pattern as every other setting.

**Format**: JSON array `[{"ip":"1.2.3.4","hostnames":["h1","h2"]}]`. Absent/empty key → zero entries.

---

## Decision 2: No New Files in `k8s/migration` — Use `pkg/common/utils/`

**Decision**: New Go logic lives in `pkg/common/utils/hosts.go` (alongside existing `net.go`). No new file in `k8s/migration/pkg/utils/`.

**Rationale**:
- `HostEntry`, `BuildUserData`, `ParseHostEntries`, `ValidateHostEntry` are all **pure functions** — no Kubernetes or OpenStack deps. They belong in the shared utils package.
- `pkg/common/utils/` already has `net.go` (network utilities) and `net_test.go` — established pattern for pure utility code tested with standard `testing` package.
- Keeping pure logic in `pkg/common` maximizes reuse (v2v-helper, vpwned, or future tools can import it without pulling in controller-runtime).
- `vjailbreaknodeutils.go` is already 1151 lines. Adding k8s-specific helpers only (not pure logic) keeps it coherent.

**What stays in `k8s/migration`**:
- `GetAgentHostEntries(ctx, client.Client) ([]HostEntry, error)` — depends on `client.Client` (controller-runtime), controller-specific. Added to `vjailbreaknodeutils.go`.
- Reprovision annotation handler — in `vjailbreaknode_controller.go`.

---

## Decision 3: Cloud-init Injection — `BuildUserData` in `pkg/common/utils/hosts.go`

**Decision**: `BuildUserData(envFilePath, masterIP, token string, entries []HostEntry) string` lives in `pkg/common/utils/hosts.go`. It constructs the cloud-init YAML string, appending `runcmd` lines for each host entry via `echo "IP hostname1 hostname2" >> /etc/hosts`.

**Rationale**: Pure function — no side effects, no I/O. Imports only `pkg/common/constants` (same module). Easily unit-tested with table-driven tests. `vjailbreaknodeutils.go` replaces its one `fmt.Sprintf` call with `utils.BuildUserData(...)`.

**Backward compatibility**: `BuildUserData(envFile, masterIP, token, nil)` produces output byte-for-byte identical to the current `fmt.Sprintf(K3sCloudInitScript, ...)` call — zero behavioral change for existing nodes.

---

## Decision 4: Interface-First + TDD (Constitution Principle IV)

**Decision**: Define function signatures in `hosts.go` as empty stubs first. Write `hosts_test.go` against those signatures (red). Then implement (green). For `GetAgentHostEntries` and the reprovision handler, use controller-runtime's `fake.NewClientBuilder()` — already the test pattern in the controller suite.

**Interface-first** means: the public API (`HostEntry`, all exported functions) is defined and agreed on before a single line of implementation. This is the "interface" in the Go sense — the package's exported surface. Tests are written against signatures, not against implementation.

**Reprovision testability**: Extract the reprovision decision logic into a helper:
```go
func reprovisionAllowed(activeMigrations []string) bool
```
Pure function, trivially testable. The controller method calls this + the existing delete utilities. Injected via existing `utils.DeleteOpenstackVM` and `utils.DeleteNodeByName` — no new interfaces needed.

---

## Decision 5: `pkg/common/constants` — Add One Constant

**Decision**: Add `AgentHostEntriesKey = "AGENT_HOST_ENTRIES"` to `pkg/common/constants/constants.go` (source) and its vendor copy.

**Rationale**: Keeps ConfigMap key in one canonical place. Vendor copy must be updated alongside source — this is the existing pattern for all cross-module constants.

---

## Decision 6: Reprovision Mechanism

**Decision**: Annotation `vjailbreak.io/reprovision: "requested"` on VjailbreakNode CR. Controller detects in `reconcileNormal`, gates on `ActiveMigrations`, then deletes VM + k3s node, clears `Status.OpenstackUUID` + `Status.Phase`, removes annotation. Existing reconcile loop then creates fresh VM with updated cloud-init.

**Rationale**: Idiomatic Kubernetes one-shot operation pattern. Reuses existing `DeleteOpenstackVM` + `DeleteNodeByName` from `reconcileDelete`. No new CRD, no new endpoint.

---

## Decision 7: Module Boundary for Vendor Copies

When `pkg/common` source files change, the corresponding files under `k8s/migration/vendor/github.com/platform9/vjailbreak/pkg/common/` must be updated in the same commit. This is the existing manual vendor pattern in this repo (not using `go mod vendor` automatically).
