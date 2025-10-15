# VJailbreak API Health Check

A comprehensive integration test utility that validates all API endpoints used by the VJailbreak UI. Written in Go, runs as a Kubernetes Job, and generates detailed reports.

## Features

- Tests all UI API endpoints (Kubernetes API, vpwned SDK, OAuth2)
- Runs inside the cluster with proper service account permissions
- Generates both console and JSON reports
- Supports self-signed SSL certificates
- Can run as one-time Job or scheduled CronJob
- Exit code indicates test success/failure (CI/CD friendly)

## Tested Endpoints

### Kubernetes API
- `GET /api/v1/namespaces` - List namespaces

### VJailbreak CRDs
- `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/vjailbreaknodes`
- `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrations`
- `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/vmwarecreds`
- `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/openstackcreds`
- `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/clustermigrations`
- `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/esximigrations`

### vpwned SDK (gRPC-Gateway)
- `GET /dev-api/sdk/vpw/v1/version` - SDK version
- `GET /dev-api/sdk/vpw/v1/idp/providers` - List identity providers

### User Management (CRUD)
- `GET /dev-api/sdk/vpw/v1/idp/local/users` - List local users
- `POST /dev-api/sdk/vpw/v1/idp/local/users` - Create test user (healthcheck@vjailbreak.local)
- `PUT /dev-api/sdk/vpw/v1/idp/local/users/{email}` - Update test user
- `DELETE /dev-api/sdk/vpw/v1/idp/local/users/{email}` - Delete test user

**Note**: User CRUD tests create a test user `healthcheck@vjailbreak.local` with role `viewer`, update it, and then delete it. Set `Skip: true` in the endpoint definition to skip individual tests.

### Other Endpoints
- `POST /dev-api/sdk/vpw/v1/validate_openstack_ip` - Validate OpenStack IP

### OAuth2 Proxy
- `GET /oauth2/auth` - Check authentication
- `GET /oauth2/userinfo` - Get user info

### UI Pages
- `GET /` - Root page
- `GET /dashboard` - Dashboard page

## Quick Start

### Build Docker Image

```bash
cd tests/api-health-check
make docker-build
```

### Deploy as Kubernetes Job

```bash
# Deploy service account, role, and job
kubectl apply -f k8s/job.yaml

# Check status
kubectl get jobs -n migration-system | grep api-health-check

# View logs
kubectl logs -n migration-system job/api-health-check

# Get report
kubectl logs -n migration-system job/api-health-check | grep -A 100 "API HEALTH CHECK REPORT"
```

### Deploy as CronJob (Scheduled Testing)

```bash
# Deploy CronJob (runs every 15 minutes)
kubectl apply -f k8s/cronjob.yaml

# Check CronJob status
kubectl get cronjobs -n migration-system

# View latest run
kubectl get jobs -n migration-system | grep api-health-check-cron

# View logs from latest run
kubectl logs -n migration-system $(kubectl get pods -n migration-system -l job-name -o name | head -1)
```

### Run Locally (Development)

```bash
# Run with default settings (Kubernetes API calls will fail without token)
make run

# Run with custom URL
BASE_URL=https://10.9.2.145 SKIP_SSL_VERIFY=true go run main.go

# Run with service account token for K8s API auth
SA_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) \
BASE_URL=https://10.9.2.145 \
SKIP_SSL_VERIFY=true \
go run main.go

# Or provide token directly
SA_TOKEN="eyJhbGciOiJSUzI1..." \
BASE_URL=https://10.9.2.145 \
SKIP_SSL_VERIFY=true \
go run main.go
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | `https://10.9.2.145` | Base URL of the cluster |
| `SKIP_SSL_VERIFY` | `true` | Skip SSL certificate validation |
| `REPORT_FILE` | `/tmp/api-health-report.json` | Path to save JSON report |
| `SA_TOKEN` | (auto-detected) | Kubernetes service account token for API auth. If not set, reads from `/var/run/secrets/kubernetes.io/serviceaccount/token` |

### Kubernetes Configuration

