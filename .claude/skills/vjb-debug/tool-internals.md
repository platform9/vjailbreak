# Tool Internals Reference

Underlying tool behavior explanations for Convert/Data Copy failures.
When a tool error surfaces, use this file to answer **WHY** the tool behaves that way.

---

## guestfish `-i` flag (auto-inspect) {#guestfish-i}

`-i` / `--inspector` tells guestfish to:
1. Call `inspect-os` across **all** disks attached via `-a`
2. Enumerate OS roots by scanning for `/etc/os-release`, Windows registry hives, bootloaders
3. If `count(roots) == 1`: auto-mount root → proceed
4. If `count(roots) > 1`: **fail** with `guestfish: multi-boot operating systems are not supported`

### When does multi-boot false-positive trigger?

| Scenario | Why guestfish sees multiple roots |
|---|---|
| **OpenSUSE/SUSE + Btrfs + Snapper (most common)** | Snapper stores snapshots at `@/.snapshots/N/snapshot` — each subvolume is a full OS tree with `/usr`, `/etc`, `/boot`. libguestfs `inspect-os` counts each snapshot as a separate root |
| Multi-disk VM where disk 2 is a data disk with `/etc/` | Data disk accidentally has OS-like directory structure |
| Two disks each with a separate Linux install | Genuinely two OS roots |
| Btrfs `@` root subvolume + multiple Snapper snapshots | libguestfs enumerates Btrfs subvolumes; some versions count each snapshot as a root |

### Diagnosing which case applies

```bash
# Test each disk INDIVIDUALLY to isolate:
sudo guestfish --ro -a /dev/vdb : run : inspect-os
sudo guestfish --ro -a /dev/vdc : run : inspect-os
```

- Disk 1 alone returns **2+ roots** → Btrfs/Snapper subvolumes on disk 1 itself
- Each disk returns **1 root** → both disks have real OS installs
- Disk 2 returns **0** → disk 2 is clean data disk; issue is on disk 1 alone (Btrfs)

### Where this hits in vJailbreak code

`prepareGuestfishCommand` at `v2v-helper/virtv2v/virtv2vops.go:821` builds:
```
guestfish --rw -a /dev/vdb -a /dev/vdc -i -- <command>
```
All disks passed together with `-i`. On multi-disk VMs with Btrfs, this always triggers multi-boot detection.

**Fix direction**: inspect disks individually, or replace `-i` with `run` + `list-filesystems` + manual `mount` on the identified boot disk only.

---

## libguestfs `inspect-os` internals {#inspect-os}

`inspect-os` is a high-level libguestfs call that:
1. Calls `list-filesystems` to enumerate all filesystems
2. Mounts each filesystem (read-only) and looks for OS signatures
3. Btrfs special handling: enumerates subvolumes, may mount each to inspect

**Btrfs subvolume enumeration**: libguestfs sees Btrfs subvolumes as separate mountable filesystems. On OpenSUSE with Snapper:
- Root subvolume `@` → looks like OS root
- `@/.snapshots/1/snapshot` → also looks like OS root (contains full `/usr`, `/etc`)
- Both get counted → `inspect-os` returns list of 2+ roots → guestfish `-i` fails

**Workaround** (for manual guestfish sessions):
```bash
# Instead of -i, use:
guestfish --rw -a /dev/vdb : run : list-filesystems
# Then manually mount the root:
guestfish --rw -a /dev/vdb : run : mount /dev/vdb2 / : <command>
```

---

## virt-v2v-in-place conversion pipeline {#virt-v2v}

**vJailbreak calls `virt-v2v-in-place` specifically** (`v2v-helper/virtv2v/virtv2vops.go:493`),
not plain `virt-v2v`. The in-place variant operates on the guest's disk(s) directly rather than
producing a separately-converted output image, auto-detects the OS location (LVM or regular
partition — see comment at `v2v-helper/migrate/conversion.go:132`), and has its own man page
(`virt-v2v-in-place(1)`) with a different option set than plain virt-v2v. When checking behavior
against upstream docs, use `virt-v2v-in-place(1)` and `virt-v2v-support(1)`, not the generic
`virt-v2v(1)` page — options and some behavior differ. For anything not resolved by this file, see
the `vjb-virtv2v-inplace-behavior` and `vjb-guestfs-behavior` specialist agents, which fetch the
live docs rather than relying on this static summary.

