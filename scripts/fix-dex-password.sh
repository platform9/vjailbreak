#!/bin/bash

set -e

PASSWORD="${1:-admin}"

echo "============================================"
echo "Fixing Dex Password with Go bcrypt"
echo "============================================"
echo ""
echo "Password will be set to: $PASSWORD"
echo ""

# Create a temporary Go program to generate the hash
cat > /tmp/genhash.go << 'GOEOF'
package main

import (
	"fmt"
	"os"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: genhash <password>")
		os.Exit(1)
	}
	
	password := os.Args[1]
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	
	// Verify it works
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		fmt.Fprintf(os.Stderr, "Verification failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Print(string(hash))
}
GOEOF

# Initialize go module if needed
cd /tmp
if [ ! -f go.mod ]; then
    go mod init genhash 2>/dev/null || true
fi

# Get bcrypt dependency
echo "Installing bcrypt dependency..."
go get golang.org/x/crypto/bcrypt 2>/dev/null || true

# Generate the hash
echo "Generating bcrypt hash using Go's bcrypt library (same as Dex)..."
HASH=$(go run genhash.go "$PASSWORD" 2>&1)

if [ -z "$HASH" ] || [[ ! "$HASH" =~ ^\$2[ab]\$ ]]; then
    echo "Error: Failed to generate valid bcrypt hash"
    echo "Output: $HASH"
    exit 1
fi

echo "Generated hash: $HASH"
echo ""

# Verify the hash format
if [[ "$HASH" =~ ^\$2y\$ ]]; then
    echo "ERROR: Generated hash has \$2y\$ prefix (PHP format)"
    echo "Dex only supports \$2a\$ or \$2b\$"
    exit 1
fi

echo "✓ Hash format is correct (Go bcrypt compatible)"
echo ""

# Escape special characters for sed
ESCAPED_HASH=$(echo "$HASH" | sed 's/[\/&$]/\\&/g')

# Update the ConfigMap
echo "Updating Dex ConfigMap..."
kubectl get configmap -n dex dex-config -o yaml | \
  sed -E "s/hash: \"\\\$2[aby]\\\$[0-9]+\\\$[a-zA-Z0-9.\/]*\"/hash: \"${ESCAPED_HASH}\"/" | \
  kubectl apply -f -

echo ""
echo "Restarting Dex pod..."
kubectl delete pod -n dex -l app=dex

echo ""
echo "Waiting for Dex to restart..."
sleep 10
kubectl wait --for=condition=Ready pod -n dex -l app=dex --timeout=60s 2>/dev/null || true

echo ""
echo "============================================"
echo "✓ Password Update Complete"
echo "============================================"
echo ""
echo "Login credentials:"
echo "  Email: admin@vjailbreak.local"
echo "  Password: $PASSWORD"
echo ""
echo "Access URL: http://10.9.2.145/"
echo ""
echo "Verify with:"
echo "  kubectl logs -n dex -l app=dex --tail=20"
echo ""
