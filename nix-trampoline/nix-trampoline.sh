#!/usr/bin/env bash
set -euo pipefail

PRJ_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRJ_DATA_DIR="${PRJ_ROOT}/.data"
DEPLOYMENT_NAME="nix-trampoline"

# =============================================================================
# UTILITY FUNCTIONS
# =============================================================================

usage() {
    echo "Usage: $0 [COMMAND] [ARGS...]"
    echo ""
    echo "Commands:"
    echo "  up        Create and start the nix pod (auto-detects SSH key)"
    echo "  push-tarball Export git archive and push to pod"
    echo "  push-flake Push nix flake archive to pod"
    echo "  build     Run nix build in the pod"
    echo "  build-local Build locally using pod as remote builder"
    echo "  shell     Open shell in the pod"
    echo "  ssh       Get SSH connection info for pod"
    echo "  copy-closure <derivation> Copy Nix closure to pod via SSH"
    echo "  down      Scale deployment to 0 replicas (preserves PVC)"
    echo "  clean     Delete all resources including PVC"
    echo ""
    echo "Examples:"
    echo "  $0 up"
    echo "  $0 push-tarball"
    echo "  $0 push-flake"
    echo "  $0 build"
    echo "  $0 build-local"
    echo "  $0 shell"
    echo "  $0 ssh"
    echo "  $0 copy-closure /nix/store/abc123-hello"
    echo "  $0 down      # Scale down, keep data"
    echo "  $0 clean     # Delete everything"
}

