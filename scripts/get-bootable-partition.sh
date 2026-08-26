#!/bin/sh
# detect-bootdisk.sh
# Prints the correct BIOS boot disk (the one containing GRUB MBR)
# Works with LVM and /boot on a separate disk.
# Assumes we are running inside a guestfish-mounted environment
#
# NOTE ON OUTPUT: everything below - the [DEBUG] trace and the final
# result - is written to plain, unredirected stdout. This used to be split
# via `exec 3>&1 1>&2` (debug to stderr, result to fd3/stdout), on the
# assumption that the caller would receive both streams. It doesn't:
# libguestfs's `sh` API only relays a successful command's stdout back to
# the host, not its stderr, so every [DEBUG] line was silently dropped and
# only the bare result ever made it back. The final result line is now
# tagged (BOOTDISK_RESULT:<path>) so the caller can pull it out of the
# single stdout stream and treat every other line as trace.
#
# NOTE ON THE RAW SCAN BELOW: production traces show this can pick up the
# libguestfs appliance's own backing disk in addition to the real VM disks -
# a migration with exactly 2 attached VM disks had this script report
# "disks found: /dev/sda /dev/sdb /dev/sdc", where /dev/sdc turned out to be
# the appliance's own root filesystem, not a VM disk. That's very likely
# related to VJAILB-225's intermittent wrong-disk selection: if the
# appliance's disk ever enumerates earlier than a real disk (ordering here
# depends on SCSI/PCI probe timing, not anything guaranteed), the phantom
# device sits in front of a real one for every heuristic below. We are
# deliberately NOT filtering it out yet (e.g. via guestfish's list-devices,
# which does exclude it) - the caller logs list-devices' answer alongside
# this script's raw scan on every run precisely so the next time this
# misfires, we have both lists side by side and can confirm the
# phantom-disk theory against a real occurrence before changing this
# script's actual disk-selection behavior.

# Step 1: Identify all disks in guest
# List block devices (assuming /dev/vda, /dev/sda, etc. are available)
disks=$(ls /dev/[sv]d[a-z] 2>/dev/null)
echo "[DEBUG] Step 1: disks found: $(echo $disks | tr '\n' ' ')"

bootdisk=""

# Step 2: Try to find which disk's MBR contains GRUB signature, or has a bios_grub partition
#   - MBR-partitioned disks: GRUB stage1 boot code is embedded in the MBR
#   - GPT-partitioned disks: GRUB uses a dedicated bios_grub partition (no GRUB in MBR)
for disk in $disks; do
  # Check MBR for GRUB signature (works for MBR-partitioned disks)
  if dd if="$disk" bs=512 count=1 2>/dev/null | grep -aq 'GRUB'; then
    echo "[DEBUG] Step 2: GRUB signature found in MBR of $disk"
    bootdisk="$disk"
    break
  fi
  # Check for GPT BIOS boot partition (grub2 with GPT uses bios_grub partition)
  if command -v parted >/dev/null 2>&1; then
    if parted -s "$disk" print 2>/dev/null | grep -q 'bios_grub'; then
      echo "[DEBUG] Step 2: bios_grub partition found on $disk"
      bootdisk="$disk"
      break
    fi
  fi
done
[ -z "$bootdisk" ] && echo "[DEBUG] Step 2: no GRUB/bios_grub found on any disk"

# Step 3: Check for /boot partition in fstab
if [ -z "$bootdisk" ]; then
  boot_entry=$(grep -w '/boot' /etc/fstab 2>/dev/null | awk '{print $1}')
  # Handle both UUID=xxxx and /dev/disk/by-uuid/xxxx formats
  boot_uuid=$(echo "$boot_entry" | sed 's/^UUID=//; s|^/dev/disk/by-uuid/||')
  echo "[DEBUG] Step 3: boot_entry=$boot_entry boot_uuid=$boot_uuid"
  # Only proceed if result is a bare UUID (not a device path)
  if [ -n "$boot_uuid" ] && ! echo "$boot_uuid" | grep -q '^/dev/'; then
    bootdisk=$(blkid 2>/dev/null | grep "$boot_uuid" | awk -F: '{sub(/[0-9][0-9]*$/, "", $1); print $1; exit}')
    echo "[DEBUG] Step 3: resolved bootdisk=$bootdisk"
  fi
fi

# Step 4: Check for LVM root device
if [ -z "$bootdisk" ]; then
  # Check if root is on LVM
  root_dev=$(mount | grep 'on / ' | awk '{print $1}')
  echo "[DEBUG] Step 4: root_dev=$root_dev"
  if echo "$root_dev" | grep -q '/dev/mapper/\|/dev/.*-vg/'; then
    # Root is on LVM, find physical volumes
    if command -v lvs >/dev/null 2>&1; then
      # Get VG name directly from lvs (avoids parsing device-mapper encoded names like ubuntu--vg)
      vg_name=$(lvs --noheadings -o vg_name "$root_dev" 2>/dev/null | tr -d ' ')
      # Find PV for this VG
      pv_dev=$(pvs --noheadings -o pv_name,vg_name 2>/dev/null | awk -v vg="$vg_name" '$2 == vg {print $1; exit}')
      echo "[DEBUG] Step 4: vg_name=$vg_name pv_dev=$pv_dev"
      if [ -n "$pv_dev" ]; then
        # Strip partition number to get disk
        bootdisk=$(echo "$pv_dev" | sed 's/[0-9][0-9]*$//')
      fi
    fi
  else
    # Root is on regular partition, strip partition number
    bootdisk=$(echo "$root_dev" | sed 's/[0-9][0-9]*$//')
  fi
  echo "[DEBUG] Step 4: resolved bootdisk=$bootdisk"
fi

# Step 5: Check for partitions with bootable or bios_grub flag set
if [ -z "$bootdisk" ]; then
  echo "[DEBUG] Step 5: checking bootable/bios_grub partition flags"
  for disk in $disks; do
    # Get partition count for this disk
    partitions=$(ls "${disk}"[0-9]* 2>/dev/null)
    for part in $partitions; do
      # Extract partition number
      part_num=$(echo "$part" | sed "s|${disk}||")
      # Check if partition is bootable using parted
      if command -v parted >/dev/null 2>&1; then
        bootable=$(parted -s "$disk" print 2>/dev/null | awk -v pnum="$part_num" '$1 == pnum && (/boot/ || /bios_grub/) {print "true"}')
        echo "[DEBUG] Step 5: $part (num=$part_num) bootable=$bootable"
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
  echo "[DEBUG] Step 6: fallback to first disk"
  bootdisk=$(echo "$disks" | head -1)
fi

echo "[DEBUG] Result: bootdisk=$bootdisk"

# The one and only tagged result line. The caller looks for this exact
# prefix in stdout and treats every other line as debug trace.
echo "BOOTDISK_RESULT:$bootdisk"
