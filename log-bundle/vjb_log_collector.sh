#!/bin/bash


OUTPUT_DIR="./vjb-log-bundle"
mkdir -p "$OUTPUT_DIR"

echo "Collecting controller logs"

PODS=$(kubectl -n migration-system get pods -o name | grep controller)

for pod in $PODS; do
    echo "Fetching logs for $pod"
    kubectl -n migration-system logs "$pod" > "$OUTPUT_DIR/$(basename $pod).log"
done

echo "Controller logs collected in $OUTPUT_DIR"

