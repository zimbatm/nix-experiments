let
  writeText = import ./writeText.nix;
  toNix = import ./toNix.nix;
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
}
