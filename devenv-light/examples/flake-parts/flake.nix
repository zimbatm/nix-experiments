{
  description = "A devenv POC";

  inputs.devenv-light.url = "path:../..";

  outputs = inputs@{ self, nixpkgs, devenv-light, flake-parts }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = nixpkgs.lib.systems.flakeExposed;

      imports = [
        devenv-light.flakeModules.default
      ];

      perSystem = args@{ self', pkgs, ... }: {
        devenv = {
          packages = [
            pkgs.hello
          ];

          env.HELLO = "world";
        };
      };
    };
}
