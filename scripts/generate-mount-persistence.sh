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

set -euo pipefail

command -v blkid >/dev/null 2>&1 || {
  echo "Error: blkid not found. Please install util-linux." >&2
  exit 1
}

SKIP_FS_TYPES="^(autofs|bpf|cgroup|cgroup2|configfs|debugfs|devpts|devtmpfs|efivarfs|fusectl|fuse\.overlayfs|hugetlbfs|mqueue|nsfs|overlay|proc|pstore|ramfs|rpc_pipefs|securityfs|selinuxfs|smb3?|squashfs|sysfs|tmpfs|tracefs)$"
SKIP_DEVICES_REGEX="^/dev/(ram|loop|fd|sr|zram|nbd|md|pmem)[0-9]"

UDEV_RULES=""
FSTAB_LINES=""

awk '
  {
    src=$1; tgt=$2; fstype=$3;
    gsub(/\\040/, " ", tgt);
    gsub(/\\011/, "\t", tgt);
    gsub(/\\012/, "\n", tgt);
    print src "\t" tgt "\t" fstype;
  }
' /proc/mounts | while IFS=$'\t' read -r SRC TGT FST; do
  [[ "$SRC" == /dev/* ]] || continue
  if [[ "$FST" =~ $SKIP_FS_TYPES ]]; then
    continue
  fi
  if [[ "$SRC" =~ $SKIP_DEVICES_REGEX ]]; then
    continue
  fi

  UUID="$(blkid -s UUID -o value -- "$SRC" 2>/dev/null || true)"
  if [[ -z "$UUID" ]] && command -v lsblk >/dev/null 2>&1; then
    UUID="$(lsblk -no UUID -- "$SRC" 2>/dev/null | head -n1 || true)"
  fi
  [[ -z "$UUID" ]] && continue

  SAFE_NAME="$TGT"
  if [[ "$SAFE_NAME" == "/" ]]; then
    SAFE_NAME="root"
  else
    SAFE_NAME="${SAFE_NAME#/}"
    SAFE_NAME="${SAFE_NAME//\//_}"
    SAFE_NAME="${SAFE_NAME// /_}"
  fi

  # Udev rules
  echo "ACTION==\"add\", SUBSYSTEM==\"block\", ENV{ID_FS_UUID}==\"$UUID\", SYMLINK+=\"disk/by-mountpoint/$SAFE_NAME\""
  echo "ACTION==\"change\", SUBSYSTEM==\"block\", ENV{ID_FS_UUID}==\"$UUID\", SYMLINK+=\"disk/by-mountpoint/$SAFE_NAME\""

  # /etc/fstab entry suggestion
  # Extract mount options from /proc/mounts
  OPTIONS="$(awk -v src="$SRC" '$1==src {print $4}' /proc/mounts | head -n1)"
  OPTIONS="${OPTIONS:-defaults}"

  # Pass and dump values: root usually 1 1, others 0 2
  if [[ "$TGT" == "/" ]]; then
    DUMP_PASS="1 1"
  else
    DUMP_PASS="0 2"
  fi

  echo "FSTAB: UUID=$UUID  $TGT  $FST  $OPTIONS  $DUMP_PASS"
done | awk '
  BEGIN {
    print "# === UDEV RULES (save to /etc/udev/rules.d/99-by-mountpoint.rules) ==="
  }
  /^ACTION/ { print; next }
  /^FSTAB:/ {
    fstab_line = substr($0, 7)
    if (!printed_fstab) {
      print ""
      print "# === /etc/fstab entries (append to /etc/fstab) ==="
      printed_fstab=1
    }
    print fstab_line
  }
'