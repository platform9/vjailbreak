#!/bin/bash

# Validation script for Dex and OAuth2 Proxy deployment
# Run this after deployment to ensure configuration is correct

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

log() {
    echo -e "${GREEN}[✓]${NC} $1"
}

error() {
    echo -e "${RED}[✗]${NC} $1"
    ((ERRORS++))
}

warn() {
    echo -e "${YELLOW}[!]${NC} $1"
    ((WARNINGS++))
}

section() {
    echo ""
    echo -e "${GREEN}======================================${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${GREEN}======================================${NC}"
}

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    error "kubectl is not installed or not in PATH"
    exit 1
fi

section "1. Checking Pod Status"

# Check Dex pods
if kubectl get pods -n dex -l app=dex --no-headers 2>/dev/null | grep -q "Running"; then
    log "Dex pod is running"
else
    error "Dex pod is NOT running"
    kubectl get pods -n dex -l app=dex 2>/dev/null || echo "Namespace 'dex' may not exist"
fi

# Check OAuth2 Proxy pods
if kubectl get pods -n oauth2-proxy -l app=oauth2-proxy --no-headers 2>/dev/null | grep -q "Running"; then
    log "OAuth2 Proxy pod is running"
else
    error "OAuth2 Proxy pod is NOT running"
    kubectl get pods -n oauth2-proxy -l app=oauth2-proxy 2>/dev/null || echo "Namespace 'oauth2-proxy' may not exist"
fi

section "2. Checking Cookie Secret Length"

