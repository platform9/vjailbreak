# Feature Specification: Agent Node Custom Host Entries

**Feature Branch**: `1890-agent-dns-config`  
**Created**: 2026-05-13  
**Status**: Draft  
**Input**: User description: "ability to be able to add custom DNS (entries or server) in agent nodes without actually needing to log into the agent nodes manually"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Custom Host Entries for Agent Nodes (Priority: P1)

A vJailbreak administrator needs infrastructure hostnames — ESXi hosts, vCenter, PCD endpoints, and OpenStack API endpoints — to be resolvable from agent nodes. Without this, migrations fail because agent nodes cannot reach these systems by hostname. The admin wants to define custom hostname-to-IP mappings once and have them automatically applied to all agent nodes — without SSHing into each one.

**Why this priority**: DNS resolution of infrastructure hostnames (ESXi, vCenter, PCD, OpenStack) is a hard requirement across the entire migration workflow. Failures here block VM discovery, disk copy, and post-migration validation.

**Independent Test**: Can be fully tested by provisioning a new agent node after configuring custom host entries for multiple infrastructure systems (e.g., ESXi host, vCenter, PCD endpoint), then verifying that each hostname resolves correctly on the agent node — and that a migration succeeds end-to-end.

**Acceptance Scenarios**:

1. **Given** an administrator has configured custom host entries (e.g., `192.168.1.101 esxi01.example.com`, `192.168.1.10 vcenter.corp.local`, `192.168.2.5 pcd.corp.local`) in the vJailbreak settings, **When** a new agent node is provisioned, **Then** all configured entries are present in the node's host resolution so that each hostname resolves to the correct IP.

2. **Given** an administrator has not configured any custom host entries, **When** a new agent node is provisioned, **Then** the node's host resolution behavior is unchanged from default.

3. **Given** an administrator adds a new host entry after some agent nodes already exist, **When** the administrator saves the configuration, **Then** the system informs the administrator that the change applies to newly provisioned nodes, and that existing nodes must be reprovisioned to receive it.

4. **Given** an administrator provides a malformed host entry (e.g., missing IP, invalid hostname), **When** they attempt to save the configuration, **Then** the system rejects the entry with a clear error describing what is wrong.

---

### User Story 2 - View and Manage Current Host Entries (Priority: P2)

An administrator wants to review what custom host entries are currently configured for agent nodes, add new ones, edit incorrect entries, and remove stale ones — all through the vJailbreak management interface without accessing any node directly.

**Why this priority**: Operationally necessary for day-2 management as infrastructure changes (new ESXi hosts added, vCenter moved, PCD endpoint updated).

**Independent Test**: Can be fully tested by adding, editing, and removing host entries via the management interface and verifying the stored configuration reflects each change accurately.

**Acceptance Scenarios**:

1. **Given** custom host entries exist, **When** an administrator opens the host entry settings, **Then** all current entries are displayed clearly showing IP and associated hostnames.

2. **Given** an administrator removes a host entry and saves, **When** a new agent node is subsequently provisioned, **Then** the removed entry is not present on the new node.

---

### User Story 3 - Reprovision an Idle Agent Node to Apply Updated Host Entries (Priority: P2)

After correcting a misconfigured host entry, an administrator wants to apply the fix to an existing agent node that received the wrong config — without SSHing into it. They trigger a reprovision from the management interface.

**Why this priority**: Without a reprovision path, fixing a misconfiguration on an existing node requires manual SSH access — exactly what this feature aims to eliminate.

**Independent Test**: Can be fully tested by: (1) provisioning a node with an incorrect host entry, (2) correcting the entry in settings, (3) triggering reprovision from the UI on an idle node, (4) verifying the reprovisioned node has the corrected entry.

**Acceptance Scenarios**:

1. **Given** an agent node has no active migrations, **When** an administrator triggers "Reprovision Node", **Then** the system deprovisions the node and provisions a new one with the current DNS configuration.

