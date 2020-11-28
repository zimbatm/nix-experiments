{ system ? builtins.currentSystem }:
let
  toNix = import ./toNix.nix;
in
{

  toNix = toNix {
    str = "string";
    num = 3;
    attr = {
      g = null;
    };
  };

}
