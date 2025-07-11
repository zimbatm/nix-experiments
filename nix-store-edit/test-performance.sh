#!/usr/bin/env bash
set -euo pipefail

echo "=== Finding a real package in the user profile ==="
# Find vim or nano in the profile
VIM_PATH=$(find /nix/store -maxdepth 1 -name "*-vim-*" -o -name "*-nano-*" 2>/dev/null | head -1 || true)
if [ -z "$VIM_PATH" ]; then
    echo "No vim/nano found, using coreutils instead"
    COREUTILS_PATH=$(find /nix/store -maxdepth 1 -name "*-coreutils-*" 2>/dev/null | head -1 || true)
    if [ -z "$COREUTILS_PATH" ]; then
        echo "No suitable test package found"
        exit 1
    fi
    TEST_PATH="$COREUTILS_PATH/bin/ls"
else
    TEST_PATH="$VIM_PATH/bin/vim"
fi

echo "Test path: $TEST_PATH"

# Create sed script
cat > edit.sed << 'EOF'
1s/^/#modified by nix-store-edit\n/
EOF

echo -e "\n=== Running ORIGINAL version for comparison ==="
# Save current binary
cp nix-store-edit nix-store-edit-optimized

# Checkout original version without optimizations
git checkout HEAD~3 -- internal/rewrite/rewrite.go internal/rewrite/cache.go internal/store/store_instance.go
go build -o nix-store-edit

echo "Running original version..."
time ./nix-store-edit \
    --editor="sed -i -f edit.sed" \
    --dry-run \
    "$TEST_PATH" 2>&1 | grep -E "(Step . completed in|Total nix-store queries|Unique paths)" || true

echo -e "\n=== Running OPTIMIZED version ==="
# Use optimized version
mv nix-store-edit-optimized nix-store-edit

echo "Running optimized version..."
time ./nix-store-edit \
    --editor="sed -i -f edit.sed" \
    --dry-run \
    "$TEST_PATH" 2>&1 | grep -E "(Step . completed in|Building dependency graph|Batch query|Paths to process)" || true

# Cleanup
rm -f edit.sed
git checkout -- internal/rewrite/rewrite.go internal/rewrite/cache.go internal/store/store_instance.go