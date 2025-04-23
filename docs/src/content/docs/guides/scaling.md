---
title: Scaling vJailbreak 
description: You can scale up vJailbreak to perform more parallel migrations
---

vJailbreak can be scaled to perform multiple migrations in parallel by deploying additional `agents`, enabling greater efficiency and workload distribution.
Additional agents can be created in the Agents tab of the vJailbreak dashboard using the "Scale Up" button. You will need to choose the destination OpenStack credentials, the size of the agent VM(s), and the number of agent nodes up to a maximum of 5.

Agent nodes can be scaled down by selecting the agent and using the "Scale Down" button.

To retrieve the `ubuntu` user's password for SSH'ing into an agent, follow these steps:
- SSH into the primary vJailbreak VM and run:
```shell
cat /var/lib/rancher/k3s/server/token | cut -c 1-12
```
The first 12 characters of this token is the password for the agent VMs. 

:::note 
The following instructions apply to versions of vJailbreak older than v0.1.7
:::

Each agent must also have a copy of the VMware VDDK libraries in their `/home/ubuntu` directories.
- Copy the latest version of the [VDDK libraries](https://developer.broadcom.com/sdks/vmware-virtual-disk-development-kit-vddk/8.0) for Linux into `/home/ubuntu` of the new agents. Untar it to a folder name `vmware-vix-disklib-distrib` in `/home/ubuntu` directory.