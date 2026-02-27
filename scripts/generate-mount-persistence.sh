#!/bin/sh
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
#   sudo sh generate-mount-persistence.sh > output.txt
#   # Review output.txt for udev rules + suggested fstab entries.
#   # Or run the command with --apply, --replace-fstab, or --force-uuid to apply the changes.

set -eu

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
                    Also fixes grub configuration (GRUB Legacy, GRUB2, YaST2) to use
                    UUID-based root device references. Creates backups of all modified files.
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

UDEV_RULES_FILE=$(mktemp)
FSTAB_LINES_FILE=$(mktemp)
trap 'rm -f "$UDEV_RULES_FILE" "$FSTAB_LINES_FILE"' EXIT

get_fstab_id() {
  DEV="$1"

  UUID="$(blkid -s UUID -o value -- "$DEV" 2>/dev/null || true)"
  LABEL="$(blkid -s LABEL -o value -- "$DEV" 2>/dev/null || true)"
  PARTUUID="$(blkid -s PARTUUID -o value -- "$DEV" 2>/dev/null || true)"

  if [ -n "$UUID" ]; then
    echo "UUID=$UUID"
  elif [ -n "$LABEL" ]; then
    echo "LABEL=$LABEL"
  elif [ -n "$PARTUUID" ]; then
    echo "PARTUUID=$PARTUUID"
  else
    echo "$DEV"
  fi
}

