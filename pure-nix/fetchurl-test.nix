let
  fetchurl = import ./fetchurl.nix;
in
{ 
  direnv-executable = fetchurl {
    url = "https://github.com/direnv/direnv/releases/download/v2.24.0/direnv.linux-amd64";
    hash = "sha256-4Ob4C0Fnwz5xLmk6vwrt2ZC2BFz/8rhWxd9ndEyZxn4=";
  };

  # Unpack seems to assume a NAR
  # direnv-unpack = fetchurl {
  #   url = "https://github.com/direnv/direnv/archive/v2.24.0.tar.gz";
  #   hash = "sha256-2GKQFHNce4Fbd8YRDMG12TmapDl7++YgcdoQrY7s8kg=";
  #   unpack = true;
  # };
}
