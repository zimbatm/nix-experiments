{
  profile ? "k8s-master",
}:
let
  nixpkgsPath = builtins.toPath ../../../NixOS/nixpkgs;
  system = builtins.currentSystem;
  pkgs = import nixpkgsPath { inherit system; };
  pythonEnv = pkgs.python3.withPackages (_: [ ]);
in
pkgs.mkShell {
  inherit system;
  name = "module-pruning-shell";

  buildInputs = [
    pythonEnv
    pkgs.nix
    pkgs.git
    pkgs.coreutils
    pkgs.gnugrep
    pkgs.gnused
    pkgs.findutils
    pkgs.time
  ];

  shellHook = ''
    export MODULE_PRUNING_PROFILE=${profile}
    echo "Loaded module-pruning shell (profile=$MODULE_PRUNING_PROFILE)"
    export PATH=$PWD/scripts:$PATH
  '';
}
