---
title: Scaling vJailbreak 
description: You can scale up vJailbreak to perform more parallel migrations
---

## vJailbreak at Scale: Managing Agents with VjailbreakNode

vJailbreak can be scaled to perform multiple migrations in parallel by deploying additional `agents`, enabling greater efficiency and workload distribution. The VjailbreakNode Custom Resource Definition (CRD) streamlines the creation and management of these agents, ensuring seamless integration into the migration workflow. Each `VjailbreakNode` represents a VM that functions as an independent migration `agent`. These agents are dynamically added to the original `VjailbreakNode`, forming a cohesive cluster that enhances scalability, reliability, and overall migration performance.

### VjailbreakNode CRD

The `VjailbreakNode` CRD allows you to manage vJailbreak nodes within your Kubernetes cluster. Here's how to define a `VjailbreakNode` resource:

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: VjailbreakNode
metadata:
  name: example-vjailbreak-node
  namespace: migration-system
spec:
  imageid: "your-openstack-image-id" # This ID is for the first vjailbreak VMimage. It auto-populates in the UI—do not delete it. 
  noderole: "migration-worker"
  openstackcreds:
    name: "sapmo1" # Reference to your OpenstackCreds
    namespace: "migration-system"
  openstackflavorid: "your-openstack-flavor-id"
 ```
 
 ## Explanation of VjailbreakNode CRD Fields  

This `VjailbreakNode` CRD defines a Kubernetes resource that provisions a VM in OpenStack to act as a migration agent. Below is a breakdown of each field:  


### Metadata  
- **`metadata:`**  
  Metadata contains identifying details about the `VjailbreakNode`.  
  - **`name: example-vjailbreak-node`**  
    Specifies the name of this `VjailbreakNode` resource in Kubernetes.  
  - **`namespace: migration-system`**  
    Indicates the namespace where this resource is deployed within the Kubernetes cluster.  

### Spec (Specification)  
The `spec` section defines the desired state of the `VjailbreakNode`.  

- **`imageid: "your-openstack-image-id"`**  
  - This is the ID of the OpenStack image used to create the VM.  
  - **It must match the image ID used to create the initial vJailbreak VM**, ensuring compatibility across all migration agents.  

- **`noderole: "worker"`**  
  - Defines the role of the node.  
  - It should be set to `"worker"` as this node functions as a migration agent within the vJailbreak cluster.  

- **`openstackcreds:`**  
  - OpenstackCreds use the variables from the openstack.rc file.
  - **`name: "sapmo1"`** → Refers to a `Secret` or `CustomResource` storing OpenStack authentication details.  
  - **`namespace: "migration-system"`** → The namespace where OpenStack credentials are stored.  

- **`openstackflavorid: "your-openstack-flavor-id"`**  
  - Specifies the OpenStack flavor ID, which determines the VM's compute resources (CPU, RAM, disk size, etc.).  
  - The chosen flavor should align with the resource requirements for migration workloads.  

This configuration ensures vJailbreak can scale efficiently by adding worker nodes dynamically to handle multiple migrations in parallel. 🚀  

🚨 ** Important note ** 🚨
After scaling up make sure that Copy over the [VDDK libraries](https://developer.broadcom.com/sdks/vmware-virtual-disk-development-kit-vddk/8.0) for Linux into `/home/ubuntu` of the new agents. Untar it to a folder name `vmware-vix-disklib-distrib` in `/home/ubuntu` directory. 

**_NOTE:_**
To retrieve the password for logging into a worker node, follow these steps:
1. SSH into the master node and run:
   ```bash
   cat /var/lib/rancher/k3s/server/token
2. The first 12 characters of this token is your password. 

### 🚀 Required Ingress Rules for Kubernetes Node with Kubelet, Metrics Server, and Prometheus

| **Component**      | **Port**  | **Protocol** | **Source** | **Purpose** |
|--------------------|----------|-------------|------------|-------------|
| **Kubelet API**    | `10250`   | TCP         | Control Plane / Prometheus | Health checks, logs, metrics |
| **Kubelet Read-Only (Optional)** | `10255` | TCP | Internal Only | Deprecated but might be used in some cases |
| **Metrics Server** | `4443`    | TCP         | Internal Cluster | K8s resource metrics (`kubectl top`) |
| **Prometheus**     | `9090`    | TCP         | Internal Cluster / Monitoring Server | Prometheus UI and API |
| **Node Exporter** (if used) | `9100` | TCP | Prometheus | Node-level metrics |
| **Cadvisor (Optional)** | `4194` | TCP | Internal Cluster / Prometheus | Container metrics collection |