---
title: Troubleshooting vJailbreak 
description: Tips on effectively troubleshooting vJailbreak deployment and migration
---

:::note
All of the following Kubernetes commands will need to be run from the vJailbreak VM, or remotely using the vJailbreak VM's kubeconfig, located at `/etc/ranger/k3s/k3s.yaml` on the vJailbreak VM.
:::
 
## Common issues

- [Windows Dynamic Disk (LDM) migration issue](windows-dynamic-disk-ldm-migration-issue/)

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

