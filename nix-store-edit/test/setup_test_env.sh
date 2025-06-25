#!/usr/bin/env bash
# Setup script for creating an isolated test environment for nix-patch
# This creates a custom store directory that doesn't interfere with system Nix

set -euo pipefail

# Configuration
TEST_ROOT="${TEST_ROOT:-/tmp/nix-patch-test}"
STORE_DIR="$TEST_ROOT/nix/store"
STATE_DIR="$TEST_ROOT/nix/var/nix"
PROFILE_DIR="$TEST_ROOT/nix/var/nix/profiles"
LOG_DIR="$TEST_ROOT/nix/var/log/nix"
TEST_PROFILE="$PROFILE_DIR/test-profile"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Create test environment
setup_environment() {
    info "Setting up test environment in $TEST_ROOT"
    
    # Create directory structure
    mkdir -p "$STORE_DIR"
    mkdir -p "$STATE_DIR"
    mkdir -p "$PROFILE_DIR"
    mkdir -p "$LOG_DIR"
    
    # Create some mock store items for testing
    create_mock_store_items
    
    # Create a test profile
    if [[ -n "${FIRST_ITEM:-}" ]]; then
        ln -sfn "$FIRST_ITEM" "$TEST_PROFILE"
        info "Created test profile: $TEST_PROFILE -> $FIRST_ITEM"
    fi
    
    # Generate environment setup script
    cat > "$TEST_ROOT/env.sh" <<EOF
# Source this file to set up the test environment
export TEST_STORE_ROOT="$TEST_ROOT"
export TEST_PROFILE="$TEST_PROFILE"

echo "Test environment configured:"
echo "  Store Root: \$TEST_STORE_ROOT"
echo "  Store: $STORE_DIR"
echo "  Profile: \$TEST_PROFILE"
echo ""
echo "Run nix-patch with:"
echo "  nix-patch --store \$TEST_STORE_ROOT --system=profile --profile=\$TEST_PROFILE <store-path>"
EOF
    
    info "Environment setup complete!"
    info "To use this environment:"
    info "  source $TEST_ROOT/env.sh"
}

# Create mock store items for testing
create_mock_store_items() {
    info "Creating mock store items..."
    
    # Create a simple text configuration package
    local config_hash=$(echo -n "config-package" | sha256sum | cut -c1-32)
    local config_path="$STORE_DIR/${config_hash}-config-1.0"
    mkdir -p "$config_path/etc"
    cat > "$config_path/etc/app.conf" <<EOF
# Application Configuration
server_host=localhost
server_port=8080
database_path=/var/lib/app/db.sqlite
log_level=info
EOF
    FIRST_ITEM="$config_path"
    info "Created config package: $config_path"
    
    # Create a shell script package
    local script_hash=$(echo -n "scripts-package" | sha256sum | cut -c1-32)
    local script_path="$STORE_DIR/${script_hash}-scripts-1.0"
    mkdir -p "$script_path/bin"
    cat > "$script_path/bin/hello.sh" <<'EOF'
#!/usr/bin/env bash
echo "Hello from the Nix store!"
echo "Config location: /nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-config-1.0/etc/app.conf"
EOF
    chmod +x "$script_path/bin/hello.sh"
    
    # Update the script to reference the actual config path
    sed -i "s|/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-config-1.0|$config_path|g" \
        "$script_path/bin/hello.sh"
    info "Created scripts package: $script_path"
    
    # Create a package with dependencies
    local app_hash=$(echo -n "app-package" | sha256sum | cut -c1-32)
    local app_path="$STORE_DIR/${app_hash}-myapp-2.0"
    mkdir -p "$app_path/bin"
    mkdir -p "$app_path/share/myapp"
    
    cat > "$app_path/bin/myapp" <<EOF
#!/usr/bin/env bash
# This app depends on both config and scripts packages
CONFIG_FILE="$config_path/etc/app.conf"
HELLO_SCRIPT="$script_path/bin/hello.sh"

echo "MyApp v2.0"
echo "Loading config from: \$CONFIG_FILE"
source "\$CONFIG_FILE" 2>/dev/null || echo "Config not found"
echo "Running hello script..."
"\$HELLO_SCRIPT"
EOF
    chmod +x "$app_path/bin/myapp"
    
    # Create a reference graph file
    cat > "$app_path/share/myapp/references" <<EOF
$config_path
$script_path
EOF
    info "Created app package: $app_path"
    
    # Create a complex package with many file types
    local complex_hash=$(echo -n "complex-package" | sha256sum | cut -c1-32)
    local complex_path="$STORE_DIR/${complex_hash}-complex-1.0"
    mkdir -p "$complex_path"/{bin,etc,lib,share/doc}
    
    # Binary file (mock ELF)
    printf '\x7fELF' > "$complex_path/bin/binary"
    echo "$config_path/lib/libconfig.so" >> "$complex_path/bin/binary"
    chmod +x "$complex_path/bin/binary"
    
    # Symlink
    ln -s "../etc/config.ini" "$complex_path/bin/config-link"
    
    # Various config files
    echo "[settings]" > "$complex_path/etc/config.ini"
    echo "path=$config_path" >> "$complex_path/etc/config.ini"
    
    # Documentation
    cat > "$complex_path/share/doc/README.md" <<EOF
# Complex Package

This package demonstrates various file types:
- Binary files with embedded store paths
- Symbolic links
- Configuration files
- Documentation

References: $config_path
EOF
    info "Created complex package: $complex_path"
    
    # Create mock NAR files (simplified)
    for item in "$config_path" "$script_path" "$app_path" "$complex_path"; do
        create_mock_nar "$item"
    done
}

# Create a mock NAR file for a store item
create_mock_nar() {
    local store_path="$1"
    local nar_file="${store_path}.nar.gz"
    
    # Create a simple compressed tar as mock NAR
    (cd "$store_path" && tar czf "$nar_file" .) 2>/dev/null || true
}

# Clean up test environment
cleanup_environment() {
    warn "Cleaning up test environment..."
    
    if [[ -d "$TEST_ROOT" ]]; then
        # Safety check - make sure we're only deleting test directories
        if [[ "$TEST_ROOT" == "/tmp/nix-patch-test"* ]]; then
            rm -rf "$TEST_ROOT"
            info "Test environment cleaned up"
        else
            error "Refusing to clean up $TEST_ROOT - not a test directory"
        fi
    else
        info "Test environment does not exist"
    fi
}

# List store items in test environment
list_store_items() {
    info "Store items in test environment:"
    if [[ -d "$STORE_DIR" ]]; then
        ls -la "$STORE_DIR" | grep -E '^d' | awk '{print "  " $9}'
    else
        warn "Store directory does not exist"
    fi
}

# Show usage
usage() {
    cat <<EOF
Usage: $0 [command]

Commands:
  setup    - Create test environment (default)
  cleanup  - Remove test environment
  list     - List store items
  help     - Show this help

Environment variables:
  TEST_ROOT - Base directory for test environment (default: /tmp/nix-patch-test)

Example workflow:
  1. $0 setup                    # Create test environment
  2. source $TEST_ROOT/env.sh    # Configure environment
  3. nix-patch --store \$TEST_STORE_ROOT --system=profile --profile=\$TEST_PROFILE <store-item>/etc/app.conf
  4. $0 cleanup                  # Clean up when done
EOF
}

# Main script logic
main() {
    local command="${1:-setup}"
    
    case "$command" in
        setup)
            setup_environment
            ;;
        cleanup)
            cleanup_environment
            ;;
        list)
            list_store_items
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            error "Unknown command: $command"
            ;;
    esac
}

# Run main function
main "$@"