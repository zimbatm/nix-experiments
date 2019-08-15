let
  pkgs = import <nixpkgs> {};
  rehash = pkgs.callPackage ./. {};
  runCommand = pkgs.runCommand;

  packageA = runCommand "package-a" {} ''
    echo CONTENT > $out
  '';

  packageA' = runCommand "package-a" {} ''
    # the build instructions have changed but the output is the same
    echo CONTENT > $out
  '';

  mkPackageB = { packageA }:
    runCommand "package-b" {} ''
      # depends on package-a
      echo ${packageA} > $out
    '';
in
  rec {
    inherit packageA packageA';

    withoutRehash = {
      packageB = mkPackageB { packageA = packageA; };
      packageB' = mkPackageB { packageA = packageA'; };
    };

    withRehash = {
      packageB = mkPackageB { packageA = rehash packageA; };
      packageB' = mkPackageB { packageA = rehash packageA'; };
    };

    test =
      assert withRehash.packageB.outPath == withRehash.packageB'.outPath;
      true
      ;
  }
