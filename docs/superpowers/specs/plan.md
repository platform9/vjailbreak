# Implementation Plan: Cluster Conversion Redesign

**Branch**: `main` | **Date**: 2026-07-06 | **Spec**: [2026-07-03-cluster-conversion-redesign.md](2026-07-03-cluster-conversion-redesign.md)

---

## Summary

Redesign the ESXi→PCD host conversion feature by introducing `ClusterConversionBatch` (passive grouper CRD), making `ESXIMigration` autonomous (optional parent ref), and adding dynamic per-host eligibility with retry/skip. The existing `RollingMigrationPlan`/`ClusterMigration` stack runs unchanged for in-flight resources. Maximum reuse of existing controller patterns, scope objects, utils, and UI component library.

---

## Technical Context

**Language/Version**: Go 1.21 (backend), TypeScript/React 18 (frontend)
**Primary Dependencies**: controller-runtime v0.17, govmomi, MUI v5, react-query v5, MUI DataGrid
**Storage**: Kubernetes CRDs (k3s) — all state in CR status/spec
**Testing**: `cd k8s/migration && make test` (Go envtest), Vitest (UI)
**Target Platform**: k3s on Linux appliance VM
**Project Type**: Kubernetes controller + React SPA
**Performance Goals**: Eligibility re-evaluated within 30s of vCenter state change; operator actions applied within one reconcile cycle (≤60s)
**Constraints**: No live vCenter/MAAS contact in unit tests; CGO not required for controller module; UI TypeScript strict mode
**Scale/Scope**: 5–50 ESXi hosts per batch; ≤10 concurrent batches

---

## Constitution Check

| Gate | Status | Notes |
|------|--------|-------|
| All state in Kubernetes CRs | ✅ Pass | ClusterConversionBatch uses CRD status for all host tracking |
| No hand-edit of generated files | ✅ Pass | Will run `make generate` after CRD type changes |
| Test-First Development | ✅ Pass | Unit tests for controller and eligibility logic required per CLAUDE.md |
| Module Independence | ✅ Pass | All Go changes are in `k8s/migration/` module only |
| Code Reuse | ✅ Pass | Reuses EnsureESXiInMass, CanEnterMaintenanceMode, EnsurePCDHasClusterConfigured, scope pattern, utils pattern, MUI components |

---

## Project Structure

### Documentation (this feature)

```text
docs/superpowers/specs/
├── 2026-07-03-cluster-conversion-redesign.md  # Spec
├── plan.md                                     # This file
├── research.md                                 # Phase 0 output
├── data-model.md                               # Phase 1 output
└── checklists/requirements.md
```

### Source Code Changes

```text
k8s/migration/
├── api/v1alpha1/
│   ├── clusterconversionbatch_types.go         [NEW]
│   ├── esximigration_types.go                  [MODIFIED]
│   └── zz_generated.deepcopy.go               [GENERATED - run make generate]
├── internal/controller/
│   ├── clusterconversionbatch_controller.go    [NEW]
│   └── esximigration_controller.go            [MODIFIED]
├── pkg/scope/
│   └── clusterconversionbatchscope.go          [NEW]
├── pkg/utils/
│   └── clusterconversionbatchutils.go          [NEW]
└── cmd/main.go                                 [MODIFIED]

ui/src/
├── api/cluster-conversion-batches/
│   ├── model.ts                                [NEW]
│   ├── clusterConversionBatches.ts             [NEW]
│   └── index.ts                               [NEW]
├── hooks/api/
│   └── useClusterConversionBatchesQuery.ts     [NEW]
└── features/clusterConversions/
    ├── pages/
    │   └── ClusterConversionsPage.tsx          [MODIFIED]
    └── components/
        ├── BatchesTable.tsx                    [NEW]
        ├── CreateBatchDialog.tsx               [NEW]
        ├── BatchDetailDrawer.tsx               [NEW]
        ├── HostStatusChip.tsx                  [NEW]
        └── RollingMigrationsTable.tsx          [UNCHANGED]

docs/src/content/docs/
└── cluster-conversion/                         [UPDATE existing pages]
```

---

## Phase 1: Backend CRD Changes

### 1.1 New file: `k8s/migration/api/v1alpha1/clusterconversionbatch_types.go`

Copy the struct definitions verbatim from the spec (section "CRD Schemas → ClusterConversionBatch"). Key points:
- `AutoStartMode`, `ClusterConversionBatchPhase`, `HostConversionPhase`, `EligibilityStatus` type aliases.
- `HostEntry`, `HostConversionStatus`, `ClusterConversionBatchSpec`, `ClusterConversionBatchStatus` structs.
- `ClusterConversionBatch` and `ClusterConversionBatchList` root objects.
- Register in `SchemeBuilder.Register(...)`.
- Add `+kubebuilder:` markers for status subresource and printcolumns.

**Constants to add in `pkg/common/constants/`** (find the existing constants file and append):
```go
const (
    ClusterConversionBatchFinalizer    = "vjailbreak.k8s.pf9.io/clusterconversionbatch"
    ClusterConversionBatchLabel        = "vjailbreak.k8s.pf9.io/cluster-conversion-batch"
    ClusterConversionBatchControllerName = "clusterconversionbatch-controller"
)
```

