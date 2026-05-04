# Contract: Token Refresh API

## Endpoint

```
POST /api/v1/namespaces/migration-system/serviceaccounts/ui-manager-sa/token
```

Authenticated with the current bearer token. Standard k8s API — no custom endpoint.

## Request

```json
{
  "apiVersion": "authentication.k8s.io/v1",
  "kind": "TokenRequest",
  "spec": {
    "expirationSeconds": 86400
  }
}
```

`expirationSeconds` should match the configured projected volume TTL. The API server may return a token with a different lifetime — always use `expirationTimestamp` from the response.

## Response (success — HTTP 201)

```json
{
  "apiVersion": "authentication.k8s.io/v1",
  "kind": "TokenRequest",
  "status": {
    "token": "<JWT string>",
    "expirationTimestamp": "2026-05-05T10:00:00Z"
  }
}
```

`status.token` replaces `currentToken`.  
`status.expirationTimestamp` drives next refresh schedule.

## Response (failure — HTTP 4xx/5xx)

Frontend retains the existing `currentToken` and retries on the next scheduled interval. No user-visible error is shown unless the token has already expired (HTTP 401 on a subsequent API call).

## RBAC Required

```yaml
- apiGroups: [""]
  resources: ["serviceaccounts/token"]
  verbs: ["create"]
```

Added to `ui-manager-role` ClusterRole in `deploy/00crds.yaml`.
