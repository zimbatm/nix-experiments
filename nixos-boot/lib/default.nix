# nixos-boot library
#
# Provides functions for building UKIs and testing with QEMU
{ inputs, flake, ... }:
let
  inherit (inputs.nixpkgs) lib;

  # Create the library for a specific system
  forSystem =
    system:
    let
      pkgs = inputs.nixpkgs.legacyPackages.${system};
      args = { inherit lib pkgs; };
    in
    {
      uki = import ../nix/lib/uki.nix args;
      vm = import ../nix/lib/vm.nix args;
    };
in
{
  inherit forSystem;
  # Convenience for common systems
  x86_64-linux = forSystem "x86_64-linux";
  aarch64-linux = forSystem "aarch64-linux";
}
