# Implementation Plan: Agent Node Custom Host Entries (v2)

**Branch**: `1890-agent-dns-config` | **Date**: 2026-05-13 | **Spec**: [spec.md](spec.md)

## Summary

Allow admins to define custom hostname-to-IP mappings (ESXi, vCenter, PCD, OpenStack FQDNs) stored in the existing `vjailbreak-settings` ConfigMap. Injected into agent node VMs via cloud-init at provisioning time. "Reprovision Node" action in NodesTable allows applying updated config to idle nodes.

Pure logic (`HostEntry`, `BuildUserData`, parse/validate) goes in `pkg/common/utils/hosts.go` alongside existing `net.go` — reusable, no k8s dep. `GetAgentHostEntries` (k8s-client-specific) stays in `vjailbreaknodeutils.go`. No new files in `k8s/migration/pkg/utils/`.

See [research.md](research.md) for all design decisions. See [data-model.md](data-model.md) for types and cloud-init format.

## Technical Context

**Language/Version**: Go 1.21+ (controller + common), TypeScript 5.x (UI)

**Primary Dependencies**: controller-runtime (k8s), gophercloud (OpenStack), React + MUI, react-hook-form

**Storage**: Kubernetes ConfigMap `vjailbreak-settings` (existing) — new key `AGENT_HOST_ENTRIES`

**Testing**:

- `pkg/common/utils/`: standard `testing` package, table-driven (matches `net_test.go`)
- `k8s/migration/`: Ginkgo/Gomega + controller-runtime `fake.NewClientBuilder()`; run via `cd k8s/migration && make test`
- UI: React Testing Library; run via `cd ui && yarn test`

**Target Platform**: Linux k3s cluster (controller), browser (UI)

**Project Type**: Kubernetes operator + shared library + web frontend

**Constraints**: No new CRDs. No new files in `k8s/migration/pkg/utils/`. Vendor copies updated with source. Backward-compatible (absent key = zero entries = current behavior).

**Scale/Scope**: ≤50 host entries; affects all newly provisioned worker VjailbreakNodes

## Constitution Check

*Constitution v1.2.0 — all 7 principles evaluated.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Kubernetes-Native | ✅ PASS | State in existing ConfigMap CR; reprovision via VjailbreakNode annotation |
| II. External Docs First | ✅ PASS | cloud-init `runcmd` behavior verified; controller-runtime fake client pattern confirmed |
| III. Generated Code Protection | ✅ PASS | `HostEntry` is a plain struct — no deepcopy regeneration; no generated files touched |
| IV. Test-First (NON-NEGOTIABLE) | ✅ PASS | Stubs in `hosts.go` defined first → `hosts_test.go` written against stubs (red) → implement (green). `GetAgentHostEntries` tested with fake k8s client. Reprovision helper `reprovisionAllowed()` is pure — trivially testable. |
| V. Module Independence | ✅ PASS | New code in `pkg/common` (one module) + `k8s/migration` (one module). Vendor copies updated manually per existing pattern. No shared `go.sum` impact. |
| VI. AI-Assisted Development | ✅ PASS | Skills invoked throughout |
| VII. Code Reuse and Simplicity | ✅ PASS | Reuses `pkg/common/utils/` pattern, existing `DeleteOpenstackVM`/`DeleteNodeByName`, existing NodesTable action pattern. Minimal additions. |

**Gate result: ALL PASS — proceed.**

**TDD enforcement (Principle IV)**:

1. Define stubs (signatures + zero-value returns) in `hosts.go`
2. Write all tests in `hosts_test.go` — all fail
3. Implement `hosts.go` — all pass
4. Same sequence for `GetAgentHostEntries` in `vjailbreaknodeutils.go` and reprovision in controller

## Project Structure

### Documentation

```text
specs/002-agent-dns-config/
├── plan.md        ← this file
├── research.md    ← design decisions
├── data-model.md  ← types and formats
└── tasks.md       ← /speckit-tasks output
```

### Source Code Changes

