# Feature Specification: Hot-Add Proxy Migration

**Feature Branch**: `1944-hot-add-proxy`  
**Created**: 2026-05-19  
**Status**: Implemented  

## Overview

This feature introduces a new data copy method — **Hot-Add** — to vJailbreak's VM migration workflow. Rather than transferring disk data directly from the source VM over the network, the Hot-Add method attaches snapshot-frozen disk images from the source VM to a dedicated Proxy VM (running on the same vCenter/datastore as the source), then streams the data from the Proxy VM to the destination disk on the vJailbreak appliance via a block-level network protocol. This approach can significantly reduce migration time for large disks and avoids saturating the network path between vCenter and vJailbreak.

Only the data-copy phase changes; all other migration steps (credential validation, PCD provisioning, disk conversion, post-migration health checks) remain unchanged from the existing migration flow.

---

## Clarifications

### Session 2026-05-19

- Q: What is the concurrency limit for multiple Hot-Add migrations sharing the same Proxy VM? → A: The vSphere hardware limit of 60 simultaneously attached disks per VM is the governing constraint. Concurrent migrations share this capacity; a new migration is accepted only if its disk count fits within the remaining slots.
- Q: What should the system do if a snapshot with the migration's name already exists on the source VM? → A: Delete the existing snapshot and create a fresh one; proceed without error.
- Q: How does the system handle a Proxy VM reboot or crash during active data transfer? → A: Retry the data transfer automatically up to 3 attempts before marking the migration failed.
- Q: What should the system do if a disk is attached to the Proxy VM but the guest OS cannot find its block device by UUID? → A: Retry block device identification up to 3 times (with a short wait between attempts) to handle transient guest OS delays; fail the migration with a descriptive error if identification still fails after all retries.
- Q: What happens if disk.EnableUUID is toggled off on the Proxy VM after it reaches "Ready" status? → A: Trust the "Ready" status; no extra pre-migration check is performed. The block device identification retry logic handles the resulting failure and surfaces a descriptive error.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Register and Verify Proxy VM (Priority: P1)

An operator needs to register a Proxy VM with vJailbreak before any Hot-Add migrations can run. They navigate to the Proxy VM section in the sidebar, provide the VM's name, and vJailbreak validates that all required components are installed and the VM is reachable. The operator is informed of any missing components and guided to resolve them before the Proxy VM status is set to "Ready."

**Why this priority**: Without a registered and verified Proxy VM, Hot-Add migrations cannot start. This is the foundational gate for the entire feature.

**Independent Test**: Can be fully tested by registering a Proxy VM and verifying the status transitions (Pending → Verifying → Ready / Failed with clear error messages) without running any actual migration.

**Acceptance Scenarios**:

1. **Given** a Proxy VM with all required components installed and `disk.EnableUUID=True`, **When** the operator registers the VM by name in the UI, **Then** vJailbreak verifies connectivity and component presence, and marks the Proxy VM status as "Ready."

2. **Given** a Proxy VM that is missing one or more required components (e.g., the network block device server is not installed), **When** the operator registers the VM, **Then** vJailbreak reports which components are missing and the Proxy VM status is set to "Verification Failed" with an actionable error message.

3. **Given** a Proxy VM where `disk.EnableUUID` is not enabled, **When** vJailbreak detects this during verification, **Then** the system automatically enables the property, reboots the VM, and continues verification; the operator is notified that a reboot occurred.

4. **Given** a registered Proxy VM in "Verification Failed" state, **When** the operator clicks the "Retry Verification" button in the UI, **Then** the system re-triggers the full verification flow (connectivity + component checks + disk.EnableUUID) and updates the status accordingly.

5. **Given** a registered Proxy VM that was previously "Ready," **When** the operator re-runs verification via the retry button, **Then** the status is refreshed accurately.

6. **Given** an operator has not yet added their vJailbreak appliance's public key to the Proxy VM's authorized keys(root), **When** they attempt to register the Proxy VM, **Then** the UI displays a clear pre-requisite message explaining the SSH key setup step before registration is attempted.

