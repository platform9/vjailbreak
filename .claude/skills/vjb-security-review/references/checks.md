# Security Check Details — vJailbreak

Detailed grep patterns and checks per category. Read this when running the actual review.

---

## Broad Sweep (run first)

Before the targeted category checks, do a broad sweep to catch unexpected patterns the specific checks might miss:

```bash
# All credential-adjacent patterns across all source files
grep -rn -i "password\|token\|secret\|apikey\|api_key\|auth" \
  --include="*.go" --include="*.ts" --include="*.sh" \
  k8s/migration/ v2v-helper/ pkg/ ui/src/ scripts/ deploy/ \
  | grep -v "_test.go\|vendor\|node_modules\|\.pb\.go" \
  | grep -v "// \|TODO\|FIXME\|comment" \
  | head -100

# Shell scripts specifically — often missed, can contain token injection
find . -name "*.sh" | xargs grep -ln "token\|password\|secret\|JWT\|Bearer" 2>/dev/null
```

Review any surprising results before running targeted checks. The broad sweep surfaces patterns not covered below.

---

## Category 1 — Logging of Sensitive Data

```bash
# Credential values near log calls
grep -rn "log\.\(Info\|Error\|Debug\|Warning\).*\(Password\|Token\|Secret\|APIKey\|ApiKey\|AuthToken\|Passwd\)" \
  k8s/migration/ v2v-helper/ pkg/vpwned/

# Env var values logged
grep -rn 'log.*os\.Getenv\|fmt.*os\.Getenv' v2v-helper/ k8s/migration/

# Struct formatting that would dump credentials
grep -rn '%v\|%+v' k8s/migration/ v2v-helper/ | grep -i "cred\|secret\|password\|token"

# Command string logging — cmd.String() or cmd.Args logged before redaction
grep -rn 'cmd\.String()\|log.*cmd\|Printf.*cmd' v2v-helper/
```

**Key file:** `v2v-helper/virtv2vops.go` — `log.Printf("Executing %s", cmd.String())` pattern appears at multiple lines. Any future auth flag added to these commands silently logs credentials. The safe variant (`RunCommandWithLogFileRedactedCategory`) exists in nbdops.go but is not used here.

**Mitigation to verify:** `redactingWriter` in `v2v-helper/pkg/utils/` — confirm it's plumbed into ALL log writers, not just nbdkit. Check main.go and any logger setup. `PrintLog`/`WriteToLogFile` paths are NOT covered by the redacting writer.

**govmomi risk:** errors from VMware calls sometimes echo the URL with embedded credentials. Check error wrapping in `pkg/common/vmware/`.

---

## Category 2 — Kubernetes RBAC

```bash
# Wildcard grants
grep -rn '"\*"' k8s/migration/config/rbac/ deploy/

# ClusterRole vs Role (over-scoped)
grep -rl "ClusterRole\b" k8s/migration/config/rbac/ deploy/

# ServiceAccount token automount
grep -rn "automountServiceAccountToken" deploy/ k8s/migration/config/
```

**Key file:** `k8s/migration/config/rbac/role.yaml` — check secrets verbs: `create/delete/get/list/patch/update/watch`. Verify each verb is actually exercised by the controller. `list`+`watch` on secrets is broad.

**Key file:** `deploy/installer.yaml` — check for `ClusterRoleBinding` that grants `cluster-admin` (present as of last audit).

**CRD creds:** `migration-manager-role` has full CRUD on `VMwareCreds`, `OpenstackCreds` — flag if broader than needed.

---

## Category 3 — UI Token Handling

```bash
# Token/credential storage
grep -rn "localStorage\|sessionStorage" ui/src/

# Token in URL or params
grep -rn "token\|apikey\|api_key" ui/src/ | grep -i "url\|href\|param\|query"

# Hardcoded secrets
grep -rn "password\s*=\s*['\"][^'\"]\|token\s*=\s*['\"][^'\"]" ui/src/
```

**Key file:** `ui/src/api/axios.ts` — `VITE_API_TOKEN` is baked into the browser bundle at build time. Determine: is this a static service-account token (ok) or a per-user credential (not ok — move to runtime API call)? Also check error handlers don't log the token.

