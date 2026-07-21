# vJailbreak Known Error Patterns

## DNS Resolution Failures

**Error:** `dial tcp: lookup <esxi-host>: no such host` or `DNS resolution failed`
**Cause:** ESXi host DNS not resolvable from vJailbreak VM during disk copy phase.
**Fix:**
1. Add ESXi host entries to `/etc/hosts` on the vJailbreak VM
2. Format: `<IP> <esxi-hostname> <esxi-short-name>`
3. Restart migration after adding entries

## VDDK Connection Failures

**Error:** `VDDK error: VixDiskLib_Connect failed` or `VDDK connection refused`
**Cause:** VDDK libraries missing or path incorrect.
**Fix:**
1. Verify VDDK is installed: `ls /home/ubuntu/vmware-vix-disklib-distrib/`
2. Download VDDK from https://developer.broadcom.com/sdks/vmware-virtual-disk-development-kit-vddk/
3. Extract to `/home/ubuntu/vmware-vix-disklib-distrib`

## CBT (Changed Block Tracking) Errors

**Error:** `CBT error` or `QueryChangedDiskAreas failed`
**Cause:** CBT not enabled on source VM, or CBT reset needed.
**Fix:**
1. Enable CBT on source VM in vCenter
2. Take a snapshot, then delete it (forces CBT reset)
3. Retry migration

## Disk Copy Timeout

**Error:** `disk copy timeout` or connection reset during copy
**Cause:** Network instability or ESXi host overloaded.
**Fix:**
1. Check network connectivity between vJailbreak VM and ESXi host
2. Increase NETWORK_TIMEOUT in vJailbreak settings
3. Retry migration — progress is saved via CBT

## OpenStack Upload Failures

**Error:** `Failed to upload image` or `glance upload error`
**Cause:** OpenStack Glance connection issue or disk quota exceeded.
**Fix:**
1. Verify OpenStack credentials and endpoints
2. Check available Glance storage quota
3. Verify network connectivity to OpenStack API

## virt-v2v Conversion Errors

**Error:** `virt-v2v: error` or `libguestfs error`
**Cause:** Unsupported guest OS or corrupt disk.
**Fix:**
1. Check supported OS list: https://libguestfs.org/virt-v2v-support.1.html
2. Run disk check on source VM before migration
3. Ensure VMware Tools is installed and up to date on source VM

## Pod Scheduling Failures

**Error:** `Insufficient memory` or `0/1 nodes available`
**Cause:** vJailbreak VM has insufficient resources for migration worker pod.
**Fix:**
1. Check node resources: `kubectl describe nodes`
2. Increase VM flavor (minimum m1.xlarge recommended)
3. Wait for other migrations to complete before retrying

## VMware Snapshot Errors

**Error:** `snapshot create failed` or `cannot take snapshot`
**Cause:** Source VM has too many snapshots or snapshot in progress.
**Fix:**
1. Remove existing snapshots from source VM in vCenter
2. Ensure no other snapshot operations are in progress
3. Retry migration
