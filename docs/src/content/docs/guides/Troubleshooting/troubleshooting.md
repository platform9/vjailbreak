---
title: Troubleshooting vJailbreak 
description: Tips on effectively troubleshooting vJailbreak deployment and migration
---

:::note
All of the following Kubernetes commands will need to be run from the vJailbreak VM, or remotely using the vJailbreak VM's kubeconfig, located at `/etc/ranger/k3s/k3s.yaml` on the vJailbreak VM.
:::
 
## Common issues

- [Windows Dynamic Disk (LDM) migration](../../how-to/windows-ldm-migration/)
- [nbdcopy fails during disk copy (often DNS resolution)](nbdcopy-fails-after-vm-moved-esxi-host/)
- [virt-v2v fails: rename /sysroot/etc/resolv.conf Operation not permitted](#virt-v2v-fails-rename-sysrootetcresolvconf-operation-not-permitted)
- [virt-v2v-in-place fails on RHEL 7: missing GRUB compatibility symlink](#virt-v2v-in-place-fails-on-rhel-7-missing-grub-compatibility-symlink)
- [Proxy VM disk attach fails when several migrations start together](#proxy-vm-disk-attach-fails-when-several-migrations-start-together)
- [vJailbreak Accelerated Copy fails: could not identify block device](#vjailbreak-accelerated-copy-fails-could-not-identify-block-device)

vJailbreak is deployed on Kubernetes running on Ubuntu 22.04.5, and distributed as a QCOW2 image. The Kubernetes namespace `migration-system` contains the vJailbreak UI and migration controller pods. Each VM migration will spawn a migration object. The status field contains a high level view of the progress of the migration of the VM. For more details about the migration, check the logs of the pod specified in the Migration object.

### Getting logs
List all pods in the migration namespace
```bash
kubectl -n migration-system get pod
```

Find a specific VM migration pod
```bash
kubectl -n migration-system get pod | grep <source VM name>
```

Get details & events for a v2v-helper pod. This is helpful if a migration is stuck in a pending state, or to track the progress of a migration without the UI.
```bash
kubectl -n migration-system describe pod <v2v-helper-pod-name>
```

Get logs for a specific migration pod. This shows more detail than `describe pod`.
```bash
kubectl logs <pod> -n migration-system
```

Get logs for the `migration-controller-manager`
```bash
kubectl logs -n migration-system deploy/migration-controller-manager
```

Turn on Debug Mode
```bash
kubectl patch configmap -n migration-system migration-config-<vm-name> --type merge -p '{"data":{"DEBUG":"true"}}'
```
### A migration is stuck in pending
If the migration was set to Retry on Failure, then delete the v2v-helper pod for that VM and collect the logs of the pod that comes up.

```bash
kubectl delete pod -n migration-system v2v-helper-<vm-name>
```
If the v2v-helper pod doesn't come back up, and you can't delete the migration in the UI, then delete the associated `migrationplan`.
- First, get the `migrationplan` object name UUID for the associated VMs:
```bash
kubectl get migrationplans -n migration-system -o yaml
```
- Then delete the `migrationplan` object, which should remove it from the UI.
```bash
kubectl delete migrationplan <UUID> -n migration-system
```
### A migration failed and I want to run it again
Use the **Retry** action on the failed migration in the vJailbreak UI. It reopens the migration form pre-filled with the original configuration, so you can correct the setting that caused the failure before starting again. To restart several failed migrations without changing anything, select them in the **Migrations** table and use **Retry Selected**.

See [Retry a Failed Migration](../how-to/retry_failed_migration/) for the full workflow and its limitations.

### Get all vJailbreak custom resource definitions (CRDs)

```bash
kubectl get migrationplans,migrations,migrationtemplates,networkmappings,openstackcreds,storagemappings,vmwarecreds,secrets -n migration-system -o yaml
```

---

## virt-v2v fails: rename /sysroot/etc/resolv.conf Operation not permitted

- **Symptom**

  `virt-v2v` or `virt-v2v-in-place` fails with an error similar to:

  ```text
  renaming /sysroot/etc/resolv.conf to /sysroot/etc/6vvk9gzd
  guestfsd: error: rename: /sysroot/etc/resolv.conf to /sysroot/etc/6vvk9gzd: Operation not permitted
  commandrvf: stdout=n stderr=n flags=0x0
  commandrvf: umount /sysroot/sys
  virt-v2v-in-place: error: libguestfs error: sh_out: rename: /sysroot/etc/resolv.conf to /sysroot/etc/6vvk9gzd: Operation not permitted
  ```

- **Cause**

  On some Linux VMs, `/etc/resolv.conf` is marked **immutable**. When `virt-v2v` tries to rename or replace this file inside the guest filesystem during conversion, the immutable attribute prevents the operation and conversion fails.

  You can confirm the immutable bit inside the source VM with:

  ```bash
  lsattr /etc/resolv.conf
  ----i----------------- /etc/resolv.conf
  ```

  The `i` flag indicates the file is immutable.

- **Resolution**

  1. Remove the immutable attribute inside the source VM before migration:

     ```bash
     chattr -i /etc/resolv.conf
     ```

  2. Verify the attribute is gone:

     ```bash
     lsattr /etc/resolv.conf
     ---------------------- /etc/resolv.conf
     ```

  3. Re-run the migration.

- **Notes**

  - This is a known and documented `virt-v2v` issue. [See upstream documentation](https://libguestfs.org/virt-v2v.1.html#linux%3A-rename%3A-sysroot-etc-resolv.conf-failure).
  - If configuration management or security hardening marks `/etc/resolv.conf` immutable, ensure this is unset before conversion, or adjust your automation so VMs intended for conversion do not have `/etc/resolv.conf` marked immutable.

---

## Disk attach fails during migration: No more available PCI slots

- **Symptom**

  During a migration, attaching a target volume to the vJailbreak VM (or an agent VM) fails. The `nova-compute` log on the OpenStack side shows an error similar to:

  ```text
  TRACE nova.virt.libvirt.driver [instance: <uuid>] libvirt.libvirtError: internal error: No more available PCI slots
  ```

- **Cause**

  During conversion, vJailbreak attaches the target Cinder volumes to the vJailbreak VM (or its agent VMs) to copy and convert the disk data. If the vJailbreak image was uploaded without a disk bus setting, OpenStack attaches these volumes using the default **virtio-blk** bus, where **every attached volume is a separate PCI device** and consumes its own PCI slot.

  The virtual PCI bus has a limited number of slots, several of which are already used by essential devices (network interfaces, video, memory balloon, and so on). Migrating VMs with many disks — or running many migrations in parallel on one agent — exhausts the available PCI slots, and the volume attach fails with the error above.

- **Resolution**

  Configure the vJailbreak image to use the **virtio-scsi** disk bus. With virtio-scsi, all attached volumes share a single SCSI controller that consumes only one PCI slot and supports up to 256 devices.

  1. Set the following properties on the vJailbreak image **before** creating the vJailbreak VM:

     ```bash
     openstack image set \
       --property hw_disk_bus=scsi \
       --property hw_scsi_model=virtio-scsi \
       <vjailbreak-image-name-or-ID>
     ```

  2. Deploy the vJailbreak VM from the updated image, then re-run the migration.

- **Notes**

  - The disk bus is fixed when the VM is created. If your vJailbreak VM is already deployed, setting the properties on the image is not enough — you must recreate the vJailbreak VM from the updated image.
  - Agent VMs created during [scale up](../../how-to/scaling/) use the same image, so set these properties before scaling up agents.
  - See also: [Known Limitations](../../../reference/known-limitations/#pci-slot-exhaustion-when-attaching-disks-with-virtio-blk).

---

## virt-v2v-in-place fails on RHEL 7: missing GRUB compatibility symlink

- **Symptom**

  A RHEL 7.x migration fails during `virt-v2v-in-place`, after disk copy and volume attach/detach have already succeeded. The migration log only shows:

  ```text
  failed to run virt-v2v-in-place: exit status 1
  ```

  The debug log under `/var/log/pf9/` (see [Debug Logs](debuglogs/)) shows the actual error:

  ```text
  virt-v2v-in-place: error: libguestfs error: command:
  error opening /boot/grub/grub.cfg for read:
  No such file or directory
  ```

- **Cause**

  RHEL 7's `grubby`, used internally by `virt-v2v-in-place`, expects `/boot/grub/grub.cfg` to be a symlink to `/boot/grub2/grub.cfg`. On affected guests GRUB2 itself is configured correctly, but this compatibility symlink is missing, so `grubby`'s file open fails. This is unrelated to [SUSE Legacy GRUB 0.97](../../../reference/known-limitations/#suse-linux-sles-sled-with-legacy-grub-097), which requires a GRUB2 upgrade instead.

- **Resolution**

  Before migrating a BIOS RHEL 7 guest, confirm:

  - `/boot/grub2/grub.cfg` exists and is non-empty (regenerate with `grub2-mkconfig -o /boot/grub2/grub.cfg` if not).
  - `/boot/grub/grub.cfg` exists as a symlink to `../grub2/grub.cfg` (`mkdir -p /boot/grub && ln -sfn ../grub2/grub.cfg /boot/grub/grub.cfg` if missing).
  - `/etc/grub2.cfg` and `/etc/grub.conf` resolve to the same file — recreate the same way if broken.

  Then re-run the migration.

- **Notes**

  - Check ahead of a migration wave: `test -L /boot/grub/grub.cfg && echo OK || echo MISSING`.
  - Observed on RHEL 7.9 (Maipo); other RHEL 7.x releases with the same layout may be affected.
  - See also: [Known Limitations](../../../reference/known-limitations/#rhel-7-guests-missing-grub-compatibility-symlink).

---

## Proxy VM disk attach fails when several migrations start together

- **Symptom**

  A batch of [vJailbreak Accelerated Copy](../../../concepts/vjailbreak-accelerated-copy/) migrations is started at once. Some of them fail early with a vCenter error while attaching the source snapshot disks to the Proxy VM, while the rest continue into the copy phase without any problem.

- **Cause**

  Each migration attaches its source disks to the Proxy VM as a vCenter VM reconfigure task. When several migrations issue these tasks against the same Proxy VM simultaneously, vCenter does not always serialize them gracefully and rejects some of the attach requests.

  This is a **transient race condition** — the Proxy VM, its credentials, and its configuration are all fine. Only the migrations that lost the race are affected.

- **Resolution**

  1. Let the surviving migrations progress past the attach step and into the copy phase.
  2. [Retry](../../how-to/retry_failed_migration/) the failed migrations. They normally succeed on the second attempt.

- **Notes**

  - To reduce the chance of hitting this, stagger migration start times rather than starting a large batch at once, or register additional Proxy VMs and distribute migrations across them.
  - Use the [data copy start time](../../../concepts/migration-options/#data-copy-start-time) option to spread a wave of migrations over a window.
  - See also: [Known Limitations](../../../reference/known-limitations/#concurrent-disk-attach-can-fail).

---

## vJailbreak Accelerated Copy fails: could not identify block device

- **Symptom**

  A [vJailbreak Accelerated Copy](../../../concepts/vjailbreak-accelerated-copy/) migration fails after the snapshot disk has been attached to the Proxy VM:

  ```text
  could not identify block device for disk <uuid>
  ```

- **Cause**

  vJailbreak locates each attached disk inside the Proxy VM by matching its disk UUID to a block device. Two conditions must be met for this to work:

  1. `disk.EnableUUID` is set to `TRUE` on the Proxy VM, so the UUID is visible to the guest.
  2. The Proxy VM's first SCSI controller (**SCSI controller 0**) is of type **VMware Paravirtual (PVSCSI)**. Only PVSCSI is supported — LSI Logic SAS, LSI Logic Parallel, and BusLogic Parallel controllers do not work.

- **Resolution**

  1. Confirm `disk.EnableUUID = TRUE` on the Proxy VM: vSphere Client → **Edit Settings** → **VM Options** → **Advanced** → **Edit Configuration**.
  2. Confirm **SCSI controller 0** is **VMware Paravirtual**. If it is not, power off the Proxy VM, then in **Edit Settings** → **Virtual Hardware** set **SCSI controller 0** → **Change Type** → **VMware Paravirtual**, and power it back on.
  3. Re-verify the Proxy VM in the vJailbreak UI and re-run the migration.

- **Notes**

  - SSH into the Proxy VM and run `lsblk` to confirm the attached disks are visible to the guest.
  - Check vCenter events on the Proxy VM for disk attach errors if the disk never appears.
  - See also: [Known Limitations](../../../reference/known-limitations/#proxy-vm-must-use-a-pvscsi-controller) and [Configure the SCSI Controller Type on the Proxy VM](../../../concepts/vjailbreak-accelerated-copy/#configure-the-scsi-controller-type-on-the-proxy-vm).
