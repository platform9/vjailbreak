---
title: Enable KVM for virt-v2v (Nested Virtualization)
description: How to enable nested virtualization so virt-v2v inside the v2v-helper pod can use KVM acceleration instead of falling back to slow TCG emulation.
---

## Overview

vJailbreak runs `virt-v2v` inside a Kubernetes pod (`v2v-helper`) to convert VM disks and inject drivers. By default, `virt-v2v` uses QEMU as its conversion backend. QEMU can run in two modes:

| Mode | How it works | Performance |
|------|--------------|-------------|
| **KVM** (hardware-accelerated) | Uses `/dev/kvm` — the host kernel's KVM module | Fast |
| **TCG** (software emulation) | Pure software fallback, no `/dev/kvm` required | ~10–20× slower |

When `/dev/kvm` is not visible inside the pod, QEMU falls back to TCG and logs:

```
qemu-kvm: Could not access KVM kernel module: No such file or directory
qemu-kvm: falling back to tcg
```

Conversions still succeed but take significantly longer.

## How vJailbreak exposes `/dev/kvm`

The v2v-helper pod already mounts the host `/dev` directory into the pod at `/dev` via a `hostPath` volume. This means **if `/dev/kvm` exists on the vJailbreak VM, it is automatically visible inside the pod** — no additional configuration is required.

The condition that must be met is that the vJailbreak VM itself has access to KVM, which requires nested virtualization to be enabled at every layer of the stack.

⚠️ **Caution — this affects all VMs on the compute host, not just vjailbreak.**

## Enabling Nested Virtualization

### Step 1 — Enable nested KVM on the compute host
On each compute host that will run vjailbreak VMs and agents:

```
cat /sys/module/kvm_intel/parameters/nested   # expect Y (or 1)
# AMD: cat /sys/module/kvm_amd/parameters/nested
```

If disabled, enable it persistently and reload:

```
# Intel
echo "options kvm-intel nested=1" | sudo tee /etc/modprobe.d/kvm-nested.conf
# AMD
# echo "options kvm-amd nested=1" | sudo tee /etc/modprobe.d/kvm-nested.conf

sudo modprobe -r kvm_intel && sudo modprobe kvm_intel   # or reboot the host
```

### Step 2 — Set the CPU mode in `nova_override.conf`

`[libvirt]` section of `/opt/pf9/etc/nova/conf.d/nova_override.conf` on each
compute host.

**Recommended — `host-passthrough`** (passes `vmx`/`svm` through automatically):

```
[libvirt]
cpu_mode = host-passthrough
```

**Alternative — `host-model`** (must add the flag explicitly; `host-model` does
**not** expose `vmx`/`svm` by default):

```
[libvirt]
cpu_mode = host-model
cpu_model_extra_flags = vmx    # use svm on AMD
```

> **Note:** `host-passthrough` ties the VM to a host with a closely matching
> CPU, model, and microcode for live migration. For vjailbreak — a
> short-lived, migration-tool VM — this is rarely a concern. 

### Step 3 — Apply

Restart Nova on the compute host, then hard-reboot the vjailbreak VM so it picks
up the new CPU definition (a soft reboot is not enough):

```
sudo systemctl restart pf9-ostackhost
openstack server reboot --hard <instance-uuid>
```

### Step 4 — Verify

Inside the vjailbreak VM, confirm the flag is present:

```
grep -E -o 'vmx|svm' /proc/cpuinfo | sort -u    # expect vmx (Intel) or svm (AMD)
ls /dev/kvm                                      # device node should exist
```

If `virt-v2v` still falls back to TCG (no acceleration), check that
`/dev/kvm` is present and that `kvm-ok` (if available) reports acceleration can
be used.

### Pod (automatic)

No changes are needed. The pod's `/dev` hostPath mount makes `/dev/kvm` visible to the `v2v-helper` container as soon as it exists on the vJailbreak VM.

## Verifying KVM is in use

After enabling nested virtualization and restarting a migration, inspect the v2v-helper pod logs. You should **not** see the TCG fallback warning. Instead, look for QEMU initializing with KVM:

```bash
kubectl -n migration-system logs <migration-name>-v2v-helper | grep -i kvm
```

A healthy KVM-accelerated run shows no `falling back to tcg` messages and noticeably faster conversion times.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `Could not access KVM kernel module` in logs | `/dev/kvm` missing on vJailbreak VM | Enable nested virt on the outer hypervisor and load the KVM module |
| `kvm_intel`/`kvm_amd` module fails to load | Virtualization extensions not exposed by hypervisor | Configure the outer hypervisor to pass through CPU virt flags |
| `/dev/kvm` present on VM but not in pod | Unlikely — the hostPath mount covers all of `/dev` | Confirm the pod spec has the `/dev` hostPath volume (default in vJailbreak) |
