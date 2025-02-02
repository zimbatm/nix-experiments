#!/usr/bin/env bash
set -euo pipefail

in_nix_store() {
  echo "$1" | grep "^/nix/store/" &>/dev/null
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

USAGE
}

# -----------------

EDITOR=${EDITOR:-vim}
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
if diff --recursive "$store_path" "$work_dir"; then
  echo "ignoring as no changes were detected"
  exit
fi

# Insert the work dir back into the store
# FIXME: re-hydrate the references
#nix-store --query --references "$store_path" | mapfile -t references

nix-store --add "$work_dir/$drv_name"

# TODO: recursively rewrite the system closure


# TODO: nixos-rebuild test

