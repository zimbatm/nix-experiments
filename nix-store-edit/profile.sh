#!/usr/bin/env bash
set -euo pipefail

echo "=== Building test derivation ==="
TEST_PATH=$(nix-build internal/integration/fixtures/minimal.nix --no-out-link)
echo "Test path: $TEST_PATH"

echo -e "\n=== Running nix-store-edit with CPU profiling (dry-run) ==="
./nix-store-edit --cpuprofile=cpu.prof --memprofile=mem.prof --dry-run --verbose "$TEST_PATH" <<< ""

echo -e "\n=== CPU Profile Analysis ==="
echo "Top 20 CPU consumers:"
go tool pprof -top20 cpu.prof

echo -e "\n=== Memory Profile Analysis ==="
echo "Top 20 memory allocations:"
go tool pprof -top20 -alloc_space mem.prof

echo -e "\n=== Generating flamegraph (if go-torch is available) ==="
if command -v go-torch &> /dev/null; then
    go-torch -b cpu.prof -f cpu-flame.svg
    echo "CPU flamegraph saved to cpu-flame.svg"
else
    echo "Install go-torch for flamegraph visualization:"
    echo "  go get -u github.com/uber/go-torch"
fi

echo -e "\n=== Profile files created: ==="
ls -la *.prof