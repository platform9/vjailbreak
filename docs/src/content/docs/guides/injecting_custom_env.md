---
title: "v2v-helper Environment Variable Injection via ConfigMap"
---
## Summary

This update enables environment variable injection for the `v2v-helper` pod using a Kubernetes ConfigMap. Multiple components, including the DaemonSet configuration, installation script, and Packer build file, have been enhanced to support this new environment injection mechanism. The `migration-controller` now references the created ConfigMap, ensuring a consistent and manageable configuration workflow.

## How It Works

1. **Cloud-init populates environment variables**

   Users must provide environment variables in the `/etc/pf9/env` file during provisioning, typically using a cloud-init script.

2. **ConfigMap creation from /etc/pf9/env**

   A helper script or manual command reads `/etc/pf9/env` and creates a Kubernetes ConfigMap named `v2v-env-config`.
   This is done while the vjailbreak VM is being provisioned.
   ```bash
   kubectl create configmap pf9-env --from-env-file=/etc/pf9/env -n <namespace>


## Injecting Environment Variables Post-Provisioning

If you would like to inject environment variables after the vjailbreak VM has been provisioned, follow these steps:
1. **Delete the existing ConfigMap**

   ```bash
   kubectl delete configmap pf9-env -n <namespace>
   ```

2. **Create a new ConfigMap**

   ```bash
   kubectl create configmap pf9-env --from-env-file=/etc/pf9/env -n <namespace>
   ```
3. **Trigger a new migration for the envs to be reflected in the pod**

    Trigger via UI or via api. 

    
   