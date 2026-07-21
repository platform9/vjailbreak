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
OS_FAMILY=""

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
                    UUID-based root device references, including device.map.
                    Creates backups of all modified files.
  --os-family=NAME  Hints the guest OS family (e.g. "suse"). When set to "suse" and
                    --force-uuid was not also given, GRUB Legacy is detected on the
                    guest and, if found, only the root=/resume= kernel cmdline
                    references are rewritten to UUID= form. device.map is never
                    touched by this path, so it cannot reintroduce the GRUB stage1
                    "Error 21" regression that --force-uuid's device.map rewrite can
                    cause when run before virt-v2v on SUSE GRUB Legacy guests.
  --help            Show this help message and exit.

Examples:
  $0               # Print udev rules + fstab suggestions (no changes made)
  $0 --apply       # Apply rules, append to /etc/fstab (no dedup)
  $0 --replace-fstab
                   # Apply rules, deduplicate existing entries in /etc/fstab
  $0 --force-uuid  # Apply rules, deduplicate, convert all entries to UUID= format
  $0 --replace-fstab --os-family=suse
                   # Apply + dedup, and safely fix a SUSE GRUB Legacy cmdline if detected

EOF
  exit 0
}

for arg in "$@"; do
  case "$arg" in
    --apply) APPLY=true ;;
    --replace-fstab) APPLY=true; REPLACE_FSTAB=true ;;
    --force-uuid) APPLY=true; REPLACE_FSTAB=true; FORCE_UUID=true ;;
    --os-family=*) OS_FAMILY="${arg#--os-family=}" ;;
    --help|-h) print_help ;;
  esac
done

UDEV_RULES_FILE=$(mktemp)
FSTAB_LINES_FILE=$(mktemp)
FSTAB_ACTIVE=""   # set after FSTAB_PATH is known; stripped of comments/blanks
trap 'rm -f "$UDEV_RULES_FILE" "$FSTAB_LINES_FILE" ${FSTAB_ACTIVE:+"$FSTAB_ACTIVE"}' EXIT

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

# detect_bootloader <root_prefix>: echo legacy|grub2|unknown, preferring SUSE's LOADER_TYPE=,
# then a grub.cfg / /boot/grub2 dir (grub2), then menu.lst / grub.conf (GRUB Legacy).
detect_bootloader() {
  _prefix="$1"

  if [ -f "${_prefix}/etc/sysconfig/bootloader" ]; then
    _loader_type=$(grep -E '^LOADER_TYPE=' "${_prefix}/etc/sysconfig/bootloader" 2>/dev/null | cut -d= -f2 | tr -d '"')
    case "$_loader_type" in
      grub) echo "legacy"; return ;;
      grub2*) echo "grub2"; return ;;
    esac
  fi

  if [ -d "${_prefix}/boot/grub2" ] || [ -f "${_prefix}/boot/grub2/grub.cfg" ] || [ -f "${_prefix}/boot/grub/grub.cfg" ]; then
    echo "grub2"
    return
  fi

  if [ -f "${_prefix}/boot/grub/menu.lst" ] || [ -f "${_prefix}/boot/grub/grub.conf" ]; then
    echo "legacy"
    return
  fi

  echo "unknown"
}

