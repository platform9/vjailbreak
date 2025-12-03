## Overview
The Firstboot Script feature allows users to run custom scripts automatically on virtual machines (VMs) immediately after they are migrated to Platform9 Cloud Director (PCD) or OpenStack environments. This capability is essential for automating post-migration configurations, installations, and other setup tasks that need to be performed on the VM upon its first boot.

Following are some use cases for Firstboot Scripts:
1. Users can automate tasks such as installing necessary software
2. Configuring network settings or any other custom setup required for their specific use cases.

---

**Allowed Script Formats:**
1. **WindowsGuests**: `Batch` (.bat)
2. **LinuxGuests**: `sh`, `bash` (.sh)

## Firstboot Script Deployment
### How to Add the Script in Migration Form

The script is deployed through the migration form interface:
1. Navigate to the **Migration Options** section in your migration form
2. Check the **Post Migration Script** option
3. Paste the complete contents of firstboot scripts into the script field, if you have multiple scripts, append it in the end of the existing script.
4. Start the migration once all the options are set.

![img1](../../../../../public/images/firstboot-form.png)
> **Note:** The script contents should be added directly into the migration form

---

## Execution Flow of Firstboot Script
### When and Where Does It Run?
The script executes **automatically after the migration completes and VM boots for the first time in PCD/OpenStack.**


### Overview

#### What is Guestfs?
[Guestfs](https://libguestfs.org/) (libguestfs) is a set of tools for accessing and modifying virtual machine disk images. During migration, guestfs runs as part of [virt-v2v in-place](https://libguestfs.org/virt-v2v-in-place.1.html) conversion process

### Order of Execution

#### 1. User Submits the Post-Migration Script
- User provides a post-migration script via the migration form
- Script is added in the **Post-Migration Script** field

#### 2. Controller Creates ConfigMap
- The migration controller intercepts the script content.
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
- when VM boots for the first time in PCD/OpenStack, the script executes automatically

---

## Troubleshooting

### Accessing FirstBoot Logs

> #### For Windows Guests:

After migration, check execution logs at: `C:\Program Files\Guestfs\log.txt`

![img3](../../../../../public/images/firstboot-guestfs-log-file.png)
![img3-1](../../../../../public/images/firstboot-log-file-content.png)

---

| Issue | Location to Check |
|-------|-------------------|
| Script Not Executing | `C:\Program Files\Guestfs\log` for errors |
| ConfigMap Not Mounted | Check v2v-helper pod volume mounts (`mountPath: /home/fedora/scripts`, `name: firstboot`) |

---

***The script's success or failure can be determined by checking its location after migration:***
1. If the script executed successfully, it will be moved to: `C:\Program Files\guestfs\Firstboot\scripts-done\`
2. If the script failed during execution, it will remain in: `C:\Program Files\Guestfs\Firstboot\scripts\`

![img4](../../../../../public/images/firstboot-scripts-done-folder.png)

---

> #### For Linux Guests:

After migration, check execution logs at: `/root/virt-sysprep-firstboot.log` with elevated privileges.

![img5](../../../../../public/images/firstboot-linux-log-path.png)
![img5-1](../../../../../public/images/firstboot-linux-log-content.png)

---

| Issue | Location to Check |
|-------|-------------------|
| Script Not Executing | `/root/virt-sysprep-firstboot.log` for errors |
| ConfigMap Not Mounted | Check v2v-helper pod volume mounts (`mountPath: /home/fedora/scripts`, `name: firstboot`) |

---

***The script's success or failure can be determined by checking its location after migration:***
1. If the script executed successfully, it will not appear in both: `/usr/lib/virt-sysprep/scripts/` and `/usr/lib/virt-sysprep/scripts-done/`
2. If the script failed during execution, it will remain in: `/usr/lib/virt-sysprep/scripts/`

![img6](../../../../../public/images/firstboot-linux-firstboot-path.png)

## Link to Readily Available Firstboot Scripts
[firstbootscripts](https://github.com/platform9/vjailbreak/tree/main/scripts/firstboot/)