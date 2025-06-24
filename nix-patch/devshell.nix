{ pkgs }:
pkgs.mkShell {
  # Add build dependencies
  packages = [
    pkgs.go
    pkgs.nixos-rebuild-ng
  ];

  # Add environment variables
  env = { };

  # Load custom bash code
  shellHook = ''

  '';
}
