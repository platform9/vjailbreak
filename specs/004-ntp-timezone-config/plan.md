# Implementation Plan: NTP Server and Timezone Configuration

**Branch**: `ntp` | **Date**: 2026-05-31 | **Spec**: [spec.md](spec.md)

## Summary

Allow administrators to configure the appliance timezone and NTP servers via the Global Settings UI. Settings persist in `vjailbreak-settings` ConfigMap. Application is triggered by calling the `ApplyTimeSettings` gRPC endpoint on vpwned-sdk, which writes systemd-timesyncd config, calls D-Bus to update the host, updates `pf9-env` ConfigMap, and rolling-restarts affected workloads.

See [research.md](research.md) for all design decisions. See [data-model.md](data-model.md) for types and effects.

## Technical Context

**Language/Version**: Go 1.21+ (pkg/common, pkg/vpwned), TypeScript 5.x (UI)

**Primary Dependencies**: godbus/dbus v5, controller-runtime (k8s client), React + MUI, react-hook-form, grpc-gateway

**Storage**: `vjailbreak-settings` ConfigMap (existing) — new keys `TIMEZONE`, `NTP_SERVERS`

**Testing**:
- `pkg/common/timesettings/`: standard `testing` package, table-driven
- `pkg/vpwned/`: go test with fake k8s client
- UI: React Testing Library

**Target Platform**: k3s cluster running on Linux appliance VM; vpwned-sdk needs D-Bus socket access to host

**Project Type**: Kubernetes operator + shared library + gRPC API server + web frontend

**Constraints**:
- No new CRDs
- D-Bus calls require AppArmor `unconfined` annotation on vpwned-sdk pod
- vpwned-sdk must be in `workloadsToRestart` (after pf9-env CM update) to avoid TZ=UTC race
- Soft errors (D-Bus, workload restarts) MUST NOT fail the apply response

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Kubernetes-Native | ✅ PASS | State in existing ConfigMap; workload restarts via annotation patch |
| II. External Docs First | ✅ PASS | systemd-timesyncd conf format, D-Bus org.freedesktop.timedate1 API, AppArmor container annotation docs consulted |
| III. Generated Code Protection | ✅ PASS | Proto generated files regenerated via `make`; no hand-edits |
| IV. Test-First | ✅ PASS | `timesettings_test.go` covers all pure functions; table-driven |
| V. Module Independence | ✅ PASS | `pkg/common/timesettings` has no k8s dep except via `Apply()` which accepts `client.Client`; `pkg/vpwned` imports it as module dep |
| VI. AI-Assisted Development | ✅ PASS | Skills invoked throughout |
| VII. Code Reuse and Simplicity | ✅ PASS | Reuses existing `vjailbreak-settings` CM read pattern; reuses rolling-restart annotation pattern |

## Project Structure

### Documentation

```text
specs/004-ntp-timezone-config/
├── spec.md         ← this feature's spec
├── plan.md         ← this file
├── research.md     ← design decisions
├── data-model.md   ← types, CM format, proto API
├── tasks.md        ← task list
└── checklists/
    └── requirements.md
```

### Source Code Changes

