#!/bin/sh
# generate-bootable-partition.sh
#
# Detects the bootable partition and generates appropriate boot configuration.
# This script identifies the boot device, validates it, and optionally generates
# GRUB configuration and fstab entries for the boot partition.
#
# Supported distros: CentOS 6/7/8, Rocky Linux, Ubuntu, SUSE, RHEL.
#
# Usage:
#   sudo sh generate-bootable-partition.sh
#   # Review output for boot device and suggested configuration
#   # Or run with --apply to apply changes to the system

set -eu

APPLY=false
VERBOSE=false

print_help() {
  cat <<EOF
Usage: $0 [OPTIONS]

Detects the bootable partition and generates boot configuration.

Options:
  --apply           Apply the generated configuration to the system
  --verbose         Enable verbose output for debugging
  --help            Show this help message and exit

Examples:
  $0               # Print boot device and configuration (no changes made)
  $0 --apply       # Apply boot configuration to the system
  $0 --verbose     # Show detailed detection information

EOF
  exit 0
}

log_verbose() {
  if $VERBOSE; then
    echo "[DEBUG] $*" >&2
  fi
}

# Parse arguments
while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=true ;;
    --verbose) VERBOSE=true ;;
    --help|-h) print_help ;;
    *) echo "Unknown option: $1" >&2; print_help ;;
  esac
  shift
done

# Step 1: Get UUID of /boot from fstab (if present)
log_verbose "Checking /etc/fstab for /boot entry..."
boot_uuid="$(grep -w '/boot' /etc/fstab 2>/dev/null | awk '{print $1}' | sed 's/^UUID=//' || true)"

if [ -z "$boot_uuid" ]; then
  log_verbose "/boot not found in fstab, using fallback method..."
  # If /boot not listed, fallback: use root filesystem's base device
  if command -v inspect-get-roots >/dev/null 2>&1; then
    boot_dev="$(inspect-get-roots | head -n1 | sed 's/[0-9]*$//')"
  else
    # Alternative fallback: use df to find root device
    boot_dev="$(df / | tail -n1 | awk '{print $1}' | sed 's/[0-9]*$//')"
  fi
  log_verbose "Fallback boot device: $boot_dev"
else
  log_verbose "Found boot UUID in fstab: $boot_uuid"
  # Step 2: Match UUID → actual block device
  boot_dev="$(blkid | grep "$boot_uuid" | awk -F: '{sub(/[0-9]+$/, "", $1); print $1; exit}')"
  
  # Step 3: Fallback if not found
  if [ -z "$boot_dev" ]; then
    log_verbose "Could not resolve UUID to device, using fallback..."
    if command -v inspect-get-roots >/dev/null 2>&1; then
      boot_dev="$(inspect-get-roots | head -n1 | sed 's/[0-9]*$//')"
    else
      boot_dev="$(df / | tail -n1 | awk '{print $1}' | sed 's/[0-9]*$//')"
    fi
  fi
fi

# Validate boot device
if [ -z "$boot_dev" ]; then
  echo "ERROR: Could not determine boot device" >&2
  exit 1
fi

if [ ! -b "$boot_dev" ]; then
  echo "ERROR: Boot device $boot_dev is not a block device" >&2
  exit 1
fi

log_verbose "Detected boot device: $boot_dev"

# Get device information
boot_dev_uuid="$(blkid -s UUID -o value "$boot_dev" 2>/dev/null || true)"
boot_dev_label="$(blkid -s LABEL -o value "$boot_dev" 2>/dev/null || true)"
boot_dev_fstype="$(blkid -s TYPE -o value "$boot_dev" 2>/dev/null || true)"

echo "# === Boot Device Information ==="
echo "Boot Device: $boot_dev"
[ -n "$boot_dev_uuid" ] && echo "UUID: $boot_dev_uuid"
[ -n "$boot_dev_label" ] && echo "Label: $boot_dev_label"
[ -n "$boot_dev_fstype" ] && echo "Filesystem: $boot_dev_fstype"
echo ""

