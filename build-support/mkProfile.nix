{ pkgs
, lib
, bashInteractive
, buildEnv
, writeText
}:

# mkProfile
{ name
, paths ? { }
, env ? { }
, profile ? ""
, interactive ? ""
}:
let
  envPairs = lib.mapAttrsToList
    (k: v: "export ${k}=${lib.escapeShellArg (toString v)}")
    env;

  profileDrv = writeText "bashrc" ''
    # Set all the environment variables
    ${builtins.concatStringsSep "\n" envPairs}

    # Load installed profiles
    shopt -s nullglob
    for profile in $PROFILE_ROOT/etc/profile.d/*.sh; do
      source "$profile"
    done

    # Extra profile
    ${profile}

    # Interactive sessions
    if [[ $- == *i* ]]; then

    # Set PS1
    PS1='\033[0;32;40m[${name}]$\033[0m '

    ${interactive}

    fi
  '';
in
buildEnv {
  inherit name paths;
  postBuild = ''
    cat <<PROFILE > $out/bashrc
    export PROFILE_ROOT=$out
    export PATH=$out/bin\''${PATH+:\$PATH}
    PROFILE
    cat "${profileDrv}" >> $out/bashrc

    cat <<'ALMOST_BASH' > $out/bash.sh
    #!${bashInteractive}/bin/bash
    #
    # Almost bash. If --pure
    #
    # Usage: bash.sh [--pure] [<args>...]
    #
    # Options:
    # * --pure: re-start the script in a pure environment. Must be
    #           the first argument.
    # * normal bash options
    set -euo pipefail

    if [[ "''${1:-}" == "--pure" ]]; then
      shift
      exec -c "$0" "$@"
    fi
    ALMOST_BASH

    # splice in the bashrc location
    echo "bashrc_path=$out/bashrc" >> $out/bash.sh

    cat <<'ALMOST_BASH' >> $out/bash.sh
    if [[ $# = 0 ]]; then
      # start an interactive shell
      set -- ${bashInteractive}/bin/bash --rcfile "$bashrc_path" --noprofile
    else
      # start a script
      source "$bashrc_path"
      set -- ${bashInteractive}/bin/bash "$@"
    fi
    exec -a "$0" "$@"
    ALMOST_BASH
    chmod +x $out/bash.sh
  '';
}
