## Overview
This document explains firstboot scripts usage in migrations performed using virt-v2v. It details how guestfs is utilized to execute user-provided firstboot scripts on the migrated VMs.

---

Allowed Script formats:
- WindowsGuests: Batch (.bat) 
- LinuxGuests: sh, bash (.sh)

## Script Deployment
### How to Add the Script

The script is deployed through the migration form interface:
1. Navigate to the **Migration Options** section in your migration form
2. Check the **Post Migration Script** option
3. Paste the complete contents of firstboot scripts into the script field, if you have multiple scripts, append it in the end of the existing script.
4. Start the migration once all the options are set.

![img1](../../../../../public/images/firstboot-form.png)
> **Note:** The script contents should be added directly into the migration form
---
## Script Execution Flow
### When Does It Run?
The script executes **automatically after the migration completes and VM boots for the first time in OpenStack.**


### Guestfs Usage Overview in virt-v2v First Boot scripts Execution

#### What is Guestfs?
Guestfs (libguestfs) is a set of tools for accessing and modifying virtual machine disk images. During migration, guestfs runs as part of virt-v2v in-place conversion process.

### Order of execution

#### 1. User Submits Post-Migration Script
- User provides a post-migration script via the migration form
- Script is added in the **Post-Migration Script** field

#### 2. Controller Creates ConfigMap
- The migration controller intercepts the script content
- Creates a Kubernetes **ConfigMap** with the script data with the name as `firstboot-config-<vmwaremachine-resource-name>`
- ConfigMap details:
  - **Key**: `user_firstboot.sh`
  - **Value**: User-provided script content

#### 3. ConfigMap Mounted to v2v-helper Pod
- The ConfigMap is mounted as a volume in the **v2v-helper pod**
- Mount path allows virt-v2v to access the script

#### 4. Guestfs Access and Execution
- Virt-v2v reads the script from the mounted ConfigMap
- Script content is retrieved from the `user_firstboot.sh` key
- Guestfs adds the scripts by applying changes to the VM disk image
- when VM boots for the first time in OpenStack, the script executes automatically
---

## Troubleshooting

### Accessing FirstBoot Logs

> **For Windows Guests:**
After migration, check execution logs at: `C:\Program Files\Guestfs\log.txt`

![img3](../../../../../public/images/firstboot-guestfs-log-file.png)
![img3-1](../../../../../public/images/firstboot-log-file-content.png)

---

| Issue | Location to Check |
|-------|-------------------|
| Script not executing | `C:\Program Files\Guestfs\log` for errors |
| ConfigMap not mounted | Check v2v-helper pod volume mounts (`mountPath: /home/fedora/scripts`, `name: firstboot`) |

---

***The script's success or failure can be determined by checking its location after migration:***

#### If the script executed successfully, it will be moved to: `C:\Program Files\guestfs\Firstboot\scripts-done\`
#### If the script failed during execution, it will remain in: `C:\Program Files\Guestfs\Firstboot\scripts\`

![img4](../../../../../public/images/firstboot-scripts-done-folder.png)

---

> **For Linux Guests:**
After migration, check execution logs at: `/root/virt-sysprep-firstboot.log` with elevated privileges.

![img5](../../../../../public/images/firstboot-linux-log-path.png)

---

| Issue | Location to Check |
|-------|-------------------|
| Script not executing | `/root/virt-sysprep-firstboot.log` for errors |
| ConfigMap not mounted | Check v2v-helper pod volume mounts (`mountPath: /home/fedora/scripts`, `name: firstboot`) |

---

***The script's success or failure can be determined by checking its location after migration:***

#### If the script executed successfully, it will not appear in both: `/usr/lib/virt-sysprep/scripts/` and `/usr/lib/virt-sysprep/scripts-done/`
#### If the script failed during execution, it will remain in: `/usr/lib/virt-sysprep/scripts/`

![img6](../../../../../public/images/firstboot-linux-firstboot-path.png)


## Link to readily available firstboot scripts
[firstbootscripts](https://github.com/platform9/vjailbreak/tree/main/scripts/firstboot/)