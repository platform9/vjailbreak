---
title: Overview
description: Overview of vJailbreak architecture
---


Below is high level architecture of how vJailbreak works. vJailbreak runs
in a virtual machine in the target OpenStack environment. vJailbreak connects with VMware environment via vSphere APIs and the VDDK library. It also uses the OpenStack SDK to interact with the OpenStack environment and perform the necessary provisioning operations including creation of volumes, VMs.

![vJailbreak Architecture](/vjailbreak/images/deployment-architecture.png)