COOKIE_SECRET=$(kubectl get secret -n oauth2-proxy oauth2-proxy-secret -o jsonpath='{.data.cookie-secret}' 2>/dev/null | base64 -d 2>/dev/null || echo "")
if [ -n "$COOKIE_SECRET" ]; then
    LENGTH=${#COOKIE_SECRET}
    if [ $LENGTH -eq 32 ] || [ $LENGTH -eq 24 ] || [ $LENGTH -eq 16 ]; then
        log "Cookie secret length is valid: $LENGTH bytes"
    else
        error "Cookie secret is $LENGTH bytes, must be 16, 24, or 32 bytes"
    fi
else
    error "Cannot read cookie secret from oauth2-proxy-secret"
fi

section "3. Checking for HOST_IP Placeholders"

# Check Dex config
if kubectl get configmap -n dex dex-config -o yaml 2>/dev/null | grep -q "HOST_IP"; then
    error "Dex config contains unreplaced HOST_IP placeholder"
    kubectl get configmap -n dex dex-config -o yaml | grep "HOST_IP" || true
else
    log "Dex config has no HOST_IP placeholders"
fi

# Check OAuth2 Proxy config
if kubectl get configmap -n oauth2-proxy oauth2-proxy-config -o yaml 2>/dev/null | grep -q "HOST_IP"; then
    error "OAuth2 Proxy config contains unreplaced HOST_IP placeholder"
    kubectl get configmap -n oauth2-proxy oauth2-proxy-config -o yaml | grep "HOST_IP" || true
else
    log "OAuth2 Proxy config has no HOST_IP placeholders"
fi

section "4. Checking Dex Issuer URL Format"

ISSUER=$(kubectl get configmap -n dex dex-config -o yaml 2>/dev/null | grep "issuer:" | head -1 | awk '{print $2}' | tr -d '"' || echo "")
if [ -n "$ISSUER" ]; then
    if [[ $ISSUER == *":5556"* ]]; then
        error "Dex issuer should NOT include port 5556 (use Ingress on port 80)"
        echo "   Current: $ISSUER"
        echo "   Expected format: http://<IP>/dex"
    else
        log "Dex issuer URL format is correct: $ISSUER"
    fi
else
    error "Cannot read issuer URL from Dex config"
fi

section "5. Checking OAuth2 Proxy OIDC Configuration"

OIDC_ISSUER=$(kubectl get configmap -n oauth2-proxy oauth2-proxy-config -o yaml 2>/dev/null | grep "oidc_issuer_url" | awk -F'"' '{print $2}' || echo "")
if [ -n "$OIDC_ISSUER" ]; then
    if [[ $OIDC_ISSUER == *":5556"* ]]; then
        error "OAuth2 Proxy oidc_issuer_url should NOT include port 5556"
        echo "   Current: $OIDC_ISSUER"
        echo "   Expected format: http://<IP>/dex"
    else
        log "OAuth2 Proxy OIDC issuer URL is correct: $OIDC_ISSUER"
    fi
else
    error "Cannot read oidc_issuer_url from OAuth2 Proxy config"
fi

section "6. Checking Issuer Consistency"

if [ -n "$ISSUER" ] && [ -n "$OIDC_ISSUER" ]; then
    if [ "$ISSUER" == "$OIDC_ISSUER" ]; then
        log "Issuer URLs match between Dex and OAuth2 Proxy"
    else
        error "Issuer mismatch detected!"
        echo "   Dex issuer:         $ISSUER"
        echo "   OAuth2 issuer:      $OIDC_ISSUER"
    fi
fi

section "7. Checking Ingress Configuration"

# Check Dex Ingress
if kubectl get ingress -n dex dex &>/dev/null; then
    log "Dex Ingress exists"
    DEX_PATH=$(kubectl get ingress -n dex dex -o jsonpath='{.spec.rules[0].http.paths[0].path}' 2>/dev/null || echo "")
    if [ "$DEX_PATH" == "/dex" ]; then
        log "Dex Ingress path is correct: $DEX_PATH"
    else
        warn "Dex Ingress path may be incorrect: $DEX_PATH (expected: /dex)"
    fi
else
    error "Dex Ingress not found"
fi

# Check OAuth2 Proxy Ingress
if kubectl get ingress -n oauth2-proxy oauth2-proxy-ingress &>/dev/null; then
    log "OAuth2 Proxy Ingress exists"
else
    warn "OAuth2 Proxy Ingress not found (may be intentional if using other routing)"
fi

section "8. Checking Service Configuration"

# Check Dex Service
if kubectl get svc -n dex dex &>/dev/null; then
    log "Dex Service exists"
    DEX_PORT=$(kubectl get svc -n dex dex -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "")
    if [ "$DEX_PORT" == "5556" ]; then
        log "Dex Service port is correct: $DEX_PORT"
    else
        warn "Dex Service port is $DEX_PORT (expected: 5556)"
    fi
else
    error "Dex Service not found"
fi

# Check OAuth2 Proxy Service
if kubectl get svc -n oauth2-proxy oauth2-proxy &>/dev/null; then
    log "OAuth2 Proxy Service exists"
    OAUTH_PORT=$(kubectl get svc -n oauth2-proxy oauth2-proxy -o jsonpath='{.spec.ports[0].port}' 2>/dev/null || echo "")
    if [ "$OAUTH_PORT" == "4180" ]; then
        log "OAuth2 Proxy Service port is correct: $OAUTH_PORT"
    else
        warn "OAuth2 Proxy Service port is $OAUTH_PORT (expected: 4180)"
    fi
else
    error "OAuth2 Proxy Service not found"
fi

section "9. Testing Connectivity"

# Get the IP from the issuer URL
if [ -n "$ISSUER" ]; then
    HOST_IP=$(echo "$ISSUER" | sed -e 's|http://||' -e 's|https://||' -e 's|/.*||' -e 's|:.*||')
    
    if [ -n "$HOST_IP" ]; then
        # Test Dex OIDC discovery
        if curl -s -o /dev/null -w "%{http_code}" "http://$HOST_IP/dex/.well-known/openid-configuration" 2>/dev/null | grep -q "200"; then
            log "Dex OIDC discovery endpoint is accessible: http://$HOST_IP/dex/.well-known/openid-configuration"
            
            # Verify issuer from discovery matches config
            DISCOVERED_ISSUER=$(curl -s "http://$HOST_IP/dex/.well-known/openid-configuration" 2>/dev/null | grep -o '"issuer":"[^"]*"' | cut -d'"' -f4 || echo "")
            if [ "$DISCOVERED_ISSUER" == "$ISSUER" ]; then
                log "Discovered issuer matches configured issuer"
            else
                error "Issuer mismatch!"
                echo "   Configured: $ISSUER"
                echo "   Discovered: $DISCOVERED_ISSUER"
            fi
        else
            warn "Cannot reach Dex OIDC discovery endpoint at http://$HOST_IP/dex/.well-known/openid-configuration"
            echo "   This may be normal if testing from outside the cluster network"
        fi
        
        # Test OAuth2 Proxy ping
        if curl -s -o /dev/null -w "%{http_code}" "http://$HOST_IP/oauth2/ping" 2>/dev/null | grep -q "200"; then
            log "OAuth2 Proxy ping endpoint is accessible: http://$HOST_IP/oauth2/ping"
        else
            warn "Cannot reach OAuth2 Proxy ping endpoint at http://$HOST_IP/oauth2/ping"
            echo "   This may be normal if testing from outside the cluster network"
        fi
    fi
fi

section "10. Checking Pod Logs for Errors"

# Check Dex logs
echo "Checking Dex logs for errors..."
DEX_ERRORS=$(kubectl logs -n dex -l app=dex --tail=100 2>/dev/null | grep -i "error\|fatal\|panic" | wc -l | tr -d ' ')
if [ "$DEX_ERRORS" -eq "0" ]; then
    log "No errors found in Dex logs"
else
    warn "Found $DEX_ERRORS error lines in Dex logs (check with: kubectl logs -n dex -l app=dex)"
fi

# Check OAuth2 Proxy logs
echo "Checking OAuth2 Proxy logs for errors..."
OAUTH_ERRORS=$(kubectl logs -n oauth2-proxy -l app=oauth2-proxy --tail=100 2>/dev/null | grep -i "error\|fatal" | grep -v "Failed to initialise OAuth2 Proxy" | wc -l | tr -d ' ')
if [ "$OAUTH_ERRORS" -eq "0" ]; then
    log "No errors found in OAuth2 Proxy logs"
else
    warn "Found $OAUTH_ERRORS error lines in OAuth2 Proxy logs (check with: kubectl logs -n oauth2-proxy -l app=oauth2-proxy)"
fi

section "11. RBAC Validation"

# Check for vjailbreak RBAC roles
RBAC_ROLES=$(kubectl get clusterrole | grep vjailbreak | wc -l | tr -d ' ')
if [ "$RBAC_ROLES" -ge "4" ]; then
    log "Found $RBAC_ROLES vjailbreak RBAC roles"
else
    warn "Expected 4 vjailbreak RBAC roles, found $RBAC_ROLES"
fi

section "Validation Summary"

echo ""
if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  ✓ ALL CHECKS PASSED${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "Your Dex and OAuth2 Proxy deployment is correctly configured!"
    echo ""
    if [ -n "$HOST_IP" ]; then
        echo "Access your application at: http://$HOST_IP/"
        echo "Login with: admin@vjailbreak.local / admin"
    fi
    exit 0
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}  ! VALIDATION COMPLETED WITH WARNINGS${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    echo "Warnings: $WARNINGS"
    echo ""
    echo "Your deployment has some warnings but should be functional."
    echo "Review the warnings above and address if necessary."
    exit 0
else
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}  ✗ VALIDATION FAILED${NC}"
    echo -e "${RED}========================================${NC}"
    echo ""
    echo "Errors: $ERRORS"
    echo "Warnings: $WARNINGS"
    echo ""
    echo "Please fix the errors above before using the deployment."
    echo "See DEX-DEPLOYMENT-GUIDE.md for troubleshooting steps."
    exit 1
fi
