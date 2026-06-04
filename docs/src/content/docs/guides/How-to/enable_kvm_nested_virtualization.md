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

## Enabling Nested Virtualization

### Layer 1 — Hypervisor (host running the vJailbreak VM)

The physical host (or outer hypervisor) must expose hardware virtualization extensions to its guest VMs.

**KVM/QEMU host**

Use `host-passthrough` or `host-model` CPU mode, or explicitly add `vmx` (Intel) / `svm` (AMD) to the CPU flags:

```xml
<cpu mode="host-passthrough"/>
```

**OpenStack**

Set the flavor's `hw:cpu_mode` to `host-passthrough` or enable the `hw:nested_virt` trait:

```bash
openstack flavor set <flavor> --property hw:cpu_mode=host-passthrough
```

---

### Layer 2 — vJailbreak VM (the Linux OS)

Once the hypervisor exposes the virtualization extensions, verify that the KVM kernel module is loaded inside the vJailbreak VM:

```bash
# For Intel CPUs
lsmod | grep kvm_intel

# For AMD CPUs
lsmod | grep kvm_amd
```

If the module is not loaded, load it manually (and add to `/etc/modules` to persist across reboots):

```bash
# Intel
modprobe kvm_intel
echo kvm_intel >> /etc/modules

# AMD
modprobe kvm_amd
echo kvm_amd >> /etc/modules
```

Verify `/dev/kvm` is present:

```bash
ls -l /dev/kvm
# Expected: crw-rw---- 1 root kvm ...
```

---

### Layer 3 — Pod (automatic)

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
