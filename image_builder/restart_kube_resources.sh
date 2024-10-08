#!/bin/bash
k3s token rotate
kubectl get deployments --all-namespaces | tail +2 | awk '{ cmd=sprintf("kubectl rollout restart deployment -n %s %s", $1, $2) ; system(cmd) }'
kubectl get daemonsets --all-namespaces | tail +2 | awk '{ cmd=sprintf("kubectl rollout restart daemonsets -n %s %s", $1, $2) ; system(cmd) }'
sleep 20
kubectl get pods --all-namespaces | grep Terminating | awk '{print $1" "$2}' | xargs -n 2 kubectl delete pod --grace-period=0 --force --namespace

