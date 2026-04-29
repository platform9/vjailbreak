---
title: Troubleshooting vJailbreak 
description: Tips on effectively troubleshooting vJailbreak deployment and migration
---

:::note
All of the following Kubernetes commands will need to be run from the vJailbreak VM, or remotely using the vJailbreak VM's kubeconfig, located at `/etc/ranger/k3s/k3s.yaml` on the vJailbreak VM.
:::
 
## Common issues

- [Windows Dynamic Disk (LDM) migration issue](windows-dynamic-disk-ldm-migration-issue/)
- [nbdcopy fails during disk copy (often DNS resolution)](nbdcopy-fails-after-vm-moved-esxi-host/)
- [virt-v2v fails: rename /sysroot/etc/resolv.conf Operation not permitted](#virt-v2v-fails-rename-sysrooteresolvconf-operation-not-permitted)

vJailbreak is deployed on Kubernetes running on Ubuntu 22.04.5, and distributed as a QCOW2 image. The Kubernetes namespace `migration-system` contains the vJailbreak UI and migration controller pods. Each VM migration will spawn a migration object. The status field contains a high level view of the progress of the migration of the VM. For more details about the migration, check the logs of the pod specified in the Migration object.

### Getting logs
List all pods in the migration namespace
```bash
kubectl -n migration-system get pod
```

Find a specific VM migration pod
```bash
kubectl -n migration-system get pod | grep <source VM name>
```

Get details & events for a v2v-helper pod. This is helpful if a migration is stuck in a pending state, or to track the progress of a migration without the UI.
```bash
kubectl -n migration-system describe pod <v2v-helper-pod-name>
```

Get logs for a specific migration pod. This shows more detail than `describe pod`.
```bash
kubectl logs <pod> -n migration-system
```

Get logs for the `migration-controller-manager`
```bash
kubectl logs -n migration-system deploy/migration-controller-manager
```

Turn on Debug Mode
```bash
kubectl patch configmap -n migration-system migration-config-<vm-name> --type merge -p '{"data":{"DEBUG":"true"}}'
```
### A migration is stuck in pending
If the migration was set to Retry on Failure, then delete the v2v-helper pod for that VM and collect the logs of the pod that comes up.

```bash
kubectl delete pod -n migration-system v2v-helper-<vm-name>
```
If the v2v-helper pod doesn't come back up, and you can't delete the migration in the UI, then delete the associated `migrationplan`.
- First, get the `migrationplan` object name UUID for the associated VMs:
```bash
kubectl get migrationplans -n migration-system -o yaml
```
- Then delete the `migrationplan` object, which should remove it from the UI.
```bash
kubectl delete migrationplan <UUID> -n migration-system
```
### Get all vJailbreak custom resource definitions (CRDs)

```bash
kubectl get migrationplans,migrations,migrationtemplates,networkmappings,openstackcreds,storagemappings,vmwarecreds,secrets -n migration-system -o yaml
```

---

## virt-v2v fails: rename /sysroot/etc/resolv.conf Operation not permitted

- **Symptom**

  `virt-v2v` or `virt-v2v-in-place` fails with an error similar to:

  ```text
  renaming /sysroot/etc/resolv.conf to /sysroot/etc/6vvk9gzd
  guestfsd: error: rename: /sysroot/etc/resolv.conf to /sysroot/etc/6vvk9gzd: Operation not permitted
  commandrvf: stdout=n stderr=n flags=0x0
  commandrvf: umount /sysroot/sys
  virt-v2v-in-place: error: libguestfs error: sh_out: rename: /sysroot/etc/resolv.conf to /sysroot/etc/6vvk9gzd: Operation not permitted
  ```

- **Cause**

  On some Linux VMs, `/etc/resolv.conf` is marked **immutable**. When `virt-v2v` tries to rename or replace this file inside the guest filesystem during conversion, the immutable attribute prevents the operation and conversion fails.

  You can confirm the immutable bit inside the source VM with:

  ```bash
  lsattr /etc/resolv.conf
  ----i----------------- /etc/resolv.conf
  ```

  The `i` flag indicates the file is immutable.

- **Resolution**

  1. Remove the immutable attribute inside the source VM before migration:

     ```bash
     chattr -i /etc/resolv.conf
     ```

  2. Verify the attribute is gone:

     ```bash
     lsattr /etc/resolv.conf
     ---------------------- /etc/resolv.conf
     ```

  3. Re-run the migration.

- **Notes**

  - This is a known and documented `virt-v2v` issue. [See upstream documentation](https://libguestfs.org/virt-v2v.1.html#linux%3A-rename%3A-sysroot-etc-resolv.conf-failure).
  - If configuration management or security hardening marks `/etc/resolv.conf` immutable, ensure this is unset before conversion, or adjust your automation so VMs intended for conversion do not have `/etc/resolv.conf` marked immutable.
