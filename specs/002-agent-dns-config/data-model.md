# Data Model: Agent Node Custom Host Entries

## Go Types (k8s/migration/pkg/utils/cloudinit.go)

### HostEntry

```go
type HostEntry struct {
    IP        string   `json:"ip"`
    Hostnames []string `json:"hostnames"`
}
```

**Validation rules**:
- `IP`: non-empty, must parse as valid IPv4 or IPv6 via `net.ParseIP`
- `Hostnames`: non-empty slice, each hostname must match `^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`
- No two entries may share the same `IP`

**Serialization**: JSON array stored in ConfigMap key `AGENT_HOST_ENTRIES`. Empty config → absent key or empty string (both treated as zero entries).

---

## ConfigMap Storage

**ConfigMap**: `vjailbreak-settings` in namespace `migration-system` (existing)

**New key**: `AGENT_HOST_ENTRIES`

**Value format** (example):
```json
[
  {"ip":"192.168.1.101","hostnames":["esxi01.corp.local","esxi01"]},
  {"ip":"192.168.1.10","hostnames":["vcenter.corp.local"]},
  {"ip":"192.168.2.5","hostnames":["pcd.corp.local","pcd-api.corp.local"]}
]
```

**Absent/empty value**: treated as zero entries — no host entries appended to cloud-init.

---

## VjailbreakNode Annotation

**Annotation key**: `vjailbreak.io/reprovision`

**Valid values**:
- `"requested"` — set by UI/user; controller acts on this
- `"blocked"` — set by controller when reprovision requested but active migrations prevent it
- absent — normal state

**State transitions**:
```
(absent) --[user sets]--> "requested"
                             |
          [active migrations?]
         /                   \
     yes                      no
      |                        |
 "blocked"              delete VM, reset status
 (requeue)              remove annotation
                               |
                        (absent) → normal reconcile creates new VM
```

---

## UI Type Extensions

### SettingsForm (helpers.ts)

New field added to existing type:
```typescript
AGENT_HOST_ENTRIES: string  // JSON string, same storage format as ConfigMap value
```

### HostEntry (new UI type)

```typescript
interface HostEntry {
  ip: string
  hostnames: string[]
}
```

**UI validation rules** (mirrors Go validation):
- `ip`: required, must match IPv4 or IPv6 pattern
- `hostnames`: at least one entry, each must be a valid hostname
- Duplicate IPs across entries not allowed

---

## Cloud-init Output (generated)

Given host entries, `BuildUserData` produces:

```yaml
#cloud-config
write_files:
- path: /etc/pf9/k3s.env
  content: |
    export IS_MASTER=false
    export MASTER_IP=192.168.1.100
    export K3S_TOKEN=<token>
runcmd:
  - echo "Created k3s env variables!" > /home/ubuntu/cloud-init.log
  - echo "192.168.1.101 esxi01.corp.local esxi01" >> /etc/hosts
  - echo "192.168.1.10 vcenter.corp.local" >> /etc/hosts
  - echo "192.168.2.5 pcd.corp.local pcd-api.corp.local" >> /etc/hosts
```

When no host entries are configured, the `runcmd` only contains the existing log line — identical to current behavior.
