---
title: Inject VirtIO Windows Driver
description: The vJailbreak appliance ships with the VirtIO driver ISOs used during Windows VM migrations. You can replace virtio-win.iso with your own version; it is used directly and propagated to agents. If the ISO is missing, vJailbreak attempts to download it. Migration fails gracefully if both methods are unavailable.
---

:::note
This feature is available from vJailbreak v0.1.13 and later.
:::

## Bundled VirtIO ISOs

The vJailbreak appliance image already includes the VirtIO driver ISOs in `/home/ubuntu/virtio-win/`, so you do not need to upload anything for a standard migration:

- `virtio-win.iso` — the stable virtio-win release, used for Windows VMs
- `virtio-win-server12.iso` — virtio-win 0.1.185, used automatically for Windows Server 2012 and 2012 R2 guests

## How to use user-provided virtio-win.iso

To use a different VirtIO driver version, replace `virtio-win.iso` at the following path on the vJailbreak master node:

```bash
/home/ubuntu/virtio-win/virtio-win.iso
```

:::note
In the vJailbreak VM, there is already a folder named `virtio-win` at `/home/ubuntu/`, containing the bundled ISOs. Please make sure to upload to that directory and the name of the file should be `virtio-win.iso`.
:::


## How it works

If the user has scaled up vJailbreak, the ISO is propagated to all the agents.
When a Windows VM migration is initiated:

The migration logic checks for `/home/ubuntu/virtio-win/virtio-win.iso` on the source node. For Windows Server 2012 and 2012 R2 guests, it uses `/home/ubuntu/virtio-win/virtio-win-server12.iso` instead.

- **If found:**
  - The ISO is used for injecting **VirtIO drivers** into the migrated disk.
  - The ISO is **automatically propagated** to all agent nodes if needed.
- **If not found:**
  - vJailbreak attempts to **download** the ISO from a known upstream source (e.g., [fedorapeople.org](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/)).
- **If both methods fail:**
  - Migration fails gracefully with a clear error message.

:::note[Does not apply to LDM system volumes]
Offline driver injection requires `virt-v2v` to write into the guest filesystem.
Windows VMs whose system volume is on a dynamic disk (LDM) skip conversion
entirely, so this ISO is not used for them — the VirtIO drivers must be installed
on the **source** VM instead. See
[Windows Dynamic Disk (LDM) Migration](../windows-ldm-migration/).
:::







