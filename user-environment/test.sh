#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

log() {
  echo "# $*" >&2
}

export NIX_PATH=nixpkgs=channel:nixos-19.09

nixpkgs=$(nix eval '(<nixpkgs>)')

log "building the user profile"
nix-build ./my-env.nix

log "the next command should show that 'groff' and 'hello' are installed"
nix-env -p ./result -q

log "now installing the same packages imperatively"
rm -f ./result-profile*
nix-env -f "$nixpkgs" -p ./result-profile -iA groff
nix-env -f "$nixpkgs" -p ./result-profile -iA hello

log "the manifest should be the same"

diff -u ./result-profile/manifest.nix ./result/manifest.nix

log "and contains the same list of files"

tree ./result-profile/manifest.nix > result-profile.txt
tree ./result/manifest.nix > result.txt
diff -u result-profile.txt result.txt