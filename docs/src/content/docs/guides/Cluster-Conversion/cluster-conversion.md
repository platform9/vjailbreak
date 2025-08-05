---
title: Cluster Conversion
description: Configuration and steps to perform cluster conversion
---

The following outlines the steps to use the vJailbreak RollingConversion feature, which is exclusively available and compatible with Platform9 Private Cloud Director (PCD).

## Pre-migration checks

Before starting the RollingConversion process, ensure the following checks are completed:

1. Ensure vCenter setup is available
* vCenter credentials must possess sufficient privileges to isolate ESXi hosts, place them into maintenance mode, and subsequently remove them from the inventory. These privileges are supplementary to existing permissions. The precise set of required permissions will be communicated imminently.
2. Ensure PCD (Platform9 Private Cloud Director) setup is available and properly configured.
* Ensure the ClusterBlueprint is configured.
** Within the Rolling Conversion form in vJailbreak UI, it is necessary to specify the desired network configuration to be implemented on the host subsequent to its conversion and onboarding to Platform Cloud Director (PCD).
* Verify network setup on PCD aligns with the ClusterBlueprint specifications.
* Make sure one host is already added to accommodate vJailbreak VM and its agents
3. Ensure Ubuntu MAAS setup is available and configured
* Only Ubuntu 22.04 is currently supported for PCD hosts within the Private Cloud Director (PCD).
* Ensure that the appropriate PXE image is configured based on requirements; a standard Ubuntu 22 image is recommended.
* It is presumed that the ESX hosts are pre-configured as “Machines” in the “Allocated or Deployed” state within MAAS.
* Verify the accuracy of the IPMI configuration for all the MAAS machines.
4. Backup current configurations and data related to vCenter.
5. Ensure vMotion configured correctly on vCenter
* I.e, If an ESX is put in maintenance mode, VMs should be able to move off that ESX
6. Make sure that all non-vmotion compatible VMs are migrated to the destination PCD or moved off to a single ESXI
* When we put an ESXi in maintenance mode, we expect all the VMs to move off of that ESXi and wait for it to become empty. 
* Even if a single VM is present on that ESXI, we cannot really re-flash it safely.
7. Ensure enough additional hosts on vCenter (Number of hosts will differ on the exact setup configuration)
* NOTE: We put the ESX hosts in maintenance mode one by one, so the VMs on these hosts are moved off until the host becomes empty. This host is further converted to PCD host. The extra hosts are required to accommodate the moved off VMs
* NOTE: It is not required to have the equal number of extra hosts, ideally just one.
8. Check available disk space on target systems within the PCD environment.

#### List of incompatible configurations for reference
* VMs with PCIe passthrough or SR-IOV devices
* VMs using local host devices (e.g. USB, CD/DVD)
* VMs with RDM disks in physical mode
* VMs with Fault Tolerance (FT) enabled
* VMs with CPU features not compatible across hosts
* VMs without Enhanced vMotion Compatibility (EVC) in mixed-CPU clusters
* VMs using host-only or non-shared network/storage
* VMs with VMCI or special device interfaces
* VMs in suspended state (for live vMotion)
* VMs with outdated VMware Tools or hardware version

## Configuration

Create VMware and OpenStack/PCD credential with "PCD" configuration only PCD is supported for rolling upgrade

### MAAS configuration

* **MAAS URL** - the MAAS system should be reachable
* **API Key** additionally the MAAS system should be configured to allow the vJailbreak VM to access it with the key
* **OS** the os configuration that vJailbreak would use in MAAS to direct the MAAS to boot the ESXi into PCD hypervisor

#### Import the ESXi into MAAS

If you have ESXi already deployed through MAAS, you can skip this step, else you will need to import ESXis into MAAS so that MAAS can recognize them.

## How cluster conversion works
After you submit the rolling conversion form, vJailbreak will take the following actions in sequence.

1. Verify your creds, especially **openstack-creds** for checking if they are **PCD creds** or not  
2. Verify Cluster information submitted in the form.  
3. Prepare a **special cloud-init script** that will run **post conversion** of this host to a ubuntu machine.  
4. Go through the list of VMs specified in the rolling conversion form  
5. Formulate and save a **list of ESXis to be converted**. 
6. Trigger the conversion process of these ESXi sequentially.  
7. Each conversion process includes following steps  
   1. Put that ESXi in **maintenance mode**  
   2. Wait for all VMs on that ESXi to move off of this ESXi to other hosts **(by DRS/vMotion)**  
   3. Once the ESXi is empty, vJailbreak starts the process of converting it to PCD host  
      1. Fetch list of all available **“Machines”** in MAAS  
      2. Find the correct **“Machine”** for the current ESXi, by checking the **hardwareUUID** received both from vCenter API and MAAS API  
      3. Once Machine is found, we fetch its **IPMI configuration** from MAAS, and use to set this machine **PXE (net) boot**  
      4. **Release** the machine in MaaS  
      5. **Deploy** the machine via MaaS, using the special cloud-init created in earlier steps  
   4. Now vJailbreak waits for MaaS to boot that machine to an **ubuntu image** and run the **cloud-init** that we provided.  
   5. Its polling mechanism checks the list of PCD hosts and verifies the **hardwareUUID** from MaaS with **hostID** from PCD. (HostID is forcefully set to **hardwareUUID** in the cloud-init)  
   6. Once vJailbreak sees the host in PCD in “unauthorised” state, vJailbreak makes API calls to PCD  
      1. To apply the “Host Network Configuration” selected for this host during the “**Rolling Conversion Form”** submission  
      2. To provide hypervisor role to this host, with an input of the **clusterName** used as **Target** in the “**Rolling Conversion Form”**  
   7. Wait for the PCD host to **converge**.  
   8. Mark the ESXi Conversion as **Successful**.  
8. If at least one ESXi conversion is successful, vJailbreak will start to migrate VMs (from the list in “**Rolling Conversion Form”**) to PCD (to the specified target Cluster)  
9. RollingConversion is marked successful if all (selected) ESXi are converted and all (selected) VMs are moved to PCD.