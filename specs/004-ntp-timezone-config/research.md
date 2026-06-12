# Research: NTP Server and Timezone Configuration

## Decision 1: Storage in Existing `vjailbreak-settings` ConfigMap

**Decision**: Store `TIMEZONE` and `NTP_SERVERS` as plain string keys in the existing `vjailbreak-settings` ConfigMap in `migration-system`.

**Rationale**: Zero new infrastructure. All controllers, the vpwned-sdk API server, and the UI already read/write this ConfigMap. New keys follow the exact same pattern as every other setting (`CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE`, `DEFAULT_MIGRATION_METHOD`, etc.).

**Format**:
- `TIMEZONE`: single IANA timezone string, e.g. `Asia/Calcutta`. Empty = use UTC.
- `NTP_SERVERS`: space-separated list of hostnames/IPv4 addresses, e.g. `ntp1.corp.local ntp2.corp.local`. Empty = use systemd-timesyncd defaults (delete override file).

---

## Decision 2: Application via vpwned-sdk, Not Controller

**Decision**: The `Apply()` function lives in `pkg/common/timesettings/` and is called from the vpwned-sdk gRPC handler (`ApplyTimeSettings` RPC), not from the Kubernetes controller.

**Rationale**: The controller runs in a container without host path mounts to `/etc/systemd/`, `/etc/pf9/`, or the D-Bus socket. The vpwned-sdk already runs with `hostNetwork: true` and has the necessary host mounts for D-Bus access (`/run/dbus/system_bus_socket`). Shared logic in `pkg/common/` is importable by any module.

**Two-step save flow**:
1. UI PUTs updated `vjailbreak-settings` ConfigMap (standard settings save)
2. UI POSTs `/vpw/v1/settings/apply-time-settings` → vpwned-sdk reads ConfigMap and applies

The ConfigMap is the source of truth; `apply-time-settings` is idempotent (re-running produces same result).

---

## Decision 3: AppArmor `unconfined` Annotation Required for D-Bus

**Decision**: Add `container.apparmor.security.beta.kubernetes.io/vpwned: unconfined` to the vpwned-sdk pod template in all three deployment YAML files (`deploy/06vpwned-deployment.yaml`, `k8s/migration/config/addons/k8s.svc.yaml`, `pkg/vpwned/deploy/k8s.svc.yaml`).

**Root cause**: containerd applies the default AppArmor profile to containers by default. This profile blocks the D-Bus `Hello` method call at the kernel level — even for root containers with `DAC_OVERRIDE` capability. The error is:
```
An AppArmor policy prevents this sender from sending this message to this recipient;
type="method_call", interface="org.freedesktop.DBus" member="Hello"
```

**Why `unconfined`**: The `unconfined` profile removes all AppArmor restrictions on the container. This is acceptable because vpwned-sdk already runs with `hostNetwork: true` and direct access to host sockets — its threat model already assumes a trusted system component.

**Alternative rejected**: Custom AppArmor profile allowing only specific D-Bus methods. Too fragile — profile names must exist on every k3s node; easier to maintain `unconfined` for a trusted system component.

---

## Decision 4: `SystemBusPrivate()` Over `SystemBus()`

**Decision**: Use `dbus.SystemBusPrivate()` (new connection per call) rather than `dbus.SystemBus()` (shared singleton) for all D-Bus calls.

**Rationale**: `SystemBus()` returns a cached global connection. If called before `Hello()` completes, or after a prior connection error, subsequent calls may fail or deadlock. `SystemBusPrivate()` creates a fresh, dedicated connection for each call — requires explicit `Auth()` + `Hello()` but is always in a known state.

---

## Decision 5: Non-Fatal Best-Effort for D-Bus and Workload Restarts

**Decision**: `Apply()` returns `("Time settings applied successfully.", nil)` regardless of whether D-Bus, workload restarts, or CronJob patches succeeded. All non-fatal errors are logged via `logrus.Warn` only.

