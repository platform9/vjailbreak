---
title: Network Persistence
description: Guide to Linux and Windows network interface persistence post-migration
---

# Network Persistence Post-Migration

This document details the mechanism for ensuring network interface persistence following a virtual machine migration for both Linux and Windows operating systems.

## Prerequisites

Network persistence is only applied when the **"Persist source network interfaces"** option is enabled in the migration form. This option must be selected during migration configuration to ensure that network interface names are preserved on the destination VM.

## Linux Network Persistence

The Linux network persistence mechanism operates on the first boot post-migration to restore network configuration to its pre-migration state.

### Persistence Mechanism

- **Statically Configured Interfaces**: The original names of network interfaces that were statically configured before migration are preserved.
- **DHCP Configured Interfaces**: Interfaces configured via DHCP may be renamed to a consistent pattern: `vjb<random_number>`.
- **Interface with No Configuration**: Interfaces that had no configuration (e.g., were left unconfigured) will remain untouched but the name may change.

### Supported Distributions

| Distribution | Expected | Verified |
| --- | --- | --- |
|   Ubuntu       | Supported | Yes |
|   OpenSuse     | Supported | Yes |
|   RHEL         | Supported | Yes |
|   CentOS       | Supported | Yes |
|   Rocky        | Supported | No |

## Windows Network Persistence

The Windows network persistence mechanism operates on the first boot post-migration to restore network configuration to its pre-migration state.

### Persistence Mechanism

The network persistence script runs on the first boot post-migration. Its primary function is to restore the network configuration to its pre-migration state by performing the following actions:

- **Windows Server 2016 and Above**:
  - **Statically Configured Interfaces**: The original interface name, IP address, and gateway from the source are persisted on the destination, ensuring continuous network connectivity.
  - **DHCP Configured Interfaces**: Interfaces configured via DHCP are renamed to a consistent pattern: `vjb_<random_number>`.

- **Windows Server 2012**:
  - **Statically Configured Interfaces**: The IP address from the source interface is preserved, but the interface configuration is converted to DHCP. The interface name and gateway are not preserved.

### Supported Versions

The network persistence mechanism has been validated and is supported on the following Windows Server operating systems:

| Version | Expected | Verified |
| --- | --- | --- |
|   Windows Server 2016 | Supported | Yes |
|   Windows Server 2019 | Supported | Yes |
|   Windows Server 2022 | Supported | Yes |
|   Windows Server 2025 | Supported | Yes |
|   Windows Server 2008 | Unsupported | No |
|   Windows Server 2012 | Unsupported | No |

:::caution
**Unsupported Windows Versions**

Windows Server 2008 and Windows Server 2012 are **not supported** for network persistence. Post-migration, VMs running these versions will receive IP addresses via DHCP on all interfaces, regardless of the original network configuration.
:::

## User Guidance for Virtio Installation

The Windows Virtual Machine (VM) will undergo multiple reboots during the installation of necessary virtio drivers post-migration.

:::caution
**Crucial Action**: The user must wait for the virtio installation and subsequent reboots to complete before attempting to log in. Interrupting the installation process by logging in prematurely can lead to an inconsistent network configuration state.
:::

## Important Considerations

:::caution
**Important: Routing Considerations**

If a VM has multiple interfaces on the same subnet and has asymmetric routing table, the destination openstack platform may not support it and drop the packets. This may cause partial connectivity. This is mainly observed when a VM with asymmetric routing is having port-security enabled.

**Recommendation:**
- To avoid asymmetric routing, ensure each interface is on a unique subnet or consolidate multiple IPs onto a single port, as multiple interfaces on the same subnet will cause connectivity issues.
:::

:::note
For DHCP-enabled ports, connectivity and DHCP functionality are preserved, but the interface name may be renamed if this feature is not selected.
:::

:::note
For cross-network migration, network persistence is currently not supported and will be blocked.
:::
