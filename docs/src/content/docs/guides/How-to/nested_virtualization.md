---
title: Enable Nested Virtualization for Faster Migrations
description: Enable nested virtualization in the vJailbreak VM so that virt-v2v can use KVM-accelerated QEMU instead of slow software emulation, significantly reducing disk conversion times.
---

During disk conversion, virt-v2v launches a lightweight QEMU VM via libguestfs to modify guest disks. If `/dev/kvm` is present in the vJailbreak VM, QEMU uses **KVM hardware acceleration**. Otherwise it falls back to **TCG software emulation**, which can be up to **10x slower**.

Since the v2v-helper pod mounts the host `/dev` directory into the container, making `/dev/kvm` available on the vJailbreak VM is all that is needed.

## Enable Nested Virtualization on the Hypervisor

### OpenStack (Nova)

Ensure nested virtualization is enabled on the compute node hosting the vJailbreak VM:

```bash
# Intel
cat /sys/module/kvm_intel/parameters/nested   # Should output: Y

# AMD
cat /sys/module/kvm_amd/parameters/nested     # Should output: 1
```

If not enabled:

```bash
# Intel
sudo modprobe -r kvm_intel && sudo modprobe kvm_intel nested=1

# AMD
sudo modprobe -r kvm_amd && sudo modprobe kvm_amd nested=1
```

To persist across reboots, add to `/etc/modprobe.d/kvm.conf`:

```
options kvm_intel nested=1   # Intel
options kvm_amd nested=1     # AMD
```

The Nova flavor or image for the vJailbreak VM must also expose the host CPU model (e.g., `cpu_mode=host-passthrough`).

## Verify Inside the vJailbreak VM

SSH into the vJailbreak VM and run:

```bash
# CPU virtualization extensions visible (vmx = Intel, svm = AMD)
grep -Ec '(vmx|svm)' /proc/cpuinfo

# KVM kernel modules loaded
lsmod | grep kvm

# /dev/kvm exists
ls -l /dev/kvm
```

If `/dev/kvm` is present, no further action is needed — v2v-helper pods will automatically use KVM acceleration.
