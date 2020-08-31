#!/usr/bin/env bash
# env.sh -- project environment
#
# Invoke or source this file to load the project's development environment
#
# Usage:
#   ./env.sh [--] <command> <args>... - runs a command within the env
#   ./env.sh --help                   - shows this help
#   source ./env.sh                   - load the env in the current shell

env-root() {
  nix-instantiate --strict --json --eval --expr '
builtins.fetchTarball {
  url = "https://gitlab.com/zimbatm/env.sh/-/archive/3ad91fa395068dcf172e83b6c5af352cd325399d/env.sh-3ad91fa395068dcf172e83b6c5af352cd325399d.tar.gz";
  sha256 = "1jc3ndqafn9ws7fdzvccisbpv513hdq5mz58arndjakv6khg34x5";
}
  ' | xargs
}

# Usage: env-log <msg>...
env-log() {
  # if stderr is a tty
  if [[ -t 2 ]]; then
    echo -e "\033[32;01m[env]\033[0m: $*" >&2
  else
    echo "[env]: $*" >&2
  fi
}

# Usage: env-ensure-nix <major> <minor> <tiny>
env-ensure-nix() {
  local nix_profile=${HOME:-/root}/.nix-profile/etc/profile.d/nix.sh

  if type -P nix-build &>/dev/null; then
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

  # return false if nix doesn't exist
  type -P nix-build &>/dev/null
}

# Usage: env-ensure-nix-version <major> <minor> <tiny>
env-ensure-nix-version() {
  local major=$1 minor=$2 tiny=$3
  local current
  current=$(nix-build --version)
  if ! [[ $current =~ ([0-9])[.]([0-9]+)([.]([0-9]+))? ]]; then
    env-log "warning: unexpected output of 'nix --version' ($current), devenv might not work properly."
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

env-load() {
  local profile_file env_top

  env_top=$(cd "${BASH_SOURCE[0]%/*}" && pwd)

  # assume that nixpkgs is pinned
  unset NIX_PATH
  # use our own nix configuration
  if [[ -f $env_top/nix/nix.conf ]]; then
    export NIX_CONF_DIR=$env_top/nix
  fi

  if ! env-ensure-nix; then
    env-log "error: nix is a dependency and needs to be installed"
    env-log "to install, run \`bash <(curl https://nixos.org/nix/install)\`"
    return 1
  fi

  # ensure nix --version >= 2.1.1
  if ! env-ensure-nix-version 2 1 1; then
    env-log "error: $(nix --version) is too old"
    env-log "to upgrade, run \`nix upgrade-nix\` or \`bash <(curl https://nixos.org/nix/install)\`"
    return 1
  fi

  local env_root
  env_root=$(env-root)

  if [[ -z $env_root ]]; then
    env-log "error: could not load the env root"
    return 1
  fi

  if ! "$env_root/bin/nix-build-cached" env.nix >/dev/null; then
    return 1
  fi

  # shellcheck disable=SC1091
  source "env/etc/profile"

  # if sourced by direnv
  if [[ $(type -t watch_file) == function ]]; then
    watch_file "${BASH_SOURCE[0]}"
    watch_file "env.cache"

    while IFS= read -r profile_file; do
      watch_file "$profile_file"
    done < <(cut -d ' ' -f 3- "env.cache")
  fi
}

# if executed directly
if [[ "$0" == "${BASH_SOURCE[0]}" ]]; then
  set -euo pipefail
  case "${1:-}" in
  "" | -h | --help)
    # show usage from script header
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
    exit
    ;;
  *)
    env-load
    if [[ $1 == -- ]]; then
      shift
    fi
    exec "$@"
    ;;
  esac
else # this file has been sourced
  env-load
  # don't leave anything behind
  unset -f \
    env-ensure-nix \
    env-ensure-nix-version \
    env-load \
    env-log \
    env-root
fi
