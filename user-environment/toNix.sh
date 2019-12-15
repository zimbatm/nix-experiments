#!/usr/bin/env bash
set -euo pipefail

expr=$(
cat <<'EXPR'
let
  toNix = (import ../nix-stdlib).toNix;
  file = import <file>;
in
  toNix file
EXPR
)

nix eval "($expr)" -I "file=$1" --raw
