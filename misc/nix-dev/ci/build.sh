#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# shellcheck disable=SC1091
source ./env.sh

# build and push to the cache
nix-build

if type -P cachix &>/dev/null; then
  {
    echo result
    echo env
  } | cachix push devenv
fi
