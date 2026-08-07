# Loki + Promtail Pod Log Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Loki + Promtail to the vJailbreak appliance so pod logs from `migration-system` are visible in Grafana, working airgapped on every fresh install.

**Architecture:** Raw Kubernetes manifests added to `k8s/kube-prometheus/manifests/` — the existing `install.sh` `kubectl apply` loop picks them up at first boot with zero script changes. Loki images are pre-baked into the appliance QCOW2 via `download_images.sh`. Grafana gains a Loki datasource entry alongside the existing Prometheus datasource.

**Tech Stack:** Loki 3.5.0 (single-binary, filesystem storage), Promtail 3.5.0, k3s `local-path` StorageClass, Grafana 12.3.3 (already deployed)

## Global Constraints

- All manifests go in `k8s/kube-prometheus/manifests/` — namespace `monitoring`
- Loki image: `docker.io/grafana/loki:3.5.0`
- Promtail image: `docker.io/grafana/promtail:3.5.0`
- Log scope: `migration-system` namespace only — Promtail must drop all other namespaces
- Retention: 7 days (168h)
- PVC: 10Gi, storageClass `local-path`
- No changes to `install.sh`, `Makefile`, or any Go code
- `grafana-dashboardDatasources.yaml` is a Secret (not ConfigMap) — patch must preserve the existing Prometheus entry

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `k8s/kube-prometheus/manifests/loki-serviceAccount.yaml` | Create | Loki identity |
| `k8s/kube-prometheus/manifests/loki-configMap.yaml` | Create | Loki server config (storage, retention, schema) |
| `k8s/kube-prometheus/manifests/loki-pvc.yaml` | Create | 10Gi persistent storage for log chunks |
| `k8s/kube-prometheus/manifests/loki-statefulset.yaml` | Create | Loki single-binary pod |
| `k8s/kube-prometheus/manifests/loki-service.yaml` | Create | ClusterIP on 3100 — `loki.monitoring.svc` |
| `k8s/kube-prometheus/manifests/promtail-serviceAccount.yaml` | Create | Promtail identity |
| `k8s/kube-prometheus/manifests/promtail-clusterRole.yaml` | Create | Read pods/nodes for label enrichment |
| `k8s/kube-prometheus/manifests/promtail-clusterRoleBinding.yaml` | Create | Bind ClusterRole to SA |
| `k8s/kube-prometheus/manifests/promtail-configMap.yaml` | Create | Promtail scrape config (migration-system filter) |
| `k8s/kube-prometheus/manifests/promtail-daemonset.yaml` | Create | Promtail pod per node |
| `k8s/kube-prometheus/manifests/grafana-dashboardDatasources.yaml` | Modify | Add Loki datasource entry |
| `image_builder/scripts/download_images.sh` | Modify | Add loki + promtail to images array |

---

## Task 1: Loki Manifests

**Files:**
- Create: `k8s/kube-prometheus/manifests/loki-serviceAccount.yaml`
- Create: `k8s/kube-prometheus/manifests/loki-configMap.yaml`
- Create: `k8s/kube-prometheus/manifests/loki-pvc.yaml`
- Create: `k8s/kube-prometheus/manifests/loki-statefulset.yaml`
- Create: `k8s/kube-prometheus/manifests/loki-service.yaml`

**Interfaces:**
- Produces: Loki HTTP API at `http://loki.monitoring.svc:3100` — consumed by Task 2 (Promtail push URL) and Task 3 (Grafana datasource URL)

- [ ] **Step 1: Create `loki-serviceAccount.yaml`**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: loki
    app.kubernetes.io/name: loki
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: loki
  namespace: monitoring
automountServiceAccountToken: false
```

- [ ] **Step 2: Create `loki-configMap.yaml`**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: loki
    app.kubernetes.io/name: loki
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: loki-config
  namespace: monitoring
data:
  config.yaml: |
    auth_enabled: false

    server:
      http_listen_port: 3100
      grpc_listen_port: 9095
      log_level: warn

    common:
      instance_addr: 127.0.0.1
      path_prefix: /loki
      storage:
        filesystem:
          chunks_directory: /loki/chunks
          rules_directory: /loki/rules
      replication_factor: 1
      ring:
        kvstore:
          store: inmemory

    query_range:
      results_cache:
        cache:
          embedded_cache:
            enabled: true
            max_size_mb: 100

    schema_config:
      configs:
        - from: 2020-10-24
          store: tsdb
          object_store: filesystem
          schema: v13
          index:
            prefix: index_
            period: 24h

    limits_config:
      retention_period: 168h
      allow_structured_metadata: false

    compactor:
      working_directory: /loki/retention
      delete_request_store: filesystem
      retention_enabled: true
      retention_delete_delay: 2h
      retention_delete_worker_count: 150

    ruler:
      alertmanager_url: http://localhost:9093

    analytics:
      reporting_enabled: false
```

