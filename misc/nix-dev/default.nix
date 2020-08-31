{ system ? builtins.currentSystem
}:
let
  pkgs = import ./nix { inherit system; };
in
pkgs.devenv
