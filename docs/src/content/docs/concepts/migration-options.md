---
title: Migration Options
description: Overview of Different Migration Options
---

vJailbreak provides a number of options to optimize and control the migration process. These options are available in the migration wizard under "Migration Options".

## Copy Options
There are two options available

### Data Copy Method
Determines how the data copy is done

* **Copy live VMs, then power off** - This option copies the data from the live VMs to the OpenStack/PCD volumes. vJailbreak uses CBT (Change Block Tracking) to copy the data that is dirtied. Then, the VMs are powered off and the remaining changed blocks are copied to the OpenStack/PCD volumes (see cut over options below)

* **Power off VMs, then copy** - This option powers off the VMs and then copies the data to the OpenStack/PCD volumes. There is no CBT involved in this case, it will be the faster option, but will impact the uptime of the application. Power off VMs are supported but may need user input to provide the IP address, Operating System type during migration.

### Data copy start time
As the name implies, determines when the copy operation should start, typically used to start the migration during off-peak hours.

## Cutover Options
There are 2 options available

* **Cutover during time window** - This option allows the user to specify a time window during which the VM would be powered off and the corresponding OpenStack/PCD VM would be configured and powered on. This window also involves copy of any remaining changed blocks to the OpenStack/PCD volumes since the last time the block were copied.

* **Cutover immediately after data copy** - This option is simpler and follows the copy operation immediately after the copy is complete. This option is recommended for applications that have flexible uptime requirements and can be powered off anytime during the migration.


## Post Migration Options

### Post Migration Script
A script to be executed after the migration is complete. This script is optional and can be used to perform post migration tasks such as starting the application, updating the application configuration, adding VM to domain controller etc.

### Rename VM
An optional parameter. Renames the source VM in VMware to have a specific suffix, good option to indicate a VM is migrated to OpenStack/PCD. The default suffix is "_migrated_to_pcd".

### Move to Folder
An optional parameter. Moves the source VM in VMware to a specific folder, good option to group migrated VMs and keep it out of the hands of the user.

