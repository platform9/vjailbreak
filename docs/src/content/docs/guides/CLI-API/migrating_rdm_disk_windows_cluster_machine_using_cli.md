# Migrating RDM Disk

This guide walks you through the steps required to migrate a VM with **RDM (Raw Device Mapping) disks** using 
RDM disks are mostly used of windows machine.

RDM disk migration is only supported for PCD version >= September 2025

**vjailbreak**.  

---

## Prerequisites

Before you begin, ensure the following:  

1. **RDM disk is attached** to the Windows machine.  
2. **vjailbreak** is deployed in your cluster.  
3. All required fields (like `cinderBackendPool` and `volumeType`) are available from your `OpenstackCreds`.  

You can fetch these values using:  

```bash
openstack volume backend pool list
openstack volume type list
```

Or by describing the Openstack credentials:  

```bash
kubectl describe openstackcreds -n migration-system
```

## On Vmware 

- Add the following annotation to the VMware Notes field for the VM:
  ```
  VJB_RDM:Hard Disk:volumeRef:"source-name"="abac111"
  ```
  Replace `Hard Disk` with the RDM disk name and `abac111` with the actual source ID.

- To obtain the `source-id`,`source-name` or source details, you can run the following command against SAN Array:

  ```bash

  os block storage volume manageable list SAN_Array_reference --os-volume-api-version 3.8

  ```

---

## Migration Steps

### 1. Verify RDM Disk Resource
Check if the **RDM disk resource** is created in Kubernetes:  

```bash
kubectl get rdmdisk -n migration-system
```

### 2. Fetch RDM Disk Details
For each VM to be migrated, list its details:  

```bash
kubectl describe vmwaremachine <vm-name> -n migration-system
```

This will show the RDM disk identifiers.

### 3. Detach RDM Disk and Power Off VM in Vmware
Since VMware does not allow snapshots of a VM with attached RDM disks, you must:  

- **Power off** the VM to be migrated.  
- **Detach the RDM disk** from the VM.  

**Steps to detach RDM disks in vmware**

1) For each VM go to the Edit Settings, click on the cross icon near the RDM disks, keep "Delete files from storage" unchecked.
2) For each VM go to the Edit Settings,click on Remove the SCSI controller used by these disks, this will be in Physical sharing mode.

![Detach RDM Disk in vmware](https://raw.githubusercontent.com/rishabh625/vjailbreak/refs/heads/docs/rdm-migration-guide/docs/src/assets/vmware-removing-rdm-disk.png)

This ensures the snapshot and migration can proceed without errors.

### 4. Patch RDM Disk With Required Fields
Edit the RDM disk to add `cinderBackendPool` and `volumeType`. Example:  

```bash
kubectl patch rdmdisk vml.020072000060002ac000000000000144002838a5656202020 -n migration-system -p '{"spec":{"openstackVolumeRef":{"cinderBackendPool":"backendpool_name","volumeType":"volume_type"}}}' --type=merge
```

### 5. Create MigrationPlan
Finally, create a migration plan using the CLI.  
Follow the detailed CLI steps here:  

[Migrating Using CLI and Kubectl](https://platform9.github.io/vjailbreak/guides/cli-api/migrating_using_cli_and_kubectl/)  


### 6. Wait For Disk To Become Available
Confirm that the disk is in **Available** state:  

```bash
kubectl get rdmdisk <disk-id> -n migration-system -o yaml
```

Look for:  

```yaml
status:
  phase: Available
```

### 7. Ensure all VM's of cluster is migrated

1) Check RDM disk is available as volume in PCD or openstack

2) Ensure all VM's of cluster is migrated

3) Power on all VM's together

### Rollback plan - if migration fails

1) Delete VMs created in PCD or openstack
2) Delete managed volume 
``` openstack volume delete volumeid --remote```
3) Attach RDM disk in VMware to powered off VM's

    - Add the reference VMDK disks

    - Add New Device > Existing Hard Disk.
  This will add the disk as New Hard disk.
    - Change the controller of this hard disk to "New SCSI Controller" which we created in firs step.

    Repeat the process for all the RDM disk.

![Re attach RDM disk on failure](https://raw.githubusercontent.com/rishabh625/vjailbreak/refs/heads/docs/rdm-migration-guide/docs/src/assets/vmware-adding-back-disk.png)

4) Power on all the VM's on Vmware