---
title: Overview
description: Overview of vJailbreak architecture
---


Below is high level architecture of how vJailbreak works. vJailbreak runs
in a virtual machine in the target OpenStack environment. vJailbreak connects with VMware environment via vSphere APIs, using the VDDK library for the Standard copy method only. vJailbreak Accelerated Copy and Storage-Accelerated Copy transfer disk data without requiring VDDK. It also uses the OpenStack SDK to interact with the OpenStack environment and perform the necessary provisioning operations including creation of volumes, VMs.

![vJailbreak Architecture](/vjailbreak/images/deployment-architecture.png)