```text
pkg/common/
├── utils/
│   ├── hosts.go           ← NEW: HostEntry, BuildUserData, ParseHostEntries,
│   │                           SerializeHostEntries, ValidateHostEntry
│   └── hosts_test.go      ← NEW: table-driven unit tests (pure functions)
└── constants/
    └── constants.go       ← MOD: add AgentHostEntriesKey constant

k8s/migration/
├── vendor/github.com/platform9/vjailbreak/pkg/common/
│   ├── utils/
│   │   ├── hosts.go       ← NEW: vendor copy of pkg/common/utils/hosts.go
│   │   └── hosts_test.go  ← NEW: vendor copy (or omitted per vendor conventions)
│   └── constants/
│       └── constants.go   ← MOD: vendor copy updated with new constant
├── pkg/utils/
│   └── vjailbreaknodeutils.go  ← MOD: add GetAgentHostEntries; replace
│                                     fmt.Sprintf call with BuildUserData
└── internal/controller/
    └── vjailbreaknode_controller.go  ← MOD: add reprovision annotation handler

ui/src/
├── features/globalSettings/
│   ├── helpers.ts                      ← MOD: add AGENT_HOST_ENTRIES to SettingsForm
│   └── components/
│       ├── GlobalSettingsPage.tsx      ← MOD: add "Host Entries" tab
│       └── HostEntriesTab.tsx          ← NEW: CRUD UI for host entries
└── features/agents/components/
    └── NodesTable.tsx                  ← MOD: add Reprovision action button
```

## Phase 0: Research

**Status**: Complete. See [research.md](research.md).

Key decisions:

- `HostEntry` + pure functions → `pkg/common/utils/hosts.go` (reusable, no k8s dep)
- `GetAgentHostEntries` (k8s-specific) → `vjailbreaknodeutils.go` (no new file)
- Reprovision via `vjailbreak.io/reprovision` annotation, pure helper for testability
- Vendor copies updated manually (existing repo pattern)

## Phase 1: Design

### Component 1: `pkg/common/utils/hosts.go` (new file)

**TDD sequence**: Define stubs → write `hosts_test.go` (all fail) → implement → all pass.

**Exported API** (defined as stubs first):

```go
package utils

// HostEntry maps a single IP to one or more hostnames (mirrors /etc/hosts format).
type HostEntry struct {
    IP        string   `json:"ip"`
    Hostnames []string `json:"hostnames"`
}

// ValidateHostEntry returns a descriptive error if IP or any hostname is invalid.
// Checks: non-empty IP parseable by net.ParseIP; at least one hostname;
// each hostname matches ^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$
func ValidateHostEntry(entry HostEntry) error { return nil }

// ParseHostEntries deserializes a JSON string from ConfigMap.
// Empty string or "[]" returns ([]HostEntry{}, nil).
// Invalid JSON returns a wrapped error.
func ParseHostEntries(jsonStr string) ([]HostEntry, error) { return nil, nil }

// SerializeHostEntries serializes entries to JSON for ConfigMap storage.
func SerializeHostEntries(entries []HostEntry) (string, error) { return "", nil }

// BuildUserData produces the cloud-init UserData YAML for a worker agent node.
// When entries is nil/empty, output is identical to fmt.Sprintf(K3sCloudInitScript,
// envFilePath, "false", masterIP, token) — zero behavioral change for existing nodes.
func BuildUserData(envFilePath, masterIP, token string, entries []HostEntry) string { return "" }
```

**Private helpers**:

- `buildHostsLines(entries []HostEntry) []string` — returns `[]string{"1.2.3.4 h1 h2", ...}`
- `isValidHostname(s string) bool` — regex check

**Invariant tested**: `BuildUserData(envFile, ip, token, nil)` == `fmt.Sprintf(constants.K3sCloudInitScript, envFile, "false", ip, token)`

---

### Component 2: `pkg/common/utils/hosts_test.go` (new file, written before implementation)

Standard `testing` package. Table-driven. Pattern matches `net_test.go`.

Tests written against the stub signatures above:

**`TestValidateHostEntry`** — table cases:

| name | input | wantErr |
|------|-------|---------|
| valid single hostname | `{IP:"1.2.3.4", Hostnames:["h1"]}` | false |
| valid multiple hostnames | `{IP:"::1", Hostnames:["h1","h2.local"]}` | false |
| empty IP | `{IP:"", Hostnames:["h1"]}` | true |
| invalid IP | `{IP:"999.x.y.z", Hostnames:["h1"]}` | true |
| no hostnames | `{IP:"1.2.3.4", Hostnames:[]}` | true |
| invalid hostname chars | `{IP:"1.2.3.4", Hostnames:["bad_host!"]}` | true |

**`TestParseHostEntries`** — table cases: empty string, `"[]"`, valid JSON, malformed JSON, entry with invalid IP

**`TestSerializeParseRoundTrip`** — serialize then parse, result equals input

**`TestBuildUserData`** — table cases:

