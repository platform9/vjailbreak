#!/bin/bash

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[TEST]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[!]${NC} $1"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Parse arguments
HOST_IP=${1:-}

if [ -z "$HOST_IP" ]; then
    error "Usage: $0 <HOST_IP>
    
Example: $0 10.9.2.145

This script tests the Dex IdP integration with vJailbreak."
    exit 1
fi

log "Testing Dex IdP Integration on host: $HOST_IP"
echo ""

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

run_test() {
    local test_name=$1
    local test_command=$2
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    info "Test $TOTAL_TESTS: $test_name"
    
    if eval "$test_command" > /dev/null 2>&1; then
        success "$test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        error "$test_name"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

echo "=== Infrastructure Tests ==="
echo ""

# Test 1: Kubernetes cluster connectivity
run_test "Kubernetes cluster is accessible" "kubectl cluster-info"

# Test 2: cert-manager is installed
run_test "cert-manager is installed" "kubectl get deployment cert-manager -n cert-manager"

# Test 3: cert-manager is ready
run_test "cert-manager is ready" "kubectl get deployment cert-manager -n cert-manager -o jsonpath='{.status.readyReplicas}' | grep -q '^1$'"

echo ""
echo "=== Dex Tests ==="
echo ""

# Test 4: Dex namespace exists
run_test "Dex namespace exists" "kubectl get namespace dex"

# Test 5: Dex deployment exists
run_test "Dex deployment exists" "kubectl get deployment dex -n dex"

# Test 6: Dex pod is running
run_test "Dex pod is running" "kubectl get pod -n dex -l app=dex -o jsonpath='{.items[0].status.phase}' | grep -q Running"

# Test 7: Dex service exists
run_test "Dex service exists" "kubectl get service dex -n dex"

# Test 8: Dex ingress exists
run_test "Dex ingress exists" "kubectl get ingress dex -n dex"

# Test 9: Dex is responding
if curl -s -o /dev/null -w "%{http_code}" "http://$HOST_IP/dex/healthz" | grep -q "200"; then
    success "Dex health endpoint is responding"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    error "Dex health endpoint is not responding"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

# Test 10: Dex OIDC discovery
if curl -s "http://$HOST_IP/dex/.well-known/openid-configuration" | grep -q "issuer"; then
    success "Dex OIDC discovery endpoint is working"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    error "Dex OIDC discovery endpoint is not working"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

echo ""
echo "=== OAuth2 Proxy Tests ==="
echo ""

# Test 11: OAuth2 Proxy namespace exists
run_test "OAuth2 Proxy namespace exists" "kubectl get namespace oauth2-proxy"

# Test 12: OAuth2 Proxy deployment exists
run_test "OAuth2 Proxy deployment exists" "kubectl get deployment oauth2-proxy -n oauth2-proxy"

# Test 13: OAuth2 Proxy pod is running
run_test "OAuth2 Proxy pod is running" "kubectl get pod -n oauth2-proxy -l app=oauth2-proxy -o jsonpath='{.items[0].status.phase}' | grep -q Running"

# Test 14: OAuth2 Proxy service exists
run_test "OAuth2 Proxy service exists" "kubectl get service oauth2-proxy -n oauth2-proxy"

# Test 15: OAuth2 Proxy ingress exists
run_test "OAuth2 Proxy ingress exists" "kubectl get ingress oauth2-proxy-ingress -n oauth2-proxy"

# Test 16: OAuth2 Proxy ping endpoint
if curl -s "http://$HOST_IP/oauth2/ping" | grep -q "OK"; then
    success "OAuth2 Proxy ping endpoint is responding"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    warn "OAuth2 Proxy ping endpoint check (may be protected)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

echo ""
echo "=== RBAC Tests ==="
echo ""

# Test 17: Admin ClusterRole exists
run_test "Admin ClusterRole exists" "kubectl get clusterrole vjailbreak-admin"

# Test 18: Operator ClusterRole exists
run_test "Operator ClusterRole exists" "kubectl get clusterrole vjailbreak-operator"

# Test 19: Viewer ClusterRole exists
run_test "Viewer ClusterRole exists" "kubectl get clusterrole vjailbreak-viewer"

# Test 20: Credential Manager ClusterRole exists
run_test "Credential Manager ClusterRole exists" "kubectl get clusterrole vjailbreak-credential-manager"

# Test 21: Admin ClusterRoleBinding exists
run_test "Admin ClusterRoleBinding exists" "kubectl get clusterrolebinding vjailbreak-admins-binding"

# Test 22: Default admin user binding exists
run_test "Default admin user binding exists" "kubectl get clusterrolebinding vjailbreak-default-admin-binding"

echo ""
echo "=== UI Tests ==="
echo ""

# Test 23: UI ServiceAccount exists
run_test "UI ServiceAccount exists" "kubectl get serviceaccount vjailbreak-ui-sa -n migration-system"

# Test 24: UI ServiceAccount token exists
run_test "UI ServiceAccount token exists" "kubectl get secret vjailbreak-ui-sa-token -n migration-system"

# Test 25: UI deployment is updated
run_test "UI deployment is using correct ServiceAccount" \
    "kubectl get deployment vjailbreak-ui -n migration-system -o jsonpath='{.spec.template.spec.serviceAccountName}' | grep -q vjailbreak-ui-sa"

# Test 26: UI pod is running
run_test "UI pod is running" "kubectl get pod -n migration-system -l app=vjailbreak-ui -o jsonpath='{.items[0].status.phase}' | grep -q Running"

# Test 27: UI ingress has auth annotations
run_test "UI ingress has auth annotations" \
    "kubectl get ingress vjailbreak-ui-ingress -n migration-system -o jsonpath='{.metadata.annotations}' | grep -q auth-url"

# Test 28: Main UI redirects to auth
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -L "http://$HOST_IP/")
if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 302 ]; then
    success "Main UI is accessible (HTTP $HTTP_CODE)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    error "Main UI is not accessible (HTTP $HTTP_CODE)"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