### 1.2 Modified: `k8s/migration/api/v1alpha1/esximigration_types.go`

Add two optional fields to `ESXIMigrationSpec` (after `VMwareCredsRef`):

```go
// RollingMigrationPlanRef - change existing doc comment to note deprecation
// +optional
RollingMigrationPlanRef corev1.LocalObjectReference `json:"rollingMigrationPlanRef,omitempty"`

// BMConfigRef directly references the BMConfig for bare-metal provisioning.
// Required when RollingMigrationPlanRef is not set (new ClusterConversionBatch flow).
// +optional
BMConfigRef *corev1.LocalObjectReference `json:"bmConfigRef,omitempty"`

// ClusterConversionBatchRef references the owning ClusterConversionBatch.
// Set by the ClusterConversionBatch controller. Absent for old-flow resources.
// +optional
ClusterConversionBatchRef *corev1.LocalObjectReference `json:"clusterConversionBatchRef,omitempty"`
```

Also add phase constant:
```go
ESXIMigrationPhaseNeedsAttention ESXIMigrationPhase = "NeedsAttention"
```

### 1.3 Regenerate

```bash
cd k8s/migration && make generate
```

This updates `zz_generated.deepcopy.go` and CRD YAML manifests. Verify no compilation errors.

---

## Phase 2: Backend Scope

### 2.1 New file: `k8s/migration/pkg/scope/clusterconversionbatchscope.go`

Follow exact pattern of `esximigrationscope.go`:

```go
package scope

type ClusterConversionBatchScopeParams struct {
    Logger                logr.Logger
    Client                client.Client
    ClusterConversionBatch *vjailbreakv1alpha1.ClusterConversionBatch
}

type ClusterConversionBatchScope struct {
    logr.Logger
    Client                 client.Client
    ClusterConversionBatch *vjailbreakv1alpha1.ClusterConversionBatch
}

func NewClusterConversionBatchScope(params ClusterConversionBatchScopeParams) (*ClusterConversionBatchScope, error) {
    if reflect.DeepEqual(params.Logger, logr.Logger{}) {
        params.Logger = ctrl.Log
    }
    return &ClusterConversionBatchScope{...}, nil
}

// Close persists ClusterConversionBatch changes (spec only — status via Status().Update())
func (s *ClusterConversionBatchScope) Close() error {
    return s.Client.Update(context.TODO(), s.ClusterConversionBatch)
}

func (s *ClusterConversionBatchScope) Name() string      { return s.ClusterConversionBatch.GetName() }
func (s *ClusterConversionBatchScope) Namespace() string { return s.ClusterConversionBatch.GetNamespace() }
```

---

## Phase 3: Backend Utils

### 3.1 New file: `k8s/migration/pkg/utils/clusterconversionbatchutils.go`

#### 3.1.1 `CreateESXIMigrationForBatch`

Mirrors `CreateESXIMigration` (rollingmigrationutils.go:137) but adapted for `ClusterConversionBatch`.

```go
func CreateESXIMigrationForBatch(
    ctx context.Context,
    k8sClient client.Client,
    batch *vjailbreakv1alpha1.ClusterConversionBatch,
    esxiName string,
) (*vjailbreakv1alpha1.ESXIMigration, error) {
    esxiK8sName, err := commonutils.GetK8sCompatibleVMWareObjectName(esxiName, batch.Spec.VMwareCredsRef.Name)
    if err != nil {
        return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
    }
    esxiMigration := &vjailbreakv1alpha1.ESXIMigration{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("%s-%s", esxiK8sName, batch.Name),
            Namespace: constants.NamespaceMigrationSystem,
            Labels: map[string]string{
                constants.ESXiNameLabel:              esxiK8sName,
                constants.VMwareCredsLabel:           batch.Spec.VMwareCredsRef.Name,
                constants.ClusterConversionBatchLabel: batch.Name,
            },
        },
        Spec: vjailbreakv1alpha1.ESXIMigrationSpec{
            ESXiName:          esxiName,
            OpenstackCredsRef: batch.Spec.OpenstackCredsRef,
            VMwareCredsRef:    batch.Spec.VMwareCredsRef,
            BMConfigRef:       &corev1.LocalObjectReference{Name: batch.Spec.BMConfigRef.Name},
            ClusterConversionBatchRef: &corev1.LocalObjectReference{Name: batch.Name},
        },
    }
    if err := k8sClient.Create(ctx, esxiMigration); err != nil {
        return nil, err
    }
    return esxiMigration, nil
}
```

**Note**: No `controllerutil.SetOwnerReference` — intentional, per research.md Decision 4 (no GC cascade).

#### 3.1.2 `GetESXIMigrationForBatch`

