---
title: Components
description: Overview of vJailbreak components
---

## vJailbreak Components

vJailbreak is composed of several key components that work together to facilitate the migration of virtual machines from VMware environments to OpenStack-compliant clouds. Below is an overview of each component and its role in the migration process.

### v2v-helper
The `v2v-helper` is the main application responsible for executing the migration process. It is designed to run as a pod within a virtual machine (VM) in the target OpenStack environment.

### UI
The `UI` component provides a user-friendly interface for vJailbreak. It allows users to manage and monitor the migration process through an intuitive graphical interface.

### migration-controller
The `migration-controller` is a Kubernetes controller that schedules and manages the migration tasks. It ensures that migrations are executed efficiently and in accordance with the defined policies.

### v2v-cli
The `v2v-cli` is a command-line interface tool that can initiate the migration process. While it is available, it is not required in the current version of vJailbreak, as the primary interface is the UI.

By understanding these components, users can better appreciate the architecture and functionality of vJailbreak, enabling them to effectively manage and execute VM migrations.