# Generate fstab entry for /boot if not present
if [ -z "$boot_uuid" ]; then
  echo "# === Suggested /etc/fstab entry for /boot ==="
  if [ -n "$boot_dev_uuid" ]; then
    echo "UUID=$boot_dev_uuid  /boot  ${boot_dev_fstype:-ext4}  defaults  0 2"
  else
    echo "$boot_dev  /boot  ${boot_dev_fstype:-ext4}  defaults  0 2"
  fi
  echo ""
fi

# Detect boot partition (the actual partition with boot files)
boot_partition=""
for part in "${boot_dev}"[0-9]* "${boot_dev}p"[0-9]*; do
  if [ -b "$part" ]; then
    # Check if this partition is mounted as /boot
    if mount | grep -q "^$part on /boot"; then
      boot_partition="$part"
      break
    fi
    # Check if this partition has boot files
    if [ -z "$boot_partition" ]; then
      part_mount="$(mktemp -d)"
      if mount -o ro "$part" "$part_mount" 2>/dev/null; then
        if [ -d "$part_mount/grub" ] || [ -d "$part_mount/grub2" ] || [ -f "$part_mount/vmlinuz" ]; then
          boot_partition="$part"
          umount "$part_mount"
          rmdir "$part_mount"
          break
        fi
        umount "$part_mount"
        rmdir "$part_mount"
      fi
    fi
  fi
done

if [ -n "$boot_partition" ]; then
  echo "# === Boot Partition ==="
  echo "Boot Partition: $boot_partition"
  boot_part_uuid="$(blkid -s UUID -o value "$boot_partition" 2>/dev/null || true)"
  [ -n "$boot_part_uuid" ] && echo "Partition UUID: $boot_part_uuid"
  echo ""
fi

# Generate GRUB device map
echo "# === GRUB Device Map ==="
echo "# Suggested content for /boot/grub/device.map or /boot/grub2/device.map"
echo "(hd0) $boot_dev"
echo ""

# Apply changes if requested
if $APPLY; then
  echo "# === Applying Configuration ==="
  
  # Backup fstab
  cp /etc/fstab /etc/fstab.bak.$(date +%s)
  echo "Backed up /etc/fstab"
  
  # Add /boot entry to fstab if missing
  if [ -z "$boot_uuid" ] && [ -n "$boot_dev_uuid" ]; then
    echo "UUID=$boot_dev_uuid  /boot  ${boot_dev_fstype:-ext4}  defaults  0 2" >> /etc/fstab
    echo "Added /boot entry to /etc/fstab"
  fi
  
  # Update GRUB device map if GRUB is installed
  if [ -d /boot/grub2 ]; then
    grub_dir="/boot/grub2"
  elif [ -d /boot/grub ]; then
    grub_dir="/boot/grub"
  else
    grub_dir=""
  fi
  
  if [ -n "$grub_dir" ]; then
    [ -f "$grub_dir/device.map" ] && cp "$grub_dir/device.map" "$grub_dir/device.map.bak.$(date +%s)"
    echo "(hd0) $boot_dev" > "$grub_dir/device.map"
    echo "Updated $grub_dir/device.map"
    
    # Regenerate GRUB configuration
    if command -v grub2-mkconfig >/dev/null 2>&1; then
      grub2-mkconfig -o "$grub_dir/grub.cfg"
      echo "Regenerated GRUB2 configuration"
    elif command -v grub-mkconfig >/dev/null 2>&1; then
      grub-mkconfig -o "$grub_dir/grub.cfg"
      echo "Regenerated GRUB configuration"
    fi
  fi
  
  echo ""
  echo "Configuration applied successfully!"
  echo "Backups created with timestamp suffix."
else
  echo "# Run with --apply to apply these changes to the system"
fi
