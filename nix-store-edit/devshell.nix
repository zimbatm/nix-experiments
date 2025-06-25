{ pkgs }:
pkgs.mkShell {
  # Add build dependencies
  packages = [
    pkgs.go
    pkgs.nixos-rebuild-ng
    pkgs.golangci-lint
  ];

  # Add environment variables
  env = { };

  # Load custom bash code
  shellHook = ''

  '';
}
