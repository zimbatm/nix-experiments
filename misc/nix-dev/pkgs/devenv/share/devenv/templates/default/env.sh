#!/usr/bin/env bash
# env.sh -- manages developer environments for a project
#
# Usage:
#   ./env.sh <command> <args>... - runs a command within the env
#   ./env.sh --help              - shows this help
#   source ./env.sh              - load the env in the current shell
#
# TODO(zimbatm): docker runtime for people who don't want to install Nix
# TODO(zimbatm): make sure it works with macOS bash 3.x
# TODO(zimbatm): avoid polluting the env with function when sourced

## Constants

GREEN=
RED=
OFF=

# set the colors if the stderr is a tty
if [[ -t 2 ]]; then
  GREEN="\033[32;01m"
  RED="\033[31;01m"
  OFF="\033[0m"
fi

## Functions

# log <args>...
log() {
  echo -e "${GREEN}[env.sh]${OFF} $*" >&2
}

## Commands

# Usage: env-help
#
# Shows the script usage from the header
env-help() {
  local line
  (
    read -r _ # ignore the shebang
    while IFS=$'\n' read -r line; do
      if [[ $line != "#"* ]]; then
        break
      fi
      line=${line###}
      line=${line## }
      echo "$line"
    done
  ) <"${BASH_SOURCE[0]}"
}

# Usage: env-ensure-nix
env-ensure-nix() {
  local nix_profile=${HOME:-/root}/.nix-profile/etc/profile.d/nix.sh

  if type -P nix &>/dev/null; then
    return 0
  fi

  # if user has installed the single-user nix but not set it up properly in
  # their profile
  if [[ -f "$nix_profile" ]]; then
    # Nix 2.x profile fails if `set -e` is active otherwise.
    : "${MANPATH:=}"
    # shellcheck disable=SC1090
    source "$nix_profile"
  fi

  if type -P nix &>/dev/null; then
    return 0
  fi

  return 1
}

# Usage: env-ensure-nix-version <major> <minor> <tiny>
env-ensure-nix-version() {
  local major=$1 minor=$2 tiny=$3
  local current
  current=$(nix --version)
  if ! [[ $current =~ ([0-9])[.]([0-9]+)([.]([0-9]+))? ]]; then
    log "warning: unexpected output of 'nix --version' ($current), devenv might not work properly."
    return 1
  fi
  local current_major=${BASH_REMATCH[1]}
  local current_minor=${BASH_REMATCH[2]}
  local current_tiny=${BASH_REMATCH[4]:-0}
  if [[ $current_major -lt $major ]] ||
    [[ $current_major -eq $major && $current_minor -lt $minor ]] ||
    [[ $current_major -eq $major && $current_minor -eq $minor && $current_tiny -lt $tiny ]]; then
    return 1
  else
    return 0
  fi
}

# Assumes that all the nix files are contained in the same folder
#
# Usage: env-nix-build-hashed <cache-dir> <folder> <attr>
env-nix-build-hashed() {
  local hash outlink cache_dir=$1 folder=$2 attr=$3

  hash=$(nix-hash "$folder")
  if [[ -z "$hash" ]]; then
    log "nix-hash is broken in $folder"
    return 1
  fi

  outlink=$cache_dir/$hash

  if ! [[ -e "$outlink" ]]; then
    log "building $RED$hash$OFF"
    if ! nix build -f "$folder" --out-link "$outlink" "$attr"; then
      return 1
    fi
  fi

  readlink "$outlink"
}

# Usage: env-load <cache_dir> <nix_dir> <attr>
env-load() {
  local out cache_dir=$1 nix_dir=$2 attr=$3

  ## configure nix

  # assume that nixpkgs is pinned
  unset NIX_PATH
  # use our own nix configuration
  if [[ -f $nix_dir/etc/nix/nix.conf ]]; then
    export NIX_CONF_DIR=$nix_dir/etc/nix
  fi

  if ! env-ensure-nix; then
    log "${RED}error:${OFF} nix is a dependency and needs to be installed"
    log "to install, run \`bash <(curl https://nixos.org/nix/install)\`"
    return 1
  fi

  # ensure nix --version >= 2.1.1
  if ! env-ensure-nix-version 2 1 1; then
    log "${RED}error:${OFF} $(nix --version) is too old"
    log "to upgrade, run \`nix upgrade-nix\` or \`bash <(curl https://nixos.org/nix/install)\`"
    return 1
  fi

  ## build the environment
  out=$(env-nix-build-hashed "$cache_dir" "$nix_dir" "$attr")
  if [[ -z "$out" ]]; then
    log "${RED}error:${OFF} failed to build '$attr' in '$nix_dir'"
    return 1
  fi

  ## configure the environment
  export PATH=$out/bin:$PATH
  if [[ -f "$out/etc/profile" ]]; then
    # shellcheck disable=SC1090
    source "$out/etc/profile"
  fi
}

# main
env-main() {
  local cmd top
  top=$(cd "${BASH_SOURCE[0]%/*}" && pwd)
  local nix_dir=$top/nix
  local cache_dir=$top/var/env

  # if there is a command to run
  if [[ $# -gt 0 ]]; then
    cmd=$1
    shift
  else
    cmd=''
  fi

  case "$cmd" in
  -h | --help)
    env-help
    return 0
    ;;
  "")
    log "no command given, use --help for more info"
    return 1
    ;;
  esac

  # avoid loading the environment twice
  if [[ -z "${DEVENV_ROOT:-}" ]]; then
    export DEVENV_ROOT=$top
    if ! env-load "$cache_dir" "$nix_dir" profile; then
      return 1
    fi
  else
    if [[ "$DEVENV_ROOT" != "$top" ]]; then
      log "error: already loaded $DEVENV_ROOT != $top"
      return 1
    fi
  fi

  # if the file was sourced
  if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
    return
  fi

  exec "$cmd" "$@"
}

## Main

# be strict if the file is not sourced
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  set -euo pipefail
fi

env-main "$@" || exit $?
