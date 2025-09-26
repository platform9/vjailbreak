---
title: "Migrating RDM Disk"
description: "A guide on how to migrate a Virtual Machine with RDM disks using the vjailbreak CLI."
---

RDM disks are primarily used for clustered Windows machines.

This guide walks you through the steps required to migrate a VM with **RDM (Raw Device Mapping) disks** using the CLI. 

RDM disk migration is only supported for **PCD version >= July 2025 (2025.7)** and is **not supported for OpenStack**. For multipath support (connecting to SAN array), **PCD version >= October 2025 (2025.10)** is required.

---

## Prerequisites

Before you begin, ensure the following:  

1. **RDM disk is attached** to the Windows machine.  
2. **vjailbreak** is deployed in your cluster.  
3. **PCD Requirements**:
   - Minimum version: **July 2025 (2025.7)**.
   - For multipath support (connecting to SAN array): **October 2025 (2025.10)** - includes default libvirt and QEMU packages.
   - **Volume type must have multi-attach support enabled** in OpenStack.  
4. All required fields (like `cinderBackendPool` and `volumeType`) are available from your `OpenstackCreds`.  

You can fetch  `cinderBackendPool` and `volumeType` values using:  

```bash
openstack volume backend pool list
openstack volume type list
```

Please refer to the following documents for commands to fetch volume backend pool and volume type lists:

