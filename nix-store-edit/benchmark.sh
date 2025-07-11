#!/usr/bin/env bash
set -euo pipefail

echo "=== Performance Benchmark ==="

# Find a test binary
TEST_PATH=$(which ls)
echo "Test path: $TEST_PATH"

# Create sed script
cat > edit.sed << 'EOF'
1s/^/# Modified\n/
EOF

echo -e "\n=== Running optimized version ==="
time ./nix-store-edit \
    --cpuprofile=cpu-optimized.prof \
    --editor="sed -i -f edit.sed" \
    --dry-run \
    --verbose \
    "$TEST_PATH" 2>&1 | tee optimized-output.txt

echo -e "\n=== Performance Statistics ==="
grep -E "(Step . completed in|Building dependency graph|Batch query|Paths to process|Getting closure|Found .* paths)" optimized-output.txt

echo -e "\n=== CPU Profile Analysis ==="
go tool pprof -top20 cpu-optimized.prof

# Cleanup
rm -f edit.sed optimized-output.txt