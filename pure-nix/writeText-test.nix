let
  writeText = import ./writeText.nix;
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
}
