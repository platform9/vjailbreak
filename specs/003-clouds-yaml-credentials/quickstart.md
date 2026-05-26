# Quickstart — clouds.yaml credentials for vjailbreak

This guide walks an operator through setting up a vjailbreak migration using the `clouds.yaml` credential format with OpenStack Application Credentials.

## Prerequisites

- vjailbreak appliance installed and running on a release containing this feature
- Destination OpenStack with Keystone Queens (2018) or newer
- `openstack` CLI configured against the destination
- Permission to create roles and Application Credentials on the destination

## Step 1 — Create a vjailbreak-scoped role (if not already present)

```bash
openstack role create vjailbreak-migrator
```

Grant the role on the destination project to the user that will own the Application Credential:

```bash
openstack role add --project <migration-project> \
  --user <issuing-user> vjailbreak-migrator
openstack role add --project <migration-project> \
  --user <issuing-user> member
```

The minimum role set vjailbreak requires for destination discovery is documented separately; for many deployments granting `member` plus a project-scoped `vjailbreak-migrator` (with the policy overrides for Cinder scheduler-stats and Nova hypervisor list) is sufficient.

## Step 2 — Create the Application Credential

```bash
openstack application credential create vjailbreak-svc \
  --role member \
  --role vjailbreak-migrator \
  --unrestricted=false \
  --expiration 2026-12-31T23:59:59Z \
  --description "vjailbreak migration appliance auth"
```

Record the `id` and `secret` from the output. The `secret` is shown once only.

## Step 3 — Build clouds.yaml

```yaml
# clouds.yaml
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

## Step 4 — Create the Kubernetes Secret

```bash
kubectl create secret generic openstack-creds-clouds \
  --namespace migration-system \
  --from-file=clouds.yaml=./clouds.yaml
```

## Step 5 — Create the OpenstackCreds resource

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

## Step 6 — Verify validation

```bash
kubectl -n migration-system get openstackcreds destination-creds -o yaml
```

Expected `status.conditions`:

- `CredentialsParsed=True` (Reason: `Parsed`)
- `CredentialsValidated=True` (Reason: `AuthSucceeded`)
- `RolesSufficient=True` (Reason: `RolesPresent`)
- `Expiring=False` (Reason: `NotApplicable` if not approaching expiration, or `Within30Days`/`Within7Days` if so)
- `Expired=False` (Reason: `Active`)

A resource showing all positive conditions above is ready for migration use.

## Step 7 — Rotate the Application Credential

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

# Verify re-validation (Secret watch triggers it within seconds)
kubectl -n migration-system get openstackcreds destination-creds \
  -o jsonpath='{.status.conditions[?(@.type=="CredentialsValidated")]}'
```

## Step 8 — Revoke after migration completion

```bash
openstack application credential delete vjailbreak-svc
```

The `OpenstackCreds` resource transitions to `CredentialsValidated=False, Reason=CredentialInvalidOrRevoked` on the next reconcile (triggered by the watch since the Application Credential is gone, but the auth attempt fails — verify with `kubectl get openstackcreds ... -o yaml`).

## Multi-cloud clouds.yaml

A single Secret can declare multiple clouds and back multiple `OpenstackCreds` resources:

```yaml
# clouds.yaml in one shared Secret
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
# OpenstackCreds resources picking different entries
---
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: OpenstackCreds
metadata:
  name: paris-creds
spec:
  secretRef: { name: openstack-multi-cloud }
  cloudName: dc-paris
---
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: OpenstackCreds
metadata:
  name: frankfurt-creds
spec:
  secretRef: { name: openstack-multi-cloud }
  cloudName: dc-frankfurt
```

## UI path (after PR #3)

Once the UI work in PR #3 is merged: navigate to the credential creation form, paste your `clouds.yaml` content into the default tab, select the cloud entry from the dropdown, confirm the "Application Credential" badge appears (when `auth_type: v3applicationcredential`), and submit. The UI writes the Secret and creates the `OpenstackCreds` resource for you.
