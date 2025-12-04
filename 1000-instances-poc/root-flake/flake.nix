{
  inputs = {
    nixpkgs.url = "path:../fake-nixpkgs";

    dep.url = "path:../dep-flake";
    dep.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    {
      self,
      nixpkgs,
      dep,
    }:
    {
      packages.x86_64-linux = {
        default = builtins.derivation {
          system = "x86_64-linux";
          name = "end-result";
          builder = "/bin/sh";
          args = [
            "-c"
            ''
              {
                echo root-result;
                < ${dep.packages.x86_64-linux.default};
                echo XXX;
                < ${nixpkgs.legacyPackages.x86_64-linux.foo};
              } > $out
            ''
          ];
        };
      };
    };
}
