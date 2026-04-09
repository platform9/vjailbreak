# Amplitude Events — Current State (UI)

## Initialization / Where Amplitude is wired

- [src/main.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/main.tsx:0:0-0:0)
  - Wraps the app with `AnalyticsProvider` (provider lives in `src/components/providers`).

- [src/hooks/useAnalytics.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/hooks/useAnalytics.ts:0:0-0:0)
  - Fetches analytics config (ConfigMap or env).
  - Calls `initializeAmplitude(...)` when API key is present.

- [src/services/amplitudeService.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/services/amplitudeService.ts:0:0-0:0)
  - `initializeAmplitude()` calls:
    - `amplitude.init(apiKey, ..., { useDynamicConfig: true, trackingOptions: ... })`
  - `trackEvent()` is the underlying function used by the app wrappers.

- [src/hooks/useAmplitude.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/hooks/useAmplitude.ts:0:0-0:0)
  - Main app-facing hook.
  - Provides `track(eventName, properties)` and enriches properties.

## Note on `Form Started`

- `Form Started` is NOT emitted explicitly from our code.
- There are no string matches of `Form Started` in the repo.
- This event is Amplitude Autocapture (SDK-generated) and can be enabled via remote/dynamic configuration (we set `useDynamicConfig: true` during init).
- The event properties in the dashboard (ex: `Form Destination`, `Page Location`, etc.) match autocapture behavior.

---

# User Journeys & Tracking Points

This section lists the most important user journeys and where Amplitude events are (and should be) called.

Legend:

- `implemented`: event is currently tracked in the code
- `gap`: recommended tracking point, not currently implemented

## 1) Credentials

### VMware credentials

| Action / Step                        | Event name                                                        | Where fired (file)                                                | When / Trigger                                                                                                                                      | Status      |
| ------------------------------------ | ----------------------------------------------------------------- | ----------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| Create VMware credentials (explicit) | `VMware Credentials Added` / `VMware Credentials Failed`          | `src/features/credentials/components/VMwareCredentialsDrawer.tsx` | Create: `...Added` (`stage: creation_start`), failures on create/validate (`stage: creation` / `validation`). No explicit validation-success event. | implemented |
| Delete VMware credential outcomes    | `VMware Credentials Deleted` / `VMware Credentials Delete Failed` | `src/features/credentials/components/CredentialsTable.tsx`        | When user deletes VMware credential(s) from the table                                                                                               | implemented |

### PCD (OpenStack) credentials

| Action / Step                     | Event name                                                  | Where fired (file)                                                   | When / Trigger                                                                                                                                        | Status      |
| --------------------------------- | ----------------------------------------------------------- | -------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| Create PCD credentials (explicit) | `PCD Credentials Added` / `PCD Credentials Failed`          | `src/features/credentials/components/OpenstackCredentialsDrawer.tsx` | Create: `...Added` (`stage: creation_success`), failures on create/validate (`stage: creation` / `validation`). No explicit validation-success event. | implemented |
| Delete PCD credential outcomes    | `PCD Credentials Deleted` / `PCD Credentials Delete Failed` | `src/features/credentials/components/CredentialsTable.tsx`           | When user deletes PCD credential(s) from the table                                                                                                    | implemented |

### Storage Array credentials

| Action / Step                                     | Event name                                                                      | Where fired (file)                                                         | When / Trigger                                                                                                                                      | Status      |
| ------------------------------------------------- | ------------------------------------------------------------------------------- | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| Create storage array credentials                  | `Storage Array Credentials Added` / `Storage Array Credentials Failed`          | `src/features/storageManagement/components/AddArrayCredentialsDrawer.tsx`  | Create: `...Added` (`stage: creation_start`), failures on create/validate (`stage: creation` / `validation`). No explicit validation-success event. | implemented |
| Update storage array credentials                  | `Storage Array Credentials Updated` / `Storage Array Credentials Update Failed` | `src/features/storageManagement/components/EditArrayCredentialsDrawer.tsx` | Update: `...Updated` (`stage: update_start`), failures on update/validate (`stage: update` / `validation`). No explicit validation-success event.   | implemented |
| Delete storage array credential outcomes (single) | `Storage Array Credentials Deleted` / `Storage Array Credentials Delete Failed` | `src/features/storageManagement/components/StorageArrayTable.tsx`          | When user deletes a single storage array credential                                                                                                 | implemented |
| Delete storage array credential outcomes (bulk)   | `Storage Array Credentials Deleted` / `Storage Array Credentials Delete Failed` | `src/features/storageManagement/components/StorageArrayTable.tsx`          | When user bulk-deletes storage array credentials                                                                                                    | implemented |

## 2) Standard Migration (Migration Plan)

