_: pkgs: {
  niv = (import pkgs.sources.niv { }).niv;
  cachix = import pkgs.sources.cachix { inherit pkgs; };
}
