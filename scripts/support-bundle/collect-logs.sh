#!/usr/bin/env bash
# collect-logs.sh — gather vjailbreak migration logs into a single tarball
# Runs wherever kubectl is configured (appliance OR bastion).

set -euo pipefail

# ============================================================
# CONFIG — edit these placeholders once, then forget them
# ============================================================
GDRIVE_REMOTE="vjailbreak-drive:support-bundles"   # rclone remote:path
SFTP_HOST="support-archive.example.com"
SFTP_USER="vjailbreak"
SFTP_PATH="/srv/vjailbreak-logs"
S3_BUCKET="s3://platform9-support/vjailbreak"      # used with rclone too: s3remote:bucket/path
S3_REMOTE="s3remote:platform9-support/vjailbreak"  # rclone-style for S3

# Namespaces vjailbreak uses
NS_CONTROLLER="vjailbreak"
NS_MIGRATION="migration-system"

# ============================================================
# USAGE
# ============================================================
usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --migration NAME    Collect logs for a single Migration CRD by name.
  --all               Collect logs for ALL migrations currently in the cluster.
  --dest TYPE         Where to send the tarball: gdrive | sftp | s3 | local
                      (local = save to /tmp and stop)
  --prompt            Interactive — ask for destination at runtime.
  --output-dir DIR    Override staging dir (default: /tmp).
  -h, --help          This message.

Examples:
  $0 --migration my-vm-migration --dest gdrive
  $0 --all --prompt
  $0 --migration foo --dest local            # just save to /tmp
EOF
  exit 0
}

# ============================================================
# ARGS
# ============================================================
MIGRATION=""
ALL=false
DEST=""
PROMPT=false
OUTPUT_DIR="/tmp"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --migration)   MIGRATION="$2"; shift 2 ;;
    --all)         ALL=true; shift ;;
    --dest)        DEST="$2"; shift 2 ;;
    --prompt)      PROMPT=true; shift ;;
    --output-dir)  OUTPUT_DIR="$2"; shift 2 ;;
    -h|--help)     usage ;;
    *) echo "Unknown arg: $1"; usage ;;
  esac
done

# ============================================================
# SANITY CHECKS
# ============================================================
need() { command -v "$1" >/dev/null 2>&1 || { echo "❌ Missing required tool: $1"; exit 1; }; }
need kubectl
need jq
need tar

if [[ -z "$MIGRATION" && "$ALL" == "false" ]]; then
  echo "Pick one: --migration NAME  or  --all"
  echo
  echo "Available migrations:"
  kubectl get migration -n "$NS_MIGRATION" --no-headers 2>/dev/null | awk '{print "  - " $1}' || echo "  (none found)"
  exit 1
fi

if ! kubectl cluster-info >/dev/null 2>&1; then
  echo "❌ kubectl can't reach the cluster. Fix your kubeconfig and try again."
  exit 1
fi

# ============================================================
# STAGING
# ============================================================
TS=$(date -u +%Y%m%d-%H%M%S)
LABEL="${MIGRATION:-all}"
STAGE="$OUTPUT_DIR/vjailbreak-bundle-${LABEL}-${TS}"
TARBALL="${STAGE}.tar.gz"
mkdir -p "$STAGE"
trap 'rm -rf "$STAGE"' EXIT

log() { printf "  • %s\n" "$*"; }
section() { printf "\n▶ %s\n" "$*"; }

# ============================================================
# COLLECT
# ============================================================
section "Cluster overview"
{
  echo "=== kubectl version ==="; kubectl version 2>&1 || true
  echo; echo "=== nodes ==="; kubectl get nodes -o wide 2>&1 || true
  echo; echo "=== namespaces ==="; kubectl get ns 2>&1 || true
} > "$STAGE/00-cluster-overview.txt"
log "cluster overview"

section "Controller pod logs (last 5000 lines)"
mkdir -p "$STAGE/controller"
kubectl -n "$NS_CONTROLLER" get pods -o wide > "$STAGE/controller/pods.txt" 2>&1 || true
for pod in $(kubectl -n "$NS_CONTROLLER" get pods -l control-plane=controller-manager -o name 2>/dev/null); do
  name=$(basename "$pod")
  kubectl -n "$NS_CONTROLLER" logs "$pod" --tail=5000 --all-containers > "$STAGE/controller/${name}.log" 2>&1 || true
  kubectl -n "$NS_CONTROLLER" logs "$pod" --tail=5000 --all-containers --previous > "$STAGE/controller/${name}.previous.log" 2>&1 || true
  log "controller pod: $name"
done

section "Migrations (CRDs + v2v-helper pod logs)"
mkdir -p "$STAGE/migrations"