fix_grub_config() {
  echo "Fixing grub configuration..."
  
  # Detect if running in guestfish appliance (mountpoints under /sysroot)
  if mount | grep -q '/sysroot'; then
    ROOT_PREFIX="/sysroot"
  else
    ROOT_PREFIX=""
  fi
  
  # Build device-to-UUID mapping for all block devices
  DEVICE_UUID_MAP=$(mktemp)
  for dev in /dev/vd[a-z]* /dev/sd[a-z]* /dev/hd[a-z]* /dev/xvd[a-z]*; do
    [ -b "$dev" ] || continue
    UUID=$(blkid -s UUID -o value "$dev" 2>/dev/null || true)
    [ -n "$UUID" ] && echo "$dev UUID=$UUID" >> "$DEVICE_UUID_MAP"
  done
  
  # Fix GRUB Legacy (menu.lst or grub.conf)
  for grub_cfg in "${ROOT_PREFIX}/boot/grub/menu.lst" "${ROOT_PREFIX}/boot/grub/grub.conf"; do
    if [ -f "$grub_cfg" ]; then
      echo " -> Fixing GRUB Legacy config: $grub_cfg"
      cp "$grub_cfg" "$grub_cfg.bak.$(date +%s)"
      
      # Replace root=/dev/sdXN and root=/dev/vdXN with UUID
      while read -r dev uuid; do
        sed -i "s|root=${dev}|root=${uuid}|g" "$grub_cfg"
        sed -i "s|resume=${dev}|resume=${uuid}|g" "$grub_cfg"
      done < "$DEVICE_UUID_MAP"
      
      # Update device.map if it exists
      if [ -f "${ROOT_PREFIX}/boot/grub/device.map" ]; then
        cp "${ROOT_PREFIX}/boot/grub/device.map" "${ROOT_PREFIX}/boot/grub/device.map.bak.$(date +%s)"
        sed -i 's|/dev/sda|/dev/vda|g; s|/dev/sdb|/dev/vdb|g; s|/dev/sdc|/dev/vdc|g' "${ROOT_PREFIX}/boot/grub/device.map"
      fi
    fi
  done
  
  # Fix GRUB2 /etc/default/grub
  if [ -f "${ROOT_PREFIX}/etc/default/grub" ]; then
    echo " -> Fixing /etc/default/grub"
    cp "${ROOT_PREFIX}/etc/default/grub" "${ROOT_PREFIX}/etc/default/grub.bak.$(date +%s)"
    
    while read -r dev uuid; do
      sed -i "s|root=${dev}|root=${uuid}|g" "${ROOT_PREFIX}/etc/default/grub"
      sed -i "s|resume=${dev}|resume=${uuid}|g" "${ROOT_PREFIX}/etc/default/grub"
    done < "$DEVICE_UUID_MAP"
  fi
  
  # Fix SUSE YaST2 bootloader config
  if [ -f "${ROOT_PREFIX}/etc/sysconfig/bootloader" ]; then
    echo " -> Fixing SUSE /etc/sysconfig/bootloader"
    cp "${ROOT_PREFIX}/etc/sysconfig/bootloader" "${ROOT_PREFIX}/etc/sysconfig/bootloader.bak.$(date +%s)"
    
    while read -r dev uuid; do
      sed -i "s|root=${dev}|root=${uuid}|g" "${ROOT_PREFIX}/etc/sysconfig/bootloader"
      sed -i "s|resume=${dev}|resume=${uuid}|g" "${ROOT_PREFIX}/etc/sysconfig/bootloader"
    done < "$DEVICE_UUID_MAP"
  fi
  
  # Regenerate GRUB2 config if not in guestfish
  if [ -z "$ROOT_PREFIX" ]; then
    if command -v grub2-mkconfig >/dev/null 2>&1; then
      echo " -> Regenerating GRUB2 config..."
      
      # Detect UEFI vs BIOS
      if [ -d /sys/firmware/efi ]; then
        # UEFI mode - find the EFI grub config
        for efi_cfg in /boot/efi/EFI/*/grub.cfg; do
          [ -f "$efi_cfg" ] && grub2-mkconfig -o "$efi_cfg" 2>/dev/null || true
        done
      else
        # BIOS mode
        if [ -f /boot/grub2/grub.cfg ]; then
          grub2-mkconfig -o /boot/grub2/grub.cfg 2>/dev/null || true
        elif [ -f /boot/grub/grub.cfg ]; then
          grub2-mkconfig -o /boot/grub/grub.cfg 2>/dev/null || true
        fi
      fi
    elif command -v update-grub >/dev/null 2>&1; then
      echo " -> Regenerating GRUB config with update-grub..."
      update-grub 2>/dev/null || true
    fi
    
    # Regenerate SUSE bootloader if YaST2 tools available
    if [ -f /etc/sysconfig/bootloader ] && command -v pbl >/dev/null 2>&1; then
      echo " -> Regenerating SUSE bootloader with pbl..."
      pbl --install 2>/dev/null || true
    fi
  fi
  
  rm -f "$DEVICE_UUID_MAP"
  echo " -> Grub configuration fixed. Backups created with .bak.* extension"
}

# --- 1. Handle mounted filesystems ---
awk '{ src=$1; tgt=$2; fstype=$3;
       gsub(/\\040/, " ", tgt);
       print src "\t" tgt "\t" fstype }' /proc/mounts | \
while IFS="$(printf '\t')" read -r SRC TGT FST; do
  case "$SRC" in
    /dev/*) ;;
    *) continue ;;
  esac
  
  echo "$FST" | grep -Eq "$SKIP_FS_TYPES" && continue
  echo "$SRC" | grep -Eq "$SKIP_DEVICES_REGEX" && continue

  FSTAB_ID=$(get_fstab_id "$SRC")

  SAFE_NAME="${TGT#/}"
  [ -z "$SAFE_NAME" ] && SAFE_NAME="root"
  SAFE_NAME=$(echo "$SAFE_NAME" | sed 's/\//_/g; s/ /_/g')

  UUID="$(blkid -s UUID -o value -- "$SRC" 2>/dev/null || true)"
  LABEL="$(blkid -s LABEL -o value -- "$SRC" 2>/dev/null || true)"
  PARTUUID="$(blkid -s PARTUUID -o value -- "$SRC" 2>/dev/null || true)"

  if [ -n "$UUID" ]; then
    MATCH="ENV{ID_FS_UUID}==\"$UUID\""
  elif [ -n "$LABEL" ]; then
    MATCH="ENV{ID_FS_LABEL}==\"$LABEL\""
  else
    MATCH="ENV{ID_PART_ENTRY_UUID}==\"$PARTUUID\""
  fi

  echo "ACTION==\"add\", SUBSYSTEM==\"block\", $MATCH, SYMLINK+=\"disk/by-mountpoint/$SAFE_NAME\"" >> "$UDEV_RULES_FILE"
  echo "ACTION==\"change\", SUBSYSTEM==\"block\", $MATCH, SYMLINK+=\"disk/by-mountpoint/$SAFE_NAME\"" >> "$UDEV_RULES_FILE"

  # Try to reuse options from existing /etc/fstab if present
  OPTIONS=$(awk -v tgt="$TGT" '$2==tgt {print $4}' /etc/fstab | head -n1)

  if [ -z "$OPTIONS" ]; then
    # Fallback: use /proc/mounts
    OPTIONS=$(awk -v src="$SRC" '$1==src {print $4}' /proc/mounts | head -n1)

    # Detect if options are equivalent to defaults
    # defaults generally == rw,suid,dev,exec,auto,nouser,async (varies per FS)
    # We'll use a conservative match: rw,relatime + no special flags
    case "$OPTIONS" in
      rw|rw,relatime) OPTIONS="defaults" ;;
    esac
  fi

  case "$FST" in
    xfs)
      DUMP_PASS="0 0"
      ;;
    ext*)
      if [ "$TGT" = "/" ]; then
        DUMP_PASS="1 1"
      else
        DUMP_PASS="0 2"
      fi
      ;;
    *)
      if [ "$TGT" = "/" ]; then
        DUMP_PASS="1 1"
      else
        DUMP_PASS="0 2"
      fi
      ;;
  esac

  echo "$FSTAB_ID  $TGT  $FST  $OPTIONS  $DUMP_PASS" >> "$FSTAB_LINES_FILE"
done

# --- 2. Handle swap devices ---
while read -r swapline; do
  DEV=$(echo "$swapline" | awk '{print $1}')
  [ "$DEV" = "Filename" ] && continue  # skip header
  echo "$DEV" | grep -Eq "$SKIP_DEVICES_REGEX" && continue

  FSTAB_ID=$(get_fstab_id "$DEV")
  echo "$FSTAB_ID  none  swap  defaults  0 0" >> "$FSTAB_LINES_FILE"
done < /proc/swaps

# --- Apply or print ---
if $APPLY; then
  echo "Applying changes..."
  RULE_FILE="/etc/udev/rules.d/99-by-mountpoint.rules"
  cp "$RULE_FILE" "$RULE_FILE.bak.$(date +%s)" 2>/dev/null || true
  cat "$UDEV_RULES_FILE" > "$RULE_FILE"
  udevadm control --reload-rules && udevadm trigger

  cp /etc/fstab /etc/fstab.bak.$(date +%s)

  if $REPLACE_FSTAB; then
    echo "Replacing existing fstab entries for detected mounts and swap..."
    tmp_fstab=$(mktemp)

    while IFS= read -r line; do
      skip=false
      while IFS= read -r entry; do
        mp=$(echo "$entry" | awk '{print $2}')
        echo "$line" | grep -q "$mp[[:space:]]" && { skip=true; break; }
      done < "$FSTAB_LINES_FILE"
      $skip || echo "$line" >> "$tmp_fstab"
    done < /etc/fstab

    # Append new entries (already in UUID format from get_fstab_id)
    cat "$FSTAB_LINES_FILE" >> "$tmp_fstab"

    mv "$tmp_fstab" /etc/fstab
  else
    cat "$FSTAB_LINES_FILE" >> /etc/fstab
  fi

  echo " -> Done. Backups created in /etc/udev/rules.d and /etc/fstab.bak.*"
  
  # Fix grub configuration if --force-uuid was specified
  if $FORCE_UUID; then
    fix_grub_config
  fi
else
  echo "# === UDEV RULES (save to /etc/udev/rules.d/99-by-mountpoint.rules) ==="
  cat "$UDEV_RULES_FILE"

  echo ""
  echo "# === /etc/fstab entries (append to /etc/fstab) ==="
  cat "$FSTAB_LINES_FILE"
fi
