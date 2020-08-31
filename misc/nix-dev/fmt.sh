#!/usr/bin/env bash
#
# Usage: ./fmt.sh [DIR]
set -euo pipefail

dir=${1:-.}

echo "$ shfmt"
./bin/shfmt-all "$dir"

echo "$ go fmt"
go fmt ./...
