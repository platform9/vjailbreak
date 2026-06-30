# Condition Types and Reasons — OpenstackCreds

## CredentialsParsed

| Status | Reason | When |
|---|---|---|
| True | `Parsed` | Credential data parsed from `clouds.yaml` or OS_* successfully |
| False | `InvalidYAML` | `clouds.yaml` content fails YAML parsing |
| False | `MissingRequiredField` | A required field (e.g., `auth_url`) is missing from the parsed content |
| False | `AmbiguousCloudName` | Multi-entry `clouds.yaml` with no `cloudName` set; message lists available entries |
| False | `UnknownAuthType` | `auth_type` value is not supported by vjailbreak |
| False | `CacertPathUnresolvable` | `cacert` references an on-disk path; inline content required |

## CredentialsValidated

| Status | Reason | When |
|---|---|---|
| True | `AuthSucceeded` | Authenticated to destination Keystone successfully |
| False | `CredentialInvalidOrRevoked` | 401 from token endpoint (includes revoked Application Credentials) |
| False | `KeystoneUnreachable` | Network failure reaching the auth endpoint |
| False | `TLSVerificationFailed` | Cert chain validation failed (distinct from network failure) |

## RolesSufficient

| Status | Reason | When |
|---|---|---|
| True | `RolesPresent` | All vjailbreak-required roles are granted to the credential |
| False | `InsufficientRoles` | One or more required roles missing; message lists them |
| Unknown | (none) | Not yet evaluated (`CredentialsValidated=False`) |

## Expiring

| Status | Reason | When |
|---|---|---|
| True | `Within30Days` | App Cred `expires_at` is within 30 days |
| True | `Within7Days` | App Cred `expires_at` is within 7 days (more urgent) |
| False | `NotApplicable` | Credential is not an Application Credential, or no `expires_at` |
| False | (none) | Application Credential not approaching expiration |

## Expired

| Status | Reason | When |
|---|---|---|
| True | `Expired` | App Cred `expires_at` has passed; subsequent auth attempts will fail |
| False | `Active` | App Cred valid by expiry check |
| False | `NotApplicable` | Not an Application Credential |

## Aggregate readiness

A resource is "ready for migration" when:
- `CredentialsParsed=True` AND
- `CredentialsValidated=True` AND
- `RolesSufficient=True` AND
- `Expired=False`

`Expiring=True` does not block readiness; it surfaces a warning for the operator to plan a rotation.

## Message conventions

- Messages MUST NOT contain credential secret material (passwords, application_credential_secret, tokens).
- Messages SHOULD reference the cloud name and auth URL when relevant for operator diagnosis.
- For `InsufficientRoles`, the message lists the missing role names (extracted from Keystone error response where available).
- For `AmbiguousCloudName`, the message lists the cloud entry keys present in the parsed YAML.

## Standard Reason code stability

Reason codes listed above are the contract: external tooling (alerts, dashboards) can rely on these strings. Additional Reason codes MAY be introduced in future releases; removal or rename is a breaking change requiring deprecation notice.