if [[ "$ALL" == "true" ]]; then
  MIGRATIONS=$(kubectl -n "$NS_MIGRATION" get migration -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
else
  MIGRATIONS="$MIGRATION"
fi

if [[ -z "$MIGRATIONS" ]]; then
  log "(no migrations found)"
else
  for m in $MIGRATIONS; do
    log "migration: $m"
    mdir="$STAGE/migrations/$m"
    mkdir -p "$mdir"
    kubectl -n "$NS_MIGRATION" get migration "$m" -o yaml > "$mdir/migration.yaml" 2>&1 || true

    # v2v-helper pod — usually named <migration>-v2v-helper or has migration label
    for pod in $(kubectl -n "$NS_MIGRATION" get pods -o name 2>/dev/null | grep -E "($m|^pod/v2v-helper-)" || true); do
      pname=$(basename "$pod")
      kubectl -n "$NS_MIGRATION" describe "$pod" > "$mdir/${pname}.describe.txt" 2>&1 || true
      kubectl -n "$NS_MIGRATION" logs "$pod" --tail=20000 --all-containers > "$mdir/${pname}.log" 2>&1 || true
      kubectl -n "$NS_MIGRATION" logs "$pod" --tail=20000 --all-containers --previous > "$mdir/${pname}.previous.log" 2>&1 || true
    done
  done
fi

section "Related CRDs (migrationplans, mappings, templates, nodes)"
mkdir -p "$STAGE/crds"
for kind in migrationplan migrationtemplate networkmapping storagemapping vjailbreaknode vmwaremachine vmwarecluster vmwarehost rdmdisk; do
  kubectl -n "$NS_MIGRATION" get "$kind" -o yaml > "$STAGE/crds/${kind}.yaml" 2>&1 || true
  log "$kind"
done

section "Credentials (REDACTED)"
mkdir -p "$STAGE/creds-redacted"
# vmwarecreds + openstackcreds + arraycreds: nullify any password / secret / token fields
for kind in vmwarecreds openstackcreds arraycreds esxisshcreds; do
  kubectl -n "$NS_MIGRATION" get "$kind" -o json 2>/dev/null \
    | jq 'walk(if type == "object" then with_entries(if (.key | test("password|secret|token|key|credential"; "i")) then .value = "***REDACTED***" else . end) else . end)' \
    > "$STAGE/creds-redacted/${kind}.json" 2>/dev/null || true
  log "$kind (redacted)"
done

section "Events"
mkdir -p "$STAGE/events"
kubectl -n "$NS_MIGRATION" get events --sort-by=.lastTimestamp > "$STAGE/events/migration-system.txt" 2>&1 || true
kubectl -n "$NS_CONTROLLER" get events --sort-by=.lastTimestamp > "$STAGE/events/vjailbreak.txt" 2>&1 || true
log "events captured"

section "vjailbreak-settings ConfigMap"
kubectl -n "$NS_CONTROLLER" get configmap vjailbreak-settings -o yaml > "$STAGE/vjailbreak-settings.yaml" 2>&1 || true
log "settings"

# ============================================================
# TAR
# ============================================================
section "Packaging tarball"
tar -czf "$TARBALL" -C "$OUTPUT_DIR" "$(basename "$STAGE")"
SIZE=$(du -h "$TARBALL" | cut -f1)
echo "  ✓ Created: $TARBALL ($SIZE)"

# ============================================================
# DESTINATION
# ============================================================
if [[ "$PROMPT" == "true" ]]; then
  echo
  echo "Where do you want to send this bundle?"
  echo "  1) Google Drive  (rclone: $GDRIVE_REMOTE)"
  echo "  2) SFTP          ($SFTP_USER@$SFTP_HOST:$SFTP_PATH)"
  echo "  3) S3            ($S3_REMOTE)"
  echo "  4) Local only    (file stays at $TARBALL)"
  read -rp "Choice [1-4]: " choice
  case "$choice" in
    1) DEST="gdrive" ;;
    2) DEST="sftp" ;;
    3) DEST="s3" ;;
    4) DEST="local" ;;
    *) echo "Invalid choice"; exit 1 ;;
  esac
fi

DEST="${DEST:-local}"

case "$DEST" in
  gdrive)
    need rclone
    section "Uploading to Google Drive: $GDRIVE_REMOTE"
    rclone copy "$TARBALL" "$GDRIVE_REMOTE/" --progress
    echo "  ✓ Uploaded to $GDRIVE_REMOTE/$(basename "$TARBALL")"
    ;;
  sftp)
    need scp
    section "Uploading via SFTP: $SFTP_USER@$SFTP_HOST:$SFTP_PATH"
    scp "$TARBALL" "${SFTP_USER}@${SFTP_HOST}:${SFTP_PATH}/"
    echo "  ✓ Uploaded to ${SFTP_USER}@${SFTP_HOST}:${SFTP_PATH}/$(basename "$TARBALL")"
    ;;
  s3)
    need rclone
    section "Uploading to S3: $S3_REMOTE"
    rclone copy "$TARBALL" "$S3_REMOTE/" --progress
    echo "  ✓ Uploaded to $S3_REMOTE/$(basename "$TARBALL")"
    ;;
  local)
    section "Saved locally"
    # Disable the trap so we don't delete the source dir before printing
    trap - EXIT
    echo "  ✓ Bundle: $TARBALL"
    echo "  (staging dir preserved at $STAGE)"
    exit 0
    ;;
  *)
    echo "Unknown --dest: $DEST"
    exit 1
    ;;
esac

echo
echo "✅ Done."
