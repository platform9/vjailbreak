# Credential Secret keys — contract

## Modes

The credential Secret operates in one of two modes, determined by the presence of the `clouds.yaml` key.

### Mode A: clouds.yaml (preferred)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openstack-creds
  namespace: migration-system
type: Opaque
stringData:
  clouds.yaml: |
    clouds:
      destination:
        auth_type: v3applicationcredential
        auth:
          auth_url: https://keystone.example.com:5000/v3
          application_credential_id: REDACTED
          application_credential_secret: REDACTED
        region_name: RegionOne
        interface: public
        compute_api_version: "2.95"
        volume_api_version: "3.70"
```

The `OpenstackCreds` resource references this Secret via `secretRef.name` and selects the cloud entry via `cloudName: destination`.

### Mode B: legacy OS_* (back-compat)

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
  OS_PASSWORD: REDACTED
  OS_DOMAIN_NAME: Default
  OS_REGION_NAME: RegionOne
  OS_TENANT_NAME: migration-project
  OS_INTERFACE: public
  OS_IDENTITY_API_VERSION: "3"
```

`cloudName` on the `OpenstackCreds` resource is ignored in this mode.

## Conflict handling

If both `clouds.yaml` AND any OS_* keys are present in the same Secret, `clouds.yaml` wins. The OS_* keys are silently ignored (no error, no warning condition).

## Sharing constraints

- A Mode A Secret MAY be referenced by multiple `OpenstackCreds` resources, each with a different `cloudName` (FR-016).
- A Mode B Secret retains its existing implicit 1:1 binding with a single `OpenstackCreds` resource.

## Mutation observation

The controller watches every credential Secret referenced by any `OpenstackCreds` resource. A change to a Secret's `data` (`clouds.yaml` content updated, OS_* values rotated, mode flipped) triggers re-reconciliation of all `OpenstackCreds` resources referencing that Secret within seconds. Already-running migrations are not aborted — see FR-018.

## Security note

`clouds.yaml` content may contain:
- User passwords (`auth.password`)
- Application Credential secrets (`auth.application_credential_secret`)
- TLS certificates / private keys (`cacert`)

Per the project's standard practice, the Secret is stored encrypted at rest by Kubernetes and access is governed by RBAC. The controller MUST NOT log or echo any of these values into Kubernetes Events, controller logs, or `OpenstackCreds.status.conditions[*].message` (see research R-9).
