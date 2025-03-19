---
title: What is vJailbreak?
description: Introduction to vJailbreak
---

[vJailbreak](https://github.com/platform9/vjailbreak) is an open-source tool featuring a user-friendly interface designed to simplify and accelerate the migration of virtual machines (VMs) from VMware vSphere environments to any OpenStack-compliant cloud. It eliminates the complexities of cross-platform VM migration, enabling you to modernize your infrastructure with minimal disruption and a streamlined, visual workflow.

### How vJailbreak Works

vJailbreak's intuitive interface leverages the OpenStack & VMware SDKs to interact directly with both your VMware vSphere environment and your target OpenStack cloud. The UI guides you through these key steps:

1.  **Connection Setup:** Easily configure connections to your source VMware vSphere environment and your target OpenStack cloud.
2.  **VM Selection:** Select the VMs you wish to migrate from your vSphere environment.
3.  **Migration Planning:** Configure migration settings, such as target storage and network configurations, through interactive forms.
4.  **Migration Execution:** Initiate and monitor the migration process with real-time progress updates.
5.  **Post-Migration Validation:** Verify the successful migration and launch of your VMs in OpenStack.

### Key Features

* **Intuitive User Interface:** Manage the entire migration process through a clear, easy-to-use graphical interface – no command-line expertise required.
* **Seamless vCenter Integration:** Easily connect to your VMware vCenter to manage and migrate VMs.
* **Effortless VM Selection:** Select the virtual machines you wish to migrate with just a few clicks.
* **Automated Disk Conversion:** VM disks are automatically converted from `vmdk` to `qcow2` format.
* **Driver and Device Installation:** Necessary virtual devices and drivers are installed to ensure smooth operation post-migration.
* **Post-Migration Health Checks:** Comprehensive health checks are performed to verify the success of the migration and the operational status of the VMs in the new environment.
### Key Benefits
*  **Reduced Migration Time:** Automate migration tasks and visualize progress, significantly reducing the time and effort compared to manual methods.
*  **Minimized Downtime:** vJailbreak's efficient migration process helps minimize downtime for your critical workloads.
*  **Cost-Effective Solution:** As an open-source tool, vJailbreak eliminates licensing costs.
*  **Broad Compatibility:** Migrate VMs to any OpenStack cloud that adheres to standard OpenStack APIs.
*  **Non-Disruptive Migration:** Perform migrations without impacting the operation of your source VMware environment.
*  **Visual Progress Tracking:** Monitor the status of your migrations in real-time through the user interface.

:::tip[Did you know?]
By leveraging vJailbreak, organizations can modernize their infrastructure with minimal disruption, ensuring a smooth transition to an OpenStack cloud.
:::