7. **Given** VMware credentials are selected in the Add Proxy VM form, **When** the operator opens the VM Name field, **Then** the system fetches and displays all available VMs from those credentials as a searchable dropdown; the operator can also type a name not in the list.

---

### User Story 2 - Initiate a Hot-Add Migration (Priority: P1)

An operator initiates a VM migration and selects "Hot-Add" as the data copy method. vJailbreak validates that a Proxy VM is registered and in the "Ready" state before allowing the migration to proceed. If the Proxy VM is not ready, the migration is blocked with a clear explanation.

**Why this priority**: This is the primary user-facing workflow change. Operators must be able to select and use the new data copy method through the existing migration UI.

**Independent Test**: Can be fully tested by attempting to start a Hot-Add migration with a ready Proxy VM (proceeds), and again without a ready Proxy VM (blocked with error), independently of verifying data integrity.

**Acceptance Scenarios**:

1. **Given** a Proxy VM in "Ready" state, **When** the operator initiates a migration and selects "Hot-Add" as the data copy method, **Then** the migration is accepted and begins the standard provisioning flow followed by the Hot-Add data transfer.

2. **Given** no Proxy VM is registered, **When** the operator selects "Hot-Add" as the data copy method, **Then** the system blocks the migration and displays a message instructing the operator to register and verify a Proxy VM first.

3. **Given** a registered Proxy VM that is not in "Ready" state (e.g., "Verification Failed"), **When** the operator attempts a Hot-Add migration, **Then** the migration is blocked with the current Proxy VM status shown and remediation guidance provided.

4. **Given** the operator selects a data copy method other than "Hot-Add," **Then** the existing migration flow is used unchanged — no regression.

---

### User Story 3 - Successful Data Transfer via Proxy VM (Priority: P1)

During a Hot-Add migration, vJailbreak takes a snapshot of the source VM, attaches the snapshot's frozen disk images to the Proxy VM, transfers the data to the destination disk, and then cleans up all snapshot and disk attachments. The operator observes progress in the migration status view and receives confirmation upon completion.

**Why this priority**: This is the core data-transfer behaviour. Without it, the feature provides no value.

**Independent Test**: Can be fully tested end-to-end for a single-disk VM: start migration, observe data copy progress, confirm completion, and verify cleanup (no dangling snapshots or attached disks on the Proxy VM).

**Acceptance Scenarios**:

1. **Given** a migration in the "Data Copy" phase using Hot-Add, **When** the system attaches snapshot disks to the Proxy VM and identifies corresponding block devices, **Then** data transfer begins and progress is visible in the migration status view.

2. **Given** data transfer has completed successfully, **When** cleanup runs, **Then** the snapshot is removed from the source VM, all disks are detached from the Proxy VM, nbd server running on the decided port is also stopped and no orphaned resources remain in vCenter.

3. **Given** a multi-disk VM, **When** a Hot-Add migration runs, **Then** all disks are transferred correctly and cleanup is complete for each disk.

4. **Given** any failure during the data transfer phase (e.g., Proxy VM becomes unreachable), **When** the failure is detected, **Then** the migration is marked failed, cleanup is attempted (best effort), and the operator sees a clear failure reason.

---

### User Story 4 - View and Manage Proxy VMs (Priority: P2)

An operator can view the list of registered Proxy VMs, their current status, and remove a Proxy VM from the registry. The UI entry point mirrors the pattern used for other registered resources in the sidebar.

**Why this priority**: Operators need lifecycle management for Proxy VMs; without it, stale entries accumulate. Lower priority because migrations can function once the first Proxy VM is registered.

**Independent Test**: Can be fully tested by adding, viewing, and removing Proxy VMs through the UI without running any migration.

**Acceptance Scenarios**:

1. **Given** at least one Proxy VM has been registered, **When** the operator navigates to the Proxy VM section, **Then** they see a list of all registered Proxy VMs with name, status, and registration date.

2. **Given** a listed Proxy VM, **When** the operator removes it, **Then** the Proxy VM is deregistered, and any pending Hot-Add migrations that depend on it are blocked (not silently broken).

