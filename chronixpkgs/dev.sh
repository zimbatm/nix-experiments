#!/usr/bin/env bash
set -euo pipefail

# Ensure we're in the nix develop shell
if ! command -v hivemind &> /dev/null; then
    echo "Error: hivemind not found. Please run 'nix develop' first."
    exit 1
fi

exec hivemind "$@"
