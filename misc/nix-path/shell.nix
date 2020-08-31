let
  nix-path = import ./lib/eval-nix-path.nix { path = ./nix-path.nix; };
in
builtins.trace <nixpkgs> (
  { pkgs ? import nix-path.nixpkgs {} }:

    pkgs.mkShell {
      shellHook = ''
        export PATH=$PWD/bin:$PATH
      '';
    }
)
