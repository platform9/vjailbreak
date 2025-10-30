---
title: "Migrating RDM Disk"
description: "A guide on how to migrate a Virtual Machine with RDM disks using the vjailbreak CLI."
---

RDM disks are primarily used for clustered Windows machines.

This guide walks you through the steps required to migrate a VM with **RDM (Raw Device Mapping) disks** using the CLI. 

RDM disk migration is only supported for **PCD version >= July 2025 (2025.7)** and is **not supported for OpenStack**.

---

## Prerequisites

Before you begin, ensure the following:  

1. **RDM disk is attached** to the Windows machine.  
2. **vjailbreak** is deployed in your cluster.  
3. **PCD Requirements**:
   - Minimum version: **July 2025 (2025.7)**.
   - For multipath support (connecting to SAN array): **October 2025 (2025.10)** - includes patched libvirt and QEMU packages.
   - **Volume type must have multi-attach support enabled** in OpenStack.  
4. All required fields (like `cinderBackendPool` and `volumeType`) are available from your `OpenstackCreds`.  
5. Source Details are added on RDM VMs in VMware described [here](#on-vmware)
6. Storage array configured in PCD is same as the one configured in VMware. Usually SAN arrays have logical pools/isolation, that must be same as well. 

You can fetch  `cinderBackendPool` and `volumeType` values using:  


By describing the OpenStack credentials in vjailbreak:  

```bash
kubectl describe openstackcreds <openstackcredsname> -n migration-system
```

After describing the OpenStack credentials, look for `volumeTypes` and `volumeBackend`. Gather the `volumeTypes` and `volumeBackend` values that need to be patched as mentioned in [step 4 of Migration steps](#4-patch-rdm-disk-with-the-required-fields).


**Alternatively** you can also gather details using openstack cli

```bash
openstack volume backend pool list
openstack volume type list
```

Please refer to the following documents for commands to fetch volume backend pool and volume type lists:

opnestack cli version >= 6.2.1

- [Volume Type List](https://docs.openstack.org/python-openstackclient/queens/cli/command-objects/volume-type.html#volume-type-list)
- [Volume Backend Pool List](https://docs.openstack.org/python-openstackclient/latest/cli/command-objects/volume-backend.html#volume-backend-pool-list)


## RDM Validation settings

In vjailbreak setting configmap we have a setting called `VALIDATE_RDM_OWNER_VMS` whose default value is `true`.  

This setting manadates all VM linked to RDM disk must be migrated in a single migration plan, to disable it set  `VALIDATE_RDM_OWNER_VMS` to false

## On VMware 

Perform the following steps on each VM from the cluster you are planning to migrate. 

- Add the following annotation to the VMware Notes field for the VM:
  ```
  VJB_RDM:{Name of Hardisk}:volumeRef:source-name=abac111
  ```
  - VJB_RDM – Key prefix indicating this entry is an RDM (Raw Device Mapping) LUN reference. 
  - {Name of Hardisk} - Name of the RDM disk attached to the VM. Replace this placeholder with the actual disk name.
  Disk Name is case sensitive.
  - volumeRef – Denotes the reference section for the volume configuration.
  source-name=abac111 – Specifies the LUN reference.
  The key can be either source-id or source-name.
   
    The value is the LUN identifier (ID or Name) used to map the disk.
    To obtain the source details ie `source-id`, `source-name`, you can run the following command against the SAN Array:

    ```bash
    openstack block storage volume manageable list <Cinder backend pool name> --os-volume-api-version 3.8
    ```

**Note: Not all SAN arrays are supported by the OpenStack block storage client, in such cases above command gives an empty output. If you cannot find your SAN array reference from the block storage client, contact your storage administrator to get the LUN reference by accessing the storage provider's interface.**

RDM disk migration has been tested with two storage arrays:

1. HPE Primera
2. NetApp ONTAP

The `manageable list` command is only supported on HPE Primera.

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

For each VM, go to **Edit Settings** and perform following steps. *Note down the details as you might need them in case you have to revert the migration.*  
1. Click on the cross icon near the RDM disks, and keep "Delete files from storage" **unchecked**.
2. Remove the SCSI controller used by these disks (this will be in Physical sharing mode).

Note: Only remove the SCSI controller in Physical Sharing mode. Other volumes or non-RDM disks use different controllers (not in Physical Sharing mode), and those must not be deleted.

![Detach RDM Disk in VMware](https://raw.githubusercontent.com/platform9/vjailbreak/refs/heads/gh-pages/docs/src/assets/vmware-removing-rdm-disk.png)

This ensures that the snapshot and migration can proceed without errors.

### 4. Patch RDM Disk with the Required Fields

Edit each RDM disk to add `cinderBackendPool` and `volumeType`. Example:  

```bash
kubectl patch rdmdisk <name_of_rdmdisk_resource> -n migration-system -p '{"spec":{"openstackVolumeRef":{"cinderBackendPool":"backendpool_name","volumeType":"volume_type"}}}' --type=merge
```

The volume type specified here must match the configuration the RDM disk volume has on the SAN array. Example: if the volume has de-duplication and compression enabled, the specified volume type on OpenStack side must have these settings enabled. 

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


### Rollback Plan - If Migration Fails

### ⚠️ Caution:
Once an RDM disk is managed in OpenStack or PCD, do not delete the corresponding volume from PCD/OpenStack during a rollback.
Deleting the volume will also remove the associated LUN reference from the storage array, resulting in irreversible data loss.

To unmanage an RDM disk safely, use the following command instead of deleting it directly:

```openstack volume delete <volume-id> --remote```

1. Delete VMs created in PCD or OpenStack.
2. Remove the managed volume from OpenStack without deleting it from the SAN array:

   ```bash
   openstack volume delete <volume-id> --remote
   ```

- [Volume Delete](https://docs.redhat.com/en/documentation/red_hat_openstack_platform/10/html/command-line_interface_reference_guide/openstackclient_subcommand_volume_delete)


3. Re-attach RDM disk in VMware to powered-off VMs:

   - Add the reference VMDK disks.
   - Add **New Device > Existing Hard Disk**. This will add the disk as a new hard disk.
   - Change the controller of this hard disk to **"New SCSI Controller"** which was created in the first step.
   - For each VM, go to **Edit Settings** and add the SCSI controller for disk and select physical sharing mode.

   Repeat this process for all RDM disks.

![Re-attach RDM disk on failure](https://raw.githubusercontent.com/platform9/vjailbreak/refs/heads/gh-pages/docs/src/assets/vmware-adding-back-disk.png)

4. Power on all the VMs on VMware.
