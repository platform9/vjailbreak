#!/bin/sh
# detect-bootdisk.sh
# Prints the correct BIOS boot disk (the one containing GRUB MBR)
# Works with LVM and /boot on a separate disk.
# Assumes we are running inside a guestfish-mounted environment

# Step 1: Identify all disks in guest
# List block devices (assuming /dev/vda, /dev/sda, etc. are available)
disks=$(ls /dev/[sv]d[a-z] 2>/dev/null)
echo "[DEBUG] Step 1: disks found: $(echo $disks | tr '\n' ' ')" >&2

bootdisk=""

# Step 2: Try to find which disk's MBR contains GRUB signature, or has a bios_grub partition
#   - MBR-partitioned disks: GRUB stage1 boot code is embedded in the MBR
#   - GPT-partitioned disks: GRUB uses a dedicated bios_grub partition (no GRUB in MBR)
for disk in $disks; do
  # Check MBR for GRUB signature (works for MBR-partitioned disks)
  if dd if="$disk" bs=512 count=1 2>/dev/null | grep -aq 'GRUB'; then
    echo "[DEBUG] Step 2: GRUB signature found in MBR of $disk" >&2
    bootdisk="$disk"
    break
  fi
  # Check for GPT BIOS boot partition (grub2 with GPT uses bios_grub partition)
  if command -v parted >/dev/null 2>&1; then
    if parted -s "$disk" print 2>/dev/null | grep -q 'bios_grub'; then
      echo "[DEBUG] Step 2: bios_grub partition found on $disk" >&2
      bootdisk="$disk"
      break
    fi
  fi
done
[ -z "$bootdisk" ] && echo "[DEBUG] Step 2: no GRUB/bios_grub found on any disk" >&2

# Step 3: Check for /boot partition in fstab
if [ -z "$bootdisk" ]; then
  boot_entry=$(grep -w '/boot' /etc/fstab 2>/dev/null | awk '{print $1}')
  # Handle both UUID=xxxx and /dev/disk/by-uuid/xxxx formats
  boot_uuid=$(echo "$boot_entry" | sed 's/^UUID=//; s|^/dev/disk/by-uuid/||')
  echo "[DEBUG] Step 3: boot_entry=$boot_entry boot_uuid=$boot_uuid" >&2
  # Only proceed if result is a bare UUID (not a device path)
  if [ -n "$boot_uuid" ] && ! echo "$boot_uuid" | grep -q '^/dev/'; then
    bootdisk=$(blkid 2>/dev/null | grep "$boot_uuid" | awk -F: '{sub(/[0-9][0-9]*$/, "", $1); print $1; exit}')
    echo "[DEBUG] Step 3: resolved bootdisk=$bootdisk" >&2
  fi
fi

# Step 4: Check for LVM root device
if [ -z "$bootdisk" ]; then
  # Check if root is on LVM
  root_dev=$(mount | grep 'on / ' | awk '{print $1}')
  echo "[DEBUG] Step 4: root_dev=$root_dev" >&2
  if echo "$root_dev" | grep -q '/dev/mapper/\|/dev/.*-vg/'; then
    # Root is on LVM, find physical volumes
    if command -v lvs >/dev/null 2>&1; then
      # Get VG name directly from lvs (avoids parsing device-mapper encoded names like ubuntu--vg)
      vg_name=$(lvs --noheadings -o vg_name "$root_dev" 2>/dev/null | tr -d ' ')
      # Find PV for this VG
      pv_dev=$(pvs --noheadings -o pv_name,vg_name 2>/dev/null | awk -v vg="$vg_name" '$2 == vg {print $1; exit}')
      echo "[DEBUG] Step 4: vg_name=$vg_name pv_dev=$pv_dev" >&2
      if [ -n "$pv_dev" ]; then
        # Strip partition number to get disk
        bootdisk=$(echo "$pv_dev" | sed 's/[0-9][0-9]*$//')
      fi
    fi
  else
    # Root is on regular partition, strip partition number
    bootdisk=$(echo "$root_dev" | sed 's/[0-9][0-9]*$//')
  fi
  echo "[DEBUG] Step 4: resolved bootdisk=$bootdisk" >&2
fi

# Step 5: Check for partitions with bootable or bios_grub flag set
if [ -z "$bootdisk" ]; then
  echo "[DEBUG] Step 5: checking bootable/bios_grub partition flags" >&2
  for disk in $disks; do
    # Get partition count for this disk
    partitions=$(ls "${disk}"[0-9]* 2>/dev/null)
    for part in $partitions; do
      # Extract partition number
      part_num=$(echo "$part" | sed "s|${disk}||")
      # Check if partition is bootable using parted
      if command -v parted >/dev/null 2>&1; then
        bootable=$(parted -s "$disk" print 2>/dev/null | awk -v pnum="$part_num" '$1 == pnum && (/boot/ || /bios_grub/) {print "true"}')
        echo "[DEBUG] Step 5: $part (num=$part_num) bootable=$bootable" >&2
        if [ "$bootable" = "true" ]; then
          bootdisk="$disk"
          break 2
        fi
      fi
    done
  done
fi

# Step 6: Final fallback - use first disk
if [ -z "$bootdisk" ]; then
  echo "[DEBUG] Step 6: fallback to first disk" >&2
  bootdisk=$(echo "$disks" | head -1)
fi

echo "[DEBUG] Result: bootdisk=$bootdisk" >&2
echo "$bootdisk"
