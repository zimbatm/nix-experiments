let
  writeText = import ./writeText.nix;
  toNix = import ./toNix.nix;
  pkgs = import <nixpkgs> {};
in
{
  xxx = writeText {
    name = "xxx";
    text = ''
      line1
      line2
      line3
      line4
    '';
  };

  toNix = writeText {
    name = "toNix";
    text = toNix {
      str = "string";
      list = [ 1 2 3 ];
      num = 3;
      attrs = {
        a = "a";
      };
    };
  };

  # A file that contains another reference to the store.
  withRef = writeText {
    name = "withRef";
    text = ''
      #!${pkgs.bash}/bin/bash
      echo XXX
    '';
  };
}
