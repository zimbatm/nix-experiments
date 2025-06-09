{ pkgs ? import <nixpkgs> {} }:

pkgs.writeShellScriptBin "hello-trampoline" ''
  echo "🎉 Hello from nix-trampoline!"
  echo "📦 Built with Nix in Kubernetes"
  echo "🕐 Built on: $(date)"
  echo "🏗️  Architecture: $(uname -m)"
  echo "📦 Nix version: $(nix --version)"
  echo "🐳 Running in container: $HOSTNAME"
''