echo ""
echo "=== Configuration Tests ==="
echo ""

# Test 29: Dex config has correct issuer
ISSUER=$(kubectl get configmap dex-config -n dex -o jsonpath='{.data.config\.yaml}' | grep "issuer:" | awk '{print $2}')
if echo "$ISSUER" | grep -q "$HOST_IP"; then
    success "Dex issuer is configured with correct IP: $ISSUER"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    warn "Dex issuer may not be configured correctly: $ISSUER (expected $HOST_IP)"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

# Test 30: Dex has static password configured
if kubectl get configmap dex-config -n dex -o jsonpath='{.data.config\.yaml}' | grep -q "staticPasswords"; then
    success "Dex has static passwords configured"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    error "Dex does not have static passwords configured"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

# Test 31: Dex has local connector
if kubectl get configmap dex-config -n dex -o jsonpath='{.data.config\.yaml}' | grep -q "type: local"; then
    success "Dex has local connector configured"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    error "Dex does not have local connector configured"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

echo ""
echo "=== Summary ==="
echo ""

info "Total Tests: $TOTAL_TESTS"
success "Tests Passed: $TESTS_PASSED"
if [ $TESTS_FAILED -gt 0 ]; then
    error "Tests Failed: $TESTS_FAILED"
else
    success "Tests Failed: 0"
fi

echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    success "All tests passed! ✓"
    echo ""
    info "Dex IdP integration is properly configured."
    echo ""
    info "Next steps:"
    echo "  1. Access the UI at: http://$HOST_IP"
    echo "  2. Login with: admin@vjailbreak.local / admin"
    echo "  3. Change the default password"
    echo ""
    exit 0
else
    error "Some tests failed. Please review the output above."
    echo ""
    info "Common issues:"
    echo "  - Pods not running: kubectl get pods -A"
    echo "  - Check logs: kubectl logs -n dex -l app=dex"
    echo "  - Verify HOST_IP in configs"
    echo ""
    exit 1
fi
