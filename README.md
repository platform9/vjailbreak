# vjailbreak

Helping VMware users migrate to Openstack

## v2v-helper
The main application that runs the migration. It is expected to run as a pod in a VM running in the target Openstack Environment.

## v2v-cli
A CLI tool that starts the migration. This will be the tool that most users will interact with to migrate VMs.


## Building
vJailbreak is intended to be run in a kubernetes environment (k3s) on the appliance VM. In order to build and deploy the kubernetes components, follow the instructions in `k8s/migration` to build and deploy the custom resources in the cluster.

In order to build v2v-helper,

    cd v2v-helper
    docker build -t <repository>:<tag> .
    docker push <repository>:<tag>

## Usage

Firstly, you need to ensure that your appliance can talk to your Openstack and VMware environments. This includes any setup required for VPNs, etc. 
Deploy all the following resources in the same namespace where you installed the Migration Controller. By default, it is `migration-system`.
1. Create the Creds objects. Ensure that after you create these objects, their status reflects that the credentials have been validated. If it is not validated, the migration will not proceed.

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
       ---
       apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
       kind: VMwareCreds
       metadata:
         name: pnapbmc1
         namespace: migration-system
       spec:
         VCENTER_HOST: vcenter.phx.pnap.platform9.horse
         VCENTER_INSECURE:  
         VCENTER_PASSWORD:
         VCENTER_USERNAME: 

2. Create the mapping between networks in VMware and networks in Openstack

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
3. Create the mapping between datastores in VMware and volume types in Openstack

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
4. Create the MigrationTemplate

       apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
       kind: MigrationTemplate
       metadata:
         name: migrationtemplate-windows
         namespace: migration-system
       spec:
         networkMapping: nwmap1
         storageMapping: stmap1
         osType: windows
         source:
           datacenter: PNAP BMC
           vmwareRef: pnapbmc1
         destination:
           openstackRef: sapmo1
5. Finally, create the MigrationPlan

       apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
       kind: MigrationPlan
       metadata:
         name: vm-migration-app1
         namespace: migration-system
       spec:
         migrationTemplate: migrationtemplate-windows
         retry: true
         migrationStrategy:
           type: cold
           dataCopyStart: 2024-08-27T17:30:25.230Z
           vmCutoverStart: 2024-08-27T17:30:25.230Z
           vmCutoverEnd: 2024-08-28T17:30:25.230Z
         virtualmachines:
           - - winserver2k12
             - winserver2k16
           - - winserver2k19
             - winserver2k22
		  
	- retry: Optional. Retries one failed migration in a migration plan once. Set to false after a migration has been retried.
	- type: 
	  - cold: Cold indicates to power off VMs in migrationplan at the start of the migration. Quicker than hot
	  - hot: Powers VM off just before cutover starts. Data copy occurs with the source VM powered on. May take longer
	- dataCopyStart: Optional.  ISO 8601 timestamp indicating when to start data copy
	- vmCutoverStart: Optional. ISO 8601 timestamp indicating when to start VM cutover
	- vmCutoverEnd: Optional. ISO 8601 timestamp indicating the latest time by when VM cutover can start. If this time has been passed before the cutover can start, migration will fail.
	- virtualmachines: Specify names of VMs to migrate. In this example the batch of VMs `winserver2k12` and `winserver2k16` migrate in parallel. `winserver2k19` and `winserver2k22` will wait for the first 2 to complete successfully, and then start in parallel. You can use this notation to specify whether VMs should migrate sequentially or parallelly within a plan.

Each VM migration will spawn a migration object. The status field contains a high level view of the progress of the migration of the VM. For more details about the migration, check the logs of the pod specified in the Migration object.