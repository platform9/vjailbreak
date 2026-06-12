# Feature Specification: NTP Server and Timezone Configuration

**Feature Branch**: `ntp`
**Created**: 2026-05-31
**Status**: Implemented (pending end-to-end verification)
**PR**: #1742

## Overview

This feature allows a vJailbreak administrator to configure the appliance's system timezone and NTP servers through the Global Settings UI. Changes are applied to the host OS (via D-Bus), to systemd-timesyncd (for NTP synchronization), and propagated to all system pods by updating the `pf9-env` ConfigMap and performing rolling restarts of affected workloads. This ensures that migration timestamps, cron schedules, and monitoring dashboards all use a consistent timezone.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Timezone (Priority: P1)

A vJailbreak administrator deploys the appliance in a datacenter with a non-UTC timezone. Migration logs, cron job schedules, and Grafana dashboards all show UTC times, making it difficult to correlate events with local operations. The admin navigates to Global Settings, selects the correct timezone from a searchable dropdown, saves, and applies. Within one minute all system pods are restarted and showing the correct local time.

**Why this priority**: Incorrect timezone causes timestamps in migration logs, version-checker schedules, and monitoring dashboards to be misread — this directly impacts incident response and auditability.

**Independent Test**: Can be fully tested by setting a timezone, clicking Apply, then verifying `timedatectl` on the host and `printenv TZ` inside system pods (controller-manager, vpwned-sdk, vjailbreak-ui) all show the configured timezone — without running any migration.

**Acceptance Scenarios**:

1. **Given** an administrator selects `Asia/Calcutta` in the timezone field and saves settings, **When** the system applies time settings, **Then** `timedatectl` shows `Time zone: Asia/Calcutta (IST, +0530)` on the host, and `TZ=Asia/Calcutta` is present in the `pf9-env` ConfigMap.

2. **Given** time settings have been applied, **When** the system restarts affected workloads (controller-manager, vpwned-sdk, vjailbreak-ui, grafana, prometheus), **Then** each pod's environment has `TZ=Asia/Calcutta` confirming it started after the ConfigMap update.

3. **Given** an administrator clears the timezone field and saves, **When** the system applies time settings, **Then** the host reverts to UTC and `pf9-env` ConfigMap is updated to `TZ=UTC`.

4. **Given** an administrator provides an invalid timezone string (e.g. path traversal `../../etc/passwd`), **When** they attempt to save, **Then** the system rejects the value with a validation error — no D-Bus call is made, no ConfigMap is written.

---

### User Story 2 - Configure NTP Servers (Priority: P1)

A vJailbreak administrator operates in an air-gapped datacenter with internal NTP servers that are not reachable from the internet. The system is drifting because no valid public NTP server is reachable. The admin enters the internal NTP server addresses in the NTP Servers field, saves, and applies. The appliance's systemd-timesyncd is reconfigured to use those servers and the system clock synchronizes.

**Why this priority**: Clock drift on the vJailbreak appliance causes CBT (Changed Block Tracking) timestamps to be unreliable, which can corrupt incremental migration data.

**Independent Test**: Can be fully tested by entering one or more NTP server addresses, clicking Apply, then verifying `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` exists with the correct `NTP=` line, and `timedatectl show` reports `NTPSynchronized=yes` — without running any migration.

**Acceptance Scenarios**:

1. **Given** an administrator enters `ntp1.corp.local ntp2.corp.local` in the NTP Servers field and saves, **When** the system applies, **Then** `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` contains `NTP=ntp1.corp.local ntp2.corp.local` and systemd-timesyncd has been restarted.

2. **Given** an administrator enters a mix of valid and invalid entries (e.g. `ntp1.corp.local http://bad.example.com`), **When** the system applies, **Then** only `ntp1.corp.local` is written to the conf file and a warning is logged (invalid entries silently dropped — not surfaced to UI).

3. **Given** an administrator clears the NTP Servers field and saves, **When** the system applies, **Then** `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` is deleted and systemd-timesyncd falls back to its default configuration.

4. **Given** an administrator enters comma-separated servers (`ntp1.corp.local,ntp2.corp.local`), **When** the system applies, **Then** they are correctly split and written as space-separated in the conf file.

---

### User Story 3 - Combined Timezone + NTP Configuration (Priority: P1)

An administrator sets both a custom timezone and custom NTP servers in a single save. The system applies both atomically: NTP conf is written first, then the host is updated via D-Bus (timezone + NTP=enabled), then timesyncd is restarted, then pf9-env ConfigMap is updated, then affected workloads are rolling-restarted.

