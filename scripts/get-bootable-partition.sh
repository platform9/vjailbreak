#!/bin/sh
# detect-bootdisk.sh
# Prints the correct BIOS boot disk (the one containing GRUB MBR)
# Works with LVM and /boot on a separate disk.
# Assumes we are running inside a guestfish-mounted environment

# Step 1: Identify all disks in guest
# List block devices (assuming /dev/vda, /dev/sda, etc. are available)
disks=$(ls /dev/[sv]d[a-z] 2>/dev/null)

# Step 2: Try to find which disk's MBR contains GRUB signature
#   GRUB stage1 MBRs contain either "GRUB" or "boot" strings near offset 0x3E
bootdisk=""
for disk in $disks; do
  # read first 512 bytes
  sig=$(dd if="$disk" bs=512 count=1 2>/dev/null | strings | grep -E 'GRUB|boot' | head -n1)
  if [ -n "$sig" ]; then
    bootdisk="$disk"
    break
  fi
done

# Step 3: If nothing found, fallback to /boot partition's parent disk
if [ -z "$bootdisk" ]; then
  boot_uuid=$(grep -w '/boot' /etc/fstab | awk '{print $1}' | sed 's/^UUID=//')
  if [ -n "$boot_uuid" ]; then
    bootdisk=$(blkid | grep "$boot_uuid" | awk -F: '{sub(/[0-9][0-9]*$/, "", $1); print $1; exit}')
  else
    # Fallback to root partition's disk
    root_dev=$(mount | grep 'on / ' | awk '{print $1}' | sed 's/[0-9][0-9]*$//')
    bootdisk="$root_dev"
  fi
fi

echo "$bootdisk"