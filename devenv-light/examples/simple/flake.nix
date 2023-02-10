{
  description = "A devenv POC";

  inputs.devenv-light.url = "path:../..";

  outputs = { self, nixpkgs, devenv-light }:
    let
      lib = nixpkgs.lib;
      forAllSystems = lib.genAttrs lib.systems.flakeExposed;
    in
    {
      packages = forAllSystems
        (system:
          let
            pkgs = nixpkgs.legacyPackages.${system};
            mkDevenv = devenv-light.lib.${system}.mkDevenv;
          in
          {
            devenv = mkDevenv {
              packages = [
                pkgs.hello
              ];

              env.HELLO = "world";
            };
          }
        );
    };
}
