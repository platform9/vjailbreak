# vJailbreak Architecture

## Deployment Model
- vJailbreak ships as a single VM ("the vJailbreak VM" / "primary VM") deployed inside the TARGET OpenStack/PCD environment — not inside VMware.
- The VM runs k3s (lightweight Kubernetes) and hosts all vJailbreak components as pods in the `migration-system` namespace.
- One vJailbreak instance can be attached to MANY VMware vCenter/vSphere instances (via multiple `VMwareCreds` objects) but only ONE PCD/OpenStack environment — because the VM itself runs on that PCD's compute.
- SSH access: user `ubuntu`, default password `password` (must be changed on first login). Web UI: `http://<vjb-vm-ip>/`, gated by HTTP basic auth (default `admin`/`password`).
- VDDK (VMware Virtual Disk Development Kit) libraries — version 8.0.x — must be extracted to `/home/ubuntu/vmware-vix-disklib-distrib` on the VM. Without VDDK, vCenter cannot expose the NFC port needed for block-level disk reads. VDDK 9.x is known to have unresolved issues; stick to 8.0.x unless a specific fix note says otherwise.
- Deployed from a `vjailbreak.qcow2` image; must be created with disk bus `virtio-scsi` (not the default `virtio-blk`) — see "PCI Slot Exhaustion" in [guest-os-issues.md](guest-os-issues.md) for why this matters even for the VJB VM itself, not just migrated guests.

## Pods (namespace: `migration-system`)

| Pod | Purpose |
|---|---|
| `migration-controller-manager` | The Kubernetes controller. Watches all migration CRDs, drives their reconciliation loop / phase state machine, decides when to spawn a `v2v-helper` pod. |
| `vjailbreak-ui` | Web UI (React/TS/MUI/Vite) — discovery, mapping, migration-plan creation, log viewing. |
| `v2v-helper` | **One pod per in-flight migration**, named `<migration-name>-v2v-helper`. Does the actual work: disk copy (NBD/XCOPY/hot-add), `virt-v2v` conversion, cutover. This is where almost all migration-specific failures surface. |
| `migration-vpwned-sdk` | REST API server (Go module `pkg/vpwned`) backing Cluster Conversion specifically — see [cluster-conversion.md](cluster-conversion.md). |

## Custom Resource Definitions (all in `migration-system`)

| CRD | Purpose |
|---|---|
| `VMwareCreds` | vCenter/ESXi connection: host/IP, username, password, datacenter, insecure-TLS toggle. Backed by a same-named `Secret`. |
| `OpenstackCreds` | PCD/OpenStack connection. Password-based (`OS_AUTH_URL`, `OS_USERNAME`, `OS_PASSWORD`, `OS_REGION_NAME`, `OS_PROJECT_NAME`, ...) or token-based (`OS_AUTH_TOKEN`). Backed by a same-named `Secret`. |
| `NetworkMapping` | Source vSphere network/port-group ↔ destination PCD network. |
| `StorageMapping` | Source VMware datastore ↔ destination PCD Cinder backend/volume-type. |
| `MigrationTemplate` | Reusable set of mapping + option defaults, referenced by a `MigrationPlan`. |
| `MigrationPlan` | A batch of VMs to migrate together, plus shared options (cutover type, data-copy method, post-migration options). Creates one `Migration` object per VM. |
| `Migration` | **The correlation ID.** One object per VM migration, named after the migration/VM name. Its `.status.phase` is the state machine (Pending → Validating → DataCopy → Convert → [waitForAdminCutover →] Cutover → Completed \| Failed). Everything else — the `v2v-helper` pod name, the debug log filename — derives from this name. |

