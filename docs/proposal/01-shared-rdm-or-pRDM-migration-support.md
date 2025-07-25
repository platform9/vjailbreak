# Introduction

This document outlines an extension to vJailbreak for migrating VMs with RDM (Raw Device Mapping) disks from VMware to OpenStack.


# High-Level Design (HLD)


#### **Objective**: Extend vJailbreak to support migration of VMs with RDM (Raw Device Mapping) disks from VMware to OpenStack.


#### **Key Enhancements**:



1. Introduction of a new RdmDisk Custom Resource (CR).
2. Integration of RDM CR into VMwareMachine and MigrationPlan resources.
3. Dependency chain between RdmDiskController and MigrationPlanController.
4. Cinder management integration for importing RDM LUNs into OpenStack.
5. Conditional logic in UI and controller 	workflows.


##### **Actors**:



* **RdmDiskController**: Manages lifecycle and state of RDM disks.
* **MigrationPlanController**: Orchestrates overall VM migration plans.
* **v2v-helper pod**: Executes actual disk conversions and OpenStack VM creation.
* **vjailbreak-UI**: Enforces user constraints based on RDM selection.


# Low-Level Design (LLD)


## Changes in vjailbreak backend and UI

We propose introducing a new Custom Resource Definition (CRD) to store Raw Device Mapping (RDM) disk information. This CRD will be generated using Kubebuilder, and we will ensure the kubebuilder:subresource:status marker is included during its creation. This will allow for status updates to be managed separately from the main resource specification.


### RdmDisk Custom Resource (CRD)

The CRD will adhere to the following format. Please note that the fields highlighted in **red** implying they are dynamic or user-configurable) will be populated or updated via the "vjailbreak UI"


### CRD Structure

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
 kind: RdmDisk
 metadata:
 name: rdm-disk-xyz
 spec:
  diskName: Disk1
  diskSize: 100000
  uuid: "abcd-efgh-1234"
  displayName: "FinanceSharedDisk"
  importToCinder: true
  ownerVMs:
    - winserver2k16
    - winserver2k19
     - inp44xpapp6470
  volumeRef:
   source-name: "unm-3lHw1AUPSySgEu1m3XTPGA"
   cinderBackendPool: "mera@TDV"
   volumeType: "PTDV-Cinder"
   openstackCreds: openstack-cred-name
 status:(outside spec)
 phase: Created | Migrate | Managing | Managed | Error
 cinderVolumeID: vol-id
 validated: true
```

**During the reconciliation of VmwareCreds, if an RDM (Raw Device Mapping) disk is encountered in the GetAllVM function, an RDM Disk Custom Resource (CR) will be created based on the disk's name and its association with the VM.**



* If an RDM disk CR with the same name already exists, the owner VM reference will be appended to the existing RDM disk CR's - ownerVMs
* Additionally, the RDM disk reference will be added to the corresponding VmwareMachine CR to establish the link between the VM and the shared RDM disk.
    * Linkage should be done via labels and rdmDisks field under spec - explained later in this document.
* A new spec field rdmDisks to be introduced in VmwareMachine CR 


---

**ps: **To determine if a virtual machine (VM) contains a shared RDM (Raw Device Mapping) disk, you need to examine the controllers attached to its disks. Each disk will have a controller key. When you retrieve the disk details from VMware, check the controller details for the SharedBus property. If SharedBus is set to VirtualSCSISharingPhysicalSharing, it indicates that the disk is a shared RDM disk.

This change is implemented with below PR

**Pull Request**: [Updated VMware custom resource to capture RDM disk information in VM details](https://github.com/platform9/vjailbreak/pull/563/files)


### Workflow Changes


#### **Openstack Creds Reconciliation**

We need to integrate the available Cinder backend pools from OpenStack into the OpenStackCreds Custom Resource (CR) as illustrated below.

Information regarding these backend details can be obtained via the OpenStack Block Storage v3 API, specifically using the below endpoint for managing existing volumes

[Block Storage API V3 (CURRENT) - Manage Volumes](https://docs.openstack.org/api-ref/block-storage/v3/index.html#manage-an-existing-volume)

**CRD structure changes**

```yaml
apiVersion: v1
items:
- apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
  kind: OpenstackCreds
  metadata:
    creationTimestamp: "2025-05-28T13:18:56Z"
    finalizers:
    - vjailbreak.k8s.pf9.io/finalizer
    - openstackcreds.k8s.pf9.io/finalizer
    generation: 5
    name: openstack-rishabh
    namespace: migration-system
  spec:
    flavors:
    - OS-FLV-EXT-DATA:ephemeral: 0
      description: ""
      disk: 0
      id: f9e381bc-a7fd-4ca7-9cd3-73baf018b76c
      name: pf9.unknown
      os-flavor-access:is_public: true
      ram: 1024
      rxtx_factor: 1
      vcpus: 1
    secretRef:
      name: openstack-rishabh-openstack-secret
  status:
    openstack:
      networks:
      - tnet2
      - tnet1
      volumeTypes:
      - Glance-cache-testing
      - TDV-Cinder
      - __DEFAULT__
      cinderBackendPool:
      - pool1
      - pool2
    openstackValidationMessage: Successfully authenticated to Openstack
    openstackValidationStatus: Succeeded
    kind: List
    metadata:
      resourceVersion: ""
