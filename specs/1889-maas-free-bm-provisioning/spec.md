# Feature Specification: Per-ESXi Cluster Conversion + MAAS-Free BM Provisioning

**Feature Branch**: `1889-maas-free-bm-provisioning`
**Created**: 2026-05-08
**Status**: Approved (derived from design doc)
**Design Doc**: `docs/superpowers/specs/2026-05-08-per-esxi-cluster-conversion-design.md`

---

## Phase 1: Per-ESXi Conversion UI + Standalone ESXIMigration

### User Story 1 — Per-ESXi VM Migration (Priority: P1)

An operator managing a VMware cluster wants to drain VMs from one ESXi host at a time
before converting it, rather than converting the entire cluster in one shot.

**Why this priority**: Core adoption blocker. Operators need partial/incremental migration.

**Independent Test**: Open Cluster Conversions page, expand a cluster, see ESXi host rows
with VM counts. Select VMs on one host, open migration form pre-populated with those VMs.

**Acceptance Scenarios**:

1. **Given** a VMware cluster with 3 ESXi hosts each running VMs, **When** user opens the
   cluster detail view, **Then** each ESXi host row shows VM count and a "Migrate VMs" button
2. **Given** a host row is expanded, **When** user selects VMs and clicks "Migrate Selected",
   **Then** the existing migration form opens pre-populated with the selected VMs
3. **Given** a host has 0 VMs, **When** user views the row, **Then** "Migrate VMs" button
   is hidden

---

### User Story 2 — Maintenance Mode + PCD Host Conversion (Priority: P2)

An operator wants to put an ESXi host in maintenance mode from vJailbreak UI, then convert
it to a PCD host once it is empty.

**Why this priority**: Completes the per-ESXi workflow end-to-end.

**Independent Test**: Click "Put in Maintenance" on any ESXi row. Verify host enters
maintenance mode in vCenter. On an empty + maintenance host, click "Convert to PCD Host".
Verify ESXIMigration CR is created and row shows conversion progress.

**Acceptance Scenarios**:

1. **Given** any ESXi host, **When** user clicks "Put in Maintenance", **Then** host enters
   vCenter maintenance mode and row state updates to Maintenance
2. **Given** host VM count = 0 AND host is in Maintenance mode, **When** user views the row,
   **Then** "Convert to PCD Host" button appears
3. **Given** user clicks "Convert to PCD Host", **When** confirmed, **Then** standalone
   ESXIMigration CR is created (without requiring ClusterMigration parent) and row shows
   conversion progress phases

---

## Phase 2: MAAS-Free Bare-Metal Provisioning

### User Story 3 — Ironic Provider (Priority: Phase 2 - P1)

An operator running Platform9 PCD (which includes OpenStack Ironic) wants to convert ESXi
hosts to PCD hosts without installing MAAS.

**Why this priority**: Primary MAAS-free path for PCD users.

**Independent Test**: Configure BMConfig with ProviderType=ironic and Ironic endpoint.
Trigger ESXi→PCD conversion. Verify Ironic node is enrolled, provisioned, and PCD host
appears without MAAS.

**Acceptance Scenarios**:

1. **Given** BMConfig with ProviderType=ironic, **When** ESXIMigration runs ConvertingToPCDHost
   phase, **Then** Ironic API is used for node provisioning instead of MAAS
2. **Given** existing MAAS BMConfig, **When** ESXIMigration runs, **Then** MAAS provider
   is used as before — no regression
3. **Given** BMConfig with ProviderType=ironic and Ironic enrollment is complete, **When** ESXIMigration reaches ConfiguringPCDHost phase, **Then** host appears in the PCD cluster using the PCDClusterRef from ESXIMigrationSpec without any MAAS interaction

---

### User Story 4 — Direct IPMI/Redfish Provider (Priority: Phase 2 - P2)

An operator without MAAS or Ironic wants to convert ESXi hosts using only BMC access.

**Why this priority**: Broadest hardware support, lowest infrastructure requirement.

**Independent Test**: Configure BMConfig with ProviderType=ipmi and BMC credentials.
Trigger conversion. Verify IPMI calls set PXE boot and power cycle, cloud-init served
from vJailbreak HTTP endpoint.

**Acceptance Scenarios**:

1. **Given** BMConfig with ProviderType=ipmi and BMC credentials, **When** ESXIMigration
   runs, **Then** IPMI is used directly for boot device + power management
2. **Given** hardware with Redfish support, **When** ProviderType=ipmi, **Then** Redfish
   REST API is preferred over raw IPMI if available

---

## Edge Cases

- **EC-001**: ESXi host unreachable at polling time — VMwareCluster controller must not overwrite last-known status; set a `LastPollError` condition instead
- **EC-002**: User clicks "Put in Maintenance" while a migration is running on that host — UI MUST warn but allow; vCenter enforces safety
- **EC-003**: VMwareCluster deleted while ESXIMigration is in progress — ESXIMigration continues independently; VMwareCluster controller reconcile-loop exits cleanly
- **EC-004**: BMConfigRef points to non-existent BMConfig — ESXIMigration transitions to Failed with descriptive error message
- **EC-005**: Ironic endpoint unreachable during provisioning — ESXIMigration retries up to 3 times with exponential backoff before transitioning to Failed

## Requirements

### Functional Requirements

- **FR-001**: UI MUST show per-ESXi VM count on the cluster detail page, refreshed from k8s
- **FR-002**: "Migrate VMs" MUST be visible only when VM count > 0
- **FR-003**: "Put in Maintenance" MUST be available on any ESXi host at any time
- **FR-004**: "Convert to PCD Host" MUST appear only when VM count = 0 AND host is in maintenance
- **FR-005**: ESXIMigration CR MUST be creatable without a ClusterMigration parent
- **FR-006**: VMwareCluster controller MUST poll vCenter for per-host VM counts and maintenance state
- **FR-007**: BMConfig MUST support ProviderType values: MAAS (existing), ironic, ipmi
- **FR-008**: MAAS provider behavior MUST be unchanged for existing BMConfig objects
- **FR-009**: "Put in Maintenance" button MUST call EnterMaintenanceMode API and wait for confirmation before updating row state
- **FR-010**: ListVMs API MUST support an optional `host_name` filter parameter; omitting it returns all VMs

### Key Entities

- **VMwareCluster**: source cluster CRD; gains `status.hosts[]` with VM count + maintenance state
- **ESXIMigration**: per-host conversion CRD; `rollingMigrationPlanRef` becomes optional
- **BMConfig**: bare-metal provider config; gains new ProviderType values
- **ESXiHostRow**: new UI component — collapsible row in cluster detail view
- **IronicProvider**: new BMCProvider implementation for OpenStack Ironic
- **IPMIProvider**: new BMCProvider implementation for direct IPMI/Redfish

## Success Criteria

- **SC-001**: Operator can drain and convert a single ESXi host without touching any other host in the cluster; full conversion completes within 4 hours for a host with 0 remaining VMs
- **SC-002**: ESXi→PCD conversion succeeds with Ironic provider on a PCD environment without MAAS installed
- **SC-003**: Existing MAAS-based ClusterMigration workflows continue to work without modification
- **SC-004**: VM count on ESXi rows reflects actual vCenter state within 60 seconds

## Assumptions

- PCD environments may or may not have Ironic; Direct IPMI covers environments without it
- ESXi hosts have IPMI/BMC accessible from the vJailbreak VM network
- Existing migration form/wizard is reused as-is for VM selection flow
- govmomi library supports EnterMaintenanceMode via HostSystem.EnterMaintenanceMode
