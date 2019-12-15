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
# the dependencies have to be installed in reverse order to get
# the same list
nix-env -f "$nixpkgs" -p ./result-profile -iA vim
nix-env -f "$nixpkgs" -p ./result-profile -iA groff
nix-env -f "$nixpkgs" -p ./result-profile -iA git
nix-env -f "$nixpkgs" -p ./result-profile -iA rclone

log "the manifest should be the same"

# FIXME: indent the toNix output
./toNix.sh ./result-profile/manifest.nix > profile-manifest.nix

# FIXME: the only diff is the outputs ordering for some reason
diff -u profile-manifest.nix ./result/manifest.nix

log "and contains the same list of files"

tree ./result-profile/manifest.nix > result-profile.txt
tree ./result/manifest.nix > result.txt
diff -u result-profile.txt result.txt

# FIXME: make sure both versions are exactly the same
