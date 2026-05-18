# Data Model — clouds.yaml credentials

## OpenstackCreds CRD (extended)

### Spec — added field

| Field | Type | Required | Description |
|---|---|---|---|
| `cloudName` | `string` | No | Selects which cloud entry to use from a multi-entry `clouds.yaml` in the referenced Secret. Ignored when the Secret contains only legacy OS_* keys. When omitted with a single-entry `clouds.yaml`, that entry is used. When omitted with a multi-entry `clouds.yaml`, the resource reports `CredentialsParsed=False, Reason=AmbiguousCloudName`. |

### Spec — existing fields (unchanged, documented as legacy)

| Field | Type | Status |
|---|---|---|
| `secretRef` | `corev1.ObjectReference` | Unchanged. Points to the credential Secret. |
| `osAuthUrl` | `string` | Legacy. Used when Secret contains only OS_* keys. |
| `osAuthToken` | `string` | Legacy. |
| `osUsername`, `osPassword`, `osDomainName`, `osRegionName`, `osTenantName`, `osInterface`, `osIdentityApiVersion`, `osInsecure` | various | Legacy. |
| `flavors`, `pcdHostConfig`, `projectName` | various | Unchanged — orthogonal to credentials. |

### Status — replaced

**Removed**:
- `openstackValidationStatus` (string)
- `openstackValidationMessage` (string)

**Added**:
- `conditions []metav1.Condition` — Kubernetes-style condition slice. Standard fields: `Type`, `Status` (`True` / `False` / `Unknown`), `Reason`, `Message`, `LastTransitionTime`, `ObservedGeneration`.

## Condition Types

| Type | True meaning | False meaning |
|---|---|---|
| `CredentialsParsed` | Credential data parsed successfully from `clouds.yaml` or OS_* | Parsing failed |
| `CredentialsValidated` | Authentication succeeded against the destination Keystone | Authentication failed |
| `RolesSufficient` | The credential carries the roles required for the operations vjailbreak performs | One or more required roles missing |
| `Expiring` | An Application Credential expires within 30 days | Not approaching expiration, or not an Application Credential |
| `Expired` | An Application Credential has passed its `expires_at` | Not expired, or not an Application Credential |

Conventions:
- All Conditions are set during every reconcile.
- A Condition is `Unknown` until the reconcile step that determines it runs (e.g., `RolesSufficient` is `Unknown` while `CredentialsValidated=False`).
- `Expiring` and `Expired` are mutually exclusive when applicable.

Detailed Reason codes per Type are documented in `contracts/conditions.md`.

## Credential Secret data keys

### Mode A: clouds.yaml (preferred)

| Key | Type | Description |
|---|---|---|
| `clouds.yaml` | YAML string | Standard OpenStack clouds.yaml content. Top-level keys are cloud entries. |

### Mode B: legacy OS_* (back-compat)

| Key | Type | Description |
|---|---|---|
| `OS_AUTH_URL` | string | Keystone endpoint. |
| `OS_USERNAME` | string | User name. |
| `OS_PASSWORD` | string | User password. |
| `OS_DOMAIN_NAME` | string | Domain name. |
| `OS_REGION_NAME` | string | Region. |
| `OS_TENANT_NAME` | string | Project (legacy term). |
| `OS_INTERFACE` | string | `public` / `internal` / `admin`. |
| `OS_IDENTITY_API_VERSION` | string | Typically `3`. |

### Resolution precedence

1. If `clouds.yaml` is present in the Secret data → use Mode A. OS_* keys are silently ignored.
2. Else → use Mode B (existing path, unchanged).

## clouds.yaml — consumed subset

The system honors the following standard fields from each cloud entry:

- `auth.auth_url`
- `auth.username` + `auth.password` (for `auth_type: v3password`)
- `auth.application_credential_id` + `auth.application_credential_secret` (for `auth_type: v3applicationcredential`)
- `auth.user_domain_name`, `auth.project_domain_name`, `auth.project_name`, `auth.project_id`
- `auth_type` (`v3password`, `v3applicationcredential`)
- `region_name`
- `interface`
- `identity_api_version`, `compute_api_version`, `volume_api_version`, `image_api_version`, `network_api_version`
- `cacert` (inline only; on-disk paths are not resolvable inside the controller pod and produce a Condition hint)
- `verify`

Any additional fields not in this list are tolerated (clientconfig may consume them) but are not part of vjailbreak's documented contract.

## Sharing semantics

- A Mode A Secret MAY be referenced by multiple `OpenstackCreds` resources, each with a different `cloudName`. The Secret content is shared; each resource validates and operates against its own selected cloud entry independently. (FR-016)
- A Mode B Secret retains its existing implicit 1:1 binding with a single `OpenstackCreds` resource.

## State transitions

```text
OpenstackCreds created or referenced Secret changed
    │
    ▼
[Reconcile] Parse Secret
    │
    ├─ clouds.yaml present? ──Yes──▶ Parse via clientconfig
    │                                  │
    │                                  ├─ Single entry, cloudName empty: use it
    │                                  ├─ Multi entry, cloudName set:    use selected
    │                                  └─ Multi entry, cloudName empty:  CredentialsParsed=False (AmbiguousCloudName)
    │
    └─ Else (legacy OS_*) ───────────▶ Build AuthOptions from OS_* fields
    │
    ▼
[Reconcile] Authenticate to Keystone
    │
    ├─ Success ──▶ CredentialsValidated=True; proceed to role + expiration checks
    │
    └─ Fail    ──▶ CredentialsValidated=False with mapped Reason
    │
    ▼
[Reconcile] Role coverage check
    │
    └─ Result ──▶ RolesSufficient=True | False
    │
    ▼
[Reconcile] App Cred expiration check (if applicable)
    │
    └─ Result ──▶ Expiring=True | False, Expired=True | False
    │
    ▼
Status updated; controller requeues on next Secret-watch event or periodic interval (1 hour default).
```
