{
  nixpkgs = {
    type = "github";
    owner = "nixos";
    repo = "nixpkgs-channels";
    ref = "626233eee6ea309733d2d98625750cca904799a5";
    rev = "nixos-unstable";
    hash = "sha256-0w1s5v96cdf57f2wzqrkxfz6bhdb6h2axjv3r8l7p8pf4kdwdky2";
  };

  home-manager = ../../../github.com/rycee/home-manager;

  nixpkgs-wayland = {
    url = "https://github.com/colemickens/nixpkgs-wayland/archive/master.tar.gz";
    unpack = true;
  };

  nix = {
    type = "git";
    url = "https://github.com/nixos/nix.git";
    rev = "841fcbd04755c7a2865c51c1e2d3b045976b7452";
    ref = "1.11-maintenance";
  };
}
