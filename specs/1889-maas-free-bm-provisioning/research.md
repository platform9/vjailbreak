# Research: Per-ESXi Conversion + MAAS-Free Provisioning

**Date**: 2026-05-08
**Feature**: 1889-maas-free-bm-provisioning

---

## Finding 1: No VMwareCluster Controller Exists

**Decision**: Create `vmwarecluster_controller.go` from scratch.

**Rationale**: `VMwareCluster` CRD exists with Pending/Running/Failed/Completed phases but no
reconciler. The new controller must poll vCenter periodically to populate `status.hosts[]`
with per-host VM counts and maintenance state. Reuse existing govmomi client pattern from
other controllers.

**Alternatives considered**: Query `ListVMs` from the UI on each page load (N calls per host)
— rejected: fan-out, no caching, slow page load.

---

## Finding 2: ESXIMigration Controller Tightly Fetches RollingMigrationPlan

**Decision**: Guard plan fetch with nil check; add `BMConfigRef` to `ESXIMigrationSpec`.

**Rationale**: Line 76 of `esximigration_controller.go` always fetches
`esxiMigration.Spec.RollingMigrationPlanRef.Name` with no nil check. All downstream phases
use `scope.RollingMigrationPlan` for credentials and BMConfig. `ESXIMigrationSpec` already
has `OpenstackCredsRef` and `VMwareCredsRef` — these can be used directly. The only missing
piece is `BMConfigRef` (where to find BMConfig when no plan). Add that field.

**Alternatives considered**: Derive BMConfig from cluster-wide default — rejected: too implicit.
Keep requiring a plan — rejected: defeats the goal.

**Scope of change**:
- `esximigration_types.go`: `RollingMigrationPlanRef` → pointer, add `BMConfigRef *corev1.LocalObjectReference`
- `esximigration_controller.go` line 76–88: wrap plan fetch in `if spec.RollingMigrationPlanRef != nil`
- `bmprovisionerutils.go`: `ConvertESXiToPCDHost` already takes `bmProvider providers.BMCProvider` as
  a param — only needs to accept creds struct directly instead of pulling from plan
- `esximigrationscope.go`: `RollingMigrationPlan` field becomes a pointer

---

## Finding 3: ListVMs Has No Host Filtering

**Decision**: Add optional `hostName` filter to `ListVMsRequest` proto and vcenter implementation.

**Rationale**: `vcenter.go:108-154` retrieves all VMs with no hostname filter. For the ESXi
VM table in UI (showing only VMs on one host), either filter server-side or client-side.
Server-side is cleaner and avoids sending all VM data. Add `host_name string` to
`ListVMsRequest` proto; vcenter implementation filters by host reference.

**Alternatives considered**: Client-side filtering — rejected: unnecessary data transfer,
especially on large clusters.

---

## Finding 4: ClusterConversionsPage Wraps RollingMigrationsTable

**Decision**: Add VMwareCluster accordion above `RollingMigrationsTable` in `ClusterConversionsPage`.

**Rationale**: `ClusterConversionsPage` renders only `<RollingMigrationsTable>` today.
The new per-ESXi view should sit above it. Add `useVMwareClustersQuery` hook (reuse pattern
from existing query hooks) and new `ESXiClusterAccordion` → `ESXiHostRow` → `ESXiVMTable`
component tree. `RollingMigrationsTable` stays intact.

**Alternatives considered**: Replace `RollingMigrationsTable` entirely — rejected: breaks
existing cluster conversion UX, need both views during transition.

---

## Finding 5: BMCProvider Interface Has 16 Methods — New Providers Must Implement All

**Decision**: Create `IronicProvider` and `IPMIProvider` each implementing the full interface.

**Rationale**: Interface at `providers.go:13-49` has 16 methods. Many (like `ListBootSource`,
`GetIPMIClient`) are MAAS-specific — new providers return `ErrNotSupported` for methods
not applicable to their protocol. Core methods needed: `Connect`, `SetBM2PXEBoot`, `StartBM`,
`StopBM`, `DeployMachine`, `ReclaimBM`, `ListResources`, `GetResourceInfo`.

**For IronicProvider**: Ironic REST API via `gophercloud` (OpenStack Go SDK —
`github.com/gophercloud/gophercloud`, a **new dependency** not currently in `pkg/vpwned/go.mod`;
must be added via `go get` before implementation). Check whether `pkg/vpwned/sdk/providers/ironic/`
already exists as a stub before writing from scratch — if so, extend rather than replace. Maps
cleanly: `DeployMachine` → Ironic node provision, `ReclaimBM` → Ironic node cleaning,
`SetBM2PXEBoot` → Ironic boot interface config.

**For IPMIProvider**: Reuse IPMI calls already in `maas.go` (ChassisControlPowerUp/Down,
boot device set). Extract into standalone provider. Use `gofish` library for Redfish when
available. `ListResources` returns a single machine (the one configured via BMC IP in BMConfig).

---

## Finding 6: ConvertESXiToPCDHost Already Accepts BMCProvider as Parameter

**Decision**: Minimal refactor — pass BMProvider from direct spec fields when plan ref is nil.

**Rationale**: `ConvertESXiToPCDHost(ctx, scope, bmProvider)` already takes a `BMCProvider`
interface. The caller (`handleESXiCordoned` in the controller) is responsible for instantiating
the provider. Refactor only the provider-instantiation call path, not `bmprovisionerutils.go`.
