---
title: "v2v-helper Environment Variable Injection via ConfigMap"
description: "Enabling environment variable injection for the v2v-helper pod using a Kubernetes ConfigMap"
---

Injecting environment variables into the v2v-helper pod is a feature that allows users to inject environment variables into the v2v-helper pod using a Kubernetes ConfigMap. 

## How It Works

1. **Cloud-init populates environment variables**

   Users must provide environment variables in the `/etc/pf9/env` file during provisioning, typically using a cloud-init script.

2. **ConfigMap creation from /etc/pf9/env**

   A helper script or manual command reads `/etc/pf9/env` and creates a Kubernetes ConfigMap named `pf9-env`.
   This is done while the vjailbreak VM is being provisioned.
   ```bash
   kubectl create configmap pf9-env --from-env-file=/etc/pf9/env -n migration-system

# Example
   If you want proxy variables to be injected into the v2v-helper pod, you can add the following to the `/etc/pf9/env` file via the cloud-init script:
   ```bash
   http_proxy=http://<proxy-server>:<proxy-port>
   https_proxy=http://<proxy-server>:<proxy-port>
   no_proxy=localhost,127.0.0.1
   ```
   You can either populate the `/etc/pf9/env` file via cloud-init or manually.

   If done manually please follow the steps mentioned in [Injecting Environment Variables Post-Provisioning](#injecting-environment-variables-post-provisioning):
   

   Now this will be picked up by the v2v-helper pod and the proxy variables will be available in the pod and it would be respected by the v2v-helper pod.

## Injecting Environment Variables Post-Provisioning

If you would like to inject environment variables after the vjailbreak VM has been provisioned, follow these steps:
1. **Delete the existing ConfigMap**

   ```bash
   kubectl delete configmap pf9-env -n migration-system
   ```
2. **Populate the `/etc/pf9/env` file with whatever env variables needed**

   ```bash
   echo "http_proxy=http://<proxy-server>:<proxy-port>" >> /etc/pf9/env
   echo "https_proxy=http://<proxy-server>:<proxy-port>" >> /etc/pf9/env
   echo "no_proxy=localhost,127.0.0.1" >> /etc/pf9/env
   ```

3. **Create a new ConfigMap**

   ```bash
   kubectl create configmap pf9-env --from-env-file=/etc/pf9/env -n migration-system
   ```
4. **Trigger a new migration for the envs to be reflected in the pod**

    Trigger via UI or via api. 

    
   