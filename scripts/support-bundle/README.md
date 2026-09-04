# vjailbreak support bundle

Two-script toolkit for **post-mortem migration analysis**. Run them when a
migration goes wrong (or just finished) to capture every relevant log into a
single tarball, then generate a self-contained HTML report.

Partial address of [#429](https://github.com/platform9/vjailbreak/issues/429)
— specifically the "Expose through logs" approach. Does **not** replace a live
in-UI dashboard; it's a support-engineer tool, not an end-user one.

## What's in the bundle

| Section | Source | Notes |
|---|---|---|
| Cluster overview | `kubectl version`, `get nodes`, `get ns` | Context |
| Controller logs | `vjailbreak` namespace, `control-plane=controller-manager` pods | Last 5000 lines + previous container |
| Migration CRDs | `kubectl get migration -o yaml` per migration | Full spec + status |
| v2v-helper pod logs | Every `v2v-helper-*` pod in `migration-system` | Last 20 000 lines + `kubectl describe` |
| Related CRDs | MigrationPlan, MigrationTemplate, NetworkMapping, StorageMapping, VjailbreakNode, VMwareMachine/Cluster/Host, RDMDisk | Raw YAML |
| Credentials | VMwareCreds, OpenstackCreds, ArrayCreds, EsxiSshCreds | **Redacted** (see below) |
| Events | `kubectl get events` in both namespaces, time-sorted | |
| Settings | `vjailbreak-settings` ConfigMap | Runtime config |

## Credentials redaction

Before anything is written to the tarball, credentials CRDs are piped through
`jq` to nullify any field whose key matches:

```
password | secret | token | key | credential   (case-insensitive)
```

The shape of the secret is preserved (so you can see it was set) but every
matching value is replaced with `***REDACTED***`. Tarballs are safe to share.

## What the HTML report shows

- **Migration health card**: phase, parsed duration, suggested root cause + remediation
- **Per-disk copy table**: name, started/finished timestamps, duration, CBT iterations, status
- **Event timeline**: snapshot, disk copy starts/ends, CBT sync iterations, virt-v2v, cutover, completion
- **CBT sync iterations table**: for hot migrations, one row per `Finished copying and syncing changed blocks` log line
- **Errors grouped by subsystem**: DNS, vCenter, NBD/VDDK, virt-v2v, OpenStack, Kubernetes, Auth, Network, Other
- **Pod restart count and exit reason**: extracted from `kubectl describe`
- **Network and storage mappings**: as tables instead of raw YAML
- **Kubernetes events**: parsed into a sortable table with severity colours
- **Search box** (top right): vanilla-JS filter over every log pane and every table row

The report is a single self-contained HTML file (no external CSS/JS). Open it
in any browser, attach it to a Zendesk ticket, share it however you want.

## Prerequisites

- `kubectl` configured to reach the vjailbreak cluster (appliance or bastion)
- `jq` for redaction
- `tar` (always present)
- `python3` with `python3-yaml` (`pip install pyyaml` works too) — for the parser
- Optional, only for non-local destinations:
  - `rclone` configured for Google Drive (`--dest gdrive`) or S3 (`--dest s3`)
  - `scp` for `--dest sftp`

## Install

```
mkdir -p ~/vjailbreak-tools
cp scripts/support-bundle/* ~/vjailbreak-tools/
chmod +x ~/vjailbreak-tools/collect-logs.sh ~/vjailbreak-tools/parse-logs.py
```

## Collect logs

```
# One specific migration
./collect-logs.sh --migration my-vm-migration --dest local

# All migrations on the cluster
./collect-logs.sh --all --dest local

# Interactive — prompts where to send each time
./collect-logs.sh --migration my-vm-migration --prompt

# List what migrations exist (run with no args)
./collect-logs.sh
```

Output: `/tmp/vjailbreak-bundle-<migration>-<UTC-timestamp>.tar.gz`

## Destinations

Configured at the top of `collect-logs.sh` — edit the placeholders once, then
pick a destination with `--dest`:

```
GDRIVE_REMOTE="vjailbreak-drive:support-bundles"
SFTP_HOST="support-archive.example.com"
SFTP_USER="vjailbreak"
SFTP_PATH="/srv/vjailbreak-logs"
S3_REMOTE="s3remote:platform9-support/vjailbreak"
```

- `--dest local`  — tarball stays in `/tmp`
- `--dest gdrive` — uploaded via `rclone copy`
- `--dest sftp`   — uploaded via `scp`
- `--dest s3`     — uploaded via `rclone copy`
- `--prompt`      — asks you to pick at runtime

## Generate the HTML report

```
./parse-logs.py /tmp/vjailbreak-bundle-<migration>-<timestamp>.tar.gz
```

Output: `report.html` next to the input tarball. Open in any browser.

## Limitations

- **Post-mortem only**: this is a snapshot tool, not a live dashboard. It
  complements but doesn't replace what #429 describes for end-user reporting.
- **No Prometheus metrics yet**. Could be added as a `--format prometheus`
  output mode if there's interest.
- Root-cause heuristics cover eight subsystems (DNS, vCenter, NBD/VDDK,
  virt-v2v, OpenStack, Kubernetes, Auth, Network). Patterns will need
  extension as new failure modes show up.