| name | entries | assertion |
|------|---------|-----------|
| zero entries | nil | output == fmt.Sprintf(K3sCloudInitScript, ...) |
| zero entries (empty slice) | `[]HostEntry{}` | same as above |
| single entry | `[{1.2.3.4 [h1 h2]}]` | output contains `echo "1.2.3.4 h1 h2" >> /etc/hosts` |
| multiple entries | 3 entries | all 3 echo lines present in order |

---

### Component 3: `pkg/common/constants/constants.go` — add constant

Add one line:

```go
AgentHostEntriesKey = "AGENT_HOST_ENTRIES"
```

Update vendor copy at `k8s/migration/vendor/github.com/platform9/vjailbreak/pkg/common/constants/constants.go`.

---

### Component 4: `vjailbreaknodeutils.go` — two targeted changes

**Change A** — add `GetAgentHostEntries` (≈15 lines):

```go
// GetAgentHostEntries reads custom host entries from the vjailbreak-settings ConfigMap.
// Returns empty slice (not error) if the key is absent or empty.
func GetAgentHostEntries(ctx context.Context, k8sClient client.Client) ([]pkgutils.HostEntry, error) {
    cm := &corev1.ConfigMap{}
    if err := k8sClient.Get(ctx, types.NamespacedName{
        Name: constants.VjailbreakSettingsConfigMapName,
        Namespace: constants.NamespaceMigrationSystem,
    }, cm); err != nil {
        return nil, errors.Wrap(err, "failed to get vjailbreak-settings configmap")
    }
    raw := cm.Data[constants.AgentHostEntriesKey]
    if raw == "" {
        return []pkgutils.HostEntry{}, nil
    }
    return pkgutils.ParseHostEntries(raw)
}
```

**Change B** — replace single `fmt.Sprintf` call in `CreateOpenstackVMForWorkerNode` (line ~367):

```go
// Before:
UserData: []byte(fmt.Sprintf(constants.K3sCloudInitScript,
    constants.ENVFileLocation, "false", GetNodeInternalIP(masterNode), token)),

// After:
hostEntries, heErr := GetAgentHostEntries(ctx, k3sclient)
if heErr != nil {
    log.Error(heErr, "Failed to get agent host entries, provisioning without custom hosts")
    hostEntries = []pkgutils.HostEntry{}
}
UserData: []byte(pkgutils.BuildUserData(constants.ENVFileLocation, GetNodeInternalIP(masterNode), string(token), hostEntries)),
```

Error from `GetAgentHostEntries` is non-fatal — log and continue with zero entries. Node provisioning must not be blocked by absent/malformed settings.

**Tests for `GetAgentHostEntries`** (add to existing `vjailbreaknode_controller_test.go` or new `vjailbreaknodeutils_test.go`):

```go
// Uses controller-runtime fake client — same pattern as existing controller tests
fakeClient := fake.NewClientBuilder().WithObjects(&corev1.ConfigMap{
    ObjectMeta: metav1.ObjectMeta{Name: "vjailbreak-settings", Namespace: "migration-system"},
    Data: map[string]string{constants.AgentHostEntriesKey: `[{"ip":"1.2.3.4","hostnames":["h1"]}]`},
}).Build()
entries, err := GetAgentHostEntries(ctx, fakeClient)
// assert entries == [{IP:"1.2.3.4", Hostnames:["h1"]}], err == nil
```

Table cases: key present + valid, key absent, key present + empty string, key present + malformed JSON.

---

### Component 5: `vjailbreaknode_controller.go` — reprovision handler

**New constants** (local to controller package, no `pkg/common` change):

```go
const (
    reprovisionAnnotation = "vjailbreak.io/reprovision"
    reprovisionRequested  = "requested"
    reprovisionBlocked    = "blocked"
)
```

**Pure helper** (testable without reconciler):

```go
// reprovisionAllowed returns true only when the node has no active migrations.
func reprovisionAllowed(activeMigrations []string) bool {
    return len(activeMigrations) == 0
}
```

**Add to `reconcileNormal`** (before UUID lookup, ~line 124):

```go
if vjNode.Annotations[reprovisionAnnotation] == reprovisionRequested {
    return r.reconcileReprovision(ctx, vjailbreakNodeScope)
}
```

**New method `reconcileReprovision`**:

