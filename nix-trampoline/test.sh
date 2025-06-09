#!/usr/bin/env bash
set -euo pipefail

echo "🚀 Testing nix-trampoline prototype..."

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Function to run test steps
run_test() {
    local cmd="$1"
    local desc="$2"
    
    log "$desc"
    if $cmd; then
        success "$desc"
    else
        error "$desc failed"
        return 1
    fi
}

# Check and start minikube if needed
check_minikube() {
    log "Checking minikube status..."
    
    if ! minikube status > /dev/null 2>&1; then
        warn "Minikube not running, starting it..."
        log "This may take a few minutes on first run..."
        
        if minikube start --memory=16384 --cpus=8 --driver=docker; then
            success "Minikube started successfully"
        else
            error "Failed to start minikube"
            return 1
        fi
    else
        success "Minikube already running"
    fi
    
    # Ensure kubectl context is correct
    kubectl config use-context minikube > /dev/null 2>&1 || true
}

echo ""
check_minikube

echo ""
log "Step 1: Setting up pod..."
run_test "./nix-trampoline.sh up" "Pod creation"

echo ""
log "Step 2: Pushing repository..."
run_test "./nix-trampoline.sh push-tarball" "Git archive push"

echo ""
log "Step 3: Testing nix build..."
run_test "./nix-trampoline.sh build -f hello.nix" "Building hello derivation"

echo ""
log "Step 4: Testing shell access..."
pod_name=$(kubectl get pods -l app=nix-trampoline -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$pod_name" ] && kubectl exec "$pod_name" -c user -- nix --version > /dev/null 2>&1; then
    success "Shell access works"
else
    error "Shell access failed"
fi

echo ""
log "Step 5: Testing built program..."
if [ -n "$pod_name" ] && kubectl exec "$pod_name" -c user -- ./result/bin/hello-trampoline > /dev/null 2>&1; then
    success "Built program execution works"
else
    error "Built program execution failed"
fi

echo ""
success "All tests completed! 🎉"
echo ""
echo "💡 Quick commands:"
echo "  ./test.sh                    - Run this test suite"
echo "  ./nix-trampoline.sh shell    - Open interactive shell"
echo "  ./nix-trampoline.sh down     - Scale down (keep data)"
echo "  ./nix-trampoline.sh clean    - Clean up everything"
