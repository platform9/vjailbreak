# Data Model: NTP Server and Timezone Configuration

## ConfigMap Storage

**ConfigMap**: `vjailbreak-settings` in namespace `migration-system` (existing)

**Keys added**:

| Key | Type | Example | Empty means |
|-----|------|---------|-------------|
| `TIMEZONE` | string | `Asia/Calcutta` | Use UTC |
| `NTP_SERVERS` | string | `ntp1.corp.local ntp2.corp.local` | Use systemd-timesyncd defaults |

**Serialization**: Both keys are plain strings. `NTP_SERVERS` is space-separated; comma and newline separators are normalized to spaces during `FilterValidNTPServers()` before any write. Absent key = empty string = same behavior as explicit empty string.

**Example ConfigMap**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vjailbreak-settings
  namespace: migration-system
data:
  TIMEZONE: "Asia/Calcutta"
  NTP_SERVERS: "ntp1.corp.local ntp2.corp.local"
  # ... other keys unchanged
```

---

## Host Filesystem Effects

### `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf`

Written when `NTP_SERVERS` is non-empty. Deleted when `NTP_SERVERS` is empty.

**Format**:
```ini
[Time]
NTP=ntp1.corp.local ntp2.corp.local
```

### `/etc/pf9/env`

Updated in-place (or created). The `TZ=` line is replaced or appended.

**Format** (relevant portion):
```bash
TZ=Asia/Calcutta
```

---

## pf9-env ConfigMap

**ConfigMap**: `pf9-env` in namespace `migration-system`

**Key updated**: `TZ` — set to configured timezone or `UTC` if none configured.

All pods with `envFrom: configMapRef: name: pf9-env` pick up the updated `TZ` on their next start. Rolling restarts of `workloadsToRestart` trigger the start of new pods after the ConfigMap update.

**Example**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: pf9-env
  namespace: migration-system
data:
  TZ: "Asia/Calcutta"
  # ... proxy env vars etc.
```

---

## CronJob Patch

**CronJob**: `vjailbreak-version-checker` in namespace `migration-system`

**Field patched**: `spec.timeZone` — set to configured timezone or `UTC` if none configured.

```yaml
spec:
  timeZone: "Asia/Calcutta"
  schedule: "0 2 * * *"  # fires at 02:00 IST
```

---

## gRPC API

**Proto file**: `pkg/vpwned/sdk/proto/v1/api.proto`

**Service**: `VjailbreakService`

**RPC**:
```protobuf
rpc ApplyTimeSettings(ApplyTimeSettingsRequest) returns (ApplyTimeSettingsResponse) {
  option (google.api.http) = {
    post: "/vpw/v1/settings/apply-time-settings"
    body: "*"
  };
}

message ApplyTimeSettingsRequest {}

message ApplyTimeSettingsResponse {
  string message = 1;
}
```

**HTTP mapping**: `POST /dev-api/sdk/vpw/v1/settings/apply-time-settings`

---

## UI Types

### `VjailbreakSettings` (model.ts)

```typescript
data: {
  TIMEZONE?: string      // e.g. "Asia/Calcutta", absent = not configured
  NTP_SERVERS?: string   // e.g. "ntp1.corp.local ntp2.corp.local"
  // ... other fields
}
```

### `SettingsForm` (helpers.ts)

```typescript
TIMEZONE: string      // empty string = not configured
NTP_SERVERS: string   // empty string = not configured
```

### Save flow

```
UI change → TIMEZONE/NTP_SERVERS in SettingsForm
         → toConfigMapData() → PUT /api/v1/namespaces/migration-system/configmaps/vjailbreak-settings
         → (if TIMEZONE or NTP_SERVERS changed) POST /dev-api/sdk/vpw/v1/settings/apply-time-settings
```

---

## `workloadsToRestart` — Workloads Receiving Rolling Restart

```go
var workloadsToRestart = []WorkloadRef{
    {WorkloadDeployment, "migration-controller-manager", "migration-system"},
    {WorkloadDeployment, "migration-vpwned-sdk",         "migration-system"},
    {WorkloadDeployment, "vjailbreak-ui",                "migration-system"},
    {WorkloadDeployment, "grafana",                      "monitoring"},
    {WorkloadStatefulSet, "prometheus-k8s",              "monitoring"},
}
```

Restart mechanism: patch `kubectl.kubernetes.io/restartedAt` annotation on pod template with current RFC3339 timestamp → triggers Kubernetes rolling update → new pod starts with updated `pf9-env` ConfigMap values.

---

## AppArmor Pod Annotation

Required on the vpwned-sdk pod template (not namespace or node level):

```yaml
metadata:
  annotations:
    container.apparmor.security.beta.kubernetes.io/vpwned: unconfined
```

Present in:
- `deploy/06vpwned-deployment.yaml`
- `k8s/migration/config/addons/k8s.svc.yaml`
- `pkg/vpwned/deploy/k8s.svc.yaml`

---

## Go Package Structure

```
pkg/common/
└── timesettings/
    ├── timesettings.go       # IsValidNTPServer, FilterValidNTPServers,
    │                         # writeTimesyncdConf, sanitizeTimezone,
    │                         # notifyTimedateViaDbus, restartTimesyncdViaDbus,
    │                         # updatePf9EnvFile, patchPf9EnvConfigMap,
    │                         # restartTZWorkloads, patchVersionCheckerTZ, Apply()
    └── timesettings_test.go  # TestIsValidNTPServer, TestFilterValidNTPServers,
                              # TestSanitizeTimezone, TestWriteTimesyncdConf,
                              # TestUpdatePf9EnvFile

pkg/vpwned/
└── server/
    └── vjailbreak_proxy.go   # ApplyTimeSettings gRPC handler
```
