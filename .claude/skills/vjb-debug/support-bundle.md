# vJailbreak Support Bundle & Log Map

**Get the migration name first** (see [architecture.md](architecture.md)) — every artifact below is keyed by it.

## On the vJailbreak VM (namespace `migration-system`)

| Artifact | Location / command | Use for |
|---|---|---|
| Per-migration debug log | `/var/log/pf9/<migration-name>.log` | The human-readable summary of what happened for THIS migration. Start here. |
| Install log | `/var/log/pf9-install.log` | vJailbreak VM installation problems only (image pull errors, auth issues, YAML apply failures, proxy/network errors during initial setup). Re-run safely with `sudo bash /etc/pf9/install.sh` if needed. |
| Raw block-copy debug logs | `pframe/<v2v-helper-pod-name>/` on the VM | Block-level NBD/XCOPY copy detail — deliberately kept OUT of the normal per-migration log due to volume. Pull this specifically for Data-Copy-phase failures. |
| Controller logs | `kubectl -n migration-system logs -l control-plane=controller-manager -f` | Kubernetes-level reconciliation: phase transitions, CRD-level errors. Filter by the migration name to skip normal controller-runtime chatter. |
| v2v-helper pod logs | `kubectl -n migration-system logs <migration-name>-v2v-helper` | **First place to check** for any Data-Copy/Convert/Cutover failure — phase-by-phase trace (port creation, copy start/end, conversion start/end, cutover events) lives here, before reaching for the raw `pframe/` logs. |
| CRDs | `kubectl get <crd> -n migration-system -o yaml`; `kubectl describe <crd> <name> -n migration-system` | `Migration`, `MigrationPlan`, `VMwareCreds`, `OpenstackCreds`, `NetworkMapping`, `StorageMapping`, `MigrationTemplate`. `describe` gives `.status` conditions and last errors in human-readable form. |
| Credential secrets | `kubectl get secret <vmwarecreds-or-openstackcreds-name> -n migration-system -o jsonpath='{.data.<field>}' \| base64 -d` | Distinguishes "wrong password" from "wrong endpoint" from "network unreachable". Field names: `username`, `password`, `OS_AUTH_URL`, etc. |
| Settings | `kubectl get configmap vjailbreak-settings -n migration-system -o yaml` | Confirm actual values of `CLEANUP_PORTS_AFTER_MIGRATION_FAILURE`, `CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE`, `PERIODIC_SYNC_*`, etc. before concluding vJailbreak should have auto-cleaned something. |

## Expected Support-Bundle ZIP Layout

```
support-bundle/
├── pf9-install.log
└── migration-system/
    ├── <migration-name>.log                  # per-migration debug log
    ├── pframe/<v2v-helper-pod-name>/          # raw block-copy debug logs
    ├── controller-manager.log                 # kubectl logs -l control-plane=controller-manager
    ├── <migration-name>-v2v-helper.log        # kubectl logs <pod>
    ├── crds/
    │   ├── migration-<name>.yaml
    │   ├── migrationplan-<name>.yaml
    │   ├── vmwarecreds-<name>.yaml            # secret VALUES should be redacted/excluded
    │   ├── openstackcreds-<name>.yaml         # secret VALUES should be redacted/excluded
    │   ├── networkmapping-<name>.yaml
    │   └── storagemapping-<name>.yaml
    └── vjailbreak-settings-configmap.yaml
```

## PCD-Side Investigation (OpenStack CLI)

When the trail crosses from vJailbreak into OpenStack, use the OpenStack CLI directly — no skill handoff needed:

| Symptom | Commands |
|---|---|
| Volume stuck in `detaching`/`reserved`/`in-use` | `openstack volume show <id> --insecure` — look for `attachments`, `status`. Force-reset if needed: `openstack volume set --state available <id> --insecure` |
| Neutron port-create failing / subnet mismatch | `openstack port list --fixed-ip subnet=<subnet-id> --insecure`; `openstack subnet show <id> --insecure` |
| Port already in use / stale port | `openstack port show <port-id> --insecure`; `openstack port delete <port-id> --insecure` |
| Floating / secondary IP unreachable (WSFC) | `openstack port show <port-id> --insecure \| grep allowed_address_pairs`; add with `openstack port set --allowed-address ip-address=<cluster-ip> <port-id> --insecure` |
| Destination VM boot failure / "no valid host" | `openstack server show <id> --insecure`; `openstack server event list <id> --insecure` |
| Cinder backend out of space | `openstack volume service list --insecure`; check backend capacity in Cinder logs on the PCD host |

## Source Code Pointers (for bug investigation)

| Area | Location |
|---|---|
| Migration phase state machine | `k8s/migration/internal/controller/migration_controller.go` |
| VMwareCreds reconciliation | `k8s/migration/internal/controller/vmwarecreds_controller.go` |
| NBD/NFC copy | `v2v-helper/pkg/nbdcopy/` |
| Storage-Accelerated Copy (XCOPY) | `v2v-helper/pkg/storage/` |
| Hot-Add Proxy | `v2v-helper/pkg/hotadd/` |
| virt-v2v conversion | `v2v-helper/pkg/virtv2v/` |
| Network/interface persistence scripts | `scripts/generate-udev-mapping.sh`, `scripts/generate-mount-persistence.sh` |
| Firstboot scripts (Windows disks offline, etc.) | `scripts/firstboot/windows/` |
| CRD types | `k8s/migration/api/v1alpha1/` |

## References
- [architecture.md](architecture.md), [migration-lifecycle.md](migration-lifecycle.md)
- Docs: https://platform9.github.io/vjailbreak/guides/troubleshooting/debuglogs/
