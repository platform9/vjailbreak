---
title: Building vJailbreak 
description: How to compile vJailbreak
---

## Building
vJailbreak is intended to be run in a kubernetes environment (k3s) on the appliance VM. In order to build and deploy the kubernetes components, follow the instructions in `k8s/migration` to build and deploy the custom resources in the cluster.

In order to build v2v-helper,

    make v2v-helper

In order to build migration-controller,

    make vjail-controller

In order to build the UI,

    make ui

Change the image names in the makefile to push to another repository