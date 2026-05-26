# OpenStack credentials for vJailbreak

This guide describes how operators configure destination-OpenStack credentials for a vJailbreak migration. Two formats are supported:

1. **`clouds.yaml`** (preferred) — the standard OpenStack credential format used by `openstack` CLI, openstacksdk, and other ecosystem tooling. Unlocks **OpenStack Application Credentials** and **per-service microversion configuration**.
2. **Legacy `OS_*` Secret keys** — the original vJailbreak credential schema. Continues to work; no change required for existing deployments.

## Quick choice

| Use case | Pick |
|---|---|
| Already maintaining a `clouds.yaml` for OpenStack tooling | clouds.yaml |
| Want revocable, time-bound, role-subset credentials (Application Credentials) | clouds.yaml |
| Need to configure a destination microversion (e.g., to satisfy a non-default `compute_api_version`) | clouds.yaml |
| Have an existing OS_*-keyed Secret and migration runbook | OS_* (no change required) |

## clouds.yaml — recommended setup

### Step 1 — Create a role for vJailbreak (optional but recommended)

```bash
openstack role create vjailbreak-migrator
openstack role add --project <migration-project> --user <issuing-user> vjailbreak-migrator
openstack role add --project <migration-project> --user <issuing-user> member
```

### Step 2 — Create an Application Credential (recommended)

Application Credentials provide:

- Revocation independent of the underlying user account.
- Role-subset scoping — grant only the roles vJailbreak needs.
- Optional `expires_at` time bound.
- No user password stored at rest in Kubernetes.

```bash
openstack application credential create vjailbreak-svc \
  --role member \
  --role vjailbreak-migrator \
  --unrestricted=false \
  --expiration 2026-12-31T23:59:59Z \
  --description "vJailbreak migration appliance auth"
```

The `id` and `secret` are shown once. Record them now.

Minimum required Keystone version: **Queens (2018)**.

### Step 3 — Build `clouds.yaml`

```yaml
clouds:
  destination:
    auth_type: v3applicationcredential
    auth:
      auth_url: https://keystone.example.com:5000/v3
      application_credential_id: <id from step 2>
      application_credential_secret: <secret from step 2>
    region_name: RegionOne
    interface: public
    compute_api_version: "2.95"
    volume_api_version: "3.70"
```

Username/password auth is also supported (`auth_type: v3password` with `username`, `password`, `project_name`, `user_domain_name`, `project_domain_name` under `auth`).

### Step 4 — Create the Kubernetes Secret

```bash
kubectl create secret generic openstack-creds-clouds \
  --namespace migration-system \
  --from-file=clouds.yaml=./clouds.yaml
```

### Step 5 — Create the `OpenstackCreds` resource

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: OpenstackCreds
metadata:
  name: destination-creds
  namespace: migration-system
spec:
  secretRef:
    name: openstack-creds-clouds
    namespace: migration-system
  cloudName: destination
```

```bash
kubectl apply -f openstackcreds.yaml
```

### Step 6 — Verify validation

```bash
kubectl -n migration-system get openstackcreds destination-creds -o yaml
```

Expected `status.conditions`:

- `CredentialsParsed=True` (Reason: `Parsed`)
- `CredentialsValidated=True` (Reason: `AuthSucceeded`)
- `Expiring=False` (Reason: `NotApplicable` for non-App-Cred auth; `Within30Days` / `Within7Days` when an App Cred's expiration is near)
- `Expired=False` (Reason: `Active`)

A resource showing the positive conditions above is ready for migration use.

## Multi-cloud `clouds.yaml`

A single Secret may declare multiple clouds and back multiple `OpenstackCreds` resources, each selecting a different cloud entry via `cloudName`:

```yaml
clouds:
  dc-paris:
    auth_type: v3applicationcredential
    auth: { ... }
    region_name: paris
  dc-frankfurt:
    auth_type: v3applicationcredential
    auth: { ... }
    region_name: frankfurt
