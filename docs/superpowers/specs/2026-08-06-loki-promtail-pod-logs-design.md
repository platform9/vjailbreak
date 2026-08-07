# Design: Loki + Promtail Pod Log Visibility in Grafana

**Date:** 2026-08-06
**Status:** Approved

## Problem

vJailbreak Grafana only has a Prometheus datasource (metrics). Pod logs from `migration-system` are not visible in Grafana — operators must `kubectl logs` manually to debug migration failures.

## Goal

Show pod logs from the `migration-system` namespace in Grafana using Loki as the log backend and Promtail as the log collector. Must work airgapped and survive appliance upgrades.

## Non-Goals

- Logs from namespaces other than `migration-system`
- Log-based alerting
- Long-term log archival beyond 7 days

## Decisions

| Question | Decision | Reason |
|----------|----------|--------|
| Log scope | `migration-system` only | Reduce noise and disk use |
| Retention | 7 days | Sufficient for active debugging |
| Storage | PVC via `local-path` provisioner | Survives Loki pod restarts; k3s ships `local-path` |
| Deployment pattern | Raw manifests in `k8s/kube-prometheus/manifests/` | Consistent with existing monitoring stack; zero `install.sh` changes |
| Image pre-baking | Add to `download_images.sh` | Airgap support; existing `ctr import` loop handles load |

## Architecture

```
[migration-system pods]
        |  stdout/stderr written to /var/log/pods/
        v
[Promtail DaemonSet]  -- scrapes /var/log/pods/, filters to migration-system ---> [Loki StatefulSet]
                                                                                          |
                                                                              [Grafana Loki datasource]
                                                                                          |
                                                                                   [Grafana Explore/Dashboards]
```

All components run in the `monitoring` namespace, matching existing kube-prometheus stack.

## Components

### Loki

- **Image:** `docker.io/grafana/loki:3.5.0`
- **Kind:** StatefulSet (1 replica), single-binary mode
- **Storage:** 10Gi PVC via `local-path` storageClass, mounted at `/loki`
- **Retention:** 7 days (`retention_period: 168h`, `retention_deletes_enabled: true`)
- **Resources:** requests 100m CPU / 128Mi RAM; limits 200m CPU / 256Mi RAM
- **Service:** ClusterIP on port 3100, name `loki.monitoring.svc`

### Promtail

- **Image:** `docker.io/grafana/promtail:3.5.0`
- **Kind:** DaemonSet (one pod per node)
- **Scope:** Scrapes `/var/log/pods/` but filters to `migration-system` namespace via pipeline stage `drop` on all other namespaces
- **Resources:** requests 50m CPU / 64Mi RAM; limits 100m CPU / 128Mi RAM
- **RBAC:** ClusterRole with `get`/`list`/`watch` on pods and nodes (needed for label enrichment)

### Grafana datasource

- Add Loki entry to `grafana-dashboardDatasources.yaml` secret alongside existing Prometheus entry
- URL: `http://loki.monitoring.svc:3100`
- `editable: false` (matches Prometheus pattern)

## Files Changed

### New files

| Path | Purpose |
|------|---------|
| `k8s/kube-prometheus/manifests/loki-serviceAccount.yaml` | ServiceAccount |
| `k8s/kube-prometheus/manifests/loki-configMap.yaml` | Loki config (retention, filesystem storage) |
| `k8s/kube-prometheus/manifests/loki-pvc.yaml` | 10Gi PVC |
| `k8s/kube-prometheus/manifests/loki-statefulset.yaml` | Single-binary StatefulSet |
| `k8s/kube-prometheus/manifests/loki-service.yaml` | ClusterIP service |
| `k8s/kube-prometheus/manifests/promtail-serviceAccount.yaml` | ServiceAccount |
| `k8s/kube-prometheus/manifests/promtail-clusterRole.yaml` | RBAC: read pods/nodes |
| `k8s/kube-prometheus/manifests/promtail-clusterRoleBinding.yaml` | Bind role to SA |
| `k8s/kube-prometheus/manifests/promtail-configMap.yaml` | Scrape config (migration-system filter) |
| `k8s/kube-prometheus/manifests/promtail-daemonset.yaml` | DaemonSet |

### Modified files

| Path | Change |
|------|--------|
| `k8s/kube-prometheus/manifests/grafana-dashboardDatasources.yaml` | Add Loki datasource |
| `image_builder/scripts/download_images.sh` | Add `grafana/loki:3.5.0` and `grafana/promtail:3.5.0` to images array |

### No changes required

- `image_builder/scripts/install.sh` — existing loops handle image import and manifest apply
- `Makefile` — `docker-build-image` already copies `k8s/kube-prometheus/` into image

## Airgap + Upgrade Path

1. `download_images.sh` pulls and exports Loki + Promtail as `.tar` files at build time
2. Packer bakes all `/images/*.tar` into appliance QCOW2
3. `install.sh` imports all `*.tar` at first boot via `ctr images import`
4. `install.sh` applies all `k8s/kube-prometheus/manifests/` files — picks up new Loki/Promtail manifests automatically
5. New appliance version = new QCOW2 with updated manifests and images baked in

## Verification

After deployment, confirm:
```bash
# Loki running
kubectl -n monitoring get pod -l app=loki

# Promtail running on node
kubectl -n monitoring get pod -l app=promtail

# Grafana shows Loki datasource
# Grafana → Explore → select "loki" → query: {namespace="migration-system"}
```