```


#### **VMwareCreds Reconciliation**



* On successful reconciliation, a VmwareMachine CR is created along with associated RdmDisk CRs for each detected RDM LUN. If a VM contains RDM disk, a label will be added to the **VMwareMachine **CR**

    **Example:** In the CR shown below, the following label is added to indicate that the VM is associated with the RDM disk:


`Label:- vjailbreak.k8s.pf9.io/is-shared-rdm: true`


    From these RDM reference, the total number of VMs attached to each RDM disk can be determined by inspecting the ownerVMs field in the corresponding RdmDisk CR’s spec.

```yaml
apiVersion: vjailbreak.k8s.pf9.io/vlalphal
 kind: VMwareMachine
metadata:
     creationTimestamp: '2025-06-17T10:48:36Z'
     generation: 41
     labels:
         openstack-rishabh: d3f3d5c7-f6d8-410e-bb9f-a7afe4b962d8
         vjailbreak.k8s.pf9.io/cluster-name: Prod-windowsI
         vjailbreak.k8s.pf9.io/is-shared-rdm: true
     name: inp44xpapp6470
     namespace: migration-system
     resourceVersion: '982995'
     uid: 3c02b201-922e-4bef-b44e-41d081974b21
spec:
     vms:
         cpu: 1
         datastores:
             - EXT-028
             - EXT-023
         disks:
             - Hard disk 1
             - Hard disk 2
   rdmDisks: 
     - rdm-disk-xyz
   memory: 4096
         name: vm001
         networks:
             - prd2
             - prd1
         os Type: linuxGuest
         vmState: notRunning
status:
     migrated: true
     powerState: notRunning
```

##### In above CR rdmDisks contains reference to RDMdisk CR


##### **Example:**

 When VMware credentials are reconciled, an RDM disk named 'rdm-disk-xyz' is created. By inspecting the corresponding Custom Resource (CR), it can be determined that this disk is associated with **3** **virtual machines.**

The volumeRef field, which follows a Key:value format, can be configured either through the UI or by specifying it directly in Vmware VM machine Notes in below format. VJB_RDM:diskName:volumeRef:value ie: VJB_RDM:Hard Disk:volumeRef:"source-id"="abac111"

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
 kind: RdmDisk
 metadata:
 name: rdm-disk-xyz
 spec:
  diskName: Disk1
  diskSize: 100000
  uuid: "abcd-efgh-1234"
  displayName: "FinanceSharedDisk"
  ownerVMs:
    - winserver2k16
    - winserver2k19
     - vm001
  volumeRef:
 source-name: "unm-3lHw1AUPSySgEu1m3XTPGA"
 cinderBackendPool: "primera@TDV"
  	 volumeType: "TDV-Cinder"
 status:(outside spec)
 phase: Pending
cinderReference: vol-id
```


