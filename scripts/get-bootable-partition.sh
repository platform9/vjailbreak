#!/bin/sh
# Picks the BIOS boot disk (GRUB) for a guestfish-mounted guest.
# Disks are passed in by the caller, not scanned, so the appliance's own disk is never a candidate.

if [ "$#" -gt 0 ]; then
  disks="$*"
  echo "[DEBUG] Step 1: disks passed by caller: $disks"
else
  # Manual/standalone fallback; can also pick up the appliance's own disk.
  disks=$(ls /dev/[sv]d[a-z] 2>/dev/null)
  echo "[DEBUG] Step 1: disks found via raw scan (no caller-provided list): $(echo $disks | tr '\n' ' ')"
fi

bootdisk=""

# Step 2: GRUB MBR signature or bios_grub partition
for disk in $disks; do
  if dd if="$disk" bs=512 count=1 2>/dev/null | grep -aq 'GRUB'; then
    echo "[DEBUG] Step 2: GRUB signature found in MBR of $disk"
    bootdisk="$disk"
    break
  fi
  if command -v parted >/dev/null 2>&1; then
    if parted -s "$disk" print 2>/dev/null | grep -q 'bios_grub'; then
      echo "[DEBUG] Step 2: bios_grub partition found on $disk"
      bootdisk="$disk"
      break
    fi
  fi
done
[ -z "$bootdisk" ] && echo "[DEBUG] Step 2: no GRUB/bios_grub found on any disk"

# Step 3: /boot UUID in fstab
if [ -z "$bootdisk" ]; then
  boot_entry=$(grep -w '/boot' /etc/fstab 2>/dev/null | awk '{print $1}')
  boot_uuid=$(echo "$boot_entry" | sed 's/^UUID=//; s|^/dev/disk/by-uuid/||')
  echo "[DEBUG] Step 3: boot_entry=$boot_entry boot_uuid=$boot_uuid"
  if [ -n "$boot_uuid" ] && ! echo "$boot_uuid" | grep -q '^/dev/'; then
    bootdisk=$(blkid 2>/dev/null | grep "$boot_uuid" | awk -F: '{sub(/[0-9][0-9]*$/, "", $1); print $1; exit}')
    echo "[DEBUG] Step 3: resolved bootdisk=$bootdisk"
  fi
fi

# Step 4: LVM root device
if [ -z "$bootdisk" ]; then
  root_dev=$(mount | grep 'on / ' | awk '{print $1}')
  echo "[DEBUG] Step 4: root_dev=$root_dev"
  if echo "$root_dev" | grep -q '/dev/mapper/\|/dev/.*-vg/'; then
    if command -v lvs >/dev/null 2>&1; then
      vg_name=$(lvs --noheadings -o vg_name "$root_dev" 2>/dev/null | tr -d ' ')
      pv_dev=$(pvs --noheadings -o pv_name,vg_name 2>/dev/null | awk -v vg="$vg_name" '$2 == vg {print $1; exit}')
      echo "[DEBUG] Step 4: vg_name=$vg_name pv_dev=$pv_dev"
      if [ -n "$pv_dev" ]; then
        bootdisk=$(echo "$pv_dev" | sed 's/[0-9][0-9]*$//')
      fi
    fi
  else
    bootdisk=$(echo "$root_dev" | sed 's/[0-9][0-9]*$//')
  fi
  echo "[DEBUG] Step 4: resolved bootdisk=$bootdisk"
fi

# Step 5: bootable/bios_grub partition flags
if [ -z "$bootdisk" ]; then
  echo "[DEBUG] Step 5: checking bootable/bios_grub partition flags"
  for disk in $disks; do
    partitions=$(ls "${disk}"[0-9]* 2>/dev/null)
    for part in $partitions; do
      part_num=$(echo "$part" | sed "s|${disk}||")
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

# Step 6: first-disk fallback; word-split works for both space- and newline-joined $disks
if [ -z "$bootdisk" ]; then
  echo "[DEBUG] Step 6: fallback to first disk"
  for disk in $disks; do
    bootdisk="$disk"
    break
  done
fi

echo "[DEBUG] Result: bootdisk=$bootdisk"
echo "BOOTDISK_RESULT:$bootdisk"