```go
func GetESXIMigrationForBatch(
    ctx context.Context,
    k8sClient client.Client,
    batch *vjailbreakv1alpha1.ClusterConversionBatch,
    esxiName string,
) (*vjailbreakv1alpha1.ESXIMigration, error) {
    esxiK8sName, err := commonutils.GetK8sCompatibleVMWareObjectName(esxiName, batch.Spec.VMwareCredsRef.Name)
    if err != nil {
        return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
    }
    esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
    err = k8sClient.Get(ctx, types.NamespacedName{
        Name:      fmt.Sprintf("%s-%s", esxiK8sName, batch.Name),
        Namespace: constants.NamespaceMigrationSystem,
    }, esxiMigration)
    return esxiMigration, err
}
```

#### 3.1.3 `CheckPerHostEligibility`

This function composes existing eligibility helpers. It does NOT modify them.

```go
// CheckPerHostEligibility evaluates all 8 eligibility criteria for a single ESXi host.
// Returns (EligibilityStatus, reason, error).
// REUSES: CanEnterMaintenanceMode, EnsureESXiInMass, EnsurePCDHasClusterConfigured, isBMConfigValid
func CheckPerHostEligibility(
    ctx context.Context,
    k8sClient client.Client,
    batch *vjailbreakv1alpha1.ClusterConversionBatch,
    hostName string,
) (vjailbreakv1alpha1.EligibilityStatus, string, error) {
    // 1. BMConfig valid
    if !isBMConfigValid(ctx, k8sClient, batch.Spec.BMConfigRef.Name) {
        return vjailbreakv1alpha1.EligibilityStatusNotReady,
            fmt.Sprintf("BMConfig %s validation has not succeeded", batch.Spec.BMConfigRef.Name),
            nil
    }

    // 2. PCD cluster configured
    // Build a minimal scope-like object to call EnsurePCDHasClusterConfigured
    // (it only needs client.Client + OpenstackCredsRef)
    openstackCreds, err := GetOpenstackCredsFromRef(ctx, k8sClient, batch.Spec.OpenstackCredsRef.Name)
    if err != nil {
        return vjailbreakv1alpha1.EligibilityStatusUnknown, "", errors.Wrap(err, "failed to get openstack creds")
    }
    clusters, err := filterPCDClustersOnOpenstackCreds(ctx, k8sClient, *openstackCreds)
    if err != nil {
        return vjailbreakv1alpha1.EligibilityStatusUnknown, "", errors.Wrap(err, "failed to list PCD clusters")
    }
    if len(clusters) == 0 {
        return vjailbreakv1alpha1.EligibilityStatusNotReady,
            fmt.Sprintf("no PCD cluster configured for OpenStack creds %s", batch.Spec.OpenstackCredsRef.Name),
            nil
    }

    // 3. MAAS match + DRS + capacity + anti-affinity via CanEnterMaintenanceMode + EnsureESXiInMass
    // Build a temporary RollingMigrationPlanScope-compatible wrapper
    // (these functions require *scope.RollingMigrationPlanScope — we create a minimal adapter)
    rmpScope, err := buildTemporaryRMPScope(ctx, k8sClient, batch)
    if err != nil {
        return vjailbreakv1alpha1.EligibilityStatusUnknown, "", errors.Wrap(err, "failed to build temp scope")
    }
    
    vmwareCreds, err := GetVMwareCredsFromRef(ctx, k8sClient, batch.Spec.VMwareCredsRef.Name)
    if err != nil {
        return vjailbreakv1alpha1.EligibilityStatusUnknown, "", errors.Wrap(err, "failed to get vmware creds")
    }

    // Get VMwareHost object for MAAS matching
    vmwareHost, err := GetVMwareHostFromESXiName(ctx, k8sClient, hostName, batch.Spec.VMwareCredsRef.Name)
    if err != nil {
        return vjailbreakv1alpha1.EligibilityStatusNotReady,
            fmt.Sprintf("VMwareHost object not found for %s", hostName),
            nil
    }

    // MAAS check (reuse existing function unchanged)
    inMAAS, reason, err := EnsureESXiInMass(ctx, rmpScope, *vmwareHost)
    if err != nil {
        return vjailbreakv1alpha1.EligibilityStatusUnknown, "", err
    }
    if !inMAAS {
        return vjailbreakv1alpha1.EligibilityStatusNotReady, reason, nil
    }

    // DRS + capacity + anti-affinity check (reuse existing function unchanged)
    config := DefaultRollingMigrationValidationConfig()  // use permissive defaults for batch flow
    canMaintenance, reason, err := CanEnterMaintenanceMode(ctx, rmpScope, vmwareCreds, hostName, config)
    if err != nil {
        return vjailbreakv1alpha1.EligibilityStatusUnknown, "", err
    }
    if !canMaintenance {
        return vjailbreakv1alpha1.EligibilityStatusNotReady, reason, nil
    }

    return vjailbreakv1alpha1.EligibilityStatusReady, "", nil
}
```

**`buildTemporaryRMPScope`**: Creates a `*scope.RollingMigrationPlanScope` with a synthetic `RollingMigrationPlan` containing only the fields that `EnsureESXiInMass` and `CanEnterMaintenanceMode` access (VMwareCredsRef, BMConfigRef, OpenstackCredsRef). This avoids forking those functions.

