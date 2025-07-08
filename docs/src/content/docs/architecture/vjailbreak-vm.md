---
title: vJailbreak VM
description: Overview of vJailbreak VM
---

As part of the deployment process, vJailbreak is shipped as a virtual machine (VM) that can be deployed in the target OpenStack environment. The VM is configured with the necessary resources, including sufficient memory and processing power, to handle the migration tasks efficiently.

The vJailbreak VM comes with a `k3s` https://k3s.io/ deployment that runs various components as pods. The components include the v2v-helper, UI, and migration-controller as described in the [components](../components/).

The vJailbreak VM can be scaled out to handle multiple migration tasks concurrently. This is achieved by deploying additional vJailbreak VMs in the target OpenStack environment. For more information on scaling vJailbreak, see the [scaling](../guides/scaling/) guide.

![vJailbreak VM](/vjailbreak/images/vjb-internal.png)

