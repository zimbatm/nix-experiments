{ system ? builtins.currentSystem
, nixpkgs ? builtins.fetchTarball "channel:nixos-19.03"
, pkgs ? import nixpkgs { inherit system; config = {}; overlays = []; }
}:
{
  profile = pkgs.buildEnv {
    name = "profile";
    paths = with pkgs; [
      hello
    ];
  };
}