- [ ] **Step 3: Create `loki-pvc.yaml`**

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: loki
    app.kubernetes.io/name: loki
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: loki-storage
  namespace: monitoring
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: local-path
  resources:
    requests:
      storage: 10Gi
```

- [ ] **Step 4: Create `loki-statefulset.yaml`**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app: loki
    app.kubernetes.io/name: loki
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: loki
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: loki
  serviceName: loki
  template:
    metadata:
      labels:
        app: loki
        app.kubernetes.io/name: loki
        app.kubernetes.io/part-of: vjailbreak-monitoring
    spec:
      automountServiceAccountToken: false
      containers:
        - args:
            - -config.file=/etc/loki/config.yaml
            - -target=all
          image: docker.io/grafana/loki:3.5.0
          imagePullPolicy: IfNotPresent
          name: loki
          ports:
            - containerPort: 3100
              name: http
            - containerPort: 9095
              name: grpc
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 15
            timeoutSeconds: 1
          resources:
            limits:
              cpu: 200m
              memory: 256Mi
            requests:
              cpu: 100m
              memory: 128Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          volumeMounts:
            - mountPath: /etc/loki
              name: loki-config
              readOnly: true
            - mountPath: /loki
              name: loki-storage
            - mountPath: /tmp
              name: tmp
      nodeSelector:
        kubernetes.io/os: linux
      securityContext:
        fsGroup: 10001
        runAsGroup: 10001
        runAsNonRoot: true
        runAsUser: 10001
      serviceAccountName: loki
      volumes:
        - configMap:
            name: loki-config
          name: loki-config
        - persistentVolumeClaim:
            claimName: loki-storage
          name: loki-storage
        - emptyDir: {}
          name: tmp
```

- [ ] **Step 5: Create `loki-service.yaml`**

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app: loki
    app.kubernetes.io/name: loki
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: loki
  namespace: monitoring
spec:
  ports:
    - name: http
      port: 3100
      targetPort: http
    - name: grpc
      port: 9095
      targetPort: grpc
  selector:
    app: loki
  type: ClusterIP
```

- [ ] **Step 6: Validate YAML syntax for all 5 files**

```bash
python3 -c "
import yaml, sys
files = [
  'k8s/kube-prometheus/manifests/loki-serviceAccount.yaml',
  'k8s/kube-prometheus/manifests/loki-configMap.yaml',
  'k8s/kube-prometheus/manifests/loki-pvc.yaml',
  'k8s/kube-prometheus/manifests/loki-statefulset.yaml',
  'k8s/kube-prometheus/manifests/loki-service.yaml',
]
for f in files:
    yaml.safe_load(open(f))
    print(f'OK: {f}')
"
```

Expected: `OK:` printed for each file, no exceptions.

- [ ] **Step 7: Commit**

```bash
git add k8s/kube-prometheus/manifests/loki-serviceAccount.yaml \
        k8s/kube-prometheus/manifests/loki-configMap.yaml \
        k8s/kube-prometheus/manifests/loki-pvc.yaml \
        k8s/kube-prometheus/manifests/loki-statefulset.yaml \
        k8s/kube-prometheus/manifests/loki-service.yaml
git commit -m "feat(monitoring): add Loki manifests for pod log storage"
```

---

## Task 2: Promtail Manifests

**Files:**
- Create: `k8s/kube-prometheus/manifests/promtail-serviceAccount.yaml`
- Create: `k8s/kube-prometheus/manifests/promtail-clusterRole.yaml`
- Create: `k8s/kube-prometheus/manifests/promtail-clusterRoleBinding.yaml`
- Create: `k8s/kube-prometheus/manifests/promtail-configMap.yaml`
- Create: `k8s/kube-prometheus/manifests/promtail-daemonset.yaml`

**Interfaces:**
- Consumes: Loki push URL `http://loki.monitoring.svc:3100/loki/api/v1/push` (from Task 1)
- Produces: Log streams labelled `{namespace="migration-system", pod="...", container="..."}` in Loki

- [ ] **Step 1: Create `promtail-serviceAccount.yaml`**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: promtail
    app.kubernetes.io/name: promtail
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: promtail
  namespace: monitoring
automountServiceAccountToken: true
```

- [ ] **Step 2: Create `promtail-clusterRole.yaml`**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app: promtail
    app.kubernetes.io/name: promtail
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: promtail
rules:
  - apiGroups:
      - ""
    resources:
      - nodes
      - nodes/proxy
      - services
      - endpoints
      - pods
    verbs:
      - get
      - watch
      - list
```

