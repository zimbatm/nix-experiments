#!/usr/bin/env bash
# This scripts represents the installable part that the user invokes.
set -euo pipefail

# FIXME: have nix set this env var instead
FLAKE_ROOT=$(dirname "$0")
export FLAKE_ROOT

nix run .#devenv -- "$@"
