#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${FLAKE_ROOT:-}" ]]; then
  echo "Please set the FLAKE_ROOT env var" >&2
  exit 1
fi

# Default to running bash if no command was passed
if [[ $# == 0 ]]; then
  exec -- "${SHELL:-bash}"
fi

# Path that points to the buildEnv root
export DEVENV_ROOT=@out@

# Load the env vars
# shellcheck disable=SC1091
source "$DEVENV_ROOT/profile.sh"

exec "$@"