- [ ] **Step 3: Create `promtail-clusterRoleBinding.yaml`**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app: promtail
    app.kubernetes.io/name: promtail
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: promtail
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: promtail
subjects:
  - kind: ServiceAccount
    name: promtail
    namespace: monitoring
```

- [ ] **Step 4: Create `promtail-configMap.yaml`**

The `relabel_configs` entry with `action: keep` and `regex: migration-system` ensures Promtail only opens log files from pods in `migration-system`. Pods in all other namespaces are dropped before any I/O occurs.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: promtail
    app.kubernetes.io/name: promtail
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: promtail-config
  namespace: monitoring
data:
  config.yaml: |
    server:
      http_listen_port: 9080
      grpc_listen_port: 0
      log_level: warn

    positions:
      filename: /run/promtail/positions.yaml

    clients:
      - url: http://loki.monitoring.svc:3100/loki/api/v1/push
        backoff_config:
          min_period: 500ms
          max_period: 5m
          max_retries: 10

    scrape_configs:
      - job_name: kubernetes-pods
        kubernetes_sd_configs:
          - role: pod
        pipeline_stages:
          - cri: {}
        relabel_configs:
          - source_labels: [__meta_kubernetes_namespace]
            action: keep
            regex: migration-system
          - source_labels: [__meta_kubernetes_pod_node_name]
            target_label: __host__
          - action: labelmap
            regex: __meta_kubernetes_pod_label_(.+)
          - source_labels: [__meta_kubernetes_namespace]
            target_label: namespace
          - source_labels: [__meta_kubernetes_pod_name]
            target_label: pod
          - source_labels: [__meta_kubernetes_pod_container_name]
            target_label: container
          - replacement: /var/log/pods/*$1/*.log
            separator: /
            source_labels:
              - __meta_kubernetes_pod_uid
              - __meta_kubernetes_pod_container_name
            target_label: __path__
```

- [ ] **Step 5: Create `promtail-daemonset.yaml`**

Promtail needs read access to `/var/log/pods/` on the host. k3s writes pod logs there via containerd CRI. The positions file is written to an emptyDir so it persists across config reloads but not node reboots (acceptable — Promtail re-reads from last known offset or start of file).

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: promtail
    app.kubernetes.io/name: promtail
    app.kubernetes.io/part-of: vjailbreak-monitoring
  name: promtail
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: promtail
  template:
    metadata:
      labels:
        app: promtail
        app.kubernetes.io/name: promtail
        app.kubernetes.io/part-of: vjailbreak-monitoring
    spec:
      containers:
        - args:
            - -config.file=/etc/promtail/config.yaml
          image: docker.io/grafana/promtail:3.5.0
          imagePullPolicy: IfNotPresent
          name: promtail
          ports:
            - containerPort: 9080
              name: http
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 10
            timeoutSeconds: 1
          resources:
            limits:
              cpu: 100m
              memory: 128Mi
            requests:
              cpu: 50m
              memory: 64Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            readOnlyRootFilesystem: true
          volumeMounts:
            - mountPath: /etc/promtail
              name: promtail-config
              readOnly: true
            - mountPath: /var/log/pods
              name: pods-logs
              readOnly: true
            - mountPath: /run/promtail
              name: positions
      nodeSelector:
        kubernetes.io/os: linux
      securityContext:
        runAsGroup: 0
        runAsUser: 0
      serviceAccountName: promtail
      tolerations:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
          operator: Exists
        - effect: NoSchedule
          key: node-role.kubernetes.io/control-plane
          operator: Exists
      volumes:
        - configMap:
            name: promtail-config
          name: promtail-config
        - hostPath:
            path: /var/log/pods
          name: pods-logs
        - emptyDir: {}
          name: positions
```

- [ ] **Step 6: Validate YAML syntax for all 5 files**

```bash
python3 -c "
import yaml, sys
files = [
  'k8s/kube-prometheus/manifests/promtail-serviceAccount.yaml',
  'k8s/kube-prometheus/manifests/promtail-clusterRole.yaml',
  'k8s/kube-prometheus/manifests/promtail-clusterRoleBinding.yaml',
  'k8s/kube-prometheus/manifests/promtail-configMap.yaml',
  'k8s/kube-prometheus/manifests/promtail-daemonset.yaml',
]
for f in files:
    yaml.safe_load(open(f))
    print(f'OK: {f}')
