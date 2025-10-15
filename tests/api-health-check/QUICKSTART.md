# Quick Start - API Health Check

## Run in 3 Steps

### Step 1: Build the Image

```bash
cd tests/api-health-check
make docker-build
```

### Step 2: Deploy to K3s Cluster

```bash
# Deploy the job
kubectl apply -f k8s/job.yaml

# This creates:
# - ServiceAccount with cluster read permissions
# - ClusterRole and ClusterRoleBinding
# - Job that runs the health check
```

### Step 3: View Results

```bash
# Watch job progress
kubectl get jobs -n migration-system -w

# View full report
kubectl logs -n migration-system job/api-health-check

# Or view just the summary
kubectl logs -n migration-system job/api-health-check | grep -A 20 "API HEALTH CHECK REPORT"
```

---

## Sample Output

```
Starting API Health Check...
Testing 15 endpoints on https://10.9.2.145
SSL Verification: false

[1/15] Testing: GET /api/v1/namespaces
  ✓ SUCCESS - 200 - 45ms

[2/15] Testing: GET /dev-api/sdk/vpw/v1/idp/local/users
  ✓ SUCCESS - 200 - 32ms

[3/15] Testing: GET /dev-api/sdk/vpw/v1/idp/providers
  ✓ SUCCESS - 200 - 28ms

=====================================
API HEALTH CHECK REPORT
=====================================
Generated At: 2025-10-15T16:00:00Z
Cluster Host: https://10.9.2.145
Total Tests:  15
Successes:    14 (93.3%)
Failures:     1 (6.7%)
=====================================
```

---

## Customize for Your Environment

Edit `k8s/job.yaml` and change:

```yaml
env:
- name: BASE_URL
  value: "https://YOUR_CLUSTER_IP"  # Change this
- name: SKIP_SSL_VERIFY
  value: "true"  # Set to "false" if using valid certs
- name: SA_TOKEN  # Optional - auto-detected from /var/run/secrets
  value: "your-token-here"  # Only if you need to override
```

**Note**: When running in the cluster as a Job, the service account token is automatically mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token` and used for Kubernetes API authentication.

---

## Schedule Automatic Checks

Deploy as CronJob (runs every 15 minutes):

```bash
kubectl apply -f k8s/cronjob.yaml

# View scheduled runs
kubectl get cronjobs -n migration-system
kubectl get jobs -n migration-system | grep api-health-check-cron
```

---

## Cleanup

```bash
# Delete one-time job
kubectl delete -f k8s/job.yaml

# Or delete CronJob
kubectl delete -f k8s/cronjob.yaml
```

---

## What Gets Tested

- Kubernetes API endpoints (`/api/v1/*`)
- VJailbreak CRDs (nodes, migrations, credentials)
- vpwned SDK endpoints (`/dev-api/sdk/vpw/v1/*`)
- Identity Provider APIs
- OAuth2 Proxy endpoints
- UI static pages

**Exit Code**: 0 = All passed, 1 = Some failed (CI/CD friendly)
