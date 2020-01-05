#!/usr/bin/env bash
#
# Usage: my-env.sh <edit|build|switch>
set -euo pipefail

here=$(dirname "$0")
: "${USER:=$(id -un)}"
profile_path=/nix/var/nix/profiles/per-user/$USER/profile
# TODO: make this configurable through NIX_PATH
my_env=$here/my-env.nix

showUsage() {
  echo "Usage: my-env.sh <edit|build|switch>"
}

# Main #

case "${1:-}" in
help | -h | --help)
  showUsage
  ;;
edit)
  "$EDITOR" "$my_env"
  ;;
build)
  # keep going
  :
  ;;
switch)
  # keep going
  :
  ;;
"")
  echo "missing command: edit|build|switch"
  exit 1
  ;;
*)
  echo "command '$1' unsupported" >&2
  exit 1
  ;;
esac

tmpdir=$(mktemp -d)
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

out=$(nix-build --out-path "$tmpdir/result" "$my_env")

if [[ $1 == switch ]]; then
  nix-env --profile "$profile_path" --set "$out"
fi
