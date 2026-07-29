---
name: vjb-security-review
description: Use when asked to review, audit, or check vJailbreak for security vulnerabilities, security issues, or doing a pre-release security pass. Covers logging of sensitive data, Kubernetes RBAC, UI token handling, insecure code patterns, and deployment YAML pod security. Does NOT check library CVEs or dependency versions.
---

# vJailbreak Security Review

Targeted security audit across five areas. Read `references/checks.md` for grep commands and file-specific details per category.

## Output format

One line per finding:
```
path:line  [SEVERITY]  category: description. Recommendation.
```
Severities: `CRITICAL` / `HIGH` / `MEDIUM` / `LOW` / `INFO`. Group by category.

---

## Categories — what to check and where

| # | Category | Where | Key concerns |
|---|---|---|---|
| 1 | **Logging** | `k8s/migration/`, `v2v-helper/`, `pkg/vpwned/` | Credential values in log calls; struct formatting that dumps passwords; `redactingWriter` coverage in v2v-helper |
| 2 | **K8s RBAC** | `k8s/migration/config/rbac/`, `deploy/` | Wildcard grants; ClusterRole vs Role scope; secrets CRUD verbs; cluster-admin binding in installer.yaml |
| 3 | **UI Tokens** | `ui/src/` | `VITE_API_TOKEN` baked into browser bundle; localStorage; token in URL params; Bearer header in axios |
| 4 | **Code Patterns** | All four Go modules | Hardcoded secrets; cleartext creds in CRD spec fields (OpenstackCredsSpec); `InsecureSkipVerify`; HTTP not HTTPS; env vars for secrets |
| 5 | **Deployment YAMLs** | `deploy/`, `k8s/migration/config/` | Root containers; hostPath mounts; `hostNetwork`; privilege escalation; missing resource limits; NetworkPolicy gaps; secretKeyRef vs volume mounts |

**Load `references/checks.md` before running** — it has a broad sweep step (run first to catch unexpected patterns like shell scripts), plus exact grep patterns and known-issue file locations per category.

---

## Summary table (append to report)

| Category | CRITICAL | HIGH | MEDIUM | LOW | INFO |
|---|---|---|---|---|---|
| Logging | | | | | |
| K8s RBAC | | | | | |
| UI Tokens | | | | | |
| Code Patterns | | | | | |
| Deployment YAMLs | | | | | |
| **Total** | | | | | |

End with: top 3 findings needing immediate action.

---

## Out of scope
Library/dependency CVEs (`govulncheck`, `npm audit`), secrets in git history (`trufflehog`), runtime network traffic.