```go
func buildTemporaryRMPScope(
    ctx context.Context,
    k8sClient client.Client,
    batch *vjailbreakv1alpha1.ClusterConversionBatch,
) (*scope.RollingMigrationPlanScope, error) {
    syntheticRMP := &vjailbreakv1alpha1.RollingMigrationPlan{
        Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
            BMConfigRef:       batch.Spec.BMConfigRef,
            // OpenstackCredsRef and VMwareCredsRef stored in MigrationPlanSpecPerVM inline struct
            // — check what EnsureESXiInMass actually reads and populate accordingly
        },
    }
    return scope.NewRollingMigrationPlanScope(scope.RollingMigrationPlanScopeParams{
        Client:               k8sClient,
        RollingMigrationPlan: syntheticRMP,
    })
}
```

**Important implementation note**: Read `EnsureESXiInMass` and `CanEnterMaintenanceMode` carefully to see exactly which fields they read from `scope.RollingMigrationPlan` and populate only those. This is the most brittle piece — do it carefully with a test.

#### 3.1.4 `ComputeRetryBackoff`

```go
func ComputeRetryBackoff(baseSeconds, retryCount int) time.Duration {
    backoff := baseSeconds
    for i := 1; i < retryCount; i++ {
        backoff *= 2
    }
    return time.Duration(backoff) * time.Second
}
```

#### 3.1.5 `ProcessBatchAnnotations`

```go
// ProcessBatchAnnotations reads trigger/retry/skip annotations and returns
// the list of actions to perform. Removes annotations from batch.Annotations.
func ProcessBatchAnnotations(batch *vjailbreakv1alpha1.ClusterConversionBatch) []BatchAction {
    // returns []BatchAction{Type: "trigger"|"retry"|"skip", ESXiName: "..."}
    // removes processed annotations from batch.Annotations
}
```

---

## Phase 4: Backend Controller

### 4.1 New file: `k8s/migration/internal/controller/clusterconversionbatch_controller.go`

#### Structure

Follow exact pattern of `clustermigration_controller.go` (method names, error wrapping, scope usage):

```go
type ClusterConversionBatchReconciler struct {
    client.Client
    Scheme *runtime.Scheme
    Logger logr.Logger
}

func (r *ClusterConversionBatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
    // 1. Get ClusterConversionBatch
    // 2. Create scope
    // 3. defer scope.Close()
    // 4. Handle deletion (remove finalizer)
    // 5. reconcileNormal
}

func (r *ClusterConversionBatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&vjailbreakv1alpha1.ClusterConversionBatch{}).
        Watches(
            &vjailbreakv1alpha1.ESXIMigration{},
            handler.EnqueueRequestsFromMapFunc(r.esxiMigrationToBatch),
        ).
        Complete(r)
}
```

#### `reconcileNormal` pseudocode

```
reconcileNormal(ctx, scope):
  batch := scope.ClusterConversionBatch
  
  // Add finalizer
  controllerutil.AddFinalizer(batch, ClusterConversionBatchFinalizer)
  scope.Close()  // persist finalizer
  
  // Initialize status.hosts if first reconcile
  if len(batch.Status.Hosts) == 0:
    initializeHostStatuses(batch)
    r.Status().Update(ctx, batch)
    return {RequeueAfter: 5s}, nil
  
  // Process annotations (operator actions)
  actions := ProcessBatchAnnotations(batch)  // mutates batch.Annotations
  for each action:
    applyAction(ctx, scope, action)
  
  // Process each host
  for i, hostStatus := range batch.Status.Hosts:
    if hostStatus.Phase is terminal (Succeeded, Skipped): continue
    
    processHost(ctx, scope, &batch.Status.Hosts[i])
  
  // Aggregate counts and batch phase
  updateBatchAggregates(batch)
  r.Status().Update(ctx, batch)
  
  // Re-update batch annotations (cleared by ProcessBatchAnnotations)
  r.Update(ctx, batch)
  
  return {RequeueAfter: 30s}, nil
```

#### `processHost` pseudocode

```
processHost(ctx, scope, hostStatus):
  // Check retry timer
  if hostStatus.Phase == Failed && hostStatus.NextRetryAt != nil && time.Now().Before(*hostStatus.NextRetryAt):
    return  // waiting for retry window
  
  // If ESXIMigration exists, sync its status
  esxiMig, err := GetESXIMigrationForBatch(ctx, client, batch, hostStatus.ESXiName)
  if err == nil:  // exists
    mirrorESXIMigrationStatus(hostStatus, esxiMig)
    if esxiMig.Status.Phase == Failed:
      hostStatus.RetryCount++
      if hostStatus.RetryCount > batch.Spec.MaxRetries:
        hostStatus.Phase = NeedsAttention
      else:
        hostStatus.NextRetryAt = now + ComputeRetryBackoff(batch.Spec.RetryBackoffSeconds, hostStatus.RetryCount)
        hostStatus.Phase = Failed
    return
  
  // No ESXIMigration — check if we should create one
  if hostStatus.Phase == Failed && hostStatus.NextRetryAt != nil && time.Now().After(*hostStatus.NextRetryAt):
    // Retry window elapsed — re-check eligibility then create
    // (ESXIMigration already deleted by applyAction or by previous retry cleanup)
    hostStatus.NextRetryAt = nil
  
  // Re-evaluate eligibility
  eligStatus, reason, err := CheckPerHostEligibility(ctx, client, batch, hostStatus.ESXiName)
  hostStatus.EligibilityStatus = eligStatus
  hostStatus.EligibilityReason = reason
  
  if eligStatus != Ready:
    hostStatus.Phase = NotReady
    return
  
  hostStatus.Phase = Ready
  
  // Start conversion if auto mode
  if batch.Spec.AutoStart == AutoStartModeAuto:
    esxiMig, err := CreateESXIMigrationForBatch(ctx, client, batch, hostStatus.ESXiName)
    if err != nil: log error, return
    hostStatus.ESXIMigrationRef = &{Name: esxiMig.Name}
    hostStatus.Phase = Converting
    hostStatus.StartedAt = now
```

