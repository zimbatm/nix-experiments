#!/usr/bin/env bash
set -euo pipefail

echo "=== Building test derivation with dependencies ==="
TEST_PATH=$(nix-build test-rewrite.nix --no-out-link)
echo "Test path: $TEST_PATH"

# Create a sed script that will make a change
cat > edit.sed << 'EOF'
s/main content/modified main content/
EOF

echo -e "\n=== Running nix-store-edit with CPU profiling ==="
echo "Using sed as editor to avoid interactive mode"
time ./nix-store-edit \
    --cpuprofile=cpu.prof \
    --memprofile=mem.prof \
    --editor="sed -i -f edit.sed" \
    --dry-run \
    --verbose \
    "$TEST_PATH"

echo -e "\n=== CPU Profile Analysis ==="
echo "Top 30 CPU consumers:"
go tool pprof -top30 cpu.prof

echo -e "\n=== Memory Profile Analysis ==="
echo "Top 30 memory allocations:"
go tool pprof -top30 -alloc_space mem.prof

echo -e "\n=== Generating detailed profiles ==="
go tool pprof -text cpu.prof > cpu_profile.txt
go tool pprof -text -alloc_space mem.prof > mem_profile.txt

echo -e "\n=== Clean up ==="
rm -f edit.sed

echo -e "\n=== Profile files created: ==="
ls -la *.prof *.txt 2>/dev/null || true