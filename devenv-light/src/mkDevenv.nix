{ lib
, buildEnv
, writeText
}:

# All those values would be replaced by the module system
{ packages ? [ ]
, env ? { }
, name ? "devenv"
}:
let
  envToSh = key: value:
    "export ${key}=${lib.escapeShellArg (toString value)}";

  # POSIX-compatible vars that will be loaded by devenv
  profile = writeText "profile.sh" ''
    # Set the environment variables
    ${lib.concatStringsSep "\n" (lib.mapAttrsToList envToSh env)}
  '';
in
buildEnv {
  inherit name;
  paths = packages;

  postBuild = ''
    # Setting for the devenv
    cp ${profile} $out/profile.sh

    # Make the bin folder writable
    if [[ -s $out/bin ]]; then
      mv $out/bin bak
      mkdir $out/bin
      cp bak/* $out/bin/
      rm bak
    fi

    # The main entrypoint script
    cp ${./entrypoint.sh} $out/bin/devenv
    patchShebangs $out/bin/devenv
    sed -i -e "s|@devenv_root@|$out|" "$out/bin/devenv"
  '';
  meta.mainProgram = "devenv";
}
