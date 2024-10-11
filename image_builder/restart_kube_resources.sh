#!/bin/bash
k3s token rotate
systemctl restart k3s
sleep 5
kubectl get deployments --all-namespaces | tail +2 | awk '{print $1" "$2}' | xargs -n 2 kubectl rollout restart deployment -n
kubectl get daemonsetss --all-namespaces | tail +2 | awk '{print $1" "$2}' | xargs -n 2 kubectl rollout restart daemonsets -n
sleep 10
kubectl get pods --all-namespaces | grep Terminating | awk '{print $1" "$2}' | xargs -n 2 kubectl delete pod --grace-period=0 --force --namespace
sleep 5
kubectl get pods --all-namespaces | grep Terminating | awk '{print $1" "$2}' | xargs -n 2 kubectl delete pod --grace-period=0 --force --namespace

