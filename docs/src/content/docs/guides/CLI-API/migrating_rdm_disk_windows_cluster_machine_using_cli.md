# Migrating RDM Disk

This guide walks you through the steps required to migrate a VM with **RDM (Raw Device Mapping) disks** using **vjailbreak**.  

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

- **Detach the RDM disk** from the VM.  
- **Power off** the VM to be migrated.  

This ensures the snapshot and migration can proceed without errors.

### 4. Patch RDM Disk With Required Fields
Edit the RDM disk to add `cinderBackendPool` and `volumeType`. Example:  

```bash
kubectl patch rdmdisk vml.020072000060002ac000000000000144002838a5656202020 -n migration-system -p '{"spec":{"openstackVolumeRef":{"cinderBackendPool":"backendpool_name","volumeType":"volume_type"}}}' --type=merge
```

### 5. Wait For Disk To Become Available
Confirm that the disk is in **Available** state:  

```bash
kubectl get rdmdisk <disk-id> -n migration-system -o yaml
```

Look for:  

```yaml
status:
  phase: Available
```

### 6. Create MigrationPlan
Finally, create a migration plan using the CLI.  
Follow the detailed CLI steps here:  

[Migrating Using CLI and Kubectl](https://platform9.github.io/vjailbreak/guides/cli-api/migrating_using_cli_and_kubectl/)  
