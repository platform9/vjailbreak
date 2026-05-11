# Data Model: Per-ESXi Conversion + MAAS-Free Provisioning

**Date**: 2026-05-08

---

## Phase 1 CRD Changes

### VMwareCluster — Add HostStatus to Status

**File**: `k8s/migration/api/v1alpha1/vmwarecluster_types.go`

```go
// HostStatus tracks per-ESXi host state within a VMwareCluster
type HostStatus struct {
    // Name is the ESXi host FQDN or IP as reported by vCenter
    Name string `json:"name"`
    // VMCount is the number of powered-on VMs currently on this host
    VMCount int `json:"vmCount"`
    // InMaintenanceMode indicates the host is in vCenter maintenance mode
    InMaintenanceMode bool `json:"inMaintenanceMode,omitempty"`
}

// VMwareClusterStatus updated
type VMwareClusterStatus struct {
    Phase VMwareClusterPhase `json:"phase,omitempty"`
    // Hosts is populated by the VMwareCluster controller; updated every 30s
    Hosts []HostStatus `json:"hosts,omitempty"`
    // Conditions holds controller-set conditions including LastPollError
    // when a vCenter host is unreachable (EC-001)
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

**State**: Populated by new VMwareCluster controller polling vCenter. Read by UI via k8s API.

### VMwareCluster — Add VMwareCredsRef to Spec

**File**: `k8s/migration/api/v1alpha1/vmwarecluster_types.go`

```go
// VMwareClusterSpec updated — add VMwareCredsRef for controller authentication
type VMwareClusterSpec struct {
    Name           string                       `json:"name,omitempty"`
    Hosts          []string                     `json:"hosts,omitempty"`
    // VMwareCredsRef identifies the VMwareCreds secret the controller uses
    // to connect to vCenter when polling per-host VM counts and maintenance state.
    VMwareCredsRef corev1.LocalObjectReference  `json:"vmwareCredsRef"`
    // BMConfigRef is the default bare-metal provider config used when creating
    // standalone ESXIMigration CRs from this cluster's "Convert to PCD Host" action.
    BMConfigRef    *corev1.LocalObjectReference `json:"bmConfigRef,omitempty"`
    // PCDClusterRef is the default PCD cluster to enroll converted hosts into,
    // used by the "Convert to PCD Host" UI action.
    PCDClusterRef  *corev1.LocalObjectReference `json:"pcdClusterRef,omitempty"`
}
```

**State**: Set by user when creating VMwareCluster CR. `VMwareCredsRef` required for controller authentication. `BMConfigRef` and `PCDClusterRef` are optional defaults used by the "Convert to PCD Host" UI action to populate standalone ESXIMigration CRs.

---

### ESXIMigration — Make RollingMigrationPlanRef Optional, Add BMConfigRef

**File**: `k8s/migration/api/v1alpha1/esximigration_types.go`

```go
type ESXIMigrationSpec struct {
    ESXiName          string                              `json:"esxiName"`
    OpenstackCredsRef corev1.LocalObjectReference         `json:"openstackCredsRef"`
    VMwareCredsRef    corev1.LocalObjectReference         `json:"vmwareCredsRef"`
    // RollingMigrationPlanRef is optional — standalone ESXIMigration omits it
    RollingMigrationPlanRef *corev1.LocalObjectReference  `json:"rollingMigrationPlanRef,omitempty"`
    // BMConfigRef is required when RollingMigrationPlanRef is nil
    BMConfigRef       *corev1.LocalObjectReference        `json:"bmConfigRef,omitempty"`
    // PCDClusterRef identifies the PCD cluster to enroll the converted host into.
    // Required when RollingMigrationPlanRef is nil; the plan-based path derives
    // the cluster from RollingMigrationPlan.Spec.ClusterMapping.
    PCDClusterRef     *corev1.LocalObjectReference        `json:"pcdClusterRef,omitempty"`
}
```

**Standalone path**: `RollingMigrationPlanRef` nil, `BMConfigRef` set → controller uses
`OpenstackCredsRef`, `VMwareCredsRef`, `BMConfigRef` directly.

**Orchestrated path** (unchanged): `RollingMigrationPlanRef` set → controller uses plan
to derive all three refs (existing behavior).

**Validation rules** (enforced by kubebuilder webhook or controller admission):

- If `RollingMigrationPlanRef` is nil, then `BMConfigRef` and `PCDClusterRef` MUST both be non-nil.
- If `RollingMigrationPlanRef` is set, then `BMConfigRef` and `PCDClusterRef` SHOULD be omitted (plan provides them).

---

### ESXIMigrationScope — RollingMigrationPlan Becomes Pointer

**File**: `k8s/migration/pkg/scope/esximigrationscope.go`

```go
type ESXIMigrationScope struct {
    // ...existing fields...
    // RollingMigrationPlan is nil for standalone ESXIMigrations
    RollingMigrationPlan *vjailbreakv1alpha1.RollingMigrationPlan
}
```

All consumers of `scope.RollingMigrationPlan` in the controller must nil-check first.

---

## Phase 2 CRD Changes

### BMConfig — Add Provider Constants and Optional Provider Config

**File**: `k8s/migration/api/v1alpha1/bmconfig_types.go`

```go
const (
    MAASProvider   BMCProviderName = "MAAS"    // existing
    IronicProvider BMCProviderName = "ironic"  // new
    IPMIProvider   BMCProviderName = "ipmi"    // new
)

