---
title: Add ESXi to MAAS
description: Configuration and steps to add ESXi to MAAS
---

vJailbreak uses MAAS to manage the ESXi hosts and boot them into a PCD hypervisor. 
MAAS has a state machine for each machine that it manages. The machines by default are 
assumed to be 'empty' and available for deployment. The machines are managed by MAAS by booting them with an ephemeral image to discover the hardware.

## Prerequisites

:::caution
Before adding ESXi hosts to MAAS, ensure that IPMI (Intelligent Platform Management Interface) is configured and accessible for each ESXi host. vJailbreak requires IPMI access to control the power state and boot order of the ESXi hosts during the rolling conversion process. Without proper IPMI configuration, the conversion process will fail.
:::

Verify the following:
* IPMI is enabled on each ESXi host
* IPMI credentials (username/password) are available
* IPMI interface is network accessible from both the MAAS server and the vJailbreak VM
* IPMI power type is supported by MAAS (e.g., `ipmi`, `LAN_2_0`)

## Adding ESXi to MAAS

To enlist an ESXi host into MAAS, you will need to add it to MAAS without MAAS rebooting it
inadvertently. The process in detail is described [here](https://maas.io/docs/reference-release-notes-maas-3-1#p-11417-enlist-deployed-machines).

This requires you to have a MAAS CLI. The MAAS CLI can be used to add any machine in the
'deployed' state.

```
$ maas $profile machines create deployed=true hostname=mymachine \
architecture=amd64 mac_addresses=00:16:3e:df:35:bb power_type=ipmi \
power_parameters_power_address=<IPMI_IP> \
power_parameters_power_user=<IPMI_USER> \
power_parameters_power_pass=<IPMI_PASSWORD>
```

:::note
Replace `power_type=manual` with the appropriate IPMI power type and include IPMI credentials as shown above.
:::