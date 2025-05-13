---
title: FAQ & Prerequisites
description: prerequisites for vJailbreak
---

### Are IPs and MAC addresses persisted?
Yes, if your OpenStack network has a valid subnet range that allows the IP to be allocated, vJailbreak will create a port with the same MAC address and IP address as the source VM.

### What OS versions are supported?
We internally use virt-v2v, so all operating systems supported for conversion by virt-v2v are supported by vJailbreak. You can find a detailed list of them [here](https://libguestfs.org/virt-v2v-support.1.html#guests).

### Do I need to perform any manual steps to remove VMware Tools?
No, vJailbreak will remove them for you, with the help of virt-v2v. The process that virt-v2v uses along with alternative approaches can be found [here](https://libguestfs.org/virt-v2v.1.html#converting-a-windows-guest).

### Do I need to perform any manual steps to install drivers for Linux and Windows VMs?
No, vJailbreak will install it for you. For Windows, we allow you to specify a URL for a specific version of virtio drivers. This is useful for older Windows versions, eg. Windows Server 2012, which specifically need [v0.1.189](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.189-1/virtio-win-0.1.189.iso) in order to work.

### What access do I need for my vCenter user to be able to perform this migration?
Please refer to the following table for the required privileges:

| Privilege | Description |
| --- | --- |
| `Virtual machine.Interaction` privileges: |     |
| `Virtual machine.Interaction.Power Off` | Allows powering off a powered-on virtual machine. This operation powers down the guest operating system. |
| `Virtual machine.Interaction.Power On` | Allows powering on a powered-off virtual machine and resuming a suspended virtual machine. |
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

### What ports do I need to open for vJailbreak to work?
Please refer the following table for the required ports:

| Port | Protocol | Source | Destination | Purpose |
| --- | --- | --- | --- | --- |
| 443 | TCP | PCD nodes | VMware vCenter API endpiont | VMware provider inventory<br><br>Disk transfer authentication |
| 443 | TCP | PCD nodes | VMware ESXi hosts | Disk transfer authentication |
| 902 | TCP | PCD nodes | VMware ESXi hosts | Disk transfer data copy |
| 5480 | TCP | PCD nodes | VMware vCenter API endpoint | VMware Site Recovery Manager Appliance Management Interface |


### What network connectivity do I need for vJailbreak?

The vJailbreak VM and any helper nodes must be able to resolve & connect to your VMware vCenter environment and all ESXi hosts, and must be able to resolve & connect to [quay.io](https://quay.io).

For a comprehensive list of network connectivity requirements, especially in restricted environments, refer to the following:

---

#### VMware & OpenStack – TLS on port 443
- **Direction**: `vJailbreak/agent` → `host:443` (TCP)
- **Protocol**: TLS (`tls.Dial("tcp", host+":443", ...)`)
- **Source Files**:
  - `k8s/migration/pkg/utils/credutils.go`
  - `v2v-helper/openstack/openstackops.go`
  - `v2v-helper/vcenter/vcenterops.go`
- **Purpose**: Fetch certificates or communicate with REST APIs (vCenter, ESXi, OpenStack)

---

#### Cloud-init Certificate Retrieval – TLS on port 443
- **Runs in**: Helper VM
- **Command**: `openssl s_client -connect <FQDN>:443`
- **Source File**: `k8s/migration/pkg/scripts/cloud-init.tmpl.yaml`

---

#### Virtio Driver ISO Download – HTTPS on port 443
- **Default URL**:  
  https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso
- **References**:
  - `k8s/migration/internal/controller/migrationplan_controller.go`
  - `v2v-cli/cmd/root.go` (flag default)

---

#### OpenStack Metadata Service – HTTP on port 80
- **Accessed by**: Helper VM on boot
- **URL**: `http://169.254.169.254/openstack/latest/network_data.json`
- **File**: `k8s/migration/pkg/utils/vjailbreaknodeutils.go`

---

#### Health-Check Probe to Migrated Guest – HTTP/HTTPS (custom port)
- **Endpoint Attempted**: `http://<ip>:<port>`; fallback to `https://<ip>:<port>`
- **Source**: `v2v-helper/migrate/migrate.go`

---

#### External Tooling / Installers – HTTPS on port 443
- **Used For**:
  - Cloud-ctl setup script
  - Prometheus-operator and cert-manager manifests pulled from GitHub
- **Source**: `k8s/migration/pkg/constants/constants.go`
- **Example**:  
  `https://cloud-ctl.s3.us-west-1.amazonaws.com/cloud-ctl-setup | bash`

---

#### K3s Install Script – HTTPS on port 443
- **Endpoint**: `get.k3s.io` and related URLs
- **Purpose**: Download and install K3s along with its dependencies for Kubernetes setup
- **Source File(s)**:
  - `k8s/migration/internal/controller/migrationplan_controller.go`
  - CLI or internal modules may invoke the script as part of cluster bring-up
- **Details**: The install script fetches K3s binaries and bootstrap components over HTTPS. Ensure internet access is available to these domains if you're running in a restricted environment.

---

#### ICMP Echo (Ping)
- **Purpose**: Connectivity checks to migrated guest VMs
- **Source**: `v2v-helper/migrate/migrate.go` (uses `go-ping`)


*Note:* Some endpoints and requirements (e.g., specific URLs for virtio ISO or K3s) may vary based on configuration or version updates. Users in restricted environments should ensure all listed connections are permitted and consult with network administrators if needed.



### Required Ingress Rules for Kubernetes Node with Kubelet, Metrics Server, and Prometheus

| **Component**      | **Port**  | **Protocol** | **Source** | **Purpose** |
|--------------------|----------|-------------|------------|-------------|
| **Kubelet API**    | 10250   | TCP         | Control Plane / Prometheus | Health checks, logs, metrics |
| **Kubelet Read-Only (Optional)** | 10255 | TCP | Internal Only | Deprecated but might be used in some cases |
| **Metrics Server** | 4443    | TCP         | Internal Cluster | K8s resource metrics (`kubectl top`) |
| **Prometheus**     | 9090    | TCP         | Internal Cluster / Monitoring Server | Prometheus UI and API |
| **Node Exporter** (if used) | 9100 | TCP | Prometheus | Node-level metrics |
| **Cadvisor (Optional)** | 4194 | TCP | Internal Cluster / Prometheus | Container metrics collection |