// IronicConfig holds Ironic-specific configuration
type IronicConfig struct {
    // Endpoint is the Ironic API endpoint URL
    Endpoint string `json:"endpoint"`
    // Username is the OpenStack/Keystone username for Ironic authentication
    Username string `json:"username"`
    // Password is the OpenStack/Keystone password (store in Secret, reference via BMConfig)
    Password string `json:"password"`
    // ProjectID is the OpenStack project for Ironic node operations
    ProjectID string `json:"projectId,omitempty"`
    // DomainName is the Keystone domain (default: "Default")
    DomainName string `json:"domainName,omitempty"`
}

// IPMIConfig holds direct IPMI/Redfish configuration for a single host
type IPMIConfig struct {
    // BMCAddress is the IP or hostname of the BMC
    BMCAddress string `json:"bmcAddress"`
    // Username is the BMC/IPMI username
    Username string `json:"username"`
    // Password is the BMC/IPMI password (store in Secret, reference via BMConfig)
    Password string `json:"password"`
    // Interface is the IPMI interface type (lanplus, lan, etc.)
    Interface string `json:"interface,omitempty"`
    // UseRedfish prefers Redfish REST API over raw IPMI when supported
    UseRedfish bool `json:"useRedfish,omitempty"`
}

// BMConfigSpec updated
type BMConfigSpec struct {
    UserName           string                    `json:"userName,omitempty"`
    Password           string                    `json:"password,omitempty"`
    APIKey             string                    `json:"apiKey,omitempty"`
    APIUrl             string                    `json:"apiUrl,omitempty"`
    Insecure           bool                      `json:"insecure,omitempty"`
    ProviderType       BMCProviderName           `json:"providerType,omitempty"`
    UserDataSecretRef  corev1.SecretReference    `json:"userDataSecretRef,omitempty"`
    BootSource         BootSource                `json:"bootSource,omitempty"`
    // IronicConfig is populated when ProviderType = "ironic"
    IronicConfig       *IronicConfig             `json:"ironicConfig,omitempty"`
    // IPMIConfig is populated when ProviderType = "ipmi"
    IPMIConfig         *IPMIConfig               `json:"ipmiConfig,omitempty"`
}
```

---

## API Changes

### ListVMs — Add Host Filter

**File**: `pkg/vpwned/sdk/proto/v1/api.proto`

```proto
message ListVMsRequest {
    TargetAccessInfo access_info = 1;
    oneof target { ... }  // existing
    // host_name filters VMs to those running on the specified ESXi host.
    // Empty string returns all VMs (no filter applied).
    // If host_name is non-empty and no matching host is found, returns empty list (no error).
    string host_name = 3;
}
```

**File**: `pkg/vpwned/sdk/targets/vcenter/vcenter.go`

Filter VM list by `summary.runtime.host` matching the requested host name after fetching.

---

### Maintenance Mode — New gRPC Method

**File**: `pkg/vpwned/sdk/proto/v1/api.proto`

```proto
service VCenter {
    // existing...
    rpc EnterMaintenanceMode(EnterMaintenanceModeRequest) returns (EnterMaintenanceModeResponse) {
        option (google.api.http) = {
            post: "/vpw/v1/enter_maintenance_mode"
            body: "*"
        };
    }
    rpc ExitMaintenanceMode(ExitMaintenanceModeRequest) returns (ExitMaintenanceModeResponse) {
        option (google.api.http) = {
            post: "/vpw/v1/exit_maintenance_mode"
            body: "*"
        };
    }
}

message EnterMaintenanceModeRequest {
    TargetAccessInfo access_info = 1;
    string host_name = 2;
    // evacuate_powered_off_vms controls whether powered-off VMs are moved
    bool evacuate_powered_off_vms = 3;
}

message EnterMaintenanceModeResponse {
    bool success = 1;
    string message = 2;
}

message ExitMaintenanceModeRequest {
    TargetAccessInfo access_info = 1;
    string host_name = 2;
}

message ExitMaintenanceModeResponse {
    bool success = 1;
    string message = 2;
}
```

---

## UI Component Tree

```
ClusterConversionsPage
├── ESXiClusterAccordion (new)          ← reads VMwareCluster status.hosts[]
│   └── ESXiHostRow (new) × N          ← one per ESXi host
│       ├── [collapsed] header row      ← name, VM count bar, state chip, action buttons
│       └── [expanded] ESXiVMTable      ← VM checkbox table + "Migrate Selected"
└── RollingMigrationsTable (existing)   ← unchanged
```

### New React Query Hooks

| Hook | Source | Purpose |
|---|---|---|
| `useVMwareClustersQuery` | `VMwareCluster` CR list | Cluster list + per-host status |
| (existing) `useESXIMigrationsQuery` | `ESXIMigration` CR list | Conversion progress per host |
