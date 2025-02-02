#! /usr/bin/env bash
# Copyright edef
set -euo pipefail

nix_number() {
    n=$1
    for i in $(seq 1 8); do
        b=$(printf "%02x" $(( n % 256 )))
        n=$(( n / 256 ))
        printf "\x$b"
    done
}

nix_string() {
    str="$1"
    nix_number ${#str}
    printf '%s' "$str"
    for _ in $(seq 1 $(( 8 - ( ( (${#str} - 1) % 8 ) + 1 ) ))); do
        printf '\0'
    done
}

path=/nix/store/ykfgyfbfbxixrx03m6mzwnbklngg5wk6-depot-3p-sources.txt
refs=(
  $(nix-store --query --references "$path")
)
deriver=$(nix-store --query --deriver "$path")

nix_number 1
nix-store --dump $path
nix_number $((0x4558494e))
nix_string "$path"
nix_number ${#refs[@]}
for ref in "${refs[@]}"; do
    nix_string "$ref"
done
nix_string "$deriver"
nix_number 0
nix_number 0
