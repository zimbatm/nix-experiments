{ pkgs }:
pkgs.mkShell {
  # Add build dependencies
  packages = [
    pkgs.k9s
    pkgs.kubectl
    pkgs.minikube
  ];

  # Add environment variables
  env = { };

  # Load custom bash code
  shellHook = ''

  '';
}
