let
  pkgs = import ./nix;
  # First run `nix-build snapshot.nix --argstr path $PWD/nix`
  pkgSnap = import ./result;

  mkFoo = drv:
    derivation {
      buildInputs = [ drv ];
      name = "xxx";
      system = builtins.currentSystem;
      builder = "/bin/sh";
      args = [ "-c" "echo XXX > $out" ];
    };
in
{
  # both should contain the same drvPath
  a = mkFoo pkgs.hello;
  b = mkFoo pkgSnap.hello;
}