```

```yaml
---
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: OpenstackCreds
metadata: { name: paris-creds }
spec:
  secretRef: { name: openstack-multi-cloud }
  cloudName: dc-paris
---
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: OpenstackCreds
metadata: { name: frankfurt-creds }
spec:
  secretRef: { name: openstack-multi-cloud }
  cloudName: dc-frankfurt
```

When `clouds.yaml` contains a single cloud entry, `cloudName` may be omitted. When it contains multiple entries and `cloudName` is unset, the resource reports `CredentialsParsed=False` with `Reason: AmbiguousCloudName` and the available entries listed in the message.

## Microversion configuration

`clouds.yaml` per-service API version values are honored as an **operator-configurable floor** over internal hardcoded microversions:

```yaml
clouds:
  destination:
    compute_api_version: "2.95"   # honored as floor for Nova
    volume_api_version: "3.70"    # honored as floor for Cinder
    image_api_version: "2.16"     # honored as floor for Glance
    network_api_version: "2.0"    # honored as floor for Neutron
    identity_api_version: "3"     # honored as floor for Keystone
```

If the configured version is higher than the internal hardcoded value for a given operation, the configured value is used. If lower or absent, the internal hardcoded value applies (so misconfiguring a low version cannot break operations that require a higher one — for example, multi-attach volume attach always uses at least microversion 2.60).

## Application Credential rotation

The controller watches the credential Secret. To rotate:

```bash
# Create a replacement Application Credential
openstack application credential create vjailbreak-svc-2 \
  --role member --role vjailbreak-migrator \
  --expiration 2027-06-30T23:59:59Z

# Update the Secret in place with the new id/secret
kubectl create secret generic openstack-creds-clouds \
  --namespace migration-system \
  --from-file=clouds.yaml=./clouds.yaml \
  --dry-run=client -o yaml | kubectl apply -f -
```

Re-validation triggers automatically within seconds via the Secret watch. Already-running migrations continue with the auth token they acquired at start; subsequent migrations and the next validation use the rotated credential.

Revoke the old credential when ready:

```bash
openstack application credential delete vjailbreak-svc
```

## Inline CA certificates

When the destination Keystone is served by a private CA, supply the certificate inline in `clouds.yaml`:

```yaml
clouds:
  destination:
    auth_url: https://keystone.example.com:5000/v3
    cacert: |
      -----BEGIN CERTIFICATE-----
      <PEM content>
      -----END CERTIFICATE-----
    verify: true
```

Filesystem paths (e.g., `cacert: /etc/ssl/certs/...`) are not resolvable inside the controller pod and are rejected with `CredentialsParsed=False, Reason=CacertPathUnresolvable`.

## Legacy `OS_*` keys (back-compat)

Existing deployments using OS_*-keyed Secrets continue to work without changes:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openstack-creds-legacy
  namespace: migration-system
type: Opaque
stringData:
  OS_AUTH_URL: https://keystone.example.com:5000/v3
  OS_USERNAME: admin
  OS_PASSWORD: <password>
  OS_DOMAIN_NAME: Default
  OS_REGION_NAME: RegionOne
  OS_TENANT_NAME: migration-project
  OS_INTERFACE: public
  OS_IDENTITY_API_VERSION: "3"
```

`cloudName` on the `OpenstackCreds` resource is ignored when the Secret has only OS_* keys. When a Secret contains both `clouds.yaml` and OS_* keys, `clouds.yaml` takes precedence.

## Required roles

vJailbreak requires the destination credential to grant at least:

- The standard `member` role on the destination project.
- Sufficient permissions for vJailbreak's destination-side discovery: Cinder `scheduler-stats/get_pools` and `os-services` (admin-equivalent by default OpenStack policy), Nova `os-hypervisors` (admin-equivalent), and cross-project network read on Neutron (admin-equivalent).

In many deployments operators grant `admin` on the destination project for simplicity during migration windows, then revoke after cutover. For tighter scoping, ship a custom `vjailbreak-migrator` role with policy overrides on the relevant Cinder/Nova/Neutron endpoints.