#### `applyAction` pseudocode

```
applyAction(ctx, scope, action):
  hostStatus := findHostStatus(batch, action.ESXiName)
  
  switch action.Type:
  case "trigger":
    if hostStatus.Phase != Ready: return  // no-op for non-Ready
    esxiMig := CreateESXIMigrationForBatch(...)
    hostStatus.ESXIMigrationRef = &{Name: esxiMig.Name}
    hostStatus.Phase = Converting
    hostStatus.StartedAt = now
  
  case "retry":
    if hostStatus.Phase != NeedsAttention: return
    // Delete existing (probably failed) ESXIMigration if any
    if hostStatus.ESXIMigrationRef != nil:
      delete existing ESXIMigration (ignore NotFound)
      hostStatus.ESXIMigrationRef = nil
    hostStatus.RetryCount = 0
    hostStatus.NextRetryAt = nil
    hostStatus.Phase = CheckingEligibility
  
  case "skip":
    hostStatus.SkippedAt = &now
    hostStatus.Phase = Skipped
    // Do NOT delete ESXIMigration if Converting — it runs independently
```

#### Watch handler for ESXIMigration → Batch mapping

```go
func (r *ClusterConversionBatchReconciler) esxiMigrationToBatch(
    ctx context.Context, obj client.Object,
) []reconcile.Request {
    esxiMig := obj.(*vjailbreakv1alpha1.ESXIMigration)
    batchName, ok := esxiMig.Labels[constants.ClusterConversionBatchLabel]
    if !ok || batchName == "" {
        return nil
    }
    return []reconcile.Request{{
        NamespacedName: types.NamespacedName{
            Name:      batchName,
            Namespace: esxiMig.Namespace,
        },
    }}
}
```

#### Deletion handler

```go
reconcileDelete(ctx, scope):
  // Do NOT delete child ESXIMigrations (they run independently)
  // Just remove finalizer
  controllerutil.RemoveFinalizer(scope.ClusterConversionBatch, ClusterConversionBatchFinalizer)
  return ctrl.Result{}, nil
```

### 4.2 Modified: `k8s/migration/internal/controller/esximigration_controller.go`

#### Change 1: Conditional RollingMigrationPlan fetch (lines 75-88)

Replace:
```go
rollingMigrationPlan := &vjailbreakv1alpha1.RollingMigrationPlan{}
rollingMigrationPlanKey := client.ObjectKey{...Name: esxiMigration.Spec.RollingMigrationPlanRef.Name}
if err := r.Get(ctx, rollingMigrationPlanKey, rollingMigrationPlan); err != nil {
    ...
}
scope.RollingMigrationPlan = rollingMigrationPlan
```

With:
```go
if esxiMigration.Spec.RollingMigrationPlanRef.Name != "" {
    rollingMigrationPlan := &vjailbreakv1alpha1.RollingMigrationPlan{}
    rollingMigrationPlanKey := client.ObjectKey{
        Namespace: esxiMigration.Namespace,
        Name:      esxiMigration.Spec.RollingMigrationPlanRef.Name,
    }
    if err := r.Get(ctx, rollingMigrationPlanKey, rollingMigrationPlan); err != nil {
        if apierrors.IsNotFound(err) && !esxiMigration.DeletionTimestamp.IsZero() {
            return r.reconcileDelete(ctx, scope)
        }
        return ctrl.Result{}, errors.Wrap(err, "failed to get RollingMigrationPlan")
    }
    scope.RollingMigrationPlan = rollingMigrationPlan
}
```

#### Change 2: BMConfig resolution (in reconcileNormal, line ~136)

Replace:
```go
bmConfigKey := client.ObjectKey{Namespace: scope.ESXIMigration.Namespace, Name: scope.RollingMigrationPlan.Spec.BMConfigRef.Name}
if err := r.Get(ctx, bmConfigKey, bmConfig); err != nil { ... }
```

With:
```go
bmConfigName, err := resolveBMConfigName(scope.ESXIMigration, scope.RollingMigrationPlan)
if err != nil {
    return ctrl.Result{}, err
}
bmConfigKey := client.ObjectKey{Namespace: scope.ESXIMigration.Namespace, Name: bmConfigName}
if err := r.Get(ctx, bmConfigKey, bmConfig); err != nil { ... }
```

