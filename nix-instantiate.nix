# What if nix evaluation could also happen in the nix build sandbox?
#
# STATUS: currently fails with `error: creating directory '/nix/var': Permission denied`
let
  nixConfig = import <nix/config.nix>;
  nixBin = builtins.storePath nixConfig.nixBinDir;
  coreutilsBin = builtins.storePath nixConfig.coreutils;

  # NOTE: this derivation is impure and depends on the user's nix version
  nixInstantiate = { name ? "nix-instantiate", path }:
    derivation {
      inherit name;
      system = builtins.currentSystem;

      PATH = "${coreutilsBin}:${nixBin}";

      builder = "/bin/sh";
      args = [ "-c" ". $buildScriptPath" ];
      passAsFile = [ "buildScript" ];
      buildScript = ''
        set -e
        nix-instantiate --readonly-mode "${path}" > $out
      '';
    };

in
nixInstantiate {
  path = ./nix;
}
