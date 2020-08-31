{ pkgsPath ? import ./nixpkgs {} }:
pkgsPath {
  config = {};
  overlay = [ (import ./overlay.nix) ];
}
