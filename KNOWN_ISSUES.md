# Known Issues

## API Endpoints Return 401 Through Ingress

**Status**: Under Investigation  
**Severity**: High  
**Date**: 2025-10-16

### Symptom
Browser requests to `/api/v1/*` and `/apis/vjailbreak.k8s.pf9.io/*` return 401 when accessed through ingress (https://10.9.0.61/apis/...).

### What Works
- Direct K8s API access with SA token: ✅ Returns 200
- UI login and dashboard: ✅ Works correctly
- OAuth2/Dex authentication: ✅ Functional
- SDK endpoints: ✅ Some working

### What Doesn't Work
- `/api/v1/namespaces` → 401 from nginx
- `/apis/vjailbreak.k8s.pf9.io/...` → 401 from nginx

### Attempted Fixes
1. ✗ Ingress priority annotations (`priority: 100`)
2. ✗ Explicit `auth-url: ""` to disable OAuth2
3. ✗ Backend protocol `HTTPS` with `proxy-ssl-verify: false`
4. ✗ Narrowing UI ingress paths (not using `/ Prefix`)
5. ✗ Custom header ConfigMap for Authorization
6. ✗ Removing UI ingress entirely

### Root Cause Hypothesis
The architecture likely requires the **UI backend to proxy API requests** rather than browsers sending service account tokens directly through ingress. This is a more secure pattern where:
- Browser → UI (with OAuth2 session)
- UI backend → K8s API (with SA token)
- Browser never handles SA tokens

### Next Steps
1. Check how cluster 10.9.2.145 handles `/apis` requests
2. Verify if UI code proxies API calls server-side
3. Consider updating UI to proxy K8s API requests through its backend
4. Review nginx ingress controller global settings

### Workaround
For now, the UI can make API calls from its backend pod where SA tokens work correctly.

---

**Last Updated**: 2025-10-16 23:51 IST
