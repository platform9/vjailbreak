---
title: Network Persistence
description: Guide to Linux and Windows network interface persistence post-migration
---

# Network Persistence Post-Migration Overview

This document details the mechanism for ensuring network interface persistence following a virtual machine migration for both Linux and Windows operating systems.

## Linux Network Persistence

The Linux network persistence mechanism operates on the first boot post-migration to restore network configuration to its pre-migration state.

### Persistence Mechanism

- **Statically Configured Interfaces**: The original names of network interfaces that were statically configured before migration are preserved.
- **DHCP Configured Interfaces**: Interfaces configured via DHCP may be renamed to a consistent pattern: `vjb<random_number>`.
- **Interface with No Configuration**: Interfaces that had no configuration (e.g., were left unconfigured) will remain untouched but the name may change.

### Supported Distributions

| Distribution | Support Status |
| --- | --- |
| Ubuntu 16 | Supported |
| Ubuntu 20 | Supported |
| Ubuntu 22 | Supported |
| Ubuntu 24 | Supported |

## Windows Network Persistence

The Windows network persistence mechanism operates on the first boot post-migration to restore network configuration to its pre-migration state.

### Persistence Mechanism

The network persistence script runs on the first boot post-migration. Its primary function is to restore the network configuration to its pre-migration state by performing the following actions:

- **Statically Configured Interfaces**: The original names of network interfaces that were statically configured before migration are restored.
- **DHCP Configured Interfaces**: Interfaces configured via DHCP are renamed to a consistent pattern: `vjb_<random_number>`.

### Supported Versions

The network persistence mechanism has been validated and is supported on the following Windows Server operating systems:

| Version | Support Status |
| --- | --- |
| Windows Server 2016 | Supported |
| Windows Server 2019 | Supported |
| Windows Server 2022 | Supported |
| Windows Server 2025 | Supported |

## User Guidance for Virtio Installation

The Windows Virtual Machine (VM) will undergo multiple reboots during the installation of necessary virtio drivers post-migration.

:::caution
**Crucial Action**: The user must wait for the virtio installation and subsequent reboots to complete before attempting to log in. Interrupting the installation process by logging in prematurely can lead to an inconsistent network configuration state.
:::