# fix_grub_cmdline <root_prefix>: rewrite root=/resume= device refs to UUID= in menu.lst/grub.conf,
# /etc/default/grub, and sysconfig/bootloader. Never touches device.map, so it's safe pre-virt-v2v.
fix_grub_cmdline() {
  _prefix="$1"
  echo "Fixing GRUB kernel cmdline (root=/resume=) references..."

  _device_uuid_map=$(mktemp)
  for _dev in /dev/vd[a-z]* /dev/sd[a-z]* /dev/hd[a-z]* /dev/xvd[a-z]*; do
    [ -b "$_dev" ] || continue
    _uuid=$(blkid -s UUID -o value "$_dev" 2>/dev/null || true)
    [ -n "$_uuid" ] && echo "$_dev UUID=$_uuid" >> "$_device_uuid_map"
  done

  # Fix GRUB Legacy (menu.lst or grub.conf)
  for _grub_cfg in "${_prefix}/boot/grub/menu.lst" "${_prefix}/boot/grub/grub.conf"; do
    if [ -f "$_grub_cfg" ]; then
      echo " -> Fixing GRUB Legacy config: $_grub_cfg"
      cp "$_grub_cfg" "$_grub_cfg.bak.$(date +%s)"

      while read -r _dev _uuid; do
        sed -i "s|root=${_dev}|root=${_uuid}|g" "$_grub_cfg"
        sed -i "s|resume=${_dev}|resume=${_uuid}|g" "$_grub_cfg"
      done < "$_device_uuid_map"
    fi
  done

  # Fix GRUB2 /etc/default/grub
  if [ -f "${_prefix}/etc/default/grub" ]; then
    echo " -> Fixing /etc/default/grub"
    cp "${_prefix}/etc/default/grub" "${_prefix}/etc/default/grub.bak.$(date +%s)"

    while read -r _dev _uuid; do
      sed -i "s|root=${_dev}|root=${_uuid}|g" "${_prefix}/etc/default/grub"
      sed -i "s|resume=${_dev}|resume=${_uuid}|g" "${_prefix}/etc/default/grub"
    done < "$_device_uuid_map"
  fi

  # Fix SUSE YaST2 bootloader config
  if [ -f "${_prefix}/etc/sysconfig/bootloader" ]; then
    echo " -> Fixing SUSE /etc/sysconfig/bootloader"
    cp "${_prefix}/etc/sysconfig/bootloader" "${_prefix}/etc/sysconfig/bootloader.bak.$(date +%s)"

    while read -r _dev _uuid; do
      sed -i "s|root=${_dev}|root=${_uuid}|g" "${_prefix}/etc/sysconfig/bootloader"
      sed -i "s|resume=${_dev}|resume=${_uuid}|g" "${_prefix}/etc/sysconfig/bootloader"
    done < "$_device_uuid_map"
  fi

  rm -f "$_device_uuid_map"
  echo " -> GRUB cmdline fixed. Backups created with .bak.* extension. device.map untouched."
}

# fix_grub_device_map <root_prefix>: rewrite device.map /dev/sdX -> /dev/vdX. Unsafe pre-virt-v2v on
# SUSE GRUB Legacy (stage1 reinstalled with wrong drives -> Error 21); only call via --force-uuid.
fix_grub_device_map() {
  _prefix="$1"
  if [ -f "${_prefix}/boot/grub/device.map" ]; then
    echo " -> Fixing device.map: ${_prefix}/boot/grub/device.map"
    cp "${_prefix}/boot/grub/device.map" "${_prefix}/boot/grub/device.map.bak.$(date +%s)"
    sed -i 's|/dev/sd\([a-z]\)|/dev/vd\1|g' "${_prefix}/boot/grub/device.map"
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

  fix_grub_cmdline "$ROOT_PREFIX"
  fix_grub_device_map "$ROOT_PREFIX"

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

  echo " -> Grub configuration fixed. Backups created with .bak.* extension"
}

# --- 1. Handle mounted filesystems ---
# Detect if running in guestfish appliance (mountpoints under /sysroot)
if mount | grep -q '/sysroot'; then
  IN_GUESTFISH=true
  FSTAB_PATH="/sysroot/etc/fstab"
else
  IN_GUESTFISH=false
  FSTAB_PATH="/etc/fstab"
fi