```text
pkg/common/
└── timesettings/
    ├── timesettings.go       ← NEW package: Apply(), pure helpers, D-Bus wrappers
    └── timesettings_test.go  ← NEW: table-driven tests for all pure functions

pkg/vpwned/
├── sdk/proto/v1/api.proto    ← MOD: add ApplyTimeSettings RPC + messages
├── api/proto/v1/service/     ← GEN: regenerated pb.go, pb.gw.go, grpc.pb.go
├── server/
│   └── vjailbreak_proxy.go   ← MOD: add ApplyTimeSettings handler
└── deploy/k8s.svc.yaml       ← MOD: AppArmor annotation on pod template

deploy/
├── 06vpwned-deployment.yaml  ← MOD: AppArmor annotation on pod template
└── installer.yaml            ← GEN: regenerated via make generate-manifests

k8s/migration/config/addons/
└── k8s.svc.yaml              ← MOD: AppArmor annotation on pod template

image_builder/
├── configs/vjailbreak-settings.yaml  ← MOD: add TIMEZONE, NTP_SERVERS keys
└── scripts/install.sh                ← MOD: NTP server validation

ui/src/
├── api/settings/
│   ├── model.ts              ← MOD: TIMEZONE?, NTP_SERVERS? in VjailbreakSettings
│   └── settings.ts           ← MOD: applyTimeSettings() → Promise<void>
└── features/globalSettings/
    ├── helpers.ts             ← MOD: TIMEZONE, NTP_SERVERS in SettingsForm + helpers
    ├── timezones.ts           ← NEW: static IANA timezone list
    ├── validators.ts          ← NEW: NTP server validation for UI
    └── components/
        └── GlobalSettingsPage.tsx  ← MOD: timezone autocomplete + NTP servers field
```

## Phase 0: Research

**Status**: Complete. See [research.md](research.md).

Key decisions:
- Store in existing `vjailbreak-settings` CM (no new infrastructure)
- Apply via vpwned-sdk (has D-Bus + host mounts; controller does not)
- AppArmor `unconfined` required for D-Bus from container
- `SystemBusPrivate()` for fresh connection per call
- All D-Bus/restart errors are non-fatal; UI always shows success
- vpwned-sdk in `workloadsToRestart` to avoid TZ=UTC race

---

## Phase 1: Shared Package (`pkg/common/timesettings/`)

**Status**: Complete.

### Component 1: `timesettings.go`

**Pure/testable functions**:

```go
func IsValidNTPServer(s string) bool
func FilterValidNTPServers(raw string) string
func sanitizeTimezone(tz string) (string, error)
```

**Host-effect functions** (best-effort, logged):
```go
func writeTimesyncdConf(servers string) error
func updatePf9EnvFile(tz string) error
func notifyTimedateViaDbus(tz string, ntpEnabled bool) error
func restartTimesyncdViaDbus() error
```

**Kubernetes-effect functions** (best-effort, logged):
```go
func patchPf9EnvConfigMap(ctx context.Context, k8sClient client.Client, tz string) error
func restartTZWorkloads(ctx context.Context, k8sClient client.Client) error
func patchVersionCheckerTZ(ctx context.Context, k8sClient client.Client, tz string) error
```

**Orchestrator** (hard error on CM read + conf write; soft error on rest):
```go
func Apply(ctx context.Context, k8sClient client.Client) (string, error)
```

### Component 2: `timesettings_test.go`

Table-driven tests for all pure functions:
- `TestIsValidNTPServer`: IPv4 valid/invalid, hostname valid/invalid, empty, URL-form, path-form
- `TestFilterValidNTPServers`: comma/newline/space separated, mixed valid/invalid, empty
- `TestSanitizeTimezone_*`: traversal rejected, valid zones, empty, leading slash
- `TestWriteTimesyncdConf`: creates dir + file, removes file on empty
- `TestUpdatePf9EnvFile`: creates file, replaces existing TZ=, appends when absent, empty→UTC

---

## Phase 2: gRPC API (`pkg/vpwned/`)

**Status**: Complete.

### Component 3: Proto changes

Added to `api.proto`:
```protobuf
rpc ApplyTimeSettings(ApplyTimeSettingsRequest) returns (ApplyTimeSettingsResponse) {
  option (google.api.http) = { post: "/vpw/v1/settings/apply-time-settings" body: "*" };
}
message ApplyTimeSettingsRequest {}
message ApplyTimeSettingsResponse { string message = 1; }
```

Generated files updated: `api.pb.go`, `api.pb.gw.go`, `api_grpc.pb.go`

### Component 4: gRPC handler (`vjailbreak_proxy.go`)

