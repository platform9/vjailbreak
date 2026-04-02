---
title: FAQ
description: frequently asked questions
---

### Are IPs and MAC addresses persisted?
Yes, if your OpenStack network has a valid subnet range that allows the IP to be allocated, vJailbreak will create a port with the same MAC address and IP address as the source VM.

### Are network interface names persisted?
Yes, vJailbreak can preserve network interface names during migration.

To enable this behavior, select **Persist source network interfaces** in the migration form under **Migration Options**.

Read more in [Migration Options](../../concepts/migration-options/#persist-source-network-interfaces).

### What OS versions are supported?
We internally use virt-v2v, so all operating systems supported for conversion by virt-v2v are supported by vJailbreak. You can find a detailed list of them [here](https://libguestfs.org/virt-v2v-support.1.html#guests).

### Do I need to perform any manual steps to remove VMware Tools?
No, vJailbreak will remove them for you, with the help of virt-v2v. The process that virt-v2v uses along with alternative approaches can be found [here](https://libguestfs.org/virt-v2v.1.html#converting-a-windows-guest).

### Do I need to perform any manual steps to install drivers for Linux and Windows VMs?
No, vJailbreak will install it for you. For Windows, we allow you to specify a URL for a specific version of virtio drivers. This is useful for older Windows versions, eg. Windows Server 2012, which specifically need [v0.1.189](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.189-1/virtio-win-0.1.189.iso) in order to work.

### Why does the conversion step take time?
The delay is typically not because a single script is slow. The overall conversion process includes OS-level changes that take time, such as installing VirtIO drivers, removing old hypervisor drivers, and (for Windows guests) performing registry changes.

The helper scripts that apply static changes (for example, writing mount persistence entries to `/etc/fstab`) are simple and usually complete quickly.

Conversion time depends heavily on your infrastructure performance (especially CPU) and VM-specific factors, including the guest OS and root disk size.

### Why does nbdcopy fail during disk copy?
If this issue is seen, most of the time it is a DNS/name-resolution problem. Debug logs typically show DNS resolution errors when vJailbreak tries to connect to an ESXi host.

Error signature:
```text
failed to run nbdcopy: exec: already started
```

See: [Debug Logs](../../guides/Troubleshooting/debuglogs/).

See the troubleshooting guide: [nbdcopy fails during disk copy (often DNS resolution)](../../guides/Troubleshooting/nbdcopy-fails-after-vm-moved-esxi-host/).

### What do when virt-v2v fails with `rename: /sysroot/etc/resolv.conf ... Operation not permitted`?

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

  On some Linux VMs, `/etc/resolv.conf` is marked **immutable**. When `virt-v2v` tries to rename or replace this file inside the guest filesystem (for example, `/sysroot/etc/resolv.conf` during conversion), the immutable attribute prevents the operation and the conversion fails.

  You can confirm the immutable bit with:

  ```bash
  lsattr /etc/resolv.conf
  ----i----------------- /etc/resolv.conf
  ```

  The `i` flag indicates the file is immutable.

- **Resolution**

  1. Remove the immutable attribute inside the source VM:

     ```bash
     chattr -i /etc/resolv.conf
     ```

  2. Verify the attribute is gone:

     ```bash
     lsattr /etc/resolv.conf
     ---------------------- /etc/resolv.conf
     ```

  3. Re-run the Migration.

- **Notes**

  - This is a known and documented `virt-v2v` issue. [See here](https://libguestfs.org/virt-v2v.1.html#linux%3A-rename%3A-sysroot-etc-resolv.conf-failure)
  - If configuration management or security hardening marks `/etc/resolv.conf` immutable, ensure this is unset before conversion, or adjust your automation so that VMs intended for conversion do not have `/etc/resolv.conf` marked immutable.

### How does Vjailbreak handle flavors of the vm in the target openstack environment? 
vJailbreak provides users the flexibility to assign desired OpenStack flavors to virtual machines during the migration setup. If the user specifies a flavor in the migration form, vJailbreak will honor that choice during provisioning on the target OpenStack environment.

If no flavor is explicitly chosen, vJailbreak automatically selects the most appropriate flavor based on the VM's resource requirements (We always try to find the exact match if not the next best match). In cases where no suitable flavor is found, the UI will display a warning. If the user proceeds despite the warning, the migration will fail with a clear error message indicating that a compatible flavor could not be found.

### Can vJailbreak migrate VMs running Docker Engine?
Yes, vJailbreak can migrate VMs running Docker Engine without any issues. vJailbreak performs VM-level migration and is agnostic to the workloads running inside the VM. Docker Engine is simply software running on the guest operating system, and the migration process handles it like any other application.

### Can vJailbreak migrate VMs that are part of a Kubernetes cluster?
Yes, you can migrate VMs that are part of a Kubernetes cluster. However, it's important to understand that vJailbreak operates purely as a VM migration tool and has no awareness of Kubernetes components or distributed applications running on the VM.

Users are responsible for taking necessary steps to ensure no disruption to applications and the distributed architecture of Kubernetes. This may include draining nodes, managing pod scheduling, and coordinating the migration with cluster operations. vJailbreak is not responsible for managing any workload-specific concerns.

### Can vJailbreak migrate Kubernetes Persistent Volume Claims (PVCs)?
No, vJailbreak does not migrate PVCs. PVCs are Kubernetes constructs managed by CSI drivers and storage backends. vJailbreak has no visibility into these workload-level abstractions.

vJailbreak migrates virtual machines along with the disks that are **currently attached** to those VMs at the time of migration. Any storage managed by Kubernetes (such as PVCs) must be handled separately using Kubernetes-native tools or storage migration solutions.

### Can vJailbreak migrate VMs running Docker Swarm clusters?
Yes, vJailbreak can migrate VMs that are part of a Docker Swarm cluster. However, the same principles apply as with any distributed workload: vJailbreak performs VM-level migration and does not manage workload-specific concerns.

Users must take appropriate precautions for Docker Swarm, such as draining nodes, managing service placement, and ensuring cluster quorum is maintained during migration. vJailbreak does not inspect or manage what is running inside the VM.


### Can vJailbreak migrate Windows VMs with GPO applied?
Yes, but GPO settings may interfere with driver injection during migration. You may need to disable restrictive Group Policy settings before migrating.
 
See the [GPO Migration Guide](../guides/how-to/gpo_migration.md) for detailed steps on how to resolve GPO-related migration issues.