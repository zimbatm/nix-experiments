{
  inputs = {
    nixpkgs.url = "path:../fake-nixpkgs";
  };

  outputs =
    { self, nixpkgs }:
    {
      packages.x86_64-linux = {
        default = builtins.derivation {
          system = "x86_64-linux";
          name = "dep-default";
          builder = "/bin/sh";
          args = [
            "-c"
            ''
              {
                < ${nixpkgs.legacyPackages.x86_64-linux.foo};
                echo dep-default;
              } > $out
            ''
          ];
        };
      };
    };
}
