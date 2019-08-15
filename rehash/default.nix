{ stdenv }:
let
  # Takes the drv and produce a new drv that won't change if the original drv
  # output is the same.
  #
  # Nix forgets the connection between the input drv and the output drv.
  rehash = drv:
    let
      name = builtins.unsafeDiscardStringContext drv.name;
      outPath = builtins.unsafeDiscardStringContext drv.outPath;
    in
    stdenv.mkDerivation {
      inherit name;
      src = builtins.path {
        # needed, otherwise the names contains the outPath folder name.
        inherit name;
        # trick nix into allowing us to import a store path.
        path = /. + outPath;
      };

      buildCommand = ''
        set -e
        cp -r $src $out
      '';

      # It would be nice to have this, but then it adds the drv to the build
      # inputs.
      #disallowedReferences = [ drv ];

      meta = (drv.meta or {}) // {
        orig = drv;
      };
    };
in
  rehash