get_pod_name() {
    local pod_name
    pod_name=$(kubectl get pods -l app=nix-trampoline -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    if [[ -z "$pod_name" ]]; then
        echo "ERROR: No running pod found" >&2
        return 1
    fi
    echo "$pod_name"
}

wait_for_pod() {
    echo "Waiting for deployment to be ready..."
    
    # Use kubectl's built-in rollout status - this waits for all containers to be ready
    if ! kubectl rollout status deployment/$DEPLOYMENT_NAME --timeout=300s; then
        echo "ERROR: Deployment failed to become ready"
        return 1
    fi
    
    # Get the actual pod name
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    echo "Pod name: $current_pod_name"
    kubectl get pod "$current_pod_name" -o wide
    
    # Since rollout status succeeded, readiness probes have passed:
    # - nix-daemon: nix store ping succeeded  
    # - user: SSH daemon on port 2222 is ready
    echo "All containers are ready!"
}

setup_ssh_key() {
    # Create data directory if it doesn't exist
    mkdir -p "$PRJ_DATA_DIR"
    
    local private_key="$PRJ_DATA_DIR/nix-trampoline-key"
    local public_key="$PRJ_DATA_DIR/nix-trampoline-key.pub"
    
    # Generate dedicated key pair if it doesn't exist
    if [[ ! -f "$private_key" ]]; then
        echo "Generating dedicated SSH key pair for nix-trampoline..."
        ssh-keygen -t ed25519 -f "$private_key" -N "" -C "nix-trampoline@$(hostname)"
    fi

    # Make sure the private key is private
    chmod 600 "$private_key"
    
    # Always copy the public key to ensure it's up to date
    cp "$public_key" container/authorized_keys
    echo "SSH key copied to authorized_keys file"
}

# Globals
port_forward_output=
port_forward_pid=

# Port forwarding utilities
setup_port_forward() {
    local pod_name="$1"
    local port="$2"
    
    # Use dynamic port allocation to avoid conflicts
    port_forward_output=$(mktemp)
    ( exec 2>&1; kubectl port-forward "$pod_name" ":$port" | tee "$port_forward_output" ) 1>&2 &
    port_forward_pid=$!
    
    # Set up trap to clean things
    trap teardown_port_forward EXIT INT TERM
    
    # Wait for port forward to be ready and extract the allocated port
    local retry_count=0
    local local_port=""
    while [[ $retry_count -lt 15 ]]; do
        if [[ -s "$port_forward_output" ]]; then
            local_port=$(grep -o "Forwarding from 127.0.0.1:[0-9]*" "$port_forward_output" | grep -o "[0-9]*$" | head -1)
            if [[ -n "$local_port" ]]; then
                break
                # Test connectivity using bash builtin /dev/tcp
                if exec 3<>"/dev/tcp/localhost/$local_port" 2>/dev/null; then
                    exec 3>&-  # Close the connection
                    break
                fi
            fi
        fi
        sleep 1
        retry_count=$((retry_count + 1))
    done

    if [[ $retry_count -eq 15 || -z "$local_port" ]]; then
        echo "ERROR: Port forward failed to start"
        cat "$port_forward_output"
        teardown_port_forward
        return 1
    fi
    
    echo "$local_port"
}

teardown_port_forward() {
    if [[ -n "$port_forward_pid" ]]; then
        kill "$port_forward_pid"
        port_forward_pid=
    fi
    if [[ -n "$port_forward_output" && -e "$port_forward_output" ]]; then
        rm -f "$port_forward_output"
        port_forward_output=
    fi

    trap - EXIT INT TERM
}

check_trusted_user() {
    if [[ "$(nix store info --json | jq -r '.trusted')" == "1" ]]; then
        return 0
    fi

    local current_user
    current_user=$(id -u)
    echo "ERROR: User '$current_user' is not a trusted Nix user"
    echo "To enable remote building, add your user to trusted-users in /etc/nix/nix.conf:"
    echo "  trusted-users = root $current_user"
    echo "Then restart the nix-daemon service"
    return 1
}

get_ssh_opts() {
    local port="$1"
    local private_key="$PRJ_DATA_DIR/nix-trampoline-key"
    echo "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o IdentityFile=$private_key -p $port"
}

get_ssh_ng_url() {
    local port="$1"
    local private_key="$PRJ_DATA_DIR/nix-trampoline-key"
    echo "ssh-ng://user@localhost?ssh-key=$private_key"
}


# =============================================================================
# PUBLIC COMMANDS
# =============================================================================

# Idempotent setup command
up() {
    setup_ssh_key
    
    echo "Applying nix trampoline deployment configuration..."
    kubectl apply -k container/
    wait_for_pod
    echo "Deployment is ready!"
}

push_tarball() {
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    echo "Exporting git archive..."
    git archive --format=tar.gz HEAD > /tmp/repo.tar.gz
    
    echo "Pushing archive to pod..."
    kubectl cp /tmp/repo.tar.gz "$current_pod_name":/workspace/repo.tar.gz -c user
    
    echo "Extracting archive in pod..."
    kubectl exec "$current_pod_name" -c user -- tar -xzf /workspace/repo.tar.gz -C /workspace
    
    echo "Cleaning up local archive..."
    rm /tmp/repo.tar.gz
    
    echo "Repository pushed to pod!"
}

push_flake() {
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    echo "Setting up SSH port forward for flake transfer..."
    local ssh_local_port
    ssh_local_port=$(setup_port_forward "$current_pod_name" 2222)
    
    echo "Creating flake archive directly on pod..."
    NIX_SSHOPTS="$(get_ssh_opts "$ssh_local_port")" nix flake archive --to "$(get_ssh_ng_url "$ssh_local_port")"
    
    teardown_port_forward
    echo "Flake archive pushed and loaded into pod!"
}

build() {
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    echo "Running nix build in pod..."
    kubectl exec "$current_pod_name" -c user -- nix build "$@"
}

build_local() {
    check_trusted_user
    
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    echo "Detecting remote architecture..."
    local remote_system
    remote_system=$(kubectl exec "$current_pod_name" -c user -- nix eval --impure --expr 'builtins.currentSystem')
    # Remove quotes from the result
    remote_system=${remote_system//\"/}
    echo "Remote system: $remote_system"
    
    echo "Setting up SSH port forward for remote building..."
    local ssh_local_port
    ssh_local_port=$(setup_port_forward "$current_pod_name" 2222)
    
    echo "Building locally using pod as remote builder..."
    echo "ERROR: currently blocked on https://github.com/NixOS/nix/pull/3425"
    false
    NIX_SSHOPTS="$(get_ssh_opts "$ssh_local_port")" \
    nix build --system "$remote_system" --max-jobs 0 --builders "$(get_ssh_ng_url "$ssh_local_port") $remote_system" "$@"
    
    teardown_port_forward
    echo "Local build completed using remote builder!"
}

shell() {
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    echo "Opening shell in pod..."
    kubectl exec -it "$current_pod_name" -c user -- sh
}

ssh() {
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    echo "Setting up SSH port forward..."
    local ssh_local_port
    ssh_local_port=$(setup_port_forward "$current_pod_name" 2222)

    echo "Connecting to pod via SSH..."
    # Connect directly with SSH client - cleanup happens via trap when this exits
    command ssh $(get_ssh_opts "$ssh_local_port") user@localhost
}

copy_closure() {
    local current_pod_name
    current_pod_name=$(get_pod_name)
    
    local derivation="$1"
    if [[ -z "$derivation" ]]; then
        echo "Usage: copy_closure <derivation>"
        echo "Example: copy_closure /nix/store/abc123-hello"
        return 1
    fi
    
    echo "Setting up port forward for nix-copy-closure..."
    local ssh_local_port
    ssh_local_port=$(setup_port_forward "$current_pod_name" 2222)
    
    echo "Copying closure to pod via SSH..."
    NIX_SSHOPTS="$(get_ssh_opts "$ssh_local_port")" nix copy --to "$(get_ssh_ng_url "$ssh_local_port")" "$derivation"
    
    # Clean up port forward and remove trap
    teardown_port_forward
    echo "Closure copied successfully!"
}

down() {
    echo "Scaling down nix trampoline deployment..."
    kubectl scale deployment "$DEPLOYMENT_NAME" --replicas=0
    echo "Deployment scaled down to 0 replicas"
}

clean() {
    echo "Deleting nix trampoline resources..."
    kubectl delete -k container/ --ignore-not-found=true
    echo "All resources deleted!"
}

# =============================================================================
# MAIN
# =============================================================================

case "${1:-}" in
    up|setup)
        up
        ;;
    push-tarball)
        push_tarball
        ;;
    push-flake)
        push_flake
        ;;
    build)
        shift
        build "$@"
        ;;
    build-local)
        shift
        build_local "$@"
        ;;
    shell)
        shell
        ;;
    ssh)
        ssh
        ;;
    copy-closure)
        shift
        copy_closure "$@"
        ;;
    down)
        down
        ;;
    clean)
        clean
        ;;
    *)
        usage
        exit 1
        ;;
esac
