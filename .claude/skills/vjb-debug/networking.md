# vJailbreak Networking

## Network Mapping
Source VMware network must be a **vSphere Standard or Distributed Port Group** — **NSX-backed networks are not supported** for mapping. The destination PCD network must already exist (vJailbreak does not create networks).

**Common failure**: mapping to a destination network whose subnet doesn't contain the source VM's IP. The Neutron port-create call is "bound to fail" because the requested fixed IP can't belong to that subnet. Recovery is per the retry-vs-cleanup tree in [migration-lifecycle.md](migration-lifecycle.md) (mapping failures require refill-and-retry) — or enable **Fallback to DHCP** so vJailbreak grabs any free IP from the mapped network instead of forcing the original one.

## Storage Mapping
Source VMware datastore (VMFS or NFS) maps to a destination Cinder backend/volume-type. Unlike network mapping, this is about **data transfer**, not connectivity — the datastore type can differ from the destination backend type (e.g. NFS datastore → iSCSI Cinder backend), **unless** using Storage-Accelerated Copy, which requires the exact same array end-to-end (see [copy-methods.md](copy-methods.md)).

Not compatible: vCenter Storage Policies / vVols. RDM disks are handled by a **separate, dedicated RDM migration feature** (detach from vCenter, `cinder manage` instead) — plain storage mapping does not cover RDM disks.

## Network / Interface Persistence

Only active when **"Persist source network interfaces"** is explicitly enabled.

### Linux
Statically-configured interfaces keep their **original interface name** post-migration (e.g. `ens3` stays `ens3`). Mechanism: vJailbreak generates a **udev rule binding the interface's MAC address to that specific name**. This works because netplan/ifcfg/network config on disk is keyed by interface name, and normally virt-v2v's NIC driver swap (VMware vmxnet3 → virtio) makes the kernel enumerate the NIC under a **new** default name (e.g. `eth1`), silently orphaning the pre-existing static config. Because the udev rule only touches the naming layer — not the IP config itself — there's no risk of it conflicting with whatever's already on disk; the OS's own network stack picks up the pre-existing config automatically once the name matches again.

DHCP-configured interfaces instead get a freshly generated name (`vjb<random_number>`) since there's nothing meaningful to preserve.

Verified distros: Ubuntu, OpenSUSE, RHEL, CentOS. Rocky is unverified (may work, not confirmed).

### Windows
- **Server 2016 and later**: static interfaces keep original name, IP, and gateway. DHCP interfaces get renamed to `vjb_<random_number>`.
- **Server 2012 and earlier**: IP is preserved but the interface **silently converts to DHCP** configuration, and neither the interface name nor the gateway are preserved — a PowerShell capability gap on those OS versions. Server 2008/2012 are considered **unsupported** for full network persistence; expect to fix manually.

### Constraints (apply to both OS families)
- "Assign IP" and "Persist Network" are **mutually exclusive** UI options.
- Persistence does **not** work across a cross-network (subnet-changing) migration — the old static config wouldn't be meaningful on a different subnet anyway.
- Only one IP per interface is settable via the UI's Assign-IP flow.

## Case Study: WSFC Cluster IP Unreachable via Neutron Anti-Spoofing

A fully worked example of a non-obvious PCD-side networking failure, from a real post-migration RCA. Recognize this pattern rather than re-deriving it from scratch.

**Setup**: 2-node Windows Server Failover Cluster (WSFC) with a floating Cluster IP, RDM-backed shared storage, migrated VMware → PCD.

**Symptom chain** (in the order actually encountered):
1. **Domain trust broken** ("The trust relationship between this workstation and the primary domain failed") on both nodes post-migration. Cause: virt-v2v powers the VM off for conversion, and the AD machine-account password state drifted out of sync with the VM's local state during that offline window. Fix: rejoin both nodes to the domain (`Test-ComputerSecureChannel` should return `True` once fixed). ~30 min per VM.
2. **The Cluster IP was completely unreachable — this is the actual novel finding.** After rejoining the domain and getting the cluster service running, `ping <cluster-ip>` from the OWNING node's own IP returned "Destination host unreachable," while ping to the gateway, the other node, and the domain controller all worked normally — isolating the failure specifically to the floating Cluster IP.

**Root cause**: When a WSFC Cluster IP resource comes online, Windows sends a **Gratuitous ARP (GARP)** — an unsolicited broadcast announcing "this IP is now at this MAC" — so the network learns where to route traffic for the floating IP. **Neutron enforces port-level anti-spoofing by default**: each port only accepts outbound traffic sourced from the single IP/MAC pair it was created with. Since the floating Cluster IP was never part of either node's Neutron port definition, Neutron **silently dropped the GARP** before it ever reached the OVS uplink — no device on the network ever learned where the floating IP lived. This was never a problem on the original VMware vSwitch, which doesn't enforce equivalent anti-spoofing by default — a classic "worked on vSphere, breaks on OpenStack" gap that has nothing to do with vJailbreak's copy/convert logic itself.

**Fix**: add the Cluster IP as a Neutron **allowed-address-pair** on **both** nodes' ports:
```bash
openstack port set --allowed-address ip-address=<cluster-ip> <node1-port-id> --insecure
openstack port set --allowed-address ip-address=<cluster-ip> <node2-port-id> --insecure
```
Verify: `openstack port show <port-id> --insecure | grep allowed_address_pairs`

This is a write action against OpenStack — per this skill's read-only stance, report these exact commands for the customer/operator to run rather than executing them yourself.

**Critical operational nuance — don't skip this when applying the fix**: adding the allowed-address-pair is a **permission change only**. It does **not** retroactively make the network aware of the IP. The GARP has to actually be **re-sent** for the fix to take effect, which happens automatically the next time the cluster service brings the Cluster IP resource online (i.e. on failover, or a cluster-group move/restart). Simply adding the Neutron rule and pinging immediately — without moving or restarting the cluster group — will still fail and look like the fix didn't work.

**Related facts from the same RCA that live elsewhere**: the WSFC's `ClusSvc` startup-type and missing-NetFT-adapter issues are guest-OS/cluster-feature problems, not networking — documented in [guest-os-issues.md](guest-os-issues.md).

**Recommended checklist additions for any WSFC (or other floating-IP) workload migration**:
- **Pre-migration**: detect the cluster's floating IP(s) from the source VM — `Get-ClusterResource | Where-Object {$_.ResourceType -eq "IP Address"} | Get-ClusterParameter` — and proactively add them as allowed-address-pairs on the destination ports **before** the VM is powered on.
- **Post-migration**: verify the floating IP is pingable, and verify the NetFT adapter is present on **all** nodes (not just the first one checked) before declaring the migration done — see [guest-os-issues.md](guest-os-issues.md).

## References
- [migration-lifecycle.md](migration-lifecycle.md), [guest-os-issues.md](guest-os-issues.md)
- Docs: `/vjailbreak/concepts/network-storage-mapping/`, `/vjailbreak/concepts/network-persistence/`
- If the root cause is confirmed Neutron-side (port/subnet/anti-spoofing), hand off to the `neutron` pcd-v skill for deeper Neutron/OVN log analysis.
