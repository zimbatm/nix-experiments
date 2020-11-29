_: pkgs: {
  devenv = pkgs.callPackage ./pkgs/devenv.nix { };
  mkLazyBin = pkgs.callPackage ./mkLazyBin.nix { };
  mkProfile = pkgs.callPackage ./mkProfile.nix { };
}
