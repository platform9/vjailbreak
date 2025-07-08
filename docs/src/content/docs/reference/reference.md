---
title: vJailbreak CRD references
description: A helpful explaination of the Kubernetes custom resource definitions (CRD) that vJailbreak uses.
---
The following custom resource definitions (CRD) are deployed in the same namespace as the Migration Controller pod. By default, the namespace is `migration-system`.

## Credentials
### OpenStack
- OpenstackCreds use the variables from the openstack.rc file. All fields are required except `OS_INSECURE`
```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: OpenstackCreds
metadata:
  name: osc1
  namespace: migration-system
spec:
  secretRef:
    name: osc1-openstack-secret
---
apiVersion: v1
data:
  OS_AUTH_URL:
  OS_DOMAIN_NAME:
  OS_INSECURE:
  OS_PASSWORD:
  OS_REGION_NAME:
  OS_TENANT_NAME:
  OS_USERNAME:
kind: Secret
metadata:
  name: osc1-openstack-secret
  namespace: migration-system
type: Opaque
```
### VMware
- All fields in VMwareCreds are required.
```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: VMwareCreds
metadata:
  name: vmc1
  namespace: migration-system
spec:
  secretRef:
    name: vmc1-vmware-secret
---
apiVersion: v1
data:
  VCENTER_HOST:
  VCENTER_INSECURE:
  VCENTER_PASSWORD:
  VCENTER_USERNAME:
kind: Secret
metadata:
  name: vmc1-vmware-secret
  namespace: migration-system
type: Opaque
```
## Network mapping
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
## Datastore mapping
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
## MigrationTemplate
```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: MigrationTemplate
metadata:
  name: migrationtemplate-windows
  namespace: migration-system
spec:
  networkMapping: name_of_networkMapping
  storageMapping: name_of_storageMapping
  osFamily: windowsGuest/linuxGuest <optional>
  source:
    datacenter: name_of_datacenter
    vmwareRef: name_of_VMwareCreds
  destination:
    openstackRef: name_of_OpenstackCreds

```
- `osFamily` is optional. If not provided, the `osFamily` is retrieved from vCenter. If it can't be automatically determined, migration will not proceed.
## MigrationPlan
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
  virtualMachines:
    - - winserver2k12
      - winserver2k16
    - - winserver2k19
      - winserver2k22
```
- `retry`: Optional. Retries one failed migration in a migration plan once. Set to false after a migration has been retried.
- `advancedOptions`: This is an optional field for granular control over migration options. MigrationTemplate with mappings must still be present. These options override the ones in the template, if set. If you use these options, you must only have 1 VM present in the virtualMachines list.
  - `granularVolumeTypes`: In case you wish to provide different volume types to disks of a VM when they are all on the same datastore, you can specify the volume type of each disk of your VM in order. You must define one volume type for one disk present on the VM
  - `granularNetworks`: In case you wish to override the default network mapping for a VM, you can provide a list of OpenStack network names to use in for each NIC on the VM, in order.
  - `granularPorts`: In case you wish to pre-create ports for a VM with certain configs and directly provide them to the target VM, you can define a list of port IDS to be used for each network on the VM. It will override options set in `granularNetworks`.
- `migrationStrategy`: This is an optional field
  - `type`: 
    - `cold`: Cold indicates to power off VMs in migrationplan at the start of the migration. Quicker than hot.
    - `hot`: Powers VM off just before cutover starts. Data copy occurs with the source VM powered on. May take longer.
  - `dataCopyStart`: Optional. ISO 8601 timestamp indicating when to start data copy
  - `vmCutoverStart`: Optional. ISO 8601 timestamp indicating when to start VM cutover
  - `vmCutoverEnd`: Optional. ISO 8601 timestamp indicating the latest time by when VM cutover can start. If this time has been passed before the cutover can start, migration will fail.
  - `adminInitiatedCutOver`: Set to true if you wish to manually trigger the cutover process. Default: `false`
  - `performHealthChecks`: Set to false if you want to disable Ping and HTTP GET health check. Failing these checks does not clean up the targeted VM. Default: `false`
  - `healthCheckPort`: Port to run the HTTP GET health check against. Default "443"
- `virtualMachines`: Specify names of VMs to migrate. In this example the batch of VMs `winserver2k12` and `winserver2k16` migrate in parallel. `winserver2k19` and `winserver2k22` will wait for the first 2 to complete successfully, and then start in parallel. You can use this notation to specify whether VMs should migrate sequentially or in parallel within a plan.

## VjailbreakNode
vJailbreak can be scaled to perform multiple migrations in parallel by deploying additional `agents`, enabling greater efficiency and workload distribution. The VjailbreakNode Custom Resource Definition (CRD) streamlines the creation and management of these agents, ensuring seamless integration into the migration workflow. Each `VjailbreakNode` represents a VM that functions as an independent migration `agent`. These agents are dynamically added to the original `VjailbreakNode`, forming a cohesive cluster that enhances scalability, reliability, and overall migration performance.
```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: VjailbreakNode
metadata:
  name: example-vjailbreak-node
  namespace: migration-system
spec:
  imageId: "your-openstack-image-id" # This ID is for the first vjailbreak VMimage. It auto-populates in the UI—do not delete it. 
  nodeRole: "worker"
  openstackCreds:
    name: "name" # Reference to your OpenstackCreds
    namespace: "migration-system"
  openstackFlavorId: "your-openstack-flavor-id"
 ```
This `VjailbreakNode` CRD defines a Kubernetes resource that provisions a VM in OpenStack to act as a migration agent. Below is a breakdown of each field:  
- `metadata:` Metadata contains identifying details about the `VjailbreakNode`.  
  - `name: example-vjailbreak-node`: Specifies the name of this `VjailbreakNode` resource in Kubernetes.  
  - `namespace: migration-system`: Indicates the namespace where this resource is deployed within the Kubernetes cluster.  

The `spec` section defines the desired state of the `VjailbreakNode`.  

- `imageId: "your-openstack-image-id"`: This is the ID of the OpenStack image used to create the VM.  
  - **It must match the image ID used to create the initial vJailbreak VM**, ensuring compatibility across all migration agents.  
- `nodeRole: "worker"`: Defines the role of the node.  
  - It should be set to `"worker"` as this node functions as a migration agent within the vJailbreak cluster.  
- `openstackCreds:`: OpenstackCreds use the variables from the openstack.rc file.
  - `name: "name"` → Refers to a `Secret` or `CustomResource` storing OpenStack authentication details.  
  - `namespace: "migration-system"` → Specifies the namespace where the credential reference is located.  
- `openstackFlavorId: "your-openstack-flavor-id"`: The ID of the OpenStack flavor to use for the VM.
  - This defines the compute resources allocated to the migration agent, so ensure it has adequate CPU and memory for the migration workload.