# Prefilter: strip comment and blank lines so lookups and deduplication never
# treat commented-out entries as live mounts.  The original file (with comments)
# is still used as the source of truth when writing to tmp_fstab.
FSTAB_ACTIVE=$(mktemp)
grep -v '^[[:space:]]*#' "$FSTAB_PATH" 2>/dev/null | grep -v '^[[:space:]]*$' > "$FSTAB_ACTIVE" || true

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

  # Strip /sysroot prefix if running in guestfish
  if [ "$IN_GUESTFISH" = "true" ]; then
    TGT_FSTAB="${TGT#/sysroot}"
    [ -z "$TGT_FSTAB" ] && TGT_FSTAB="/"
  else
    TGT_FSTAB="$TGT"
  fi

  SAFE_NAME="${TGT_FSTAB#/}"
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

  # Try to reuse options from existing fstab if present.
  # Use FSTAB_ACTIVE (comments stripped) so commented-out entries never
  # bleed their options (e.g. "bind") into newly generated live entries.
  OPTIONS=$(awk -v tgt="$TGT_FSTAB" '$2==tgt {print $4}' "$FSTAB_ACTIVE" 2>/dev/null | head -n1)

  if [ -z "$OPTIONS" ]; then
    # Fallback: use /proc/mounts
    OPTIONS=$(awk -v src="$SRC" '$1==src {print $4}' /proc/mounts | head -n1)

    # Detect if options are equivalent to defaults
    # defaults generally == rw,suid,dev,exec,auto,nouser,async (varies per FS)
    # We'll use a conservative match: rw,relatime + no special flags
  fi
  case "$OPTIONS" in
    rw|rw,relatime) OPTIONS="defaults" ;;
    on) OPTIONS="defaults" ;;
  esac
  case "$FST" in
    xfs)
      DUMP_PASS="0 0"
      ;;
    ext*)
      if [ "$TGT_FSTAB" = "/" ]; then
        DUMP_PASS="1 1"
      else
        DUMP_PASS="0 2"
      fi
      ;;
    *)
      if [ "$TGT_FSTAB" = "/" ]; then
        DUMP_PASS="1 1"
      else
        DUMP_PASS="0 2"
      fi
      ;;
  esac

  echo "$FSTAB_ID  $TGT_FSTAB  $FST  $OPTIONS  $DUMP_PASS" >> "$FSTAB_LINES_FILE"
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

  cp "$FSTAB_PATH" "$FSTAB_PATH.bak.$(date +%s)"

  if $REPLACE_FSTAB; then
    echo "Replacing existing fstab entries for detected mounts and swap..."
    tmp_fstab=$(mktemp)

    while IFS= read -r line; do
      skip=false
      # Never remove comment or blank lines — they are not live mount entries.
      # Skipping them was the root cause of issue #2007 (commented bind-mount
      # entries being rewritten as active UUID entries).
      case "$line" in
        ''|'#'*) ;;
        *' #'*) ;;   # inline comment on an otherwise active line — treat as active
        *)
          while IFS= read -r entry; do
            mp=$(echo "$entry" | awk '{print $2}')
            # Use exact field match instead of substring grep to avoid /boot matching /sysroot/boot
            line_mp=$(echo "$line" | awk '{print $2}')
            [ "$line_mp" = "$mp" ] && { skip=true; break; }
          done < "$FSTAB_LINES_FILE"
          ;;
      esac
      $skip || echo "$line" >> "$tmp_fstab"
    done < "$FSTAB_PATH"

    # Append new entries (already in UUID format from get_fstab_id)
    cat "$FSTAB_LINES_FILE" >> "$tmp_fstab"

    mv "$tmp_fstab" "$FSTAB_PATH"
  else
    cat "$FSTAB_LINES_FILE" >> "$FSTAB_PATH"
  fi

  echo " -> Done. Backups created in /etc/udev/rules.d and /etc/fstab.bak.*"
  
  # --force-uuid runs the full grub fix; --os-family=suse only fixes a detected
  # GRUB Legacy cmdline, never device.map (see print_help for why).
  if $FORCE_UUID; then
    fix_grub_config
  elif [ "$OS_FAMILY" = "suse" ]; then
    if mount | grep -q '/sysroot'; then
      _suse_root_prefix="/sysroot"
    else
      _suse_root_prefix=""
    fi
    _suse_bootloader=$(detect_bootloader "$_suse_root_prefix")
    if [ "$_suse_bootloader" = "legacy" ]; then
      echo "SUSE guest with GRUB Legacy detected -> applying safe root=/resume= UUID fix"
      fix_grub_cmdline "$_suse_root_prefix"
    else
      echo "SUSE guest, bootloader=$_suse_bootloader -> no GRUB Legacy cmdline fix needed"
    fi
  fi
else
  echo "# === UDEV RULES (save to /etc/udev/rules.d/99-by-mountpoint.rules) ==="
  cat "$UDEV_RULES_FILE"

  echo ""
  echo "# === /etc/fstab entries (append to /etc/fstab) ==="
  cat "$FSTAB_LINES_FILE"
fi