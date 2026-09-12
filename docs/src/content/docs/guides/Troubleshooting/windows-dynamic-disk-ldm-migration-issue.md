---
title: Windows Dynamic Disk (LDM) migration issue
description: Windows VMs with a dynamic disk (LDM) system volume are supported through a dedicated migration path.
sidebar:
  hidden: true
---

:::note[This page has moved]
Windows VMs with an LDM system volume are supported. Everything about this
migration path — prerequisites, what vJailbreak does, the cutover, and
troubleshooting — is documented in one place:
[Windows Dynamic Disk (LDM) Migration](../../how-to/windows-ldm-migration/).
:::

## Why LDM needs a different path

**LDM (Logical Disk Manager)** is Windows' volume manager for "dynamic disks". It
is conceptually similar to Linux LVM, but it stores its volume metadata in a
private database at the end of each disk rather than in a standard partition
table.

`virt-v2v` converts a guest by inspecting it with libguestfs, reading the
`SYSTEM` and `SOFTWARE` registry hives with Hivex, and writing VirtIO drivers and
registry changes back into the offline filesystem. When the Windows system volume
is an LDM volume, libguestfs cannot reliably assemble it, so inspection fails
before conversion can start.

Rather than requiring the disk to be converted to basic beforehand, vJailbreak
skips conversion for these guests and brings the VM up on an emulated SATA
controller, which Windows can boot without VirtIO drivers. A temporary virtio
temporary disk lets Windows install the `viostor` driver itself, and the migration
then waits at the **LDM Boot Verification** phase for you to move the root disk to
virtio.

:::caution
Only the **system volume** matters. If just the data disks are dynamic, the VM
migrates normally and no special handling is needed.
:::

## Converting to basic disks is no longer required

Earlier versions of this guide recommended running `diskpart` → `convert basic` on
the source VM before migrating. That is no longer necessary, and `convert basic`
requires an empty disk in any case. Follow the
[LDM migration guide](../../how-to/windows-ldm-migration/) instead.