"
```

Expected: `OK:` printed for each file, no exceptions.

- [ ] **Step 7: Commit**

```bash
git add k8s/kube-prometheus/manifests/promtail-serviceAccount.yaml \
        k8s/kube-prometheus/manifests/promtail-clusterRole.yaml \
        k8s/kube-prometheus/manifests/promtail-clusterRoleBinding.yaml \
        k8s/kube-prometheus/manifests/promtail-configMap.yaml \
        k8s/kube-prometheus/manifests/promtail-daemonset.yaml
git commit -m "feat(monitoring): add Promtail DaemonSet to ship migration-system logs to Loki"
```

---

## Task 3: Grafana Datasource

**Files:**
- Modify: `k8s/kube-prometheus/manifests/grafana-dashboardDatasources.yaml`

**Interfaces:**
- Consumes: Loki service URL `http://loki.monitoring.svc:3100` (from Task 1)
- Produces: Grafana "loki" datasource available in Explore and dashboards

- [ ] **Step 1: Read current file**

Open `k8s/kube-prometheus/manifests/grafana-dashboardDatasources.yaml`. The `stringData.datasources.yaml` field currently contains one entry for `prometheus`. We must add a second entry for `loki` without removing the prometheus entry.

- [ ] **Step 2: Add Loki datasource entry**

The file is a Kubernetes Secret with `stringData`. Edit the `datasources` array to add the Loki entry:

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    app.kubernetes.io/component: grafana
    app.kubernetes.io/name: grafana
    app.kubernetes.io/part-of: kube-prometheus
    app.kubernetes.io/version: 12.3.3
  name: grafana-datasources
  namespace: monitoring
stringData:
  datasources.yaml: |-
    {
        "apiVersion": 1,
        "datasources": [
            {
                "access": "proxy",
                "editable": false,
                "name": "prometheus",
                "orgId": 1,
                "type": "prometheus",
                "url": "http://prometheus-k8s.monitoring.svc:9090",
                "version": 1
            },
            {
                "access": "proxy",
                "editable": false,
                "name": "loki",
                "orgId": 1,
                "type": "loki",
                "url": "http://loki.monitoring.svc:3100",
                "version": 1
            }
        ]
    }
type: Opaque
```

- [ ] **Step 3: Validate YAML syntax**

```bash
python3 -c "
import yaml
yaml.safe_load(open('k8s/kube-prometheus/manifests/grafana-dashboardDatasources.yaml'))
print('OK')
"
```

Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add k8s/kube-prometheus/manifests/grafana-dashboardDatasources.yaml
git commit -m "feat(monitoring): add Loki datasource to Grafana"
```

---

## Task 4: Image Pre-Baking for Airgap

**Files:**
- Modify: `image_builder/scripts/download_images.sh`

**Interfaces:**
- Produces: `image_builder/images/docker.io_grafana_loki_3.5.0.tar` and `image_builder/images/docker.io_grafana_promtail_3.5.0.tar` baked into appliance QCOW2

- [ ] **Step 1: Read current `download_images.sh`**

Open the file. Find the variable declarations section (around line 8-30) and the `images=(...)` array (around line 40-65). Note the naming pattern: one variable per image, then the variable referenced in the array.

- [ ] **Step 2: Add loki and promtail image variables**

After the existing image variables (e.g., after the `grafana=` line), add:

```bash
loki="docker.io/grafana/loki:3.5.0"
promtail="docker.io/grafana/promtail:3.5.0"
```

- [ ] **Step 3: Add variables to the images array**

In the `images=(...)` array, add after the `"$grafana"` entry:

```bash
  "$loki"
  "$promtail"
```

- [ ] **Step 4: Verify the diff looks correct**

```bash
git diff image_builder/scripts/download_images.sh
```

Expected diff: two new variable declarations and two new entries in the images array. No other changes.

- [ ] **Step 5: Commit**

```bash
git add image_builder/scripts/download_images.sh
git commit -m "feat(image-builder): pre-bake Loki and Promtail images for airgap support"
```

---

## Verification (on a running vJailbreak appliance)

After applying all tasks to a running appliance or deploying a newly built image:

```bash
# 1. Confirm Loki pod is Running
kubectl -n monitoring get pod -l app=loki
# Expected: loki-0   1/1   Running

# 2. Confirm Promtail pod is Running on each node
kubectl -n monitoring get pod -l app=promtail
# Expected: promtail-<hash>   1/1   Running

# 3. Check Loki is ready
kubectl -n monitoring exec loki-0 -- wget -qO- http://localhost:3100/ready
# Expected: ready

# 4. Check Promtail is scraping migration-system
kubectl -n monitoring logs -l app=promtail | grep migration-system
# Expected: log lines referencing migration-system pods

# 5. Verify in Grafana
# Navigate to Grafana → Explore → select "loki" datasource
# Run query: {namespace="migration-system"}
# Expected: log lines from migration-system pods appear
```
