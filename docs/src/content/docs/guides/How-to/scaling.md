---
title: Scale vJailbreak 
description: You can scale up vJailbreak to perform more parallel migrations
---

vJailbreak can be scaled to perform multiple migrations in parallel by deploying additional `agents`, enabling greater efficiency and workload distribution.

Additional agents can be created in the Agents tab of the vJailbreak dashboard using the "Scale Up" button. You will need to choose the destination OpenStack credentials, the size of the agent VM(s), and the number of agent nodes up to a maximum of 5 per scale up. Additional agent nodes can be scaled up in batches of 5, providing the flexibility to change agent VM sizes to help with throttling network traffic.

:::caution
It is entirely possible to fully saturate a 10Gb network with many parallel migrations!
:::

Agent nodes can be scaled down by selecting the agent and using the "Scale Down" button.

## Logging into Agent VMs

Agent VMs use the same login process as the primary vJailbreak VM:
- **Username**: `ubuntu`
- **Default Password**: `password`
- On first login, you will be prompted to change the password immediately. 

:::note
VDDK libraries are automatically synced from the primary vJailbreak VM to all agent nodes. You only need to upload VDDK to the primary vJailbreak VM.
:::

:::note 
The following instructions apply to versions of vJailbreak older than v0.1.7
:::

Each agent must also have a copy of the VMware VDDK libraries in their `/home/ubuntu` directories.
- Copy the latest version of the [VDDK libraries](https://developer.broadcom.com/sdks/vmware-virtual-disk-development-kit-vddk/8.0) for Linux into `/home/ubuntu` of the new agents. Untar it to a folder name `vmware-vix-disklib-distrib` in `/home/ubuntu` directory.