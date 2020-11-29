let
  nixConfig = import <nix/config.nix>;
  nixBin = builtins.storePath nixConfig.nixBinDir;
in
{ system ? builtins.currentSystem }:
derivation {
  inherit system;
  name = "snapshot-in-drv";
  builder = "/bin/sh";
  args = [ "-ec" ". $buildScriptPath" ];
  passAsFile = [ "buildScript" ];
  buildScript = ''
    export HOME=$PWD/home
    export NIX_PATH=nixpkgs=${<nixpkgs>}
    ${nixBin}/nix-instantiate --store $PWD/nix ${<path>} > $out
  '';
}
