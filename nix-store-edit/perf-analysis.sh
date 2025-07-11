#!/usr/bin/env bash
set -euo pipefail

echo "=== Building test derivation ==="
TEST_PATH=$(nix-build internal/integration/fixtures/minimal.nix --no-out-link)
echo "Test path: $TEST_PATH"

# Create sed script
cat > edit.sed << 'EOF'
s/original content/modified content/
EOF

echo -e "\n=== Running with perf stat to get system-level stats ==="
sudo perf stat -e cycles,instructions,cache-misses,context-switches,cpu-migrations,page-faults \
    ./nix-store-edit \
    --editor="sed -i -f edit.sed" \
    --dry-run \
    "$TEST_PATH" 2>&1 | tee perf-stat.txt

echo -e "\n=== Running with strace to analyze system calls ==="
strace -c -o strace-summary.txt \
    ./nix-store-edit \
    --editor="sed -i -f edit.sed" \
    --dry-run \
    "$TEST_PATH" 2>/dev/null || true

echo -e "\n=== System call summary ==="
cat strace-summary.txt

echo -e "\n=== Counting nix-store executions ==="
strace -e trace=execve -o strace-exec.txt \
    ./nix-store-edit \
    --editor="sed -i -f edit.sed" \
    --dry-run \
    "$TEST_PATH" 2>/dev/null || true

echo "Number of nix-store --query --references calls:"
grep -c "nix-store.*--query.*--references" strace-exec.txt || echo "0"

echo -e "\n=== Clean up ==="
rm -f edit.sed strace-*.txt perf-stat.txt