Get this name FIRST in any investigation: `kubectl get migration -n migration-system` (or from the UI's migration list).

## Credential Validation and Revalidation

- Revalidation does two things: (1) an auth check against the target (vCenter or OpenStack), (2) if auth succeeds, a full inventory resync (VMs, networks, datastores refreshed).
- Triggers: automatically on credential creation, automatically every **1 hour** by default, and on-demand via the UI's refresh button.
- If VM/network/datastore lists in the UI look stale, check when the credential last revalidated before assuming a real data-fetch bug — it may just be due for its hourly refresh.
- To read the actual configured values (not just "does auth work"): `kubectl get secret <vmwarecreds-or-openstackcreds-name> -n migration-system -o jsonpath='{.data.username}' | base64 -d` (substitute the field name for `password`, `OS_AUTH_URL`, etc). This is the fastest way to distinguish "wrong password" from "wrong endpoint" from "network unreachable" without going back through the UI.

## `vjailbreak-settings` ConfigMap

`kubectl get configmap vjailbreak-settings -n migration-system -o yaml`

| Setting | Default | Meaning |
|---|---|---|
| `DEFAULT_MIGRATION_METHOD` | `cold` | Whether new migrations default to hot (live) or cold (powered-off) copy. |
| `CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD` | `20` | Max iterations of changed-block (CBT) copy during hot migration before forcing convergence. |
| `VCENTER_SCAN_CONCURRENCY_LIMIT` | `10` | Concurrent VM-scan workers during vCenter discovery. Raise for faster discovery in large environments. |
| `VM_ACTIVE_WAIT_INTERVAL_SECONDS` / `VM_ACTIVE_WAIT_RETRY_LIMIT` | — | Timeout tuning for "wait for destination VM to become ACTIVE" post-boot. |
| `VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS` / `VOLUME_AVAILABLE_WAIT_RETRY_LIMIT` | — | Timeout tuning for "wait for Cinder volume to become available." Relevant to attach/detach-timeout symptoms — see [support-bundle.md](support-bundle.md). |
| `PERIODIC_SYNC_INTERVAL` | `1h` | Interval between delta-block syncs during Periodic Sync mode (see [migration-lifecycle.md](migration-lifecycle.md)). |
| `PERIODIC_SYNC_MAX_RETRIES` / `PERIODIC_SYNC_RETRY_CAP` | — | Retry/backoff tuning for periodic sync. |
| `CLEANUP_PORTS_AFTER_MIGRATION_FAILURE` | — | If enabled, Neutron ports created for a failed migration are deleted automatically. If disabled (check current value — don't assume), they're deliberately left so a retry can reuse them. |
| `CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE` | — | Same idea, for Cinder volumes created for a failed migration. |
| `VALIDATE_RDM_OWNER_VMS` | — | Ensures all VMs sharing an RDM disk are migrated together (prevents split-brain on shared RDM data). |
| v2v-helper pod CPU/memory/ephemeral-storage requests+limits | — | Tunable if migrations are getting OOMKilled or throttled; see agent sizing below for the underlying budget. |

**Always check these two `CLEANUP_*` flags before concluding "VJB should have cleaned up that stray port/volume automatically."** If they're off, leftover artifacts after a failed migration are expected behavior, not a bug — see the retry-vs-cleanup decision tree in [migration-lifecycle.md](migration-lifecycle.md).

## Agent Scaling (parallel migrations)

vJailbreak scales out via additional **agent VMs** — up to 5 total (1 primary + 4 additional), each running its own k3s and its own set of `v2v-helper` pods.

Per-migration resource budget on any one agent:

| Resource | Request | Limit |
|---|---|---|
| CPU | 1 core | 2 cores |
| Memory | 1 GiB | 3 GiB |
| Ephemeral storage | — | 3 GiB |

Recommended agent flavor sizing:

| Flavor | vCPUs | RAM | Storage | Concurrent migrations |
|---|---|---|---|---|
| Small | 8 | 16 GiB | 60 GiB | 2–3 |
| Medium | 16 | 32 GiB | 100 GiB | 5–7 |
| Large | 32 | 64 GiB | 200 GiB | 10–14 |
| X-Large | 48 | 96 GiB | 300 GiB | 15–21 |

Formula: `max concurrent migrations = min(available_CPU / 2, available_memory_GiB / 3, available_storage_GiB / 3)`, reserving ~20–25% of the flavor's resources as system overhead before applying the formula.

Operational notes:
- **Network bandwidth, not CPU/memory, is usually the actual limiting factor** — monitor utilization even if the formula says there's compute headroom.
- VDDK is auto-synced from the primary VM to every agent — don't manually re-upload it per agent.
- **L2-only networks without DHCP cannot support agent scale-up** — only the primary VM can run migrations in that topology, since agents need DHCP to get their own IP.
- Minimum disk per agent: 60 GiB.
- Agent VMs use the same `ubuntu`/`password` default SSH credential as the primary VM.

## Compatibility Matrix

**VMware vCenter Server**: 6.7, 7.0, 8.0.

**Guest OS — Linux/BSD (verified)**: CentOS 6, 7, 9 · Debian 12 · Oracle Linux 8 · RHEL 8, 9, 10 · Rocky 8, 9, 10 · SUSE Linux Enterprise 15 · Ubuntu 14, 15, 16, 17, 22.04, 24.04 · FreeBSD 14.
**Guest OS — Linux (expected, untested)**: AlmaLinux, Amazon Linux 2, CentOS 4/5, CentOS Stream 10, Debian 8, Oracle Linux 7, RHEL 4/5/7.
**Guest OS — Windows (verified)**: 11, 11 Enterprise, Server 2012, 2016, 2019, 2022, 2025.
**Notable gap**: VMware Photon OS has no verified or expected support — treat as unsupported until proven otherwise.

## Known Limitations (treat as expected behavior, not bugs)

| # | Limitation | Detail |
|---|---|---|
| 1 | Windows Dynamic Disk (LDM) | Cannot migrate directly — see [guest-os-issues.md](guest-os-issues.md). |
| 2 | Domain Controllers | Strongly not recommended — VM-GenerationID loss risks AD USN rollback. |
| 3 | Persist Network on Server 2012 and earlier | Depends on PowerShell capability unavailable on those OS versions. |
| 4 | "Assign IP" + "Persist Network" | Mutually exclusive UI options — cannot enable both. |
| 5 | Multi-IP per interface | Only one IP per NIC configurable via the UI's Assign-IP flow. |
| 6 | VMware Tools residual artifacts | Cosmetic only, no action needed — see [guest-os-issues.md](guest-os-issues.md). |
| 7 | Multi-boot VMs | Unsupported outright. |
| 8 | Hotplug flavor | Live CPU/RAM resize requires an admin-configured hotplug-enabled flavor on PCD; not automatic. |
| 9 | PCI slot exhaustion | Default `virtio-blk` caps around 26 attached devices — use `virtio-scsi`. |
| 10 | Low disk space | Source VM needs minimum free space (~100 MB for most filesystems) before conversion. |
| 11 | Virtual hardware version | Hot migration (CBT) requires VMware virtual hardware version 7+. |
| 12 | Reboots required | Both hot and cold migration require a guest reboot during conversion — applications must tolerate this. |

## References
- Upstream repo `CLAUDE.md`: https://github.com/platform9/vjailbreak/blob/main/CLAUDE.md
- Docs: `/vjailbreak/architecture/*`, `/vjailbreak/reference/compatibility/`, `/vjailbreak/reference/known-limitations/`, `/vjailbreak/guides/how-to/vjailbreak_settings/`, `/vjailbreak/guides/how-to/scaling/`, `/vjailbreak/concepts/credential-management/`
