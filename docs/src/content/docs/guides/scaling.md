---
title: Scaling vJailbreak 
description: You can scale up vJailbreak to perform more parallel migrations
---

vJailbreak can be scaled to perform multiple migrations in parallel by deploying additional `agents`, enabling greater efficiency and workload distribution.
- Additional agents can be created in the Agents tab of the vJailbreak dashboard.
- Each agent must also have a copy of the VMware VDDK libraries in their `/home/ubuntu` directories.
   - Copy the latest version of the [VDDK libraries](https://developer.broadcom.com/sdks/vmware-virtual-disk-development-kit-vddk/8.0) for Linux into `/home/ubuntu` of the new agents. Untar it to a folder name `vmware-vix-disklib-distrib` in `/home/ubuntu` directory.
- To retrieve the password for logging into an agent, follow these steps:
   - SSH into the primary vJailbreak VM and run:
   ```shell
   cat /var/lib/rancher/k3s/server/token
   ```
   - The first 12 characters of this token is the password for the agent VMs. 

