# Requirements Checklist: NTP Server and Timezone Configuration

**Spec**: [../spec.md](../spec.md)

## Functional Requirements

| ID | Requirement | Status | Notes |
|----|-------------|--------|-------|
| FR-001 | Admin can configure timezone via searchable dropdown in Global Settings UI | ✅ Done | `RHFAutocomplete` with `timezones.ts` list |
| FR-002 | Admin can configure NTP servers via text input in Global Settings UI | ✅ Done | Space/comma separated input |
| FR-003 | TIMEZONE and NTP_SERVERS persisted in `vjailbreak-settings` ConfigMap | ✅ Done | `toConfigMapData` updated |
| FR-004 | Apply writes/removes `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` | ✅ Done | `writeTimesyncdConf()` |
| FR-005 | Apply calls D-Bus `SetTimezone` + `SetNTP` on host | ✅ Done | `notifyTimedateViaDbus()` |
| FR-006 | Apply restarts `systemd-timesyncd.service` via D-Bus when NTP servers set | ✅ Done | `restartTimesyncdViaDbus()` |
| FR-007 | Apply updates `pf9-env` ConfigMap `TZ` key | ✅ Done | `patchPf9EnvConfigMap()` |
| FR-008 | Apply updates `/etc/pf9/env` with `TZ=<timezone>` | ✅ Done | `updatePf9EnvFile()` |
| FR-009 | Rolling restart of 5 workloads after pf9-env CM update | ✅ Done | `restartTZWorkloads()` includes vpwned-sdk |
| FR-010 | Update `vjailbreak-version-checker` CronJob `spec.timeZone` | ✅ Done | `patchVersionCheckerTZ()` |
| FR-011 | Invalid NTP entries silently dropped (logged, not surfaced to UI) | ✅ Done | `FilterValidNTPServers()` |
| FR-012 | Timezone path traversal sanitization | ✅ Done | `sanitizeTimezone()` |
| FR-013 | D-Bus and workload restart failures are non-fatal; UI always shows success | ✅ Done | `Apply()` returns success on soft errors |
| FR-014 | AppArmor `unconfined` annotation on vpwned-sdk pod template | ✅ Done | All 3 YAML files patched |
| FR-015 | `applyTimeSettings` called AFTER ConfigMap save | ✅ Done | UI save flow: PUT CM → POST apply |
| FR-016 | `applyTimeSettings` called only when TZ or NTP fields changed | ✅ Done | `timeSettingsChanged` guard in UI |

## End-to-End Verification (pending deployment)

| Test | Status |
|------|--------|
| timedatectl shows correct TZ after apply | 🔲 Pending new image deploy |
| vpwned-sdk pod has correct TZ env var | 🔲 Pending new image deploy |
| controller-manager pod shows correct TZ in logs | 🔲 Pending new image deploy |
| NTP conf file written correctly | ⚠️ Partial (host-level tested in T2/T3 from prior session) |
| NTP synchronization active after apply | ⚠️ Partial |
| Clear both fields reverts to UTC + deletes conf | ⚠️ Partial |
| UI shows current values on page load | ⚠️ Partial |
