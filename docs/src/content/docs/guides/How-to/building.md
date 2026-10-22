---
title: Build vJailbreak
description: How to compile vJailbreak
---

vJailbreak is intended to be run in a Kubernetes environment (k3s) on the appliance VM. In order to build and deploy the Kubernetes components, follow the instructions in `k8s/migration` to build and deploy the custom resources in the cluster. 

:::tip[Did you know?]
Manually building vJailbreak is not required for deployment, only development.
:::

In order to build v2v-helper,

    make v2v-helper

In order to build migration-controller,

    make vjail-controller

In order to build the UI,

    make ui

To push to another repository, you do not need to edit the Makefile. The image names are built from the `REGISTRY`, `REPO` and `TAG` variables (defaults: `quay.io`, `platform9` and the current version), so override them on the command line:

    REGISTRY=myregistry.io REPO=myteam TAG=v1.2.3 make ui