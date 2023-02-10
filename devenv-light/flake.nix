{
  description = "A devenv POC";

  outputs = { self, nixpkgs }:
    let
      lib = nixpkgs.lib;
      forAllSystems = lib.genAttrs lib.systems.flakeExposed;
    in
    {
      # lib because mkDevenv is a constructor.
      lib = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          mkDevenv = pkgs.callPackage ./src/mkDevenv.nix { };
        }
      );

      flakeModules.default = ./flake-module.nix;
    };
}