---

### Edge Cases

- What happens if the Proxy VM runs out of available ports for serving block devices?
- **Proxy VM crash during transfer (resolved)**: If the Proxy VM becomes unreachable during data transfer, the system retries the transfer automatically up to 3 attempts. If all retries are exhausted, the migration is marked failed, cleanup is attempted (best effort), and the operator sees a clear failure reason indicating the Proxy VM was unreachable.
- What if the source VM has no disks (or only OS disks with no additional data disks)?
- **Snapshot name collision (resolved)**: If a snapshot with the migration's designated name already exists on the source VM, the system deletes it and creates a fresh snapshot before proceeding; no error is raised.
- **disk.EnableUUID toggled off post-Ready (resolved)**: The system trusts the existing "Ready" status and does not re-validate this property at migration start. If it has been disabled, the block device identification retry logic will catch the failure and surface a descriptive error to the operator.
- **Multiple concurrent migrations (resolved)**: The Proxy VM supports a maximum of 60 simultaneously attached disks (vSphere hardware limit). Before attaching disks for a new migration, the system checks that the aggregate attached-disk count across all active migrations will not exceed 60. If adding the new migration's disks would breach this limit, the migration is blocked with a clear capacity message indicating how many slots are currently in use.
- **Block device mapping failure (resolved)**: If a disk is confirmed attached by vCenter but the Proxy VM's guest OS cannot find the corresponding block device by UUID, the system retries identification up to 3 times with a short wait between attempts (to handle transient guest OS delays in surfacing newly attached disks). If identification still fails after all retries, the migration is marked failed with a descriptive error identifying the affected disk.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a "Proxy VM" section in the sidebar navigation that allows operators to register, view, and remove Proxy VMs.

- **FR-002**: During Proxy VM registration, the system MUST validate SSH connectivity from the vJailbreak appliance to the Proxy VM.

- **FR-003**: During Proxy VM verification, the system MUST check that all required block-device and disk-serving utilities are installed and available on the Proxy VM.

- **FR-004**: The system MUST check and, if needed, enable the vCenter disk unique ID property on the Proxy VM during verification, triggering a reboot if the property was changed.

- **FR-005**: The system MUST display a clear status for each registered Proxy VM (e.g., Pending, Verifying, Ready, Verification Failed) and surface actionable error messages when verification fails.

- **FR-006**: The migration creation UI MUST offer "Hot-Add" as a selectable data copy method alongside existing methods.

- **FR-007**: Before accepting a Hot-Add migration, the system MUST validate that a Proxy VM with "Ready" status exists; if not, the migration MUST be blocked with a descriptive error.

- **FR-008**: During a Hot-Add migration's data copy phase, the system MUST create a snapshot of the source VM (first removing any pre-existing snapshot with the same name), attach the snapshot's frozen disk images to the registered Proxy VM, and identify each attached disk as a block device within the Proxy VM. If a disk's block device cannot be identified, the system MUST retry identification up to 3 times with a short wait between attempts before failing the migration with a descriptive error.

- **FR-009**: The system MUST expose each attached disk on the Proxy VM as a network block device resource on an available port, and stream data from that resource to the corresponding destination disk attached to the vJailbreak appliance.

- **FR-010**: Upon successful completion of data transfer for all disks, the system MUST remove the snapshot from the source VM and detach all transferred disks from the Proxy VM in vCenter.

- **FR-011**: If the Proxy VM becomes unreachable during data transfer, the system MUST automatically retry the transfer up to 3 times before marking the migration failed. After all retries are exhausted, the system MUST attempt cleanup (snapshot removal, disk detachment) and report the failure reason in the migration status.

- **FR-012**: All migration phases other than data copy (provisioning, disk conversion, post-migration health checks) MUST be unaffected by the Hot-Add method selection.

- **FR-013**: The UI pre-requisite instructions for Proxy VM setup MUST include a step informing the operator to copy the vJailbreak appliance's public SSH key to the Proxy VM before registration.

