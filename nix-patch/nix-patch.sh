#!/usr/bin/env bash
# shellcheck disable=SC2155,SC2207,SC2317
set -euo pipefail

shopt -u nullglob

## Globals

declare -A rewrites
EDITOR=${EDITOR:-vim}

## Functions

nix_store_hash() {
  echo "$1" | cut -d'/' -f4 | cut -d'-' -f 1
}

in_nix_store() {
  echo "$1" | grep "^/nix/store/" &>/dev/null
}

log() {
  echo "$*" >&2
}

fail() {
  echo "error: $*"
  exit 1
}

at_exit() {
  set -e
  if [[ -n "$work_dir" && -e "$work_dir" ]]; then
    rm -rf "$work_dir"
  fi
}

show_usage() {
  cat <<USAGE
nixos-patch <path>

Opens the path in a mutable buffer in your editor. When editing is finished,
add the new content to the store, and rewrite your system closure recursively
with it.

Set \$EDITOR to change the editor.

USAGE
}

nix_number() {
  local b n=$1
  for _ in $(seq 8); do
    b=$(printf "%02x" $(( n % 256 )))
    n=$(( n / 256 ))
    # shellcheck disable=SC2059
    printf "\x$b"
  done
}

nix_string() {
  local str="$1"
  nix_number ${#str}
  printf '%s' "$str"
  for _ in $(seq 1 $(( 8 - ( ( (${#str} - 1) % 8 ) + 1 ) ))); do
    printf '\0'
  done
}

nixe() {
  local old_path=$1
  local path_to_add=${2:-1}
  local refs=()
  local sed_args=()
  # We're generating a random ID as the derivation "hash"
  local new_hash=$(nix hash convert --hash-algo sha1 --to nix32 "$(head -c20 /dev/urandom | xxd -p)")
  local drv_name=$(echo "$path" | cut -d'/' -f4 | cut -d'-' -f 2-)
  local new_path=/nix/store/${new_hash}-${drv_name}
  # The deriver doesn't make sense for us
  # local deriver=$(nix-store --query --deriver "$path")
  local deriver=""

  # Record the new path reference
  rewrites[$old_path]=$new_path

  # Replace refs with mapping
  for ref in $(nix-store --query --references "${old_path}"); do
    refs+=("${rewrites[$ref]:-$ref}")
  done

  for ref in "${!rewrites[@]}"; do
    sed_args+=(
      -e
      "s|$(nix_store_hash "$ref")|$(nix_store_hash "${rewrites[$ref]}")|g"
    )
  done

  if [[ ${#sed_args} -eq 0 ]]; then
    sed_args=(cat)
  else
    sed_args=(sed "${sed_args[@]}")
  fi

  {
    # Number of NAR files to add
    nix_number 1
    nix-store --dump "$path_to_add"
    nix_number $((0x4558494e))
    nix_string "$new_path"
    nix_number "${#refs[@]}"
    for ref in "${refs[@]}"; do
      nix_string "$ref"
    done
    nix_string "$deriver"
    nix_number 0
    nix_number 0
  } | "${sed_args[@]}"
}

declare -A parents  # Stores child -> parent relationships

# First pass: Build dependency graph from Root
build_dependency_graph() {
  local root="$1"
  local path
  declare -A visited
  queue=("$root")

  while [[ ${#queue[@]} -gt 0 ]]; do
    path="${queue[0]}"
    log "path=$path"
    queue=("${queue[@]:1}")  # Dequeue

    # Skip if already visited
    if [[ -n "${visited[$path]:-}" ]]; then
      continue
    fi
    visited["$path"]=1

    # Get references (dependencies)
    for ref in $(nix-store --query --references "$path"); do
      parents["$ref"]="$path"  # Child -> Parent mapping
      queue+=("$ref")  # Enqueue references
    done
  done
}

# Second pass: Find the path from B to Root
find_path_to_root() {
  local target="$1"
  path_chain=()

  while [[ -n "$target" ]]; do
    path_chain+=("$target")
    target="${parents[$target]}"  # Move to parent
  done

  log "Path from Root to B found:"
  printf '%s\n' "${path_chain[@]}" | tac  # Print in correct order
}

# Third pass: Rewrite paths from Root to B
rewrite_paths() {
  local -a chain=("$@")
  declare -A rewritten_paths

  for path in "${chain[@]}"; do
    if [[ -n "${rewritten_paths[$path]}" ]]; then
      continue
    fi

    # Rewrite path (modify as needed)
    new_path=$(nixe "$store_path" "$work_dir/$drv_name" | nix-store --import)
    rewritten_paths["$path"]="$new_path"

    log "Rewritten: $path -> $new_path"
  done
}


# -----------------

system_closure=$(readlink /run/current-system)
path=
work_dir=

while [[ $# -gt 0 ]]; do
  opt=$1
  shift

  case "$opt" in 
    -h | --help)
      show_usage
      exit
      ;;
    -*)
      fail "unknown option $opt, --help for usage."
      ;;
    *)
      if [[ -n $path ]]; then
        fail "you can pass only one path"
      fi
      path=$opt
  esac
done

if [[ -z "$path" ]]; then
  fail "ERROR: <path> missing. --help for usage."
fi

# Check that the given path is in the /nix/store
if ! in_nix_store "$path"; then
  path=$(readlink "$path")
  if ! in_nix_store "$path"; then
    fail "$path is not in the /nix/store"
  fi
fi

# Check that the given path in part of the system closure
nix why-depends "$system_closure" "$path"

# Split the path into a store_path and a file_path
store_path=$(echo "$path" | cut -d'/' -f1-4)
file_path=$(echo "$path" | cut -d'/' -f5-)
drv_name=$(echo "$path" | cut -d'/' -f4 | cut -d'-' -f 2-)

# Create workspace for editing the store path
trap at_exit EXIT
work_dir=$(mktemp -d)
cp -r "$store_path" "$work_dir/$drv_name"
chmod -R "a+w" "$work_dir"

if [[ -z $file_path ]]; then
  edit_path=$work_dir/$drv_name
else
  edit_path=$work_dir/$drv_name/$file_path
fi

# Open the file into the editor
"$EDITOR" "$edit_path"

# Compare the work dir with the old one
if diff --recursive "$store_path" "$work_dir/$drv_name"; then
  log "ignoring as no changes were detected"
  exit
fi

# Insert the work dir back into the store
# new_path=$(nixe "$store_path" "$work_dir/$drv_name" | nix-store --import)
# log "new_path=$new_path"

# TODO: Recursively rewrite the system closure
# First pass: Build dependency graph from Root
build_dependency_graph "$system_closure"

path_chain=($(find_path_to_root "$store_path"))

log "store_path=$store_path"
log "path_chain=${path_chain[*]}"

exit 1
# rewrite_paths "${path_chain[@]}"

# new_system_closure=${rewrites[$system_closure]}

# log "new_system_closure=$new_system_closure"

# TODO: nixos-rebuild test

