---
title: Using vJailbreak via APIs
description: Usage
---

## Usage

1. **Download and Install ORAS**

   Download and install [ORAS](https://oras.land/docs/installation). Then, download the latest version of the vjailbreak image with the following command:
   ```bash
   oras pull quay.io/platform9/vjailbreak:v0.1.4
   ```
   This will download the vjailbreak qcow2 image locally. Upload it to your OpenStack environment and create your vjailbreak VM with it.

2. **Ensure Connectivity**

   Ensure that your vjailbreak VM can communicate with your OpenStack and VMware environments. This includes any setup required for VPNs, etc. If you do not have an OpenStack environment, you can download the community edition of [Private Cloud Director](https://platform9.com/private-cloud-director/#experience) to get started.

3. **Copy VDDK Libraries**

   Copy over the [VDDK libraries](https://developer.broadcom.com/sdks/vmware-virtual-disk-development-kit-vddk/8.0) for Linux into `/home/ubuntu` of the vjailbreak VM. Untar it to a folder named `vmware-vix-disklib-distrib` in the `/home/ubuntu` directory.

4. **Deploy Resources**

   Deploy all the following resources in the same namespace where you installed the Migration Controller. By default, it is `migration-system`.

   - **Create the Creds Objects**

     Ensure that after you create these objects, their status reflects that the credentials have been validated. If it is not validated, the migration will not proceed.
     ```yaml
     apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
     kind: OpenstackCreds
     metadata:
       name: sapmo1
       namespace: migration-system
     spec:
       OS_AUTH_URL: 
       OS_DOMAIN_NAME: 
       OS_USERNAME: 
       OS_PASSWORD:
       OS_REGION_NAME:  
       OS_TENANT_NAME:  
       OS_INSECURE: true/false <optional>
     ---
     apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
     kind: VMwareCreds
     metadata:
       name: pnapbmc1
       namespace: migration-system
     spec:
       VCENTER_HOST: vcenter.phx.pnap.platform9.horse
       VCENTER_INSECURE:  true/false
       VCENTER_PASSWORD:
       VCENTER_USERNAME: 
     ```
     - `OpenstackCreds` use the variables from the `openstack.rc` file. All these fields are compulsory except `OS_INSECURE`.
     - All the fields in `VMwareCreds` are compulsory.

   - **Create Network Mapping**

     Create the mapping between networks in VMware and networks in OpenStack.
     ```yaml
     apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
     kind: NetworkMapping
     metadata:
       name: nwmap1
       namespace: migration-system
     spec:
       networks:
       - source: VM Network
         target: vlan3002
       - source: VM Network 2
         target: vlan3003
     ```

   - **Create Storage Mapping**

     Create the mapping between datastores in VMware and volume types in OpenStack.
     ```yaml
     apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
     kind: StorageMapping
     metadata:
       name: stmap1
       namespace: migration-system
     spec:
       storages:
       - source: vcenter-datastore-1
         target: lvm
       - source: vcenter-datastore-2
         target: ceph
     ```

   - **Create the MigrationTemplate**

     ```yaml
     apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
     kind: MigrationTemplate
     metadata:
       name: migrationtemplate-windows
       namespace: migration-system
     spec:
       networkMapping: name_of_networkMapping
       storageMapping: name_of_storageMapping
       osType: windows/linux <optional>
       source:
         datacenter: name_of_datacenter
         vmwareRef: name_of_VMwareCreds
       destination:
         openstackRef: name_of_OpenstackCreds
     ```
     - `osType` is optional. If not provided, the `osType` is retrieved from vCenter. If it cannot be automatically determined, migration will not proceed.

   - **Create the MigrationPlan**

     ```yaml
     apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
     kind: MigrationPlan
     metadata:
       name: vm-migration-app1
       namespace: migration-system
     spec:
       migrationTemplate: migrationtemplate-windows
       retry: true/false <optional>
       advancedOptions:
         granularVolumeTypes: 
         - newvoltype1
         granularNetworks:
         - newnetworkname1
         - newnetworkname2
         granularPorts:
         - <port uuid 1>
         - <port uuid 2>
       migrationStrategy:
         type: hot/cold
         dataCopyStart: 2024-08-27T17:30:25.230Z
         vmCutoverStart: 2024-08-27T17:30:25.230Z
         vmCutoverEnd: 2024-08-28T17:30:25.230Z
         adminInitiatedCutOver: true/false
         performHealthChecks: true/false
         healthCheckPort: string
       virtualmachines:
         - - winserver2k12
           - winserver2k16
         - - winserver2k19
           - winserver2k22
     ```
     - `retry`: Optional. Retries one failed migration in a migration plan once. Set to false after a migration has been retried.
     - `advancedOptions`: This is an optional field for granular control over migration options. `MigrationTemplate` with mappings must still be present. These options override the ones in the template, if set. If you use these options, you must only have 1 VM present in the `virtualmachines` list.
       - `granularVolumeTypes`: In case you wish to provide different volume types to disks of a VM when they are all on the same datastore, you can specify the volume type of each disk of your VM in order. You must define one volume type for one disk present on the VM.
       - `granularNetworks`: In case you wish to override the default network mapping for a VM, you can provide a list of OpenStack network names to use for each NIC on the VM, in order.
       - `granularPorts`: In case you wish to pre-create ports for a VM with certain configs and directly provide them to the target VM, you can define a list of port IDs to be used for each network on the VM. It will override options set in `granularNetworks`.
     - `migrationStrategy`: This is an optional field.
       - `type`: 
         - `cold`: Cold indicates to power off VMs in `migrationPlan` at the start of the migration. Quicker than hot.
         - `hot`: Powers VM off just before cutover starts. Data copy occurs with the source VM powered on. May take longer.
       - `dataCopyStart`: Optional. ISO 8601 timestamp indicating when to start data copy.
       - `vmCutoverStart`: Optional. ISO 8601 timestamp indicating when to start VM cutover.
       - `vmCutoverEnd`: Optional. ISO 8601 timestamp indicating the latest time by when VM cutover can start. If this time has been passed before the cutover can start, migration will fail.
       - `adminInitiatedCutOver`: Set to true if you wish to manually trigger the cutover process. Default false.
       - `performHealthChecks`: Set to false if you want to disable Ping and HTTP GET health check. Failing these checks does not clean up the targeted VM. Default false.
       - `healthCheckPort`: Port to run the HTTP GET health check against. Default "443".
     - `virtualmachines`: Specify names of VMs to migrate. In this example, the batch of VMs `winserver2k12` and `winserver2k16` migrate in parallel. `winserver2k19` and `winserver2k22` will wait for the first two to complete successfully, and then start in parallel. You can use this notation to specify whether VMs should migrate sequentially or in parallel within a plan.

Each VM migration will spawn a migration object. The status field contains a high-level view of the progress of the migration of the VM. For more details about the migration, check the logs of the pod specified in the Migration object.