Where:
```go
func resolveBMConfigName(esxiMig *vjailbreakv1alpha1.ESXIMigration, rmp *vjailbreakv1alpha1.RollingMigrationPlan) (string, error) {
    if esxiMig.Spec.BMConfigRef != nil && esxiMig.Spec.BMConfigRef.Name != "" {
        return esxiMig.Spec.BMConfigRef.Name, nil
    }
    if rmp != nil && rmp.Spec.BMConfigRef.Name != "" {
        return rmp.Spec.BMConfigRef.Name, nil
    }
    return "", errors.New("no BMConfig reference: set spec.bmConfigRef on ESXIMigration or ensure RollingMigrationPlanRef is valid")
}
```

**No other changes to esximigration_controller.go.**

### 4.3 Modified: `k8s/migration/cmd/main.go`

Add controller registration after the existing ESXIMigration/ClusterMigration/BMConfig blocks:

```go
if err = (&controller.ClusterConversionBatchReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "ClusterConversionBatch")
    os.Exit(1)
}
```

---

## Phase 5: Unit Tests (Backend)

### 5.1 `k8s/migration/internal/controller/clusterconversionbatch_controller_test.go`

Key test cases (table-driven where applicable):

| Test | Setup | Assert |
|------|-------|--------|
| Initialize host statuses | Batch with 3 hosts, empty status | status.hosts has 3 entries, all CheckingEligibility |
| Auto-start eligible host | Host eligibility = Ready, AutoStart=Auto | ESXIMigration created, phase=Converting |
| Manual-mode no auto-start | Host eligibility = Ready, AutoStart=Manual | No ESXIMigration created, phase=Ready |
| Trigger annotation starts host | AutoStart=Manual, phase=Ready, trigger annotation set | ESXIMigration created, annotation removed |
| Skip annotation | Host in NeedsAttention | phase=Skipped, ESXIMigration NOT deleted |
| Retry annotation | Host in NeedsAttention | RetryCount reset, phase=CheckingEligibility |
| Host failure increments retry | ESXIMigration phase=Failed, RetryCount=0, MaxRetries=3 | RetryCount=1, phase=Failed, NextRetryAt set |
| Retry exhaustion → NeedsAttention | ESXIMigration phase=Failed, RetryCount=3, MaxRetries=3 | phase=NeedsAttention |
| Sibling isolation | Host A=Failed, Host B=Ready | Host B advances to Converting, batch=Running |
| Batch phase Succeeded | All hosts Succeeded | batch.Status.Phase=Succeeded |
| Batch phase PartialFail | 2 Succeeded, 1 Skipped | batch.Status.Phase=PartialFail |
| Batch phase Failed | All NeedsAttention | batch.Status.Phase=Failed |
| Delete batch | batch.DeletionTimestamp set | finalizer removed, ESXIMigrations NOT deleted |

**Mock eligibility**: Use an interface `EligibilityChecker` injected into the reconciler to avoid real vCenter calls.

### 5.2 `k8s/migration/pkg/utils/clusterconversionbatchutils_test.go`

| Test | Assert |
|------|--------|
| ComputeRetryBackoff(60, 1) | 60s |
| ComputeRetryBackoff(60, 2) | 120s |
| ComputeRetryBackoff(60, 3) | 240s |
| ProcessBatchAnnotations - trigger | Returns trigger action, annotation removed |
| ProcessBatchAnnotations - retry | Returns retry action, annotation removed |
| ProcessBatchAnnotations - no annotations | Returns empty slice |
| CreateESXIMigrationForBatch | ESXIMigration has correct labels, spec.bmConfigRef set, no ownerRef |

### 5.3 `k8s/migration/internal/controller/esximigration_controller_test.go` (additions)

Add tests for the changed behavior:

| Test | Assert |
|------|--------|
| No RollingMigrationPlanRef, BMConfigRef set | Controller fetches BMConfig from spec.bmConfigRef |
| RollingMigrationPlanRef absent and not deleting | Returns error (missing BMConfig) |
| RollingMigrationPlanRef set (old flow) | Behavior unchanged from current tests |

---

## Phase 6: UI — API Layer

### 6.1 New: `ui/src/api/cluster-conversion-batches/model.ts`

Copy TypeScript interfaces from `data-model.md` (ClusterConversionBatch, ClusterConversionBatchSpec, etc.). Reuse `ItemMetadata` and `NameReference` from existing models (import from `rolling-migration-plans/model.ts` or create shared `ui/src/api/common/model.ts`).

### 6.2 New: `ui/src/api/cluster-conversion-batches/clusterConversionBatches.ts`

Mirror `ui/src/api/rolling-migration-plans/rollingMigrationPlans.ts` exactly. API endpoint: `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/clusterconversionbatches`

Functions:
- `getClusterConversionBatches(namespace?)` → `ClusterConversionBatch[]`
- `getClusterConversionBatch(name, namespace?)` → `ClusterConversionBatch`
- `postClusterConversionBatch(body, namespace?)` → `ClusterConversionBatch`
- `deleteClusterConversionBatch(name, namespace?)` → void
- `patchClusterConversionBatch(name, body, namespace?)` → `ClusterConversionBatch` (use `Content-Type: application/merge-patch+json`)