| Action / Step                   | Event name                                      | Where fired (file)                                | When / Trigger                                                  | Status      |
| ------------------------------- | ----------------------------------------------- | ------------------------------------------------- | --------------------------------------------------------------- | ----------- |
| Create migration plan (per VM)  | `Migration Created`                             | `src/features/migration/MigrationForm.tsx`        | After `postMigrationPlan(...)` succeeds; one event per `vmName` | implemented |
| Create migration plan (failure) | `Migration Creation Failed`                     | `src/features/migration/MigrationForm.tsx`        | On `postMigrationPlan(...)` error                               | implemented |
| Migration execution failed      | `Migration Execution Failed`                    | `src/hooks/useMigrationStatusMonitor.ts`          | When Migration phase transitions to `Failed`                    | implemented |
| Migration succeeded             | `Migration Succeeded`                           | `src/hooks/useMigrationStatusMonitor.ts`          | When Migration phase transitions to `Succeeded`                 | implemented |
| Delete migration outcomes       | `Migration Deleted` / `Migration Delete Failed` | `src/features/migration/pages/MigrationsPage.tsx` | When user deletes migration(s) from the migrations table        | implemented |

## 3) Rolling Migration / Cluster Conversion

| Action / Step                           | Event name                            | Where fired (file)                                | When / Trigger                                                     | Status      |
| --------------------------------------- | ------------------------------------- | ------------------------------------------------- | ------------------------------------------------------------------ | ----------- |
| Submit rolling migration plan (success) | `Rolling Migration Created`           | `src/features/migration/RollingMigrationForm.tsx` | On successful submission                                           | implemented |
| Submit rolling migration plan (failure) | `Rolling Migration Submission Failed` | `src/features/migration/RollingMigrationForm.tsx` | On submission error                                                | implemented |
| Rolling migration execution failed      | `Cluster Conversion Execution Failed` | `src/hooks/useRollingMigrationsStatusMonitor.ts`  | When RollingMigrationPlan phase transitions to `Failed`            | implemented |
| Rolling migration succeeded             | `Cluster Conversion Succeeded`        | `src/hooks/useRollingMigrationsStatusMonitor.ts`  | When RollingMigrationPlan phase transitions to `Succeeded`         | implemented |
| Cluster conversion triggered            | `Cluster Conversion Triggered`        | (TBD)                                             | When conversion is initiated (if there is a distinct user trigger) | gap         |

## 4) Agents

| Action / Step                     | Event name                                       | Where fired (file)                                 | When / Trigger                                  | Status      |
| --------------------------------- | ------------------------------------------------ | -------------------------------------------------- | ----------------------------------------------- | ----------- |
| Scale up agents (start/failure)   | `Agents Scale Up`                                | `src/features/agents/components/ScaleUpDrawer.tsx` | Around `createNodes(...)` with `stage` property | implemented |
| Scale down agents (start/failure) | `Agents Scale Down` / `Agents Scale Down Failed` | `src/features/agents/components/NodesTable.tsx`    | When user confirms scale down in dialog         | implemented |

## 5) ESXi SSH Credentials

| Action / Step                 | Event name                                                            | Where fired (file)                                            | When / Trigger                                        | Status      |
| ----------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------------- | ----------------------------------------------------- | ----------- |
| Configure ESXi SSH key (add)  | `ESXi SSH Credentials Added` / `ESXi SSH Credentials Failed`          | `src/features/esxiSshKeys/components/AddEsxiSshKeyDrawer.tsx` | When user saves a new SSH private key secret          | implemented |
| Configure ESXi SSH key (edit) | `ESXi SSH Credentials Updated` / `ESXi SSH Credentials Update Failed` | `src/features/esxiSshKeys/components/AddEsxiSshKeyDrawer.tsx` | When user updates the existing SSH private key secret | implemented |

## Legend

- Where fired: file/component that calls `track(...)`
- Trigger: what user/system action causes the event
- Notes: extra context like `stage` values

# Helper wrappers used for tracking

## `useAmplitude` (preferred UI hook)

- File: [src/hooks/useAmplitude.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/hooks/useAmplitude.ts:0:0-0:0)
- Usage:
  - `const { track } = useAmplitude({ component: 'ComponentName' })`
- Behavior:
  - Enriches event properties with defaults (`component`, optional `userId`, `userEmail`)
  - Delegates to `trackEvent(...)`

## [trackApiCall](cci:1://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/api/helpers.ts:321:0-360:1) (generic API wrapper)

- File: [src/api/helpers.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/api/helpers.ts:0:0-0:0)
- Behavior:
  - Takes an `operation()` Promise
  - On success: `trackEvent(AMPLITUDE_EVENTS[successEvent], ...)`
  - On failure: `trackEvent(AMPLITUDE_EVENTS[failureEvent], ...)` with `errorMessage`
- Note: This exists, but the current usage we reviewed is mostly direct `track(...)` calls inside components/hooks.
