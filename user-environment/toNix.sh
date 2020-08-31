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

nix-instantiate --eval -I "file=$1" --expr "($expr)"
