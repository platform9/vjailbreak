---
title: "Windows Dynamic Disk (LDM) Migration"
description: "Migrate Windows VMs whose system volume is on a dynamic disk (LDM), including the prerequisites and the LDM Boot Verification cutover."
---

Windows VMs whose **system volume sits on a dynamic disk (LDM)** follow a different
migration path. `virt-v2v` cannot convert these guests, so vJailbreak brings the VM
up on an emulated SATA controller first and lets you move it to virtio once you have
confirmed it boots.

vJailbreak detects this automatically during the migration, but you have to prepare the source VM for such VMs. There is nothing to select in the migration form.
form.

:::note[Only the system volume matters]
If just the **data disks** are dynamic, none of this applies. The VM migrates
normally, with conversion and every post-migration step running as usual.
:::

Because conversion is skipped, these do not run for LDM guests: **VMware Tools
removal**, **network persistence** and **user firstboot scripts**. That is why the
tasks below are manual.

## 1. Before you start

vJailbreak detects LDM on its own, but if you want to know in advance which VMs
will take this path, run the precheck script on the source VM as Administrator. It
is read-only, prints a plain **YES** or **NO**, and writes a transcript to
`%TEMP%\vjb-ldm-check.log`.

<a href="../../../scripts/Test-VjbLdmSystemDisk.ps1" download>Download Test-VjbLdmSystemDisk.ps1</a>

```powershell
powershell -ExecutionPolicy Bypass -File .\Test-VjbLdmSystemDisk.ps1
```

It also sets an exit code — `1` for LDM, `0` for basic, `2` if inconclusive — so it
can be run across a fleet to build the list of VMs that need the steps below.

**Take a snapshot of the source VM in vCenter before making any of the changes
below.** Both steps modify the guest, and the driver installation requires a
reboot. The snapshot is your way back if either one leaves the VM in a state you
did not intend.

On the source VM, as Administrator:

1. **Install the version-compatible VirtIO drivers.** Mount the virtio-win ISO that
   matches the guest, run `virtio-win-guest-tools.exe`, accept the defaults, reboot.

   | Guest | virtio-win ISO |
   | --- | --- |
   | Windows Server 2016 and later | Current stable release |
   | Windows Server 2012 / 2012 R2 | `virtio-win-0.1.185.iso` |

   :::caution
   Server 2012 and 2012 R2 must use the pinned **0.1.185** build. Current
   virtio-win releases have dropped support for them, and installing one leaves the
   guest without a usable storage driver.
   :::

   Nothing will look different afterwards — there is no virtio device in vCenter
   yet, so the drivers sit staged until one appears at the destination.

2. **Set the SAN policy.** Windows marks migrated disks offline because the
   controller changed; skipping this leaves the LDM pool broken.

   ```
   diskpart
   san policy=onlineall
   exit
   ```

## 2. Trigger the migration

Start the migration as usual. vJailbreak skips conversion, creates the VM with
`hw_disk_bus: sata`, and attaches a **1 GB virtio probe disk**. Windows performs a
real driver installation against that device on first boot, which is what gets the
VirtIO storage driver installed and bound — offline injection cannot do this. The
probe disk is temporary and is removed later.

The status of the migration then changes to **LDM Boot Verification**, and the
migration waits for you to perform the cutover.

## 3. Confirm the VM booted

Open the console of the new VM in PCD and log in. What you should check inside the guest is that
Windows bound a VirtIO driver to the probe disk — if it did, the root disk will work
on virtio too. Below are the commands to check that.

**Run these before performing the cutover.** The temporary disk only exists while the
migration is held at **LDM Boot Verification**; it is removed whichever option you
select, so the output changes afterwards.

```powershell
# The 1 GB probe disk must be present, with VirtIO in its model name.
Get-WmiObject Win32_DiskDrive | Select-Object Model, Size

# The controller must be healthy.
Get-WmiObject Win32_PnPEntity | Where-Object { $_.Name -like '*VirtIO*' } |
  Select-Object Name, Status
```

Expect a disk of roughly 1 GB whose model names VirtIO, and a controller reporting
`OK`. Device Manager shows the same thing under **Storage controllers**. Both
commands work on every supported Windows version, including Server 2012.

:::caution[Do not use `sc query viostor`]
On a VM booted from SATA this reports `STOPPED` even when the driver is installed
and working, so it will make a healthy VM look broken. The service state of a
storage miniport is not a reliable signal here — check for the device, as above.
:::

If you re-run the same commands after the cutover, expect different output.

Once above is verified, you will have 3 cutover options:

| Cutover option | What the checks show afterwards |
| --- | --- |
| **Move to virtio** | The temporary disk is gone and the **root** disk now reports a VirtIO model. This is the successful end state. |
| **Keep on SATA** | The probe disk is gone and no VirtIO disk remains, because the root disk stayed on SATA. A VirtIO controller may linger in Device Manager as a non-present device. Expected — not a failure. |
| **Rollback Migration** | The VM no longer exists in PCD. |

## 4. Perform the cutover

There is no timeout. The migration remains at **LDM Boot Verification** until the
cutover is performed, so it can be scheduled for a maintenance window.

### Cutover from the UI

Click the cutover button on the migration, either in the migrations table or on the
migration details page. A confirmation dialog appears with three options:

| Choice | Result |
| --- | --- |
| **Move to virtio** | The VM is shut down, deleted and recreated with the root disk on virtio, keeping its name, IP and MAC. |
| **Keep on SATA** | The migration completes with the VM disk left on SATA. |
| **Rollback Migration** | The VM is deleted from PCD and the source VM in vCenter is returned to its pre-migration state. |

**Leave the VM running** — the shutdown is handled for you. Expect a short outage
while it is recreated; the phase shows **Moving to virtio** during the rebuild.

If the checks in step 3 did not pass, select **Keep on SATA**. The VM remains fully
functional on the SATA controller; only the performance benefit of virtio is lost.
Select **Rollback Migration** only if the VM did not boot at all.

### Cutover using kubectl patch

The cutover can also be performed by patching the migration `Pod`.

```bash
kubectl get migration <migration-name> -n migration-system -o jsonpath='{.spec.podRef}'

kubectl patch pod <pod-name> -n migration-system \
  -p '{"metadata":{"labels":{"ldmBootStatus":"success"}}}'
```

| Label value | Equivalent option |
| --- | --- |
| `success` | Move to virtio |
| `finish` | Keep on SATA |
| `failed` | Rollback Migration |

## Troubleshooting

### Disks show "Failed Redundancy"

Seen on mirrored LDM volumes when the SAN policy was not set beforehand. Windows
marked a disk offline, so the mirror ran on one plex; by the time the disk returned,
the copies had diverged and LDM refuses to merge them.

Confirm which disk is stale with `detail volume` or Disk Management, then:

```
diskpart> san policy=onlineall
Set-Disk -Number 2 -IsOffline $false
diskpart> select volume 0 ; online volume
diskpart> select volume 0 ; break disk=2 nokeep
diskpart> select volume 0 ; add disk=2
```

:::danger
`break ... nokeep` deletes the named disk's plex. Confirm the disk number first —
breaking the **live** disk destroys the copy the VM has been running from.
:::

No data is lost when the correct disk is named. There is no redundancy while the
mirror resyncs, but the window is bounded.

### The VM did not boot on SATA

Perform the cutover with **Rollback Migration**, correct the prerequisites, and
migrate again.
