---
title: Windows Network Persistence
description: Guide for Windows network interface persistence post-migration
---

# Windows Network Persistence Post-Migration Overview

This document details the mechanism for ensuring network interface persistence following a virtual machine migration for Windows operating systems. The core component is a post-migration firstboot script executed via a resilient scheduler.

## Persistence Mechanism

The network persistence script runs on the first boot post-migration. Its primary function is to restore the network configuration to its pre-migration state by performing the following actions:

- **Statically Configured Interfaces**: The original names of network interfaces that were statically configured before migration are restored.
- **DHCP Configured Interfaces**: Interfaces configured via DHCP are renamed to a consistent pattern: `vjb_<random_number>`.

## Supported Operating Systems

The network persistence mechanism has been validated and is supported on the following Windows Server operating systems:

| Operating System | Support Status |
| --- | --- |
| Windows Server 2016 | Supported |
| Windows Server 2019 | Supported |
| Windows Server 2022 | Supported |
| Windows Server 2025 | Supported |

## Firstboot Scheduler and Resilience

The firstboot scheduler is designed to manage the execution of multiple post-migration scripts, including the network persistence script, with robustness against common post-migration challenges.

### Scheduler Features

- **Exponential Backoff**: Scripts are executed with an exponential backoff mechanism to ensure repeated attempts under transient failure conditions.
- **Resilience to Reboots**: The scheduler is designed to be resilient to reboots caused by the installation of virtio drivers, ensuring script execution resumes appropriately.

## User Guidance for Virtio Installation

The Windows Virtual Machine (VM) will undergo multiple reboots during the installation of necessary virtio drivers post-migration.

:::caution
**Crucial Action**: The user must wait for the virtio installation and subsequent reboots to complete before attempting to log in. Interrupting the installation process by logging in prematurely can lead to an inconsistent network configuration state.
:::

## User-Defined Firstboot Script (user_firstboot)

The `user_firstboot` script is the final script executed by the scheduler. This is an **optional** mechanism for users to inject custom post-migration logic.

:::note
**Requirement**: The user is required to enter a valid PowerShell script in the designated field when filling out the migration form. This script will be executed at the last stage of the firstboot scheduler.
:::