**Key file:** `ui/startup.sh` — check for `envsubst` or `cat /var/run/secrets/kubernetes.io/serviceaccount/token` patterns that inject K8s JWTs directly into HTML at container startup. This is a HIGH severity issue: the live SA token gets served in the HTTP response to every browser.

**`withCredentials: true`** — verify server-side CORS restricts origin to vJailbreak host.

---

## Category 4 — Code Patterns

```bash
# Hardcoded credentials
grep -rn -i 'password\s*:=\s*"\|password\s*=\s*"\|secret\s*:=\s*"\|apikey\s*:=\s*"' \
  k8s/migration/ v2v-helper/ pkg/

# Env vars for sensitive data (visible in /proc/<pid>/environ)
grep -rn 'os\.Getenv.*\(PASSWORD\|TOKEN\|SECRET\|KEY\|AUTH\)' v2v-helper/ k8s/migration/ pkg/

# Insecure TLS
grep -rn "InsecureSkipVerify" k8s/migration/ v2v-helper/ pkg/

# HTTP (non-TLS) to external systems
grep -rn '"http://' k8s/migration/ v2v-helper/ pkg/ ui/src/ | grep -v "localhost\|127\.0\.0\.1"
```

**Key issue:** `k8s/migration/api/v1alpha1/openstackcreds_types.go` — `OpenstackCredsSpec` stores `OsPassword`, `OsAuthToken`, `OsUsername` as plain strings. These land in etcd unencrypted. The controller creates a k8s Secret from these but does NOT clear the spec fields afterward — credentials persist in etcd in both places. Compare with VMwareCreds `SecretRef` pattern — that's the better approach.

**Key issue:** `k8s/migration/pkg/sdk/keystone/util.go` — `KEYSTONE_PASSWORD` read from env var. Verify the source is a k8s Secret (`secretRef`) not a ConfigMap (`configMapRef`). ConfigMaps store plaintext in etcd; passwords must come from Secrets.

```bash
# Check if pf9-env is a ConfigMap or Secret source
grep -rn "pf9-env\|KEYSTONE_PASSWORD" deploy/ k8s/migration/config/
```

**Mitigation:** `os.Getenv` for secrets — prefer k8s Secret volume mounts (file-based, rotation-friendly, not visible in process list).

---

## Category 5 — Deployment YAMLs & Pod Security

```bash
# Root containers
grep -rn "runAsUser:\s*0\|runAsGroup:\s*0\|runAsNonRoot:\s*false" deploy/ k8s/migration/config/

# Privileged / escalation
grep -rn "privileged:\s*true\|allowPrivilegeEscalation:\s*true" deploy/ k8s/migration/config/

# Host path mounts
grep -rn "hostPath:" deploy/ k8s/migration/config/

# Host namespace sharing
grep -rn "hostNetwork:\s*true\|hostPID:\s*true\|hostIPC:\s*true" deploy/ k8s/migration/config/

# Missing resource limits
grep -rn "resources:" deploy/ k8s/migration/config/

# Unpinned images (latest tag or no tag)
grep -rn "image:.*:latest\|image:.*[^:]$" deploy/ k8s/migration/config/

# Secret env var injection
grep -rn "secretKeyRef:" deploy/ k8s/migration/config/

# Network policies
find deploy/ k8s/migration/config/ -name "*.yaml" | xargs grep -l "NetworkPolicy" 2>/dev/null
```

**Known issues (verify current state):**
- `deploy/05controller-deployment.yaml`: `runAsUser: 0`, `hostNetwork: true` — document if intentional (VDDK?)
- `deploy/06vpwned-deployment.yaml`: AppArmor `unconfined`, D-Bus socket mount, systemd config mount
- `deploy/installer.yaml`: ClusterRoleBinding granting `cluster-admin` to controller SA
- No NetworkPolicy anywhere in deploy/

**Compare:** `deploy/vjailbreak-ai/deployment.yaml` uses `runAsNonRoot: true, runAsUser: 1001` — use as reference for what good looks like.
