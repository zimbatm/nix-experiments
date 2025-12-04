let
  system = "x86_64-linux";
  nixpkgsPath = builtins.toPath ../../../NixOS/nixpkgs;
  evalConfig = import (nixpkgsPath + "/nixos/lib/eval-config.nix");
in
evalConfig {
  inherit system;
  modules = [
    (nixpkgsPath + "/nixos/modules/virtualisation/qemu-vm.nix")
    ./configuration.nix
  ];
}
