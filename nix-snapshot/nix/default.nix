let
  sources = import ./sources.nix;
  overlay = self: super: {
    dev-env = super.callPackage ./dev-env.nix {};
  };
  pkgs = import sources.nixpkgs {
    config = {};
    overlays = [ overlay ];
  };
in
{ inherit (pkgs) hello; }
