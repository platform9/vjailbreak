# Research: Cluster Conversion Redesign

**Phase 0 Output** | Spec: [2026-07-03-cluster-conversion-redesign.md](2026-07-03-cluster-conversion-redesign.md)

---

## Decision 1: Operator Action Mechanism (trigger/retry/skip)

**Decision**: Use Kubernetes metadata annotations on `ClusterConversionBatch` as the operator action channel.

**Rationale**:
- Kubernetes patches annotations atomically (merge-patch+json) — no risk of spec drift.
- UI can do a single `PATCH /clusterconversionbatches/{name}` without knowing internal CR state.
- Controller reads and clears annotations on each reconcile — self-cleaning, idempotent.
- No dedicated sub-resource endpoint needed in the API server.
- Same pattern used internally for pause/resume in the existing `RollingMigrationPlan` (label-based).

**Alternatives considered**:
- Sub-resource (e.g., `POST /clusterconversionbatches/{name}/retry`): cleaner API but requires vpwned API server changes (new proto + gRPC route), significantly more work.
- Spec mutation (add `spec.hostActions[]`): pollutes spec with imperative commands, breaks Kubernetes spec-as-desired-state semantics.

**Annotation keys**:
```
vjailbreak.k8s.pf9.io/trigger-host: "<esxiName>"
vjailbreak.k8s.pf9.io/retry-host:   "<esxiName>"
vjailbreak.k8s.pf9.io/skip-host:    "<esxiName>"
```

---

## Decision 2: Eligibility Calculation — Maximum Code Reuse

**Decision**: Reuse existing functions directly; compose them into a new `PerHostEligibilityCheck` function in a new utils file.

**Existing functions to reuse (zero changes)**:
- `EnsureESXiInMass(ctx, scope, vmwarehost)` — checks MAAS match by UUID/MAC (rollingmigrationutils.go:1016)
- `EnsurePCDHasClusterConfigured(ctx, scope)` — checks PCD cluster exists (rollingmigrationutils.go:1191)
- `isBMConfigValid(ctx, client, name)` — BMConfig validation check (rollingmigrationutils.go:1001)
- `CanEnterMaintenanceMode(ctx, scope, vmwcreds, hostName, config)` — covers DRS enabled, DRS fully automated, cluster host count, VM anti-affinity, capacity (maintenance_mode.go)

**Problem**: These functions take `*scope.RollingMigrationPlanScope`. For the new flow, we have `*scope.ClusterConversionBatchScope`.

**Solution**: Factor out the common `client.Client` and credential lookup so the eligibility functions can be called with `client.Client` directly. New wrapper function `CheckPerHostEligibility` in `clusterconversionbatchutils.go` calls them after constructing appropriate parameters. **No changes to existing functions.**

The new scope carries: `client.Client`, `*ClusterConversionBatch`, resolved `*VMwareCreds`, `*OpenstackCreds`, `*BMConfig`. This provides everything needed to call the existing helpers.

---

## Decision 3: ESXIMigration Retry Strategy

**Decision**: Retry by deleting and recreating the `ESXIMigration` CR.

**Rationale**:
- Matches existing idiom — each `ESXIMigration` runs once to completion (succeeded or failed).
- Avoids adding reset logic to the ESXIMigration controller (minimizes changes).
- The `ClusterConversionBatch` controller owns the retry loop, not ESXIMigration.
- Clean state: new ESXIMigration starts fresh from phase `""` → `Waiting`.

**Alternatives considered**:
- Add a `spec.reset: true` field to ESXIMigration that the controller reacts to: adds complexity to ESXIMigration controller and blurs ownership.
- Reset `status.phase` directly via status subresource: violates Kubernetes reconciliation semantics (controller re-sets phase on next reconcile anyway).

---

## Decision 4: Reconcile Frequency

**Decision**: Requeue every 30 seconds. No Watch on ESXIMigration changes.

**Rationale**:
- 30s is fast enough for operator visibility without hammering vCenter APIs (eligibility check hits vCenter).
- Adding a Watch on ESXIMigrations would require a second reconcile trigger path — more logic for marginal latency gain.
- Existing reconcilers use 1-minute requeue; 30s is adequate for batch conversion visibility.

**Note**: The controller should use a **Watch on ESXIMigration** to immediately detect completion (phase = Succeeded/Failed). This avoids a 30s delay in detecting success. Use `ctrl.NewControllerManagedBy(mgr).For(&ClusterConversionBatch{}).Owns(&ESXIMigration{})` so that ESXIMigration status changes trigger a reconcile of the owning batch. **Set owner reference on ESXIMigration pointing to ClusterConversionBatch.**

Actually - the spec says "never aborts a sibling". With `Owns()`, if the batch is deleted, owned ESXIMigrations get garbage collected by Kubernetes. This conflicts with the spec's "don't delete ESXIMigration on batch delete" requirement.