The `patchClusterConversionBatch` function is used for: changing AutoStart mode, and setting operator action annotations (trigger/retry/skip).

### 6.3 New: `ui/src/api/cluster-conversion-batches/index.ts`

Re-export all from `clusterConversionBatches.ts` and `model.ts`.

### 6.4 New: `ui/src/hooks/api/useClusterConversionBatchesQuery.ts`

Mirror `useRollingMigrationPlansQuery.ts` exactly. Query key constant `CLUSTER_CONVERSION_BATCHES_QUERY_KEY`.

---

## Phase 7: UI — Components

### 7.1 New: `ui/src/features/clusterConversions/components/HostStatusChip.tsx`

Extends existing `StatusChip` from `RollingMigrationsTable.tsx` with new phase values:

```typescript
// Map HostConversionPhase → color
const HOST_PHASE_COLORS = {
  CheckingEligibility: 'default',
  NotReady: 'warning',
  Ready: 'info',
  Converting: 'info',
  Succeeded: 'success',
  Failed: 'warning',      // has retries remaining
  NeedsAttention: 'error',
  Skipped: 'default',
}
```

### 7.2 New: `ui/src/features/clusterConversions/components/CreateBatchDialog.tsx`

**Two-step MUI Dialog/Stepper:**

Step 1: Configuration
- `Autocomplete` for VMware cluster (fetches via `getVMwareClusters(namespace, vmwareCredName)`)
- After cluster selected: shows host list with eligibility pre-check (reads from a temporary batch or VMwareHost objects)
- Checkboxes for host selection
- `Switch` for AutoStart (Auto/Manual)
- `Select` for BMConfig (from `useBMConfigQuery`)
- `Select` for OpenStack creds (filtered to PCD, from `useOpenstackCredentialsQuery`)
- Advanced section (collapsible): MaxRetries, RetryBackoffSeconds

Step 2: Review + Create
- Summary table of selected hosts
- Creates batch via `postClusterConversionBatch`
- On success: invalidates `CLUSTER_CONVERSION_BATCHES_QUERY_KEY`, closes dialog

**Pre-flight eligibility**: Display `VMwareHost.status.state` and `VMwareHost.spec.hostConfigId` from existing `useVMwareHostsQuery` for a quick health indicator. Full eligibility only available after batch creation.

### 7.3 New: `ui/src/features/clusterConversions/components/BatchDetailDrawer.tsx`

Reuses `StyledDrawer`, `DrawerHeader`, `DrawerFooter`, `DrawerContent` styled components from `RollingMigrationsTable.tsx` (extract to shared file or copy — follow existing pattern).

**Per-host table columns:**
| Column | Source |
|--------|--------|
| ESX Host | `hostStatus.esxiName` |
| Phase | `<HostStatusChip phase={hostStatus.phase} />` |
| Eligibility | `hostStatus.eligibilityStatus` + `hostStatus.eligibilityReason` (tooltip) |
| Retries | `{hostStatus.retryCount ?? 0} / {batch.spec.maxRetries ?? 3}` |
| Duration | `calculateTimeElapsed(hostStatus.startedAt, hostStatus)` (reuse existing util) |
| Actions | Context-sensitive buttons (see below) |

**Action buttons per host phase:**
| Phase | Buttons |
|-------|---------|
| Ready (Manual mode) | `<Button onClick={() => triggerHost(esxiName)}>Start</Button>` |
| NeedsAttention | `<Button onClick={() => retryHost(esxiName)}>Retry</Button>` + `<Button color="warning" onClick={() => skipHost(esxiName)}>Skip</Button>` |
| All others | None |

**triggerHost / retryHost / skipHost**: Calls `patchClusterConversionBatch(batchName, { metadata: { annotations: { 'vjailbreak.k8s.pf9.io/trigger-host': esxiName } } })` then refetches.

**AutoStart toggle** in drawer header:
```tsx
<Switch
  checked={batch.spec.autoStart === 'Auto'}
  onChange={(e) => {
    patchClusterConversionBatch(batch.metadata.name, {
      spec: { autoStart: e.target.checked ? 'Auto' : 'Manual' }
    }).then(refetch)
  }}
  label="Auto-Start"
/>
```

**Progress bar** (reuse `StatusSummary` component from `RollingMigrationsTable.tsx` with adaptor).

### 7.4 New: `ui/src/features/clusterConversions/components/BatchesTable.tsx`

Mirrors structure of `RollingMigrationsTable.tsx` but for `ClusterConversionBatch`. 

**Table columns:**
| Column | Source |
|--------|--------|
| Cluster | `batch.spec.vmwareClusterName` (with cluster icon) |
| Status | `<StatusChip status={batch.status?.phase} />` |
| Progress | `succeededHosts/totalHosts` linear progress bar |
| Running | `runningHosts` count |
| NeedsAttention | `needsAttentionHosts` count (amber badge if > 0) |
| AutoStart | `batch.spec.autoStart` chip |
| Age | `calculateTimeElapsed(batch.metadata.creationTimestamp, ...)` |
| Actions | "Details" button → opens `BatchDetailDrawer` |