#### UI Constraints (Changes and validations)



* When RDM disks are detected for a VM:
    * UI mandates selection of **all** VMs in the cluster.
    * Error shown if user attempts partial selection.

If a user wants to trigger the migration of a VM (e.g., vm001), they must select **all VMs listed in the ownerVMs reference** of the associated RdmDisk CR. \
 If any of the newly selected VMs are linked to additional RDM disks, then **all VMs referenced by those RDM disks must also be selected**.

Once all required VMs are selected, the UI should display a **dedicated section for RDM disk configuration**, allowing the user to choose the appropriate **OpenStack Cinder backend pool and volume type and also editing volumeRef: key:value field**. \


 The **OpenStack Cinder backend pool and volume type** details can be obtained from OpenstackCreds Custom Resource

The UI must also enforce that **all selected VMs are powered off** before initiating a migration using the **Data Copy** method.


##### Steps:

The UI should fetch all VMs with shared RDM disks using the following API query:      

```shell
GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/default/vmwaremachines?labelSelector=vjailbreak.k8s.pf9.io/is-shared-rdm:true
```


1. If a selected VM contains the aforementioned label, retrieve the RDM disks from its `vmwaremachine` specification. Then, iterate through the list of obtained RDM disk names, fetch the corresponding RDM disk Custom Resources (CRs), and retrieve the list of VMs from the `ownerVMs` specification of each RDM disk.
2. Iterate through the list of retrieved **RDM disk names**. For each, fetch its corresponding **RDM disk Custom Resource (CR)** and extract the associated VMs from the `ownerVMs` specification. Both the **UI** and the **migration plan controller** must then ensure all these VMs are selected as a group.
3. Mandate from UI to select PowerOff all VMs and then copy in Data Copy Method


##### Example:

Given:



* A VmwareMachine named inp44xpapp6470
* An RdmDisk named rdm-disk-xyz with ownerVMs: [winserver2k16, winserver2k19, vm001] \



---

**Step 1: User selects a VM to migrate**



* User selects inp44xpapp6470 for migration. \



---

**Step 2: UI fetches all VMs with RDM associations**

* Query all VMs that may have RDM disks using:

```shell
GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/default/vmwaremachines?labelSelector=vjailbreak.k8s.pf9.io/is-shared-rdm:true
```

** Step 3: UI checks if the selected VM has rdmDisks defined**

- Inspect the VmwareMachine spec of vm001:

```yaml
spec:
  rdmDisks:
    - rdm-disk-xyz
```
**Step 4: UI fetches RDM disk details**
- For each entry in rdmDisks, fetch the corresponding RdmDisk CR:
```yaml
spec:
  ownerVMs:
    - winserver2k16
    - winserver2k19
    - vm001
```

**Step 5: UI ensures full VM selection**

<li>UI <strong>automatically selects or prompts the user to select</strong> all VMs listed in ownerVMs: 
</li> 
<ul>
 
<li>winserver2k16 
</li>
 
<li>winserver2k19 
</li>
 
<li>vm001 
</li> 
</ul>

<li>UI disables the migration button until all required VMs are selected.

<p>
<strong>Step 6: Repeat check for additional RDM disks (if any)</strong></li>

<li>For every newly included VM (winserver2k16, winserver2k19), check their own VmwareMachine CRs to see if they reference additional RDM disks. 
</li>

<li>If found, repeat Steps 4–5 recursively. 

<hr>
<p>
<strong>Step 7: UI checks power state</strong></li>

<li>Ensure all selected VMs have status.powerState: notRunning. 
</li>

<li>If any are powered on, show validation error: 
 <em>"All VMs participating in RDM migration must be powered off." 
</em>
<hr>
<p>
<strong>Step 8: UI displays RDM disk configuration panel</strong></li>

<li>For each RDM disk (e.g., rdm-disk-xyz), show a configuration panel: 
</li> 
<ul>
 
<li>Dropdown to select Cinder Backend Pool (e.g., "primera@ATDV") 
</li>
 