**Revised**: Do NOT set owner reference. Use label `vjailbreak.k8s.pf9.io/cluster-conversion-batch: <batch-name>` on the ESXIMigration and watch via label selector. Add a manual Watch on ESXIMigrations labeled to this batch.

```go
ctrl.NewControllerManagedBy(mgr).
    For(&ClusterConversionBatch{}).
    Watches(
        &ESXIMigration{},
        handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
            // map ESXIMigration label vjailbreak.k8s.pf9.io/cluster-conversion-batch → batch name
        }),
    ).
    Complete(r)
```

This gives immediate reconcile on ESXIMigration phase changes without owner reference (no GC cascade).

---

## Decision 5: ESXIMigration Changes (Minimal)

**Decision**: Add `BMConfigRef *corev1.LocalObjectReference` and `ClusterConversionBatchRef *corev1.LocalObjectReference` as optional fields to `ESXIMigrationSpec`. Make `RollingMigrationPlanRef` optional (omitempty, already works with empty struct but controller skips fetch if Name is empty).

**Controller change**: In `esximigration_controller.go`:
- Check `esxiMigration.Spec.RollingMigrationPlanRef.Name != ""` before fetching RollingMigrationPlan.
- Add `resolveBMConfig(esxiMigration, rollingMigrationPlan)` helper that returns BMConfig from `esxiMigration.Spec.BMConfigRef` first, falling back to `rollingMigrationPlan.Spec.BMConfigRef` if RMP is set.
- All other phases (InMaintenanceMode, WaitingForPCDHost, etc.) remain unchanged.

---

## Decision 6: CreateESXIMigration for New Flow

**Decision**: Write `CreateESXIMigrationForBatch` in `clusterconversionbatchutils.go` that mirrors `CreateESXIMigration` (rollingmigrationutils.go:137) but:
- Uses `ClusterConversionBatch` as the source of creds refs.
- Sets `spec.bmConfigRef` and `spec.clusterConversionBatchRef`.
- Does NOT set `spec.rollingMigrationPlanRef`.
- Labels: `vjailbreak.k8s.pf9.io/cluster-conversion-batch: <batch-name>` (no RollingMigrationPlan label).
- No owner reference (to prevent GC cascade on batch delete).

Reuses: `commonutils.GetK8sCompatibleVMWareObjectName`, `commonutils.SanitizeLabelValue` — no changes.

---

## Decision 7: Scope Design

**Decision**: New `ClusterConversionBatchScope` follows exact pattern of `RollingMigrationPlanScope`:
- Struct: `Client`, `Logger`, `*ClusterConversionBatch`.
- `Close()` calls `client.Update(ctx, batch)`.
- No resolved credentials in scope — resolving creds is done inline in the controller and utils (existing pattern).

---

## Decision 8: UI Architecture

**Decision**: Keep `RollingMigrationsTable.tsx` component unchanged (legacy data). Introduce new `BatchesTable.tsx` as primary listing. `ClusterConversionsPage.tsx` renders `BatchesTable` first, then a collapsible "Legacy" section with `RollingMigrationsTable`.

**API pattern reuse**:
- New `ui/src/api/cluster-conversion-batches/` mirrors `ui/src/api/rolling-migration-plans/` exactly (GET list, GET one, POST, DELETE, PATCH).
- New `useClusterConversionBatchesQuery` hook mirrors `useRollingMigrationPlansQuery`.
- Annotation patch uses `patchClusterConversionBatch` with `Content-Type: application/merge-patch+json`.

---

## Decision 9: Pre-flight Eligibility API

**Decision**: Pre-flight eligibility is computed by the `ClusterConversionBatch` controller; the UI reads it from `status.hosts[].eligibilityStatus` and `status.hosts[].eligibilityReason`.

For the Create Batch dialog pre-flight view **before** the batch exists: the UI creates a temporary/dry-run batch OR lists `ESXIMigration`-level eligibility. 

**Revised**: Don't create a dry-run batch. Instead, the UI calls a lightweight "check eligibility" endpoint. Since vJailbreak's API server is k8s itself, we can:
1. Create the batch immediately on "Create" button click.
2. The Create dialog becomes a two-step wizard: Step 1 select cluster/hosts/config → "Review" → Step 2 shows live eligibility from the created batch.

This is simpler and avoids a new API endpoint. The batch starts in `Pending`/`CheckingEligibility` and the UI polls it.

---

## Decision 10: Documentation Changes

**Decision**: Update `docs/` Astro site pages:
- `docs/src/content/docs/cluster-conversion/overview.mdx` — new architecture description, deprecation notice
- `docs/src/content/docs/cluster-conversion/quickstart.mdx` — new wizard walkthrough
- Keep old RollingMigrationPlan docs with "DEPRECATED" notice (don't delete — in-flight users still need them)