```go
func (p *vjailbreakProxy) ApplyTimeSettings(ctx context.Context, _ *api.ApplyTimeSettingsRequest) (*api.ApplyTimeSettingsResponse, error) {
    msg, err := timesettings.Apply(ctx, p.K8sClient)
    if err != nil {
        return nil, err
    }
    return &api.ApplyTimeSettingsResponse{Message: msg}, nil
}
```

No deferred goroutine. No background restart of self. `Apply()` handles all side effects synchronously.

---

## Phase 3: AppArmor Annotation

**Status**: Complete.

All three deployment YAML files patched:
```yaml
template:
  metadata:
    annotations:
      container.apparmor.security.beta.kubernetes.io/vpwned: unconfined
```

Files: `deploy/06vpwned-deployment.yaml`, `k8s/migration/config/addons/k8s.svc.yaml`, `pkg/vpwned/deploy/k8s.svc.yaml`

---

## Phase 4: UI

**Status**: Complete.

### Component 5: `model.ts` — `VjailbreakSettings`

Added `TIMEZONE?: string` and `NTP_SERVERS?: string` to `data` shape.

### Component 6: `settings.ts` — `applyTimeSettings()`

```typescript
export const applyTimeSettings = async (): Promise<void> => {
  await post<{ message?: string }>({
    endpoint: '/dev-api/sdk/vpw/v1/settings/apply-time-settings',
    data: {},
    config: { mock: false }
  })
}
```

Returns `void` — UI does not parse message content.

### Component 7: `helpers.ts` — `SettingsForm` + helpers

Added `TIMEZONE: string` and `NTP_SERVERS: string` to `SettingsForm`.
`toConfigMapData` and `fromConfigMapData` updated to include both fields.

### Component 8: `timezones.ts` — static timezone list

Static array of IANA timezone strings. Used by the autocomplete widget.

### Component 9: `validators.ts` — UI-side NTP validation

Client-side validation matching backend `IsValidNTPServer` behavior (no URL schemes, no path components, valid hostname/IPv4 syntax).

### Component 10: `GlobalSettingsPage.tsx` — form changes

- Timezone: `RHFAutocomplete` with `timezones` list
- NTP Servers: text input (comma or space separated, normalized on save)
- Save flow: calls `applyTimeSettings()` only when `timeSettingsChanged`
- Always shows `"Time settings applied successfully."` on success (never surfaces D-Bus errors)

---

## Phase 5: End-to-End Verification

**Status**: Pending.

### Verification Matrix

| Test | Command | Expected |
|------|---------|----------|
| Host TZ after apply | `timedatectl` | Shows configured zone |
| vpwned-sdk TZ | `kubectl -n migration-system exec deploy/migration-vpwned-sdk -- cat /proc/1/environ \| tr '\0' '\n' \| grep TZ` | `TZ=<configured>` |
| controller-manager TZ | `kubectl -n migration-system exec deploy/migration-controller-manager -- cat /proc/1/environ \| tr '\0' '\n' \| grep TZ` | `TZ=<configured>` |
| NTP conf file | `cat /etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` | `NTP=<servers>` |
| NTP sync | `timedatectl show \| grep NTPSynchronized` | `NTPSynchronized=yes` |
| pf9-env CM | `kubectl -n migration-system get cm pf9-env -o jsonpath='{.data.TZ}'` | `<configured>` |
| CronJob TZ | `kubectl -n migration-system get cronjob vjailbreak-version-checker -o jsonpath='{.spec.timeZone}'` | `<configured>` |
| Clear both fields | All above | UTC / conf file deleted |

### Build + Deploy

```bash
# Build new vpwned image with all fixes
make build-vpwned
# Tag and push
# On test VM: kubectl -n migration-system set image deployment/migration-vpwned-sdk vpwned=<new-image>
# Or: make deploy (per project deploy tooling)
```

---

## Test Execution

```bash
# pkg/common timesettings tests
cd pkg/common && go test ./timesettings/... -v

# pkg/vpwned build check
cd pkg/vpwned && go build ./...

# UI build check
cd ui && yarn build
```
