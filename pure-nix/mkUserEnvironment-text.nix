{ system ? builtins.currentSystem }:
let
  mkUserEnvironment = import ./mkUserEnvironment.nix;

  pkgs = import <nixpkgs> {};
in
{
  xxx = mkUserEnvironment {
    derivations = [
      pkgs.bash
      pkgs.curl
    ];
    inherit system;
  };
}
