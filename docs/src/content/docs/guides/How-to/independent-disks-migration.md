---
title: "Migrating VMs with Independent Disks"
description: "Identify VMware VMs that use Independent Persistent or Independent Nonpersistent disks and choose a migration option that actually works for them."
---

VMware lets you mark a virtual disk as **Independent**, which excludes it from the VM's snapshot lifecycle. Two of vJailbreak's data-copy paths cannot migrate a disk in this mode at all — one because of a known VMware VDDK bug, the other because it depends on Changed Block Tracking (CBT), which independent disks don't support. This guide explains why, how to identify these disks before you migrate, and which combination of settings actually works.

## What an independent disk is

A normal virtual disk is a `.vmdk` referenced in the VM's configuration. When ESXi snapshots the VM, it freezes that disk as read-only and redirects new writes to a **delta disk** layered on top.

Marking a disk **Independent** in **Edit Settings → Disk → Disk Mode** tells ESXi to skip it entirely when the VM is snapshotted — no delta disk is created for it. There are two variants:

| Disk mode | Behavior |
|---|---|
| **Independent – Persistent** | Writes commit immediately and are never rolled back by a snapshot revert. Common for data that must survive a revert of the OS disk — for example, a transaction log or audit disk. |
| **Independent – Nonpersistent** | Writes are discarded every time the VM powers off or resets, regardless of snapshot state. Used for disks that must always come back clean, such as scratch or kiosk-style disks. |

VMware's own documentation is explicit that the disk keeps existing normally — it just doesn't participate in the snapshot:

> "An independent disk does not participate in virtual machine snapshots. That is, the disk state is independent of the snapshot state and creating, consolidating, or reverting to snapshots does not have effect on the disk."
> — [VMware vSphere documentation: Change Disk Mode to Exclude Virtual Disks from Snapshots](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/7-0/vsphere-virtual-machine-administration/managing-virtual-machinesvm-admin/using-snapshots-to-manage-virtual-machinesvm-admin/take-snapshots-of-a-virtual-machinevm-admin/change-disk-mode-to-exclude-virtual-disks-from-snapshotsvm-admin.html)

## Why this matters for vJailbreak

There isn't a single "independent disks don't work" rule — it depends on which **storage copy method** and which **data copy method** you pick, because each uses a different mechanism to read the source disk.

| Storage copy method | Data copy method | Result | Why |
|---|---|---|---|
| **Normal (Standard)** | Copy live VMs, then power off (Hot) | ❌ Fails | Needs CBT — see below |
| **Normal (Standard)** | Power off VMs, then copy (Cold) | ❌ Fails | VDDK cannot open the disk at all — see below |
| **vJailbreak Accelerated Copy** *(default)* | Copy live VMs, then power off (Hot) | ❌ Fails | Needs CBT — see below |
| **vJailbreak Accelerated Copy** *(default)* | Power off VMs, then copy (Cold) | ✅ Works | No CBT, no VDDK — reads the disk's file directly |
| **Storage-Accelerated Copy** | Either | ✅ Works | No CBT, no VDDK, array-level copy; always runs cold internally |

### Normal (Standard) copy: a VDDK bug, not just CBT

The **Normal** storage copy method uses VMware VDDK to open and read the source disk. This is a known VDDK defect, not something vJailbreak can work around:

> "If the Disk Mode is 'Independent-Persistent' or 'Independent-Nonpersistent', then VDDK ≥ 7 has a bug where it cannot open these disks."
> — [`nbdkit-vddk-plugin(1)` man page](https://libguestfs.org/nbdkit-vddk-plugin.1.html)

**Symptom**: the copy phase fails with a debug line like:

```
nbdkit: vddk[1]: debug: GetFileName: Cannot create disk spec for disk scsi0:0. Error occurred when obtaining the file name for scsi0:0.
```

This happens regardless of Hot or Cold — VDDK fails to open the disk before the copy even starts, so Normal copy cannot migrate a VM with an independent disk in either mode. Use **vJailbreak Accelerated Copy** or **Storage-Accelerated Copy** instead — neither uses VDDK. (VMware's man page also lists downgrading to VDDK 6.7 or earlier as an alternative, but switching storage copy method is simpler and doesn't require managing a different VDDK version.)

### Hot migration (any storage copy method): needs CBT

Hot migration (**Copy live VMs, then power off**) enables VMware Changed Block Tracking and queries a change ID for every disk to copy only the blocks that changed since the last sync. A disk in independent mode never has a change ID — VMware never populates one, with or without CBT enabled at the VM level.

**Symptom**:

```
CBT is not enabled on disk <id>
```

This applies even when using **vJailbreak Accelerated Copy** with Hot migration selected — Hot migration always depends on CBT, independent of which storage copy method is transporting the bytes.

**Cold migration** (**Power off VMs, then copy**) never uses CBT or change IDs — it copies each disk once, in full, while the VM is off.

## Identify independent disks before you migrate

vJailbreak does not surface disk mode in the migration form — check it in vCenter before starting.

**vSphere Client:**

1. Select the VM → **Edit Settings**.
2. Expand each **Hard disk** entry.
3. Check the **Disk Mode** dropdown. Normal disks show **Dependent**; independent disks show **Independent - Persistent** or **Independent - Nonpersistent**.

**PowerCLI:**

```powershell
Get-VM <vm-name> | Get-HardDisk | Select-Object Name, Persistence
```

`Persistence` reports `Persistent`, `IndependentPersistent`, or `IndependentNonPersistent` for each disk.

## Migrating the VM

Use **vJailbreak Accelerated Copy** (the default storage copy method) or **Storage-Accelerated Copy**, with **Power off VMs, then copy** (Cold) as the data copy method. This is the only combination that requires no changes to the source VM and correctly copies the disk regardless of its mode.

### If you need Hot migration for minimal downtime

Change the independent disk back to normal (dependent) mode on the source VM first:

1. Power off the source VM.
2. **Edit Settings** → select the disk → **Disk Mode** → change to **Dependent**.
3. Power the VM back on.
4. Proceed with Hot migration, using **vJailbreak Accelerated Copy** or **Storage-Accelerated Copy** (not Normal — see the VDDK bug above, which is independent of Hot/Cold).

:::caution
Changing a disk's mode away from independent restores normal snapshot behavior for it on the **source** VM going forward — a snapshot revert will now roll this disk back along with the others. Only do this if that trade-off is acceptable for the disk in question (for example, don't do this for a disk that's independent specifically so a database's transaction log survives OS-disk reverts).
:::

### Independent Nonpersistent disks

An Independent - Nonpersistent disk discards its writes on every VM power-off or reset by design — its content at migration time isn't meant to persist anyway. Before migrating this disk:

- Confirm whether the destination VM actually needs this disk's current contents, or whether it should simply be recreated empty/blank on the destination instead.
- If it does need to be copied as-is, use the same Cold + Accelerated/Storage-Accelerated combination as above.

## Related

- [Migration Options](../../../concepts/migration-options/) — Hot vs. cold migration and other copy settings.
- [Known Limitations](../../../reference/known-limitations/#hot-migration-requires-virtual-hardware-version-7-or-newer) — the related CBT/hardware-version limitation.
- [Migrating an RDM disk Windows cluster machine using the CLI](../../cli-api/migrating_rdm_disk_windows_cluster_machine_using_cli/) — a different disk type with its own migration path; don't confuse RDM with independent disks.