**Why this priority**: Timezone and NTP are operationally coupled — a timezone without NTP sync will drift, and NTP without correct TZ still produces unreadable local logs.

**Independent Test**: Set both fields, apply, verify both `timedatectl` and the conf file reflect the settings.

**Acceptance Scenarios**:

1. **Given** both timezone (`America/New_York`) and NTP servers (`pool.ntp.org`) are set and saved, **When** applied, **Then** host shows `Time zone: America/New_York`, NTP conf file contains `NTP=pool.ntp.org`, NTP service is active, and `pf9-env` ConfigMap has `TZ=America/New_York`.

2. **Given** both fields are cleared, **When** applied, **Then** host reverts to UTC, NTP conf file is deleted, and `pf9-env` ConfigMap is updated to `TZ=UTC`.

---

### User Story 4 - View Current Time Settings (Priority: P2)

An administrator opens Global Settings and can immediately see the currently configured timezone and NTP servers — without needing to SSH into the appliance or inspect ConfigMaps with kubectl.

**Why this priority**: Without read visibility, administrators cannot audit current settings or detect drift between what was configured and what is running.

**Independent Test**: Can be fully tested by directly updating the `vjailbreak-settings` ConfigMap with known values and verifying those values appear correctly in the Global Settings UI when the page loads.

**Acceptance Scenarios**:

1. **Given** `TIMEZONE=Europe/London` and `NTP_SERVERS=ntp.corp.local` exist in `vjailbreak-settings`, **When** an administrator opens Global Settings, **Then** the timezone field shows `Europe/London` and the NTP servers field shows `ntp.corp.local`.

2. **Given** neither key exists in `vjailbreak-settings`, **When** an administrator opens Global Settings, **Then** the timezone field is empty and the NTP servers field is empty (not defaulting to UTC or any hard-coded value).

---

### Edge Cases

- What happens if the D-Bus socket is blocked by AppArmor? → D-Bus calls fail silently (logged as warning); ConfigMap and conf-file changes still apply. AppArmor `unconfined` annotation on the vpwned-sdk pod template is required for D-Bus to work.
- What happens if vpwned-sdk is mid-restart when the pf9-env ConfigMap update arrives? → Pod starts after CM is updated because `restartTZWorkloads` runs after `patchPf9EnvConfigMap` inside `Apply()` — new pod always reads the updated value.
- What happens if a workload (e.g. grafana) doesn't exist? → `IsNotFound` errors are silently skipped; other workloads still restart.
- What happens if the timezone string passes sanitization but is not a real IANA zone? → D-Bus `SetTimezone` will fail; this is a non-fatal warning in logs. UI does not surface it.
- What if the administrator enters more than one NTP server in different formats (comma, newline, space)? → All three separators are normalized to spaces before parsing.
- What happens if `/etc/pf9/env` does not exist? → It is created with `TZ=<tz>` as the only line.
- What happens if the CronJob `vjailbreak-version-checker` does not exist? → `IsNotFound` is silently skipped; no error.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow administrators to configure a system timezone by selecting from a list of valid IANA timezone identifiers in the Global Settings UI.
- **FR-002**: The system MUST allow administrators to configure one or more NTP servers by entering hostnames or IPv4 addresses in the Global Settings UI.
- **FR-003**: Timezone and NTP server values MUST be persisted in the `vjailbreak-settings` ConfigMap under keys `TIMEZONE` and `NTP_SERVERS` respectively.
- **FR-004**: When time settings are applied, the system MUST write `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` with the configured NTP servers, or remove the file if no NTP servers are configured.
- **FR-005**: When time settings are applied, the system MUST notify the host OS via D-Bus (`org.freedesktop.timedate1` SetTimezone and SetNTP) to update the running timezone and NTP-enabled state.
- **FR-006**: When NTP servers are configured, the system MUST restart `systemd-timesyncd.service` via D-Bus after writing the conf file so that it re-reads the configuration.
- **FR-007**: The system MUST update the `pf9-env` ConfigMap `TZ` key so that newly launched pods that source it receive the correct timezone.
- **FR-008**: The system MUST update `/etc/pf9/env` with `TZ=<timezone>` so that processes sourcing this file on the host receive the correct timezone.
- **FR-009**: The system MUST perform a rolling restart of the following workloads after updating `pf9-env` ConfigMap: `migration-controller-manager` (Deployment), `migration-vpwned-sdk` (Deployment), `vjailbreak-ui` (Deployment), `grafana` (Deployment), `prometheus-k8s` (StatefulSet).
- **FR-010**: The system MUST update the `spec.timeZone` field of the `vjailbreak-version-checker` CronJob to ensure its schedule fires in the correct timezone.
- **FR-011**: NTP server entries MUST be validated before use: entries containing `://`, `/`, or that fail hostname/IPv4 syntax checks MUST be silently dropped with a warning log — never written to the conf file.
- **FR-012**: Timezone values MUST be sanitized to prevent path traversal: strings containing `..`, leading `/`, or null bytes MUST be rejected.
- **FR-013**: D-Bus and workload restart failures MUST NOT prevent the primary ConfigMap and conf-file changes from being applied. All such errors are best-effort and logged as warnings only — the UI always shows "Time settings applied successfully."
- **FR-014**: The vpwned-sdk pod container MUST have the `container.apparmor.security.beta.kubernetes.io/vpwned: unconfined` annotation in its pod template so that the D-Bus calls from within the container can reach the host system bus.
- **FR-015**: Time settings MUST be applied by calling the `ApplyTimeSettings` gRPC endpoint on the vpwned-sdk after the `vjailbreak-settings` ConfigMap has been saved — not before.
- **FR-016**: The UI MUST call `applyTimeSettings` only when the timezone or NTP servers fields have changed in the current save operation.

