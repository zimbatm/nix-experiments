{
  description = "A fake nixpkgs instance";

  outputs = { self }: {
    legacyPackages.x86_64-linux = builtins.trace "evaluation of nixpkgs" {
      foo = builtins.derivation {
        name = "nixpkgs-foo";
        system = "x86_64-linux";
        builder = "/bin/sh";
        args = ["-c" "echo nixpkgs-foo > $out"];
      };
    };
  };
}