2. **Given** an agent node has one or more active migrations, **When** an administrator attempts to trigger "Reprovision Node", **Then** the system blocks the action and displays a clear message identifying the active migrations preventing reprovision.

3. **Given** a reprovision is in progress, **When** the operation completes successfully, **Then** the node returns to a Ready state with the updated DNS configuration applied.

---

### Edge Cases

- What happens when a custom host entry conflicts with an existing system entry on the agent node?
- What happens when a configured DNS server is unreachable — does name resolution fall back gracefully?
- What happens if an agent node is provisioned while DNS configuration is being saved (race condition)?
- What happens when the list of host entries is very large (100+ entries)?
- What if the same hostname is added twice with different IPs?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow administrators to define a list of custom hostname-to-IP mappings (host entries) covering any infrastructure hostname — including ESXi hosts, vCenter, PCD endpoints, and OpenStack API endpoints — that agent nodes need to resolve.
- **FR-002**: Custom host entries MUST be applied automatically to newly provisioned agent nodes without requiring manual access to those nodes.
- **FR-003**: The system MUST validate host entries at input time — each entry must have a valid IP address and at least one valid hostname; invalid entries must be rejected with a clear error.
- **FR-004**: Administrators MUST be able to view, add, edit, and remove individual host entries without affecting other system settings.
- **FR-005**: Host entry changes apply to newly provisioned agent nodes only. The system MUST notify the administrator that existing running nodes are not affected and must be reprovisioned to receive the updated configuration.
- **FR-006**: The system MUST provide a "Reprovision Node" action that destroys and recreates an agent node with the latest host entry configuration. This action MUST be blocked — with a clear error identifying the blocking migrations — if the target node has any active migrations in progress.
- **FR-007**: Custom host entry configuration MUST be persistently stored and survive controller restarts.
- **FR-008**: The system MUST support at least 50 custom host entries.

### Key Entities

- **CustomHostEntry**: A mapping of one IP address to one or more hostnames (mirrors `/etc/hosts` format). Attributes: IP address, list of hostnames. Covers any infrastructure system — ESXi hosts, vCenter, PCD, OpenStack endpoints.
- **AgentHostConfig**: The ordered collection of all custom host entries to be applied to agent nodes. Stored as part of the global vJailbreak configuration.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An administrator can configure custom DNS settings in under 5 minutes without consulting documentation or accessing any node directly.
- **SC-002**: 100% of agent nodes provisioned after a host entry change receive the correct entries — verified by resolving all configured hostnames (ESXi, vCenter, PCD, OpenStack) from the provisioned nodes.
- **SC-003**: Migrations that previously failed due to unresolvable infrastructure hostnames (ESXi, vCenter, PCD) succeed after configuring the corresponding host entries, with no other changes required.
- **SC-004**: Invalid host entries (malformed IPs, empty hostnames) are rejected at input time in 100% of cases, preventing broken configurations from being saved.
- **SC-005**: Host entry configuration is persisted and survives controller restarts — all configured entries are intact after a restart.

## Assumptions

- Agent nodes are provisioned by the vJailbreak controller into OpenStack and receive their configuration via cloud-init at boot time; the feature leverages this existing provisioning path.
- The master/appliance node (the node running the controller itself) is out of scope — DNS configuration for the master is managed separately (e.g., via `/etc/hosts` directly or `/etc/resolv.conf` as documented in the README).
- Administrators have access to the vJailbreak management interface (UI or kubectl) to configure DNS settings; no direct node access is assumed or required for this feature.
- DNS configuration is set at provisioning time (via cloud-init). Existing nodes are not updated in-place — administrators use the "Reprovision Node" action to apply updated config to a running node. Reprovision is blocked while active migrations are running on that node.
- Custom DNS server (nameserver) configuration is out of scope for this feature. Administrators who need custom resolvers must configure DNS at the network/DHCP level or use host entries as the workaround.
- The number of custom host entries is bounded (≤50) to stay within practical cloud-init size limits.