Edit `k8s/job.yaml` or `k8s/cronjob.yaml` to customize:
- Base URL
- Schedule (CronJob only)
- Resource limits
- Namespace

## Output

### Console Output

```
Starting API Health Check...
Testing 15 endpoints on https://10.9.2.145
SSL Verification: false

[1/15] Testing: GET /api/v1/namespaces
  ✓ SUCCESS - 200 - 45ms

[2/15] Testing: GET /api/v1/namespaces/migration-system/vjailbreaknodes
  ✓ SUCCESS - 200 - 32ms

...

=====================================
API HEALTH CHECK REPORT
=====================================
Generated At: 2025-10-15T16:00:00Z
Cluster Host: https://10.9.2.145
Total Tests:  15
Successes:    14 (93.3%)
Failures:     1 (6.7%)
=====================================

FAILED ENDPOINTS:
-------------------------------------
✗ OAuth2 User Info
  Method: GET
  Path:   /oauth2/userinfo
  Error:  HTTP 401: Unauthorized

SUCCESSFUL ENDPOINTS:
-------------------------------------
✓ List Namespaces - HTTP 200 (45ms)
✓ List VJailbreakNodes - HTTP 200 (32ms)
...
```

### JSON Report

```json
{
  "TotalTests": 15,
  "SuccessCount": 14,
  "FailureCount": 1,
  "TestResults": [
    {
      "Endpoint": {
        "Name": "List Namespaces",
        "Method": "GET",
        "Path": "/api/v1/namespaces",
        "Description": "Kubernetes API - List namespaces",
        "RequiresAuth": true
      },
      "StatusCode": 200,
      "Success": true,
      "Error": "",
      "ResponseTime": 45000000,
      "Timestamp": "2025-10-15T16:00:00Z"
    }
  ],
  "GeneratedAt": "2025-10-15T16:00:00Z",
  "ClusterHost": "https://10.9.2.145"
}
```

## Integration with CI/CD

The tool exits with:
- **Exit Code 0**: All tests passed
- **Exit Code 1**: One or more tests failed

Use in CI/CD pipeline:

```bash
# Run as part of integration tests
kubectl apply -f k8s/job.yaml

# Wait for completion
kubectl wait --for=condition=complete --timeout=300s job/api-health-check -n migration-system

# Get exit code
kubectl get job api-health-check -n migration-system -o jsonpath='{.status.succeeded}'

# Fail pipeline if tests failed
if [ "$(kubectl get job api-health-check -n migration-system -o jsonpath='{.status.failed}')" != "" ]; then
  echo "API health check failed!"
  kubectl logs -n migration-system job/api-health-check
  exit 1
fi
```

## Cleanup

```bash
# Delete one-time job
kubectl delete -f k8s/job.yaml

# Delete CronJob
kubectl delete -f k8s/cronjob.yaml

# Delete just the job (keep RBAC)
kubectl delete job api-health-check -n migration-system
```

## Development

### Add New Endpoints

Edit `main.go` and add to `getAPIEndpoints()`:

```go
{
    Name:        "My New Endpoint",
    Method:      "GET",
    Path:        "/api/v1/my-endpoint",
    Description: "Description",
    RequiresAuth: true,
}
```

### Build and Test

```bash
# Build binary
make build

# Run locally
./api-health-check

# Build Docker image
make docker-build

# Push to registry
make docker-push REGISTRY=quay.io IMAGE_TAG=v1.0.0
```

## Troubleshooting

### Job Fails to Start

```bash
# Check pod status
kubectl get pods -n migration-system -l app=api-health-check

# Check pod logs
kubectl logs -n migration-system <pod-name>

# Check RBAC
kubectl auth can-i list namespaces --as=system:serviceaccount:migration-system:api-health-check-sa
```

### SSL Certificate Errors

Ensure `SKIP_SSL_VERIFY=true` is set for self-signed certificates.

### Permission Denied Errors

Check service account has proper ClusterRole:

```bash
kubectl get clusterrolebinding api-health-check-binding -o yaml
```

## License

Same as VJailbreak project.
