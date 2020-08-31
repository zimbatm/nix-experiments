{ ... }@args:
let
  pkgs = import ./. args;
in
{
  inherit (pkgs) nix-src;
}
