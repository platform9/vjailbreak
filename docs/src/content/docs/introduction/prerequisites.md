---
title: FAQ & Prerequisites
description: prerequisites for vJailbreak
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

### What access do I need for my vCenter user to be able to perform this migration?
Please refer to the following table for the required privileges:

| Privilege | Description |
| --- | --- |
| `Virtual machine.Interaction` privileges: |     |
| `Virtual machine.Interaction.Power Off` | Allows powering off a powered-on virtual machine. This operation powers down the guest operating system. |
| `Virtual machine.Interaction.Power On` | Allows powering on a powered-off virtual machine and resuming a suspended virtual machine. |
| `Virtual machine.Config.ChangeTracking`| Allows enabling or disabling change tracking on a virtual machine. |
| `Virtual machine.Guest operating system management by VIX API` | Allows managing a virtual machine by the VMware VIX API. |
| `Virtual machine.Provisioning` | Note: All `Virtual machine.Provisioning` privileges are required.  |
| `Virtual machine.Provisioning.Allow disk access` | Allows opening a disk on a virtual machine for random read and write access. Used mostly for remote disk mounting. |
| `Virtual machine.Provisioning.Allow file access` | Allows operations on files associated with a virtual machine, including VMX, disks, logs, and NVRAM. |
| `Virtual machine.Provisioning.Allow read-only disk access` | Allows opening a disk on a virtual machine for random read access. Used mostly for remote disk mounting. |
| `Virtual machine.Provisioning.Allow virtual machine download` | Allows read operations on files associated with a virtual machine, including VMX, disks, logs, and NVRAM. |
| `Virtual machine.Provisioning.Allow virtual machine files upload` | Allows write operations on files associated with a virtual machine, including VMX, disks, logs, and NVRAM. |
| `Virtual machine.Provisioning.Clone template` | Allows cloning of a template. |
| `Virtual machine.Provisioning.Clone virtual machine` | Allows cloning of an existing virtual machine and allocation of resources. |
| `Virtual machine.Provisioning.Create template from virtual machine` | Allows creation of a new template from a virtual machine. |
| `Virtual machine.Provisioning.Customize guest` | Allows customization of a virtual machine’s guest operating system without moving the virtual machine. |
| `Virtual machine.Provisioning.Deploy template` | Allows deployment of a virtual machine from a template. |
| `Virtual machine.Provisioning.Mark as template` | Allows marking an existing powered-off virtual machine as a template. |
| `Virtual machine.Provisioning.Mark as virtual machine` | Allows marking an existing template as a virtual machine. |
| `Virtual machine.Provisioning.Modify customization specification` | Allows creation, modification, or deletion of customization specifications. |
| `Virtual machine.Provisioning.Promote disks` | Allows promote operations on a virtual machine’s disks. |
| `Virtual machine.Provisioning.Read customization specifications` | Allows reading a customization specification. |
| `Virtual machine.Snapshot management` privileges: |     |
| `Virtual machine.Snapshot management.Create snapshot` | Allows creation of a snapshot from the virtual machine’s current state. |
| `Virtual machine.Snapshot management.Remove Snapshot` | Allows removal of a snapshot from the snapshot history. |
| `Datastore` privileges: |     |
| `Datastore.Browse datastore` | Allows exploring the contents of a datastore. |
| `Datastore.Low level file operations` | Allows performing low-level file operations - read, write, delete, and rename - in a datastore. |
| `Sessions` privileges: |     |
| `Sessions.Validate session` | Allows verification of the validity of a session. |
| `Cryptographic` privileges: |     |
| `Cryptographic.Decrypt` | Allows decryption of an encrypted virtual machine. |
| `Cryptographic.Direct access` | Allows access to encrypted resources. |

### Understanding VMware NFC Performance Limitations

vJailbreak uses nbdkit to transfer disk data from VMware ESXi hosts via the NFC (Network File Copy) protocol over port 902. It's important to understand the inherent performance characteristics and limitations of VMware's NFC protocol:

#### NFC Protocol Characteristics

- **Per-VMDK throughput limit**: NFC is limited to approximately **1 Gbps per VMDK** due to VMware's internal implementation
- **Single-threaded**: NFC operations are single-threaded, limiting performance to what a single thread can achieve
- **Encrypted by default**: NFC traffic is SSL-encrypted, which adds overhead (disabling SSL can improve speed by up to 20% but reduces security)
- **Synchronous operations**: NFC must complete READ/WRITE/CHECK operations sequentially before proceeding
- **Latency-aware throttling**: NFC will automatically throttle when network latency increases

#### Impact on vJailbreak Migrations

