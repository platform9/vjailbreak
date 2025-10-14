#!/usr/bin/env bash
# generate-mount-persistence.sh
#
# Prints:
#   1. udev rules that bind each currently mounted real block device (under /dev)
#      to a stable symlink under /dev/disk/by-mountpoint/<sanitized-mount-path>.
#   2. Equivalent /etc/fstab entries with UUID=<uuid>.
#
# Supported distros: CentOS 6/7/8, Rocky Linux, Ubuntu, SUSE, RHEL.
#
# Usage:
#   sudo bash generate-mount-persistence.sh > output.txt
#   # Review output.txt for udev rules + suggested fstab entries.
#   # Or run the command with --apply, --replace-fstab, or --force-uuid to apply the changes.

#!/usr/bin/env bash
set -euo pipefail

SKIP_FS_TYPES="^(autofs|bpf|cgroup|cgroup2|configfs|debugfs|devpts|devtmpfs|efivarfs|fusectl|fuse\.overlayfs|hugetlbfs|mqueue|nsfs|overlay|proc|pstore|ramfs|rpc_pipefs|securityfs|selinuxfs|smb3?|squashfs|sysfs|tmpfs|tracefs)$"
SKIP_DEVICES_REGEX="^/dev/(ram|loop|fd|sr|zram|nbd|md|pmem)[0-9]"

APPLY=false
REPLACE_FSTAB=false
FORCE_UUID=false

print_help() {
  cat <<EOF
Usage: $0 [OPTIONS]

Detects currently mounted devices, prints equivalent udev rules and fstab entries,
and optionally applies them to the system.

Options:
  --apply           Generate udev rules + fstab entries and apply them (append only).
  --replace-fstab   Same as --apply, but removes any existing fstab entries for the
                    same mountpoints before writing new ones (safe deduplication).
  --force-uuid      Same as --replace-fstab, but converts all device references to
                    UUID=, LABEL=, or PARTUUID= format for consistent naming.
  --help            Show this help message and exit.

Examples:
  $0               # Print udev rules + fstab suggestions (no changes made)
  $0 --apply       # Apply rules, append to /etc/fstab (no dedup)
  $0 --replace-fstab
                   # Apply rules, deduplicate existing entries in /etc/fstab
  $0 --force-uuid  # Apply rules, deduplicate, convert all entries to UUID= format

EOF
  exit 0
}

case "${1:-}" in
  --apply) APPLY=true ;;
  --replace-fstab) APPLY=true; REPLACE_FSTAB=true ;;
  --force-uuid) APPLY=true; REPLACE_FSTAB=true; FORCE_UUID=true ;;
  --help|-h) print_help ;;
esac

udev_rules=()
fstab_lines=()

get_fstab_id() {
  local DEV="$1"
  local UUID LABEL PARTUUID

  UUID="$(blkid -s UUID -o value -- "$DEV" 2>/dev/null || true)"
  LABEL="$(blkid -s LABEL -o value -- "$DEV" 2>/dev/null || true)"
  PARTUUID="$(blkid -s PARTUUID -o value -- "$DEV" 2>/dev/null || true)"

  if [[ -n "$UUID" ]]; then
    echo "UUID=$UUID"
  elif [[ -n "$LABEL" ]]; then
    echo "LABEL=$LABEL"
  elif [[ -n "$PARTUUID" ]]; then
    echo "PARTUUID=$PARTUUID"
  else
    echo "$DEV"
  fi
}

