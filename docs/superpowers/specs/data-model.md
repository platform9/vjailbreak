# Data Model: Cluster Conversion Redesign

## Entity Map

```
ClusterConversionBatch
  ├── spec.hosts[]          → HostEntry (ESXiName)
  ├── spec.vmwareCredsRef   → VMwareCreds (existing)
  ├── spec.openstackCredsRef→ OpenstackCreds (existing)
  ├── spec.bmConfigRef      → BMConfig (existing)
  ├── status.hosts[]        → HostConversionStatus
  └── (no owner reference to anything)

HostConversionStatus
  ├── esxiMigrationRef     → ESXIMigration (optional, set when conversion started)
  └── (embedded in ClusterConversionBatch.Status.Hosts[])

ESXIMigration (modified)
  ├── spec.bmConfigRef              → BMConfig (new, optional)
  ├── spec.clusterConversionBatchRef→ ClusterConversionBatch (new, optional)
  ├── spec.rollingMigrationPlanRef  → RollingMigrationPlan (existing, now optional)
  └── label: vjailbreak.k8s.pf9.io/cluster-conversion-batch: <batch-name>
```

## State Machine: HostConversionPhase

```
                  ┌─────────────────────────────────────────┐
                  │                                         │
    [batch created]│                                         │
         ↓        │                                         │
  CheckingEligibility                                        │
         ↓ eligible                                          │
       Ready ←──────────────── [eligibility re-checked, now eligible]
         │ (Auto mode: auto-start)                           │
         │ (Manual mode: await trigger annotation)           │
         ↓                                                   │
     Converting ──── ESXIMigration.Phase==Failed ──→ Failed │
         │                                             │     │
         │ ESXIMigration.Phase==Succeeded              │ retry backoff + RetryCount <= MaxRetries
         ↓                                             │     │
     Succeeded                                         ↓     │
                                               NeedsAttention│
                                                   │         │
                                          [retry annotation]─┘
                                          [skip annotation]
                                                   ↓
                                                Skipped
```

**Transitions**:
| From | To | Trigger |
|------|-----|---------|
| (init) | CheckingEligibility | Batch created; host entry initialized |
| CheckingEligibility | NotReady | Eligibility check fails |
| CheckingEligibility | Ready | Eligibility check passes |
| NotReady | Ready | Re-check passes (continuous) |
| Ready | Converting | AutoStart=Auto OR trigger annotation received |
| Converting | Succeeded | ESXIMigration.Status.Phase == Succeeded |
| Converting | Failed | ESXIMigration.Status.Phase == Failed AND RetryCount < MaxRetries |
| Failed | Converting | RetryCount < MaxRetries AND NextRetryAt elapsed |
| Failed | NeedsAttention | RetryCount >= MaxRetries |
| NeedsAttention | CheckingEligibility | retry annotation received |
| Any (except Succeeded) | Skipped | skip annotation received |

## State Machine: ClusterConversionBatchPhase

```
Pending → Running → Succeeded
                 ↘ PartialFail  (some Succeeded, some not)
                 ↘ Failed        (none Succeeded, all Failed/Skipped)
```

**Derivation rules** (evaluated after each reconcile):
1. If any host is Converting or in retry backoff window → `Running`
2. If all hosts are terminal (Succeeded/NeedsAttention/Skipped) AND ≥1 Succeeded → `Succeeded` (or `PartialFail` if some not succeeded)
3. If all hosts are terminal AND 0 Succeeded → `Failed`
4. Otherwise → `Pending`

## ESXIMigration Label Schema

New labels added to ESXIMigration created by ClusterConversionBatch controller:
```
vjailbreak.k8s.pf9.io/cluster-conversion-batch: <batch-name>
vjailbreak.k8s.pf9.io/esxi-name:                <esxi-k8s-name>       (existing)
vjailbreak.k8s.pf9.io/vmwarecreds:              <vmwarecreds-name>    (existing)
```

Note: `vjailbreak.k8s.pf9.io/rollingmigrationplan` label is NOT set (only set by old flow).

## Key Validation Rules

- `ClusterConversionBatch.Spec.Hosts` must have ≥1 entry.
- `ClusterConversionBatch.Spec.MaxRetries` ≥ 0.
- `ClusterConversionBatch.Spec.RetryBackoffSeconds` ≥ 30.
- `ESXIMigration.Spec.BMConfigRef` is required when `RollingMigrationPlanRef.Name == ""`.
- `HostConversionStatus.ESXiName` must match an entry in `Spec.Hosts`.

## Backoff Formula

```
backoff(retryCount) = RetryBackoffSeconds * 2^(retryCount-1) seconds

Example with default RetryBackoffSeconds=60:
  Attempt 1: wait 60s
  Attempt 2: wait 120s
  Attempt 3: wait 240s
  → MaxRetries=3 exhausted → NeedsAttention
```

## UI TypeScript Interfaces

### ClusterConversionBatch

```typescript
export interface ClusterConversionBatch {
  apiVersion: string
  kind: string
  metadata: ItemMetadata  // reuse from rolling-migration-plans/model.ts
  spec: ClusterConversionBatchSpec
  status?: ClusterConversionBatchStatus
}

export interface ClusterConversionBatchSpec {
  vmwareClusterName: string
  vmwareCredsRef: NameReference
  openstackCredsRef: NameReference
  bmConfigRef: NameReference
  cloudInitConfigRef?: { name: string; namespace: string }
  hosts: HostEntry[]
  autoStart: 'Auto' | 'Manual'
  maxRetries?: number
  retryBackoffSeconds?: number
}

export interface HostEntry {
  esxiName: string
}

export interface ClusterConversionBatchStatus {
  phase?: ClusterConversionBatchPhase
  hosts?: HostConversionStatus[]
  totalHosts?: number
  succeededHosts?: number
  needsAttentionHosts?: number
  skippedHosts?: number
  runningHosts?: number
  pendingHosts?: number
  startedAt?: string
  completedAt?: string
  message?: string
}

export type ClusterConversionBatchPhase =
  | 'Pending'
  | 'Running'
  | 'Succeeded'
  | 'PartialFail'
  | 'Failed'

export interface HostConversionStatus {
  esxiName: string
  phase: HostConversionPhase
  eligibilityStatus?: 'Ready' | 'NotReady' | 'Unknown'
  eligibilityReason?: string
  retryCount?: number
  nextRetryAt?: string
  esxiMigrationRef?: NameReference
  message?: string
  startedAt?: string
  completedAt?: string
  skippedAt?: string
}

export type HostConversionPhase =
  | 'CheckingEligibility'
  | 'NotReady'
  | 'Ready'
  | 'Converting'
  | 'Succeeded'
  | 'Failed'
  | 'NeedsAttention'
  | 'Skipped'
```
