---
title: Migration Options
description: Overview of Different Migration Options
---

vJailbreak provides a number of options to optimize and control the migration process. These options are available in the migration wizard under "Migration Options".

## Copy options
There are several options available to control how data is copied during migration.

### Data copy method
Determines how the data copy is done

* **Copy live VMs, then power off** - This option copies the data from the live VMs to the OpenStack/PCD volumes. vJailbreak uses CBT (Change Block Tracking) to copy the data that is dirtied. Then, the VMs are powered off and the remaining changed blocks are copied to the OpenStack/PCD volumes (see cut over options below)

* **Power off VMs, then copy** - This option powers off the VMs and then copies the data to the OpenStack/PCD volumes. There is no CBT involved in this case, it will be the faster option, but will impact the uptime of the application. Power off VMs are supported but may need user input to provide the IP address, Operating System type during migration.

### Storage copy method
Determines the underlying mechanism used to transfer disk data.

* **Normal** (default) - Uses the traditional network-based copy via VMware's NFC protocol. Data is transferred from ESXi hosts to OpenStack Cinder volumes over the network. This method is limited to approximately 1 Gbps per VMDK due to NFC protocol constraints.

* **Storage-Accelerated Copy** - Leverages storage array-level XCOPY operations for dramatically faster migrations. Instead of copying data over the network, this method offloads the copy to the storage array itself. Requires:
  - Supported storage array (Pure Storage or NetApp)
  - Both VMware datastores and OpenStack Cinder backed by the same array
  - ESXi SSH access configured
  - VMs must be powered off during copy (cold migration only)

:::tip
Storage-Accelerated Copy can be 10-100x faster than normal copy for large VMs. For a 1 TB disk, normal copy takes ~2.5 hours while Storage-Accelerated Copy can complete in 5-30 minutes.
:::

See [Storage-Accelerated Copy](../storage-accelerated-copy/) for detailed configuration instructions.

### Data copy start time
As the name implies, determines when the copy operation should start, typically used to start the migration during off-peak hours.

## Cutover options
There are 2 options available

* **Cutover during time window** - This option allows the user to specify a time window during which the VM would be powered off and the corresponding OpenStack/PCD VM would be configured and powered on. This window also involves copy of any remaining changed blocks to the OpenStack/PCD volumes since the last time the block were copied.

* **Cutover immediately after data copy** - This option is simpler and follows the copy operation immediately after the copy is complete. This option is recommended for applications that have flexible uptime requirements and can be powered off anytime during the migration.


## Post migration options

### Post migration script
A script to be executed after the migration is complete. This script is optional and can be used to perform post migration tasks such as starting the application, updating the application configuration, adding VM to domain controller etc.

### Rename VM
An optional parameter. Renames the source VM in VMware to have a specific suffix, good option to indicate a VM is migrated to OpenStack/PCD. The default suffix is "_migrated_to_pcd".

### Move to folder
An optional parameter. Moves the source VM in VMware to a specific folder, good option to group migrated VMs and keep it out of the hands of the user.

## Network persistence

### Persist source network interfaces
When enabled, vJailbreak preserves the source VM's network interface names on the destination VM (for example, `eth0` or `ens3`). This prevents breaking guest configurations—such as firewall rules or legacy scripts—that depend on specific interface names.

For statically configured interfaces, vJailbreak also preserves routes defined in configuration files, ensuring the guest retains its original network behavior after migration.

To enable this behavior, check **Persist source network interfaces** under **Migration Options** in the migration form.

:::caution
**Important: Routing Considerations**

If a VM has multiple interfaces on the same subnet and has asymmetric routing table, the destination openstack platform may not support it and drop the packets. This may cause partial connectivity. This is mainly observed when a VM with asymmetric routing is having port-security enabled.

**Recommendation:**
- To avoid asymmetric routing, ensure each interface is on a unique subnet or consolidate multiple IPs onto a single port, as multiple interfaces on the same subnet will cause connectivity issues.
:::

:::note
For DHCP-enabled ports, connectivity and DHCP functionality are preserved, but the interface name may be renamed if this feature is not selected.
:::

:::note
For cross-network migration, network persistence is currently not supported and will be blocked.
:::

#### Supported operating systems

| Operating system | Supported |
| --- | --- |
| Red Hat Enterprise Linux (all versions) | Yes |
| Rocky Linux (all versions) | Yes |
| CentOS Linux (all versions) | Yes |
| Ubuntu 17 and later | Yes |
| Ubuntu less than 17 | No |
| Windows | No |