**For cli client versions less than 2025.1:**
- [Volume Type List](https://docs.openstack.org/python-openstackclient/queens/cli/command-objects/volume-type.html#volume-type-list)
- [Volume Backend Pool List](https://docs.openstack.org/python-openstackclient/latest/cli/command-objects/volume-backend.html#volume-backend-pool-list)

**For cli client versions >= 2025.1:**
- [Cinder Command Options Reference](https://docs.openstack.org/python-openstackclient/latest/contributor/command-options.html)

**Alternatively**, you can easily retrieve this information by describing the OpenStack credentials in vjailbreak:  

```bash
kubectl describe openstackcreds <openstackcredsname> -n migration-system
```

After describing the OpenStack credentials, look for `volumeTypes` and `volumeBackend`. Gather the `volumeTypes` and `volumeBackend` values that need to be patched as mentioned in step 4 of Migration steps.


## On VMware 

- Add the following annotation to the VMware Notes field for the VM:
  ```
  VJB_RDM:Hard Disk:volumeRef:source-name=abac111
  ```
  Replace `Hard Disk` with the RDM disk name and `abac111` with the actual source details.

- To obtain the source details ie `source-id`, `source-name`, you can run the following command against the SAN Array:

  ```bash
  openstack block storage volume manageable list SAN_Array_reference --os-volume-api-version 3.8
  ```

**Note: Not all SAN arrays are supported by the OpenStack block storage client. If you cannot find your SAN array reference from the block storage client, contact your storage administrator to get the LUN reference by accessing the storage provider's interface.**

RDM disk migration has been tested with two storage arrays:

1. **HP Primera**
2. **NetApp ONTAP** 

The `manageable list` command is only supported on HP Primera.

---

## Migration Steps

### 1. Verify RDM Disk Resource
Check if the **RDM disk resource** is created in Kubernetes:  

```bash
kubectl get rdmdisk <vml-id> -n migration-system
```

Ensure the added annotations `source-name` or `source-id` are reflected in the vjailbreak RDM disk custom resource. Use the VML ID of the RDM disk from VMware. 

If source details are not correct, edit the Notes section of VMware VM for correct value and wait for reconcilation ( few minutes ), to get source details updated.

### 2. Ensure RDM disk reference is correctly populated in vmwaremachine
For each VM's to be migrated, list vm details on vjailbreak using below command:  

```bash
kubectl describe vmwaremachine <vm-name> -n migration-system
```

Ensure vml id of all RDM disks to be migrated appear in the vmwaremachine custom resource.

### 3. Detach the RDM Disk and Power Off the VM in VMware
Since VMware does not allow snapshots of a VM with attached RDM disks, you must:  

- **Power off** the VM to be migrated.  
- **Detach the RDM disk** from the VM (steps are mentioned below).

Optional: Once the RDM disk is detached,you can list the vmwaremachine custom resource and ensure the VML ID of all RDM disks to be migrated appear in rdmDisk section of vmwaremachine custom resource in vjailbreak.

```bash
kubectl describe vmwaremachine <vm-name> -n migration-system
```

**Note:** Once the RDM disk is detached, the `source-name` or `source-id` should not change, and the VMs that own the RDM disk should not change. If you need to detach the RDM disk from the VM and remove all RDM references from the VMs, you must handle it manually
<br>

Delete the `vmwaremachine` and `rdmdisk` custom resources on vjailbreak. After deletion wait for the configured reconciliation time, and re ensure that deleted resources are recreated by vjailbreak.

**Commands to delete VMware machine and RDM disk:**

```bash
kubectl delete vmwaremachine <vm-name> -n migration-system
```

```bash
kubectl delete rdmdisk <rdm-vml-id> -n migration-system
```

**Commands to verify VMware machine and RDM disk are recreated**

```bash
kubectl describe vmwaremachine <vm-name> -n migration-system
```

```bash
kubectl describe rdmdisk <vml-id> -n migration-system
```


**Steps to detach RDM disks in VMware:**

1. For each VM, go to **Edit Settings**, click on the cross icon near the RDM disks, and keep **"Delete files from storage" unchecked**.
2. For each VM, go to **Edit Settings** and remove the SCSI controller used by these disks (this will be in Physical sharing mode).

Note: Only remove the SCSI controller in Physical Sharing mode. Other volumes or non-RDM disks use different controllers (not in Physical Sharing mode), and those must not be deleted.

![Detach RDM Disk in VMware](https://raw.githubusercontent.com/platform9/vjailbreak/refs/heads/gh-pages/docs/src/assets/vmware-removing-rdm-disk.png)

This ensures that the snapshot and migration can proceed without errors.

### 4. Patch RDM Disk with the Required Fields

Edit the RDM disk to add `cinderBackendPool` and `volumeType`. Example:  

```bash
kubectl patch rdmdisk <name_of_rdmdisk_resource> -n migration-system -p '{"spec":{"openstackVolumeRef":{"cinderBackendPool":"backendpool_name","volumeType":"volume_type"}}}' --type=merge
```

### 5. Create Migration Plan
Create a migration plan using the CLI.  
Follow the detailed CLI steps here:  

[Migrating Using CLI and Kubectl](https://platform9.github.io/vjailbreak/guides/cli-api/migrating_using_cli_and_kubectl/)  

Note:

- While creating migration plan , make sure actual VM name is passed in `spec.virtualMachines` of migrationplan and not vm custom resource name.
- Migration plan `spec.migrationStrategy.type` should be cold - RDM disk can only be migrated with cold migrationStrategy






### 6. Wait for Disk to Become Available
Confirm that the rdm disk is in **Available** state:  

```bash
kubectl get rdmdisk <disk-id> -n migration-system -o yaml
```

Look for:  

```yaml
status:
  phase: Available
```

### 7. Ensure All the VMs in Cluster are Migrated

1. Check that the RDM disk is available as a volume in PCD or OpenStack.

2. Ensure all VMs in the cluster are migrated.

3. Power on all VMs together.

### Rollback Plan - If Migration Fails

1. Delete VMs created in PCD or OpenStack.
2. Delete the managed volume:

   ```bash
   openstack volume delete <volume-id> --remote
   ```

  **For cli client versions less than 2025.1:**
- [Volume Delete](https://docs.redhat.com/en/documentation/red_hat_openstack_platform/10/html/command-line_interface_reference_guide/openstackclient_subcommand_volume_delete)

**For cli client versions >= 2025.1:**
- [Cinder Command Options Reference](https://docs.openstack.org/python-openstackclient/latest/contributor/command-options.html)

3. Re-attach RDM disk in VMware to powered-off VMs:

   - Add the reference VMDK disks.
   - Add **New Device > Existing Hard Disk**. This will add the disk as a new hard disk.
   - Change the controller of this hard disk to **"New SCSI Controller"** which was created in the first step.

   Repeat this process for all RDM disks.

![Re-attach RDM disk on failure](https://raw.githubusercontent.com/platform9/vjailbreak/refs/heads/gh-pages/docs/src/assets/vmware-adding-back-disk.png)

4. Power on all the VMs on VMware.
