{ pkgs }:
pkgs.mkShell {
  # Add build dependencies
  packages = [
    pkgs.cargo
    pkgs.clippy
    pkgs.rustfmt
  ] ++ (pkgs.lib.optionals pkgs.stdenv.isLinux [
    pkgs.bubblewrap
  ]);

  # Add environment variables
  env = { };

  # Load custom bash code
  shellHook = ''

  '';
}