- **Per-disk transfer speed**: Each VMDK transfers at approximately 1 Gbps (125 MB/s), regardless of available network bandwidth
- **Network saturation**: Multiple parallel VM migrations can saturate network links (e.g., a 10 Gbps link can theoretically support ~10 concurrent VM migrations)
- **Migration time estimation**: Expect transfer times of approximately 8-9 minutes per 100 GB per VMDK

#### Recommendations

- Plan migration schedules accounting for the ~1 Gbps per-VMDK limitation
- For VMs with large single disks, migration time will be constrained by NFC throughput rather than network capacity
- Use parallel migrations across multiple VMs to better utilize available network bandwidth
- Monitor network utilization to optimize the number of concurrent migrations
- Consider scheduling large VM migrations during maintenance windows

#### Alternative Protocols

VMware vSphere 8.0 and later supports **UDT (Unified Data Transport)** protocol, which offers significantly better performance than NFC. However, vJailbreak currently uses NFC via nbdkit for compatibility with a wider range of vSphere versions.

**References:**
- [Veeam Forum: 1Gbit/s per VMDK Limit](https://forums.veeam.com/vmware-vsphere-f24/1gbit-s-per-vmdk-limit-t66468.html)
- [Broadcom KB: NFC Performance](https://knowledge.broadcom.com/external/article/307001/nfc-performance-is-slow.html)

### What ports do I need to open for vJailbreak to work?
Please refer the following table for the required ports:

| Port | Protocol | Source | Destination | Purpose |
| --- | --- | --- | --- | --- |
| 443 | TCP | PCD nodes | VMware vCenter API endpiont | VMware provider inventory<br><br>Disk transfer authentication |
| 443 | TCP | PCD nodes | VMware ESXi hosts | Disk transfer authentication |
| 902 | TCP | PCD nodes | VMware ESXi hosts | Disk transfer data copy via NFC protocol (see NFC limitations above) |
| 5480 | TCP | PCD nodes | VMware vCenter API endpoint | VMware Site Recovery Manager Appliance Management Interface |


### What network connectivity do I need for vJailbreak?

<!-- The vJailbreak VM and any helper nodes must be able to resolve & connect to your VMware vCenter environment and all ESXi hosts, and must be able to resolve & connect to [quay.io](https://quay.io). -->

The vJailbreak VM and any helper nodes must be able to resolve and connect to the following:

- **vCenter, ESXi, and OpenStack API endpoints** — required for API communication.
- **Cloud-init certificate endpoints**
- **Virtio ISO download source**:
  - [https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso)
- **Health-check endpoints on migrated guest VMs** — over user-defined HTTP/HTTPS ports.
- **External tooling sources**:
  - [https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml](https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml)
  - [https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml](https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml)
- **K3s installation sources** — includes scripts and supporting binaries fetched from:
  - [https://get.k3s.io](https://get.k3s.io)
  - [https://github.com/k3s-io/k3s](https://github.com/k3s-io/k3s)
  - [https://update.k3s.io](https://update.k3s.io)
  - [https://github.com/rancher/k3s-root](https://github.com/rancher/k3s-root)
- **Helm chart repository for NGINX ingress** — used during setup:
  - [https://kubernetes.github.io/ingress-nginx](https://kubernetes.github.io/ingress-nginx)
- **Container registries required to pull images** — needed for K3s, vJailbreak components (controller, UI), Prometheus, Grafana, CoreDNS, NGINX ingress, exporters, etc.:
  - [https://docker.io](https://docker.io)
  - [https://ghcr.io](https://ghcr.io)
  - [https://quay.io](https://quay.io)
  - [https://registry.k8s.io](https://registry.k8s.io)
- **ICMP (ping) access to guest VM IPs** — for connectivity verification




### Required Ingress Rules for Kubernetes Node with Kubelet, Metrics Server, and Prometheus

| **Component**      | **Port**  | **Protocol** | **Source** | **Purpose** |
|--------------------|----------|-------------|------------|-------------|
| **Kubelet API**    | 10250   | TCP         | Control Plane / Prometheus | Health checks, logs, metrics |
| **Kubelet Read-Only (Optional)** | 10255 | TCP | Internal Only | Deprecated but might be used in some cases |
| **Metrics Server** | 4443    | TCP         | Internal Cluster | K8s resource metrics (`kubectl top`) |
| **Prometheus**     | 9090    | TCP         | Internal Cluster / Monitoring Server | Prometheus UI and API |
| **Node Exporter** (if used) | 9100 | TCP | Prometheus | Node-level metrics |
| **Cadvisor (Optional)** | 4194 | TCP | Internal Cluster / Prometheus | Container metrics collection |