**Toolbar**: "Create Conversion Batch" button (opens `CreateBatchDialog`). Same disabled logic as existing toolbar (needs VMware + PCD creds).

**Delete**: Multiple select + delete with `ConfirmationDialog` (reuse existing pattern).

### 7.5 Modified: `ui/src/features/clusterConversions/pages/ClusterConversionsPage.tsx`

```tsx
export default function ClusterConversionsPage() {
  // existing queries (unchanged) for legacy section
  const { data: esxiMigrations, refetch: refetchESXIMigrations } = useESXIMigrationsQuery(...)
  const { data: rollingMigrationPlans, refetch: refetchRollingMigrationPlans } = useRollingMigrationPlansQuery(...)
  const { data: migrations, refetch: refetchMigrations } = useMigrationsQuery()
  
  // new query for primary section
  const { data: batches, refetch: refetchBatches } = useClusterConversionBatchesQuery({
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })
  
  return (
    <Box>
      {/* Primary: ClusterConversionBatch */}
      <BatchesTable
        batches={batches || []}
        refetchBatches={refetchBatches}
      />
      
      {/* Legacy: RollingMigrationPlans (only if any exist) */}
      {(rollingMigrationPlans || []).length > 0 && (
        <Box sx={{ mt: 4 }}>
          <Typography variant="subtitle2" color="text.secondary">
            Legacy Cluster Conversions (in progress)
          </Typography>
          <RollingMigrationsTable
            rollingMigrationPlans={rollingMigrationPlans || []}
            esxiMigrations={esxiMigrations || []}
            migrations={migrations || []}
            refetchRollingMigrationPlans={refetchRollingMigrationPlans}
            refetchESXIMigrations={refetchESXIMigrations}
            refetchMigrations={refetchMigrations}
          />
        </Box>
      )}
    </Box>
  )
}
```

---

## Phase 8: Documentation

### 8.1 Locate existing docs structure

```bash
find docs/src/content/docs -name "*.md" -o -name "*.mdx" | grep -i cluster
```

Look for existing cluster conversion documentation pages. If they exist:

### 8.2 Update cluster conversion overview page

Add to the top of any existing cluster conversion guide:

```markdown
> **New in v0.5.0**: The cluster conversion feature has been redesigned.
> Existing `RollingMigrationPlan` resources continue to work unchanged.
> New workflows should use **Cluster Conversion Batch** — see [Create a Batch](#create-a-batch).
```

Add new section: **Cluster Conversion Batch** explaining:
- Select VMware cluster → select hosts → configure → create batch
- Dynamic eligibility (no blocking pre-validation)
- Auto vs Manual start modes
- Per-host retry/skip via UI

### 8.3 Deprecation notice for RollingMigrationPlan docs

Add `:::caution[Deprecated]` callout to any RollingMigrationPlan-specific pages:
```markdown
:::caution[Deprecated]
`RollingMigrationPlan` is deprecated as of v0.5.0. In-flight plans continue to work.
New cluster conversions should use `ClusterConversionBatch`. [Learn more →](./cluster-conversion-batch)
:::
```

### 8.4 Add CHANGELOG entry

In `CHANGELOG.md` or equivalent (check if file exists at repo root):
```markdown
## [Unreleased]
### Added
- `ClusterConversionBatch` CRD for resilient ESXi→PCD host conversion with per-host isolation and retry
### Changed  
- `ESXIMigration` now runs autonomously (no required parent `RollingMigrationPlan`)
### Deprecated
- `RollingMigrationPlan` and `ClusterMigration`: use `ClusterConversionBatch` for new workflows
```

---

## Phase 9: CRD Manifest Generation

After all Go changes:

```bash
cd k8s/migration
make generate          # regenerates zz_generated.deepcopy.go + CRD YAMLs
make test              # run all controller tests
```

Verify `config/crd/bases/vjailbreak.k8s.pf9.io_clusterconversionbatches.yaml` is generated.

---

## Implementation Order (Recommended)

1. **Phase 1** (CRD types + ESXIMigration spec change + constants) — no behavior change
2. **Phase 2** (Scope) — trivial, no behavior change
3. **Phase 9** (make generate) — catch type errors early
4. **Phase 3** (Utils — clusterconversionbatchutils.go) — eligibility logic with tests
5. **Phase 5a** (ESXIMigration controller change) — minimal, with tests
6. **Phase 4** (ClusterConversionBatch controller) — main behavioral logic, test-first
7. **Phase 5b** (main.go registration) — wire it up
8. **Phase 6** (UI API layer) — no logic
9. **Phase 7** (UI components) — visual, test with dev server
10. **Phase 8** (Docs) — final

---

## Complexity Tracking

No constitution violations. All changes are within existing module and component boundaries.

| Change | Simplest Approach Used |
|--------|----------------------|
| Eligibility reuse | `buildTemporaryRMPScope` adapter to call existing functions without forking |
| Operator actions | Annotation-based (no new API endpoints) |
| Retry | Delete+recreate ESXIMigration (no new state machine in ESXIMigration) |
| Watch on ESXIMigration | Label-based mapping handler (no owner reference, no GC cascade) |
