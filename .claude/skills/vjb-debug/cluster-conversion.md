# vJailbreak Cluster Conversion

**This is a structurally different feature from VM migration** — it converts an entire VMware **cluster**, including the physical ESXi hosts themselves, into PCD hypervisors. Everything else in this skill (`migration-lifecycle.md`, `copy-methods.md`, etc.) is about moving individual VMs; this file is about repurposing the hosts.

## What It Does
Converts a VMware cluster's ESXi hosts into PCD compute hypervisors, migrating each host's VMs along the way, so that at the end the entire cluster (hosts and VMs) runs on PCD instead of VMware.

## How It Works
1. User selects a source VMware cluster and a destination PCD cluster.
2. The wizard shows the cluster's ESXi hosts and their VMs; host setup options (NIC-to-network associations) derive from the **PCD cluster blueprint**.
3. Per-host sequential loop:
   - Host enters VMware maintenance mode.
   - Its running VMs evacuate (VMware live-migrate) to other ESXi hosts still in the source cluster.
   - The now-empty host is converted into a PCD hypervisor — Canonical **MAAS** performs the bare-metal OS install via IPMI/PXE.
   - VMs then migrate onto the newly-converted hypervisor.
   - Repeat for the next host.

## Prerequisites and Constraints
- The VMware side must be a genuinely **DRS-enabled cluster** — VM evacuation during the per-host loop depends on DRS actually being able to move VMs off the host being converted.
- **Irreversible.** Converting a host to a PCD hypervisor is explicitly documented as a one-time operation that cannot be undone — once converted, that host and its VMs are permanently PCD.

## Maturity Caveat — Read This Before Reassuring a Customer

Unlike standard VM migration (which is the well-exercised, primary code path), Cluster Conversion has a **documented history of being under-tested and regressing between releases**. Specifically:
- Engineering has had to actively re-test this feature after unrelated changes broke it — e.g. appending a VM-ID suffix (added to support duplicate-named VMs across folders in standard migration) broke Cluster Conversion's own VM-discovery logic, and open timeout issues were being reworked as of the most recent internal review.
- A customer-facing regression was found where the converted host's **NIC/interface-naming binding** (matching the PCD blueprint's interface names to the interfaces actually installed on the host) had tested and worked ~10–15 releases earlier, then silently regressed and was untested again by the time a customer needed it — that customer ultimately dropped their plan to use the feature.

**Do not treat Cluster Conversion as production-hardened by default.** Before advising a customer to rely on it:
- Check the current release notes for the vJailbreak version in use for any Cluster Conversion changes/fixes.
- Search for open tickets referencing cluster conversion, VM-ID discovery, or interface-name binding on converted hosts.
- If the customer's use case depends on a specific behavior (e.g. interface names matching the blueprint exactly), explicitly recommend they validate it in a non-production cluster first, rather than assuming it works because it's a documented feature.

## Debugging Pointers
- Uses the same `migration-controller-manager` and CRD reconciliation model as standard migration (see [architecture.md](architecture.md)) — the `migration-vpwned-sdk` pod backs the REST API specifically for this feature.
- If VM discovery inside a cluster-conversion wizard behaves inconsistently with duplicate-named VMs, that matches the known VM-ID-append regression above — don't assume it's a new bug, check whether it's already tracked.
- If a converted host doesn't come up with the expected interface names, that matches the known blueprint/interface-binding regression above.

## References
- [architecture.md](architecture.md), [migration-lifecycle.md](migration-lifecycle.md)
- Docs: `/vjailbreak/concepts/cluster-conversion/`, `/vjailbreak/guides/cluster-conversion/cluster-conversion/`, `/vjailbreak/guides/cluster-conversion/maas-enablement/`