### Key Entities

- **TimeSetting**: The pair of `TIMEZONE` (IANA string, e.g. `Asia/Calcutta`) and `NTP_SERVERS` (space-separated hostnames/IPs) stored in `vjailbreak-settings` ConfigMap.
- **pf9-env ConfigMap**: Kubernetes ConfigMap in `migration-system` namespace; `envFrom` source for system pods; `TZ` key propagated to all pods that mount it.
- **timesyncd override**: `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` on the host; contains `[Time]\nNTP=<servers>`. Absent = default systemd-timesyncd behavior.
- **WorkloadRef**: A Deployment or StatefulSet that consumes `TZ` via `pf9-env` and must be rolling-restarted when timezone changes.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After applying a timezone change, `timedatectl` on the host shows the correct timezone within 30 seconds.
- **SC-002**: After applying NTP server changes, `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` contains exactly the valid NTP servers entered, and `timedatectl show` reports `NTPSynchronized=yes` within 60 seconds of apply.
- **SC-003**: After applying a timezone change, all five workloads listed in FR-009 have `TZ=<configured-timezone>` in their environment within 2 minutes (time for rolling restart to complete).
- **SC-004**: Invalid NTP server entries (URLs, entries with path components, invalid hostnames) are dropped in 100% of cases before being written to the conf file.
- **SC-005**: Timezone path traversal attempts (`../../etc/passwd`, `/etc/shadow`) are rejected in 100% of cases at input sanitization — no D-Bus call or file write occurs.
- **SC-006**: The UI shows "Time settings applied successfully." for every successful apply, regardless of whether any D-Bus or workload-restart best-effort steps had non-fatal errors.
- **SC-007**: When both timezone and NTP fields are cleared and applied, the system reverts to UTC and NTP defaults — `timedatectl` shows UTC, conf file is deleted.

---

## Assumptions

- The vpwned-sdk pod runs with `hostNetwork: true` and the D-Bus socket is available via the host's socket path. The AppArmor `unconfined` annotation is required to allow D-Bus `Hello` calls from within the container.
- The `/usr/share/zoneinfo` directory is present on the host. Timezone sanitization validates that the provided string would resolve inside this directory — it does not check if the zone actually exists on disk.
- `/etc/pf9/env` is optional: the file is created if absent, updated if present.
- The `pf9-env` ConfigMap already exists in `migration-system`; if absent, the ConfigMap update step returns silently (no-op).
- Rolling restarts are performed by patching the `kubectl.kubernetes.io/restartedAt` annotation on pod templates. The order guarantee: `patchPf9EnvConfigMap` runs before `restartTZWorkloads` — new pods always start after the ConfigMap is updated.
- The vpwned-sdk is the component that calls D-Bus and writes host files because it runs with `hostNetwork: true` and the required host path mounts; the controller does not have these mounts.
- The gRPC response is sent before the vpwned-sdk pod receives SIGTERM from its own rolling restart (gRPC `GracefulStop` guarantees in-flight RPCs finish; SIGTERM is sent only after the new pod is Ready, which takes 5–10 seconds).
- Only cold migration-style coordination is required — there is no per-migration TZ dependency (TZ only affects log timestamps and scheduling, not migration data integrity).