# --- 1. Handle mounted filesystems ---
while IFS=$'\t' read -r SRC TGT FST; do
  [[ "$SRC" == /dev/* ]] || continue
  [[ "$FST" =~ $SKIP_FS_TYPES ]] && continue
  [[ "$SRC" =~ $SKIP_DEVICES_REGEX ]] && continue

  FSTAB_ID=$(get_fstab_id "$SRC")

  SAFE_NAME="${TGT#/}"
  [[ "$SAFE_NAME" == "" ]] && SAFE_NAME="root"
  SAFE_NAME="${SAFE_NAME//\//_}"
  SAFE_NAME="${SAFE_NAME// /_}"

  UUID="$(blkid -s UUID -o value -- "$SRC" 2>/dev/null || true)"
  LABEL="$(blkid -s LABEL -o value -- "$SRC" 2>/dev/null || true)"
  PARTUUID="$(blkid -s PARTUUID -o value -- "$SRC" 2>/dev/null || true)"

  if [[ -n "$UUID" ]]; then
    MATCH="ENV{ID_FS_UUID}==\"$UUID\""
  elif [[ -n "$LABEL" ]]; then
    MATCH="ENV{ID_FS_LABEL}==\"$LABEL\""
  else
    MATCH="ENV{ID_PART_ENTRY_UUID}==\"$PARTUUID\""
  fi

  udev_rules+=("ACTION==\"add\", SUBSYSTEM==\"block\", $MATCH, SYMLINK+=\"disk/by-mountpoint/$SAFE_NAME\"")
  udev_rules+=("ACTION==\"change\", SUBSYSTEM==\"block\", $MATCH, SYMLINK+=\"disk/by-mountpoint/$SAFE_NAME\"")

  #OPTIONS="$(awk -v src="$SRC" '$1==src {print $4}' /proc/mounts | head -n1)"
  #OPTIONS="${OPTIONS:-defaults}"
  # Try to reuse options from existing /etc/fstab if present
OPTIONS=$(awk -v tgt="$TGT" '$2==tgt {print $4}' /etc/fstab | head -n1)

if [[ -z "$OPTIONS" ]]; then
  # Fallback: use /proc/mounts
  OPTIONS=$(awk -v src="$SRC" '$1==src {print $4}' /proc/mounts | head -n1)

  # Detect if options are equivalent to defaults
  # defaults generally == rw,suid,dev,exec,auto,nouser,async (varies per FS)
  # We'll use a conservative match: rw,relatime + no special flags
  if [[ "$OPTIONS" =~ ^rw(,relatime)?$ ]]; then
    OPTIONS="defaults"
  fi
fi


  if [[ "$FST" == xfs ]]; then
    DUMP_PASS="0 0"
  elif [[ "$FST" == ext* ]]; then
    [[ "$TGT" == "/" ]] && DUMP_PASS="1 1" || DUMP_PASS="0 2"
  else
    [[ "$TGT" == "/" ]] && DUMP_PASS="1 1" || DUMP_PASS="0 2"
  fi

  fstab_lines+=("$FSTAB_ID  $TGT  $FST  $OPTIONS  $DUMP_PASS")
done < <(
  awk '{ src=$1; tgt=$2; fstype=$3;
         gsub(/\\040/, " ", tgt);
         print src "\t" tgt "\t" fstype }' /proc/mounts
)

# --- 2. Handle swap devices ---
while read -r swapline; do
  read -r DEV TYPE _ <<<"$swapline"
  [[ "$DEV" == Filename ]] && continue  # skip header
  [[ "$DEV" =~ $SKIP_DEVICES_REGEX ]] && continue

  FSTAB_ID=$(get_fstab_id "$DEV")
  fstab_lines+=("$FSTAB_ID  none  swap  defaults  0 0")
done < /proc/swaps

# --- Apply or print ---
if $APPLY; then
  echo "Applying changes..."
  RULE_FILE="/etc/udev/rules.d/99-by-mountpoint.rules"
  cp "$RULE_FILE" "$RULE_FILE.bak.$(date +%s)" 2>/dev/null || true
  printf "%s\n" "${udev_rules[@]}" > "$RULE_FILE"
  udevadm control --reload-rules && udevadm trigger

  cp /etc/fstab /etc/fstab.bak.$(date +%s)

  if $REPLACE_FSTAB; then
    echo "Replacing existing fstab entries for detected mounts and swap..."
    tmp_fstab=$(mktemp)

    while IFS= read -r line; do
      skip=false
      for entry in "${fstab_lines[@]}"; do
        mp=$(echo "$entry" | awk '{print $2}')
        [[ "$line" =~ $mp[[:space:]] ]] && { skip=true; break; }
      done
      $skip || echo "$line" >> "$tmp_fstab"
    done < /etc/fstab

    # When forcing UUIDs, convert existing device references too
    if $FORCE_UUID; then
      echo "Forcing UUID/LABEL/PARTUUID references for all devices..."
      > "$tmp_fstab"
      for entry in "${fstab_lines[@]}"; do
        DEV=$(echo "$entry" | awk '{print $1}')
        DEV_REAL=$(echo "$DEV" | sed 's/UUID=.*//;s/LABEL=.*//;s/PARTUUID=.*//')
        NEW_ID=$(get_fstab_id "$DEV_REAL")
        echo "$entry" | sed "s|^$DEV|$NEW_ID|" >> "$tmp_fstab"
      done
    else
      printf "%s\n" "${fstab_lines[@]}" >> "$tmp_fstab"
    fi

    mv "$tmp_fstab" /etc/fstab
  else
    printf "%s\n" "${fstab_lines[@]}" >> /etc/fstab
  fi

  echo " -> Done. Backups created in /etc/udev/rules.d and /etc/fstab.bak.*"
else
  echo "# === UDEV RULES (save to /etc/udev/rules.d/99-by-mountpoint.rules) ==="
  printf "%s\n" "${udev_rules[@]}"

  echo ""
  echo "# === /etc/fstab entries (append to /etc/fstab) ==="
  printf "%s\n" "${fstab_lines[@]}"
fi