**Rationale**:
- The ConfigMap and conf-file writes (the primary effects) always happen first. D-Bus calls are host reconciliation; they may fail if AppArmor is mis-configured, D-Bus daemon is unavailable, etc.
- Exposing raw D-Bus error strings (e.g. AppArmor message) to the UI is confusing to operators who cannot act on them.
- Operators who need to debug can check pod logs. The UI showing a generic error for every D-Bus hiccup would erode trust in the feature.

**What IS a hard error**: Failure to read the `vjailbreak-settings` ConfigMap (cannot determine what to apply) or failure to write the NTP conf file (core function of the feature). These are returned as errors to the caller.

---

## Decision 6: vpwned-sdk Must Be in `workloadsToRestart`

**Decision**: Add `migration-vpwned-sdk` to `workloadsToRestart` so it is restarted by `restartTZWorkloads` (inside `Apply()`, after `patchPf9EnvConfigMap`).

**Root cause of race condition (initial approach failed)**:
The initial implementation tried a deferred goroutine in the gRPC handler that would sleep 10s then restart vpwned-sdk. This failed because:
1. `InjectEnvVariables` (a separate call in the UI's save flow) restarts vpwned-sdk immediately when proxy env changes — before `apply-time-settings` is called.
2. The new vpwned-sdk pod (pod A) starts with `TZ=UTC` because the `pf9-env` ConfigMap hasn't been updated yet.
3. The deferred goroutine in the *old* pod intends to restart vpwned-sdk again after 10s. But Kubernetes sends SIGTERM to the old pod when pod A becomes Ready (~5s), and gRPC `GracefulStop()` exits the old pod — killing the goroutine.
4. Result: no second restart, pod A keeps `TZ=UTC`.

**Why `workloadsToRestart` works**:
- `restartTZWorkloads` runs AFTER `patchPf9EnvConfigMap` inside `Apply()` — the `pf9-env` ConfigMap is already updated with the new TZ before the rolling restart is triggered.
- The rolling restart creates pod B which reads the updated ConfigMap → `TZ=Asia/Calcutta`.
- The gRPC response is sent in milliseconds. SIGTERM to the old pod arrives only after pod B is Ready (5–10s). `GracefulStop()` ensures the response is sent before the process exits.

---

## Decision 7: `syncEnabled` Logic

**Decision**: The `syncEnabled` flag passed to D-Bus `SetNTP` is `true` when either `ntpServers != ""` OR `cleanTZ != ""`. It is `false` only when both are empty.

**Rationale**: If the user has configured a timezone but no NTP servers, NTP should still be enabled (using systemd-timesyncd's built-in defaults). Only a complete reset of both fields should disable NTP.

**Target timezone logic**:
- NTP servers set + TZ set → `targetTZ = cleanTZ`, `syncEnabled = true`
- NTP servers set + TZ empty → `targetTZ = "UTC"`, `syncEnabled = true`
- NTP servers empty + TZ set → `targetTZ = cleanTZ`, `syncEnabled = true`
- Both empty → `targetTZ = "UTC"`, `syncEnabled = false`

---

## Decision 8: Conf File Drop-in vs Full Replacement

**Decision**: Write to `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` (drop-in override) rather than modifying `/etc/systemd/timesyncd.conf` (base file).

**Rationale**: The drop-in directory is the standard systemd pattern for package/operator overrides — it survives OS updates without overwriting the base config. `99-` prefix ensures it takes highest precedence. Removing the file cleanly reverts to defaults without leaving residue in the base config.

---

## Decision 9: Timezone List in UI

**Decision**: Provide a curated list of IANA timezone identifiers in `ui/src/features/globalSettings/timezones.ts` as a static TypeScript file rather than fetching from an API.

**Rationale**: The list of valid IANA timezones is stable (added infrequently, never removed). A static file has zero runtime cost. The backend sanitizes the value anyway — the list is a UI convenience, not a security gate.
