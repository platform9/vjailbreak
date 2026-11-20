---
title: Components
description: Overview of vJailbreak components
---


Below is an overview of the core components and their roles in the migration process.

## v2v-helper
The `v2v-helper` is the main application responsible for executing the migration process. It is designed to run as a pod within the vJailbreak virtual machine (VM) in the target OpenStack environment. It supports three storage copy methods: Standard (VDDK-based), vJailbreak Accelerated Copy (Hot-Add, VDDK-free), and Storage Accelerated Copy (XCOPY, VDDK-free). Only Standard copy requires VDDK.

## UI
The `UI` component provides a user-friendly interface for vJailbreak. It allows users to manage and monitor the migration process through an intuitive graphical interface.

## API server
The vJailbreak API server (`vpwned`) provides REST endpoints used by the UI and the migration pods. For example, it deploys Proxy VMs from the OVA, generates SSH key pairs, applies time settings, runs appliance upgrades, and coordinates disk attaches on shared Proxy VMs.

## migration-controller
The `migration-controller` is a Kubernetes controller that schedules and manages the migration tasks. It ensures that migrations are executed efficiently and in accordance with the defined policies.

By understanding these components, users can better appreciate the architecture and functionality of vJailbreak, enabling them to effectively manage and execute VM migrations.