Guest-side conversion steps (same underlying libguestfs machinery as plain virt-v2v, hence the
same multi-boot risk below):

```
Input disk(s)
    ↓
inspect-os (same libguestfs call — same multi-boot risk)
    ↓
Mount guest root filesystem
    ↓
Inject virtio drivers (Linux: dracut rebuild; Windows: install-virtio-win12.ps1)
    ↓
Remove VMware Tools remnants
    ↓
Fix bootloader (grub2 device.map, /boot/grub2/grub.cfg)
    ↓
Fix /etc/fstab device references
    ↓
Rebuild initramfs (dracut or mkinitrd)
    ↓
Output: guest ready for KVM boot
```

**Key failure points:**

| Step | Common failure | Why |
|---|---|---|
| inspect-os | multi-boot / no root found | Btrfs subvolumes, multi-disk, missing OS markers |
| initramfs rebuild | virtio modules missing | dracut not finding virtio-blk/scsi/net modules |
| /etc/resolv.conf rename | immutable attribute | `chattr +i` set on source — virt-v2v tries to rename it |
| Fix bootloader | wrong device in grub.cfg | grub.cfg references `/dev/sda`; virtio disk is `/dev/vda` |
| VirtIO driver inject (Windows) | PCI slot exhaustion | `virtio-blk` driver uses 1 slot per disk; >~24 disks exhausts slots. Use `virtio-scsi`. |

### OpenSUSE-specific: mkinitrd vs dracut

OpenSUSE 15.x uses **dracut** (modern). Older SUSE used `mkinitrd`.
vJailbreak has `FixLegacyMkinitrd` at `v2v-helper/virtv2v/virtv2vops.go:1143` that detects old `mkinitrd` (no dracut) and wraps it.

If dracut is present: uses dracut directly → no issue.
If only `mkinitrd`: LVM path translation needed → wrapper installed automatically.

---

## nbdkit / nbdcopy {#nbdkit}

nbdkit serves VMware VDDK disk images over NBD protocol. nbdcopy reads them.

**Connection chain:**
```
VMware ESXi (NFC protocol, port 902)
    ↓ VDDK
nbdkit vddk plugin (unix socket in /tmp/nbdkit-*/nbdkit.sock)
    ↓ NBD
nbdcopy (reads blocks, writes to /dev/vdX)
```

**Key failure modes:**

| Error | Cause |
|---|---|
| nbdkit connection refused | ESXi NFC port 902 blocked, or VDDK libs not found at `/home/fedora/vmware-vix-disklib-distrib` |
| DNS lookup failure in nbdkit log | ESXi hostname not in `/etc/hosts` or DNS — add entry on vJailbreak VM |
| nbdcopy stalls at X% | Network partition to ESXi, NFC throttling, or ESXi host under load |
| VDDK: failed to open disk | Snapshot deleted mid-copy (another process?), or wrong moref |
| `transports=file:nbdssl:nbd` all fail | VDDK transport negotiation failed — check thumbprint mismatch |

**VDDK thumbprint mismatch**: if vCenter thumbprint in migration form doesn't match ESXi thumbprint during copy, VDDK silently falls back to plain NBD or fails. Check thumbprint with:
```bash
openssl s_client -connect <esxi-host>:443 </dev/null 2>/dev/null | openssl x509 -fingerprint -sha1 -noout
```

---

## Second-order diagnostic checklist

After finding a surface error in the log, run through:

1. **Which tool emitted it?** (guestfish / libguestfs / virt-v2v / nbdkit / nbdcopy / VDDK)
2. **What internal step of that tool was running?** (inspect-os, initramfs rebuild, disk copy, transport negotiation)
3. **What is that tool's assumption that broke?** (single OS root, dracut present, ESXi reachable by hostname, etc.)
4. **Is this a guest-OS characteristic** (Btrfs/Snapper, dynamic disk, immutable resolv.conf) **or an environment issue** (DNS, network, VDDK path)?
5. **Is it reproducible on disk 1 alone?** (isolate multi-disk interactions)
