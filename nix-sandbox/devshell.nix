{ pkgs }:
pkgs.mkShell {
  # Add build dependencies
  packages = [
    pkgs.cargo
    pkgs.clippy
    pkgs.bubblewrap
  ];

  # Add environment variables
  env = { };

  # Load custom bash code
  shellHook = ''

  '';
}