<li>Dropdown to select Volume Type (e.g., "TDV-Cinder")</li>
 
<li>Text Box to edit volumeRef 
</li> 
</ul>

<li>These options can be fetched from OpenstackCreds CR. 


<hr>
<p>
<strong>Step 9: Submit Migration Plan</strong></li>

<li>Once all VMs are selected, validated, and RDM disk configuration is completed, allow the user to proceed with migration using <strong>Data Copy</strong> method.</li>
</ul>
   </td>
  </tr>
  <tr>
   <td>
   </td>
  </tr>
</table>



#### **MigrationPlanController**



* Before launching v2v-helper:
    * Detect associated RdmDisks.
    * Wait until all RdmDisk.status.phase == Managed.

For all RDM disks selected through the UI via associated VMs, the **Migration Plan Controller** must perform validation to ensure that:



* All above mentioned UI constraints are satisfied 
    * To be migrated VMs are powered off 
    * All referenced ownerVMs from RDMDisk list should be present in Migration PlanController.
    * Required fields are correctly populated in the corresponding Custom Resources

   **  Not sure : Migration Plan Controller should reference RDM disks which are migrated by it.**


```

apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: MigrationPlan
metadata:
 name: vm-migration-windows-app
 namespace: migration-system
spec:
 migrationTemplate: migrationtemplate-windows
 retry: true
 migrationStrategy:
   type: hot
   dataCopyStart: 2024-08-2717:30:25.230Z
   vmCutoverStart: 2024-08-27T17:30:25.230Z
   vmCutoverEnd: 2024-08-28T17:30:25.230Z
rdmDisks:
- rdm-disk-xyz
- rdm-disk-2
 virtualmachines:
   - - winserver2k12
     - winserver2k26
   - - winserver2k16
     - winserver2k19

```



After successful validation, the Migration Plan Controller will **emit events for each RDM disk**, delegating control to the **RDM Disk Controller **status reconcillation.

The **RDM Disk Controller** will:



* Set all of the RDM disk's status to **Migrate** \

* Reconcile the resource to initiate provisioning through Cinder API as mentioned below

[Openstack block API -Manage an Existing Volume](https://docs.openstack.org/api-ref/block-storage/v3/index.html#manage-an-existing-volume)


```

POST /v3/{project_id}/volumes/manage
{
  "volume": {
    "host": "pTDV",
    "ref": {
      "source-name": "unm-3lHw1AUPSySgEu1m3XTPGA"
    },
    "name": "Disk1",
    "volume_type": "TDV-Cinder",
    "description": "Volume for Disk1",
    "bootable": false
  }
}

```



         \




* Updates the status to Managed once the Cinder reference is successfully obtained
* Controller will update cinderReference in RDM disk Custom Resource

    **ps: ** Cinder Reference is volume id obtained from above manage operation. \



During this process, the **Migration Plan Controller will requeue and wait** until the status of all involved RDM disks transitions from Managing to Managed.


#### **RdmDiskController**

   To do following:



* Performs Cinder `cinder manage` to import LUNs.
* Updates RdmDisk status to Managed on success.
* Handles retry on failure with backoff.
* RdmDisk CRs have a status.phase field.
* MigrationPlanController checks RDM disk status before triggering Migration Job.
* Prevent race conditions during concurrent RDM and VM migration.


### **v2v-helper Chaining ( Controller chaining Migration plan controller and RDM disk controller).**



* vJailbreak only spawns v2v-helper **after** all RDM imports are completed. ie all RDM disk referenced have status managed and cinderReference field in CR is not empty
* Ensures volumes are available before VM provisioning in OpenStack.


### **Error Handling and Recovery**



* Retries on cinder manage failures (e.g. LUN conflict).
* Timeout enforcement for RdmDisk state transitions.
* MigrationPlan re queue on RDM disk failure.


### **Future Enhancements**



* Support for live migration (if RDM detachment becomes feasible).
* Graphical visualization of disk-chain dependencies in UI.
* Cinder snapshot support pre-migration.
