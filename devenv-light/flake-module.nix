{ self, lib, flake-parts-lib, ... }:
let
  inherit (flake-parts-lib)
    mkPerSystemOption;
  inherit (lib)
    mkOption
    types;
in
{
  options = {
    perSystem = mkPerSystemOption
      ({ config, self', inputs', pkgs, system, ... }: {
        options.devenv = {
          packages = mkOption {
            description = "List of packages to add to the environment";
            type = lib.types.listOf lib.types.package;
            default = [ ];
          };

          env = mkOption {
            description = "Env vars to set on the project";
            type = lib.types.attrsOf lib.types.str;
            default = { };
          };
        };
        config =
          let
            mkDevenv = pkgs.callPackage ./src/mkDevenv.nix { };
          in
          {
            packages.devenv = mkDevenv config.devenv;
          };
      });
  };
}
