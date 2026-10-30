---
title: Cluster Conversion
description: Configuration and steps to perform cluster conversion
---

The following outlines the steps to use the vJailbreak RollingConversion feature, which is exclusively available and compatible with Platform9 Private Cloud Director (PCD).

## Pre-migration checks

Before starting the RollingConversion process, ensure the following checks are completed:

1. Ensure vCenter setup is available
* vCenter credentials must possess sufficient privileges to place ESXi hosts into maintenance mode (and take them out again) and subsequently remove them from the inventory. These privileges are supplementary to the [migration privileges](../../../introduction/prerequisites/#what-access-do-i-need-for-my-vcenter-user-to-be-able-to-perform-this-migration):
  * `Host.Config.Maintenance` (**Host > Configuration > Maintenance**), to enter and exit maintenance mode.
  * `Host.Inventory.RemoveHostFromCluster` (**Host > Inventory > Remove host from cluster**), to remove a converted host from vCenter. Grant it on the host and on its parent cluster.
2. Ensure PCD (Platform9 Private Cloud Director) setup is available and properly configured.
* Ensure the ClusterBlueprint is configured.
  * Within the Rolling Conversion form in the vJailbreak UI, it is necessary to specify the desired network configuration to be implemented on the host subsequent to its conversion and onboarding to Private Cloud Director (PCD).
* Verify network setup on PCD aligns with the ClusterBlueprint specifications.
* Make sure one host is already added to accommodate the vJailbreak VM and its agents.
3. Ensure Ubuntu MAAS setup is available and configured
* Only Ubuntu 22.04 is currently supported for PCD hosts within the Private Cloud Director (PCD).
* Ensure that the appropriate PXE image is configured based on requirements; a standard Ubuntu 22 image is recommended.
* It is presumed that the ESXi hosts are pre-configured as "Machines" in the "Allocated or Deployed" state within MAAS.
* **IMPORTANT**: All ESXi hosts added to MAAS must have IPMI details configured. vJailbreak uses IPMI to control the power state and boot order of the ESXi hosts during the conversion process. Verify the accuracy of the IPMI configuration for all the MAAS machines before starting the rolling conversion.
4. Backup current configurations and data related to vCenter.
5. Ensure vMotion is configured correctly on vCenter
* I.e., if an ESXi is put in maintenance mode, VMs should be able to move off that ESXi
6. Make sure that all non-vMotion-compatible VMs are migrated to the destination PCD or moved off to a single ESXi
* When we put an ESXi in maintenance mode, we expect all the VMs to move off of that ESXi and wait for it to become empty. 
* Even if a single VM is present on that ESXi, we cannot really re-flash it safely.
7. Ensure enough additional hosts on vCenter (the number of hosts will differ based on the exact setup configuration)
* NOTE: We put the ESXi hosts in maintenance mode one by one, so the VMs on these hosts are moved off until the host becomes empty. This host is further converted to a PCD host. The extra hosts are required to accommodate the moved-off VMs.
* NOTE: It is not required to have an equal number of extra hosts, ideally just one.
8. Check available disk space on target systems within the PCD environment.

### List of incompatible configurations for reference
* VMs with PCIe passthrough or SR-IOV devices
* VMs using local host devices (e.g., USB, CD/DVD)
* VMs with RDM disks in physical mode
* VMs with Fault Tolerance (FT) enabled
* VMs with CPU features not compatible across hosts
* VMs without Enhanced vMotion Compatibility (EVC) in mixed-CPU clusters
* VMs using host-only or non-shared network/storage
* VMs with VMCI or special device interfaces
* VMs in suspended state (for live vMotion)
* VMs with outdated VMware Tools or hardware version

## Configuration

Create VMware and OpenStack/PCD credentials with the "PCD" configuration. Only PCD is supported for rolling upgrade.

### MAAS configuration

* **MAAS URL** - the MAAS system should be reachable
* **API Key** - additionally, the MAAS system should be configured to allow the vJailbreak VM to access it with the key
* **OS** - the OS configuration that vJailbreak would use in MAAS to direct MAAS to boot the ESXi into a PCD hypervisor

#### Import the ESXi into MAAS

If you have ESXi already deployed through MAAS, you can skip this step; otherwise, you will need to import ESXis into MAAS so that MAAS can recognize them.

## How cluster conversion works
After you submit the rolling conversion form, vJailbreak will take the following actions in sequence.

1. Verify your creds, especially **openstack-creds** for checking if they are **PCD creds** or not  
2. Verify Cluster information submitted in the form.  
3. Prepare a **special cloud-init script** that will run **post conversion** of this host to an Ubuntu machine.  
4. Go through the list of VMs specified in the rolling conversion form  
5. Formulate and save a **list of ESXis to be converted**. 
6. Trigger the conversion process of these ESXi sequentially.  
7. Each conversion process includes the following steps  
   1. Put that ESXi in **maintenance mode**  
   2. Wait for all VMs on that ESXi to move off of this ESXi to other hosts **(by DRS/vMotion)**  
   3. Once the ESXi is empty, vJailbreak starts the process of converting it to a PCD host  
      1. Fetch list of all available **“Machines”** in MAAS  
      2. Find the correct **“Machine”** for the current ESXi, by checking the **hardwareUUID** received both from vCenter API and MAAS API. If no machine matches by hardware UUID, vJailbreak falls back to matching the MAC addresses of the ESXi host's physical NICs against the machines' MAC addresses  
      3. Once Machine is found, we fetch its **IPMI configuration** from MAAS, and use it to set this machine to **PXE (net) boot**  
      4. **Release** the machine in MAAS  
      5. **Deploy** the machine via MAAS, using the special cloud-init created in earlier steps  
   4. Now vJailbreak waits for MAAS to boot that machine to an **Ubuntu image** and run the **cloud-init** that we provided.  
   5. Its polling mechanism checks the list of PCD hosts and verifies the **hardwareUUID** from MAAS with **hostID** from PCD. (HostID is forcefully set to **hardwareUUID** in the cloud-init)  
   6. Once vJailbreak sees the host in PCD in “unauthorised” state, vJailbreak makes API calls to PCD  
      1. To apply the “Host Network Configuration” selected for this host during the “**Rolling Conversion Form”** submission  
      2. To provide hypervisor role to this host, with an input of the **clusterName** used as **Target** in the “**Rolling Conversion Form”**  
   7. Wait for the PCD host to **converge**.  
   8. Mark the ESXi Conversion as **Successful**.  
8. If at least one ESXi conversion is successful, vJailbreak will start to migrate VMs (from the list in “**Rolling Conversion Form”**) to PCD (to the specified target Cluster)  
9. RollingConversion is marked successful if all (selected) ESXi are converted and all (selected) VMs are moved to PCD.