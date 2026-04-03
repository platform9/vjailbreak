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

# Tracked Events (custom / explicit in code)

## Legend

- Where fired: file/component that calls `track(...)`
- Trigger: what user/system action causes the event
- Notes: extra context like `stage` values

## Events table

| Event Name (Amplitude)                                        | Where fired                                                                                                                                                                                                     | Trigger (action)                                                       | Properties / Notes                                                                                                                               |
| ------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `Credentials Added`                                           | [src/features/credentials/components/VMwareCredentialsDrawer.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/credentials/components/VMwareCredentialsDrawer.tsx:0:0-0:0)       | User submits “Add VMware Credentials” form (start + success)           | Uses `stage`: `creation_start`, `creation_success`, `validation_success`. Also sends `credentialType: 'vmware'`, `credentialName`, `vcenterHost` |
| `Credentials Failed`                                          | [src/features/credentials/components/VMwareCredentialsDrawer.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/credentials/components/VMwareCredentialsDrawer.tsx:0:0-0:0)       | Credential creation fails OR validation fails                          | Uses `stage`: `creation`, `validation`. Includes `errorMessage`                                                                                  |
| `Credentials Added`                                           | [src/features/credentials/components/OpenstackCredentialsDrawer.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/credentials/components/OpenstackCredentialsDrawer.tsx:0:0-0:0) | User submits “Add PCD Credentials” form (success) + validation success | Uses `stage`: `validation_success` (on validation success). Includes `credentialType: 'openstack'`, `credentialName`, `isPcd`, `namespace`       |
| `Credentials Failed`                                          | [src/features/credentials/components/OpenstackCredentialsDrawer.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/credentials/components/OpenstackCredentialsDrawer.tsx:0:0-0:0) | OpenStack cred creation fails OR validation fails                      | Uses `stage`: `creation`, `validation`. Includes `errorMessage`                                                                                  |
| `Migration Created`                                           | [src/features/migration/MigrationForm.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/migration/MigrationForm.tsx:0:0-0:0)                                                     | Migration plan POST succeeds                                           | Includes `migrationName`, `migrationTemplateName`, `virtualMachineCount`, etc.                                                                   |
| `Migration Creation Failed`                                   | [src/features/migration/MigrationForm.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/migration/MigrationForm.tsx:0:0-0:0)                                                     | Migration plan POST fails                                              | Includes `migrationTemplateName`, `virtualMachineCount`, `migrationType`, `errorMessage`                                                         |
| `Migration Execution Failed`                                  | [src/hooks/useMigrationStatusMonitor.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/hooks/useMigrationStatusMonitor.ts:0:0-0:0)                                                         | Migration status transitions to `Failed`                               | Fires once per migration per phase transition. Includes `migrationName`, `vmName`, phases, `errorMessage`, reason, time, namespace               |
| `Migration Succeeded`                                         | [src/hooks/useMigrationStatusMonitor.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/hooks/useMigrationStatusMonitor.ts:0:0-0:0)                                                         | Migration status transitions to `Succeeded`                            | Fires once per migration success transition. Includes `migrationName`, phases, `vmName`, namespace                                               |
| `Rolling Migration Created`                                   | [src/features/migration/RollingMigrationForm.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/migration/RollingMigrationForm.tsx:0:0-0:0)                                       | Rolling migration plan submission succeeds                             | Includes `clusterMigrationName`, `sourceCluster`, `destinationCluster`, etc.                                                                     |
| `Rolling Migration Submission Failed`                         | [src/features/migration/RollingMigrationForm.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/migration/RollingMigrationForm.tsx:0:0-0:0)                                       | Rolling migration submission fails                                     | Includes `clusterMigrationName`, clusters, selected VMs context, `errorMessage`                                                                  |
| `Cluster Conversion Execution Failed`                         | [src/hooks/useRollingMigrationsStatusMonitor.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/hooks/useRollingMigrationsStatusMonitor.ts:0:0-0:0)                                         | Rolling migration plan transitions to `Failed`                         | Includes `rollingMigrationPlanName`, clusterName, phases, counts, namespace, strategy, `errorMessage`                                            |
| `Cluster Conversion Succeeded`                                | [src/hooks/useRollingMigrationsStatusMonitor.ts](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/hooks/useRollingMigrationsStatusMonitor.ts:0:0-0:0)                                         | Rolling migration plan transitions to `Succeeded`                      | Includes `rollingMigrationPlanName`, clusterName, counts, namespace, strategy                                                                    |
| `Agents Scale Up` (string literal, not in `AMPLITUDE_EVENTS`) | [src/features/agents/components/ScaleUpDrawer.tsx](cci:7://file:///home/abhijeet/Projects/Platform9/vjailbreak/ui/src/features/agents/components/ScaleUpDrawer.tsx:0:0-0:0)                                     | User scales up agents: start/success/failure                           | Uses `stage`: `start`, `success`, `failure`. Includes `nodeCount`, `flavorId`, `credentialName`, optional `errorMessage`                         |

---

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