1. If `!reprovisionAllowed(vjNode.Status.ActiveMigrations)` → set annotation to `reprovisionBlocked`, update object, requeue after 1 min
2. Else:
   - Call `utils.DeleteOpenstackVM(ctx, vjNode.Status.OpenstackUUID, r.Client, vjNode)` (reuse from `reconcileDelete`)
   - Call `utils.DeleteNodeByName(ctx, r.Client, vjNode.Name)` (reuse from `reconcileDelete`)
   - Clear `vjNode.Status.OpenstackUUID = ""` and `vjNode.Status.Phase = ""`
   - Delete annotation from `vjNode.Annotations`
   - Update object + status
   - Return `ctrl.Result{RequeueAfter: 5 * time.Second}` → next reconcile creates new VM

**Tests** (Ginkgo, extend `vjailbreaknode_controller_test.go`):

- `reprovisionAllowed([]string{})` → true
- `reprovisionAllowed([]string{"migration-1"})` → false
- Controller test: annotation `"requested"` on node with active migrations → annotation becomes `"blocked"`
- Controller test: annotation `"requested"` on idle node → `OpenstackUUID` cleared, annotation removed

---

### Component 6: UI — `SettingsForm` extension (`helpers.ts`)

Add to `SettingsForm` type:

```typescript
AGENT_HOST_ENTRIES: string  // JSON string, default ""
```

Add to `toConfigMapData`: `AGENT_HOST_ENTRIES: f.AGENT_HOST_ENTRIES ?? ''`

Add to `fromConfigMapData`: `AGENT_HOST_ENTRIES: typeof data?.AGENT_HOST_ENTRIES === 'string' ? data.AGENT_HOST_ENTRIES : ''`

Add to `DEFAULTS` in `GlobalSettingsPage.tsx`: `AGENT_HOST_ENTRIES: ''`

---

### Component 7: UI — `HostEntriesTab.tsx` (new component)

**Props**:

```typescript
interface HostEntriesTabProps {
  value: string          // current JSON string from form state
  onChange: (v: string) => void
  disabled?: boolean
}
```

**Responsibilities**:

- Parse `value` JSON → `HostEntry[]` on mount/prop change
- Render MUI DataGrid or Table with columns: IP, Hostnames (comma-joined), Actions (Edit / Delete)
- "Add Entry" opens inline form row or Dialog with IP + hostnames fields
- Validate: valid IPv4/IPv6, ≥1 hostname, no duplicate IPs
- On add/edit/delete: serialize back to JSON → call `onChange`

**Validation** (TypeScript mirrors Go `ValidateHostEntry`):

```typescript
const isValidIP = (ip: string) => /^(\d{1,3}\.){3}\d{1,3}$|^[0-9a-fA-F:]+$/.test(ip.trim())
const isValidHostname = (h: string) => /^[a-zA-Z0-9]([a-zA-Z0-9\-.]*[a-zA-Z0-9])?$/.test(h)
```

**Tests** (`HostEntriesTab.test.tsx`, React Testing Library):

- Renders empty state with "Add Entry" button
- Parses pre-populated `value` JSON and shows rows
- Add entry with valid IP + hostname → row appears, `onChange` called with correct JSON
- Rejects invalid IP → inline error
- Rejects duplicate IP → inline error
- Delete row → row removed, `onChange` called

---

### Component 8: UI — `GlobalSettingsPage.tsx` + `NodesTable.tsx` changes

**GlobalSettingsPage**: Add "Host Entries" tab using `<LanOutlinedIcon />` (already imported). Pass `watch('AGENT_HOST_ENTRIES')` to `<HostEntriesTab value={...} onChange={v => setValue('AGENT_HOST_ENTRIES', v)} />`. Save flows through existing `updateSettingsConfigMap`.

**NodesTable**: Add "Reprovision" `IconButton` in actions column.

- Disabled when `activeMigrations.length > 0` → tooltip `"Node has active migrations"`
- Disabled when `isDeleting` or `role === 'master'`
- On click: PATCH annotation `vjailbreak.io/reprovision: "requested"` on VjailbreakNode via new `reprovisionNode(nodeName)` API helper
- Snackbar feedback (same pattern as existing delete)

**New API helper** (extend or add alongside `src/api/nodes/nodeMappings.ts`):

```typescript
export const reprovisionNode = async (nodeName: string): Promise<void> => {
  // PATCH /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/vjailbreaknodes/{name}
  // body: { metadata: { annotations: { "vjailbreak.io/reprovision": "requested" } } }
}
```

## Complexity Tracking

No constitution violations. No new CRDs, no new modules, no new ConfigMaps, no new files in controller pkg.

## Test Execution

```bash
# Go (pkg/common module — pure function tests)
cd pkg/common && go test ./utils/... -v

# Go (controller module — fake client tests)
cd k8s/migration && make test

# UI
cd ui && yarn test
```
