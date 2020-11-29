{ sources ? (import ./sources.nix)
    // {
    devenv = ../.;
  }
, system ? builtins.currentSystem
}:
import sources.nixpkgs {
  inherit system;
  config = { };
  overlays = [
    (_: __: { inherit sources; })
    (import "${toString sources.devenv}/overlay.nix")
    (import ./overlay.nix)
  ];
}
