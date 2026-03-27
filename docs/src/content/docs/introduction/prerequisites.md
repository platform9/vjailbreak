---
title: Prerequisites
description: prerequisites for vJailbreak
---

For frequently asked questions, see [FAQ](../faq/).

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