- **FR-014**: Before attaching disks for a Hot-Add migration, the system MUST check the current number of disks attached to the Proxy VM across all active migrations and reject the new migration with a descriptive capacity error if adding its disks would exceed 60 total attached disks.

- **FR-015**: The Proxy VM list page MUST display a "Retry Verification" button for any Proxy VM whose status is "Verification Failed." Clicking it MUST re-trigger the full verification flow without deleting and re-creating the Proxy VM resource. The retry mechanism patches the ProxyVM resource with a timestamp annotation (`vjailbreak.k8s.pf9.io/retry-at`) to trigger controller reconciliation.

- **FR-016**: The Add Proxy VM dialog MUST fetch all available VMs from the selected VMware credentials and present them as a searchable dropdown in the VM Name field. Free-text input MUST also be accepted so operators can specify a VM not yet discovered by the credentials label selector.

### Key Entities

- **Proxy VM**: A VM registered with vJailbreak to serve as an intermediary for Hot-Add data copy. Attributes: name, IP address, SSH accessibility status, component verification status, overall readiness status, registration timestamp, current attached-disk count (max 60).

- **Hot-Add Migration**: A migration record where the data copy method is "Hot-Add." Carries a reference to the Proxy VM used and the snapshot created for the copy.

- **Snapshot**: A point-in-time frozen copy of the source VM's disks created at the start of the data copy phase. Must be cleaned up after the transfer is complete.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can register, verify, and reach "Ready" status for a correctly configured Proxy VM in under 5 minutes.

- **SC-002**: A Hot-Add migration for a single-disk VM completes the data copy phase with all cleanup steps (snapshot removal, disk detachment) confirmed, with no orphaned resources left in vCenter.

- **SC-003**: Selecting a data copy method other than "Hot-Add" produces identical migration behaviour to the existing flow — zero regressions in existing copy methods.

- **SC-004**: When a Hot-Add migration is attempted without a ready Proxy VM, 100% of such attempts are blocked before provisioning begins, with a user-visible reason displayed.

- **SC-005**: Verification failures surface at least one specific, actionable error message per failed check (missing component, connectivity failure, UUID property not set).

- **SC-006**: Multi-disk VM migrations via Hot-Add successfully transfer all disks and clean up all corresponding snapshots and attachments.

- **SC-007**: After clicking "Retry Verification" on a failed Proxy VM, the status transitions from "Verification Failed" back through "Verifying" and reaches "Ready" or "Verification Failed" (with updated messages) within 60 seconds for a reachable VM.

- **SC-008**: The Add Proxy VM form populates the VM Name dropdown within 3 seconds of selecting VMware credentials for a vCenter with up to 500 VMs.

---

## Assumptions

- The Proxy VM is a Linux-based VM running on the same vCenter and accessible by the vJailbreak appliance over SSH.
- Only one Proxy VM is required to be registered at a time for the initial version; the feature does not need to support load-balancing across multiple Proxy VMs (this can be a future enhancement).
- The Proxy VM is pre-provisioned by the operator; vJailbreak does not create or provision the Proxy VM itself.
- The operator is responsible for manually copying the vJailbreak appliance's public SSH key to the Proxy VM before registration — this step is instructional only and not automated by the system.
- Port selection for disk serving on the Proxy VM is handled automatically by the system within an available range; the operator does not specify ports manually.
- Block device identification on the Proxy VM relies on the disk unique ID (UUID/wwid) provided by vCenter, which is why `disk.EnableUUID=True` is required.
- The "Hot-Add" name in the UI is the user-visible label; the underlying mechanism is consistent with the description above.
- The existing SAM (Storage Accelerated Copy) feature serves as the UI and backend reference pattern for introducing this new data copy method.
- Cleanup is best-effort on failure: the system will attempt to remove snapshots and detach disks, but in severe failure cases (e.g., vCenter unreachable), manual cleanup may be required.
- The feature does not require changes to the OpenStack side of the migration workflow — disk attachment and identification on the destination side reuse the existing mechanism.
