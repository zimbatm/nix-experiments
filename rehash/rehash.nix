{ stdenv }:
let
  # Takes the drv and produce a new drv that won't change if the original drv
  # output is the same.
  #
  # Nix forgets the connection between the input drv and the output drv, which
  # has a few implications.
  rehash = drv:
    stdenv.mkDerivation {
      name = builtins.unsafeDiscardStringContext drv.name;

      src = builtins.path {
        name = builtins.unsafeDiscardStringContext drv.name;
        path = /. + builtins.unsafeDiscardStringContext drv.outPath;
      };

      buildCommand = ''
        set -e
        cp -r $src $out

        # replace all references of the original drv
        chmod -R +w $out
        find $out -type f -exec sed -i -e "s|${drv}|$out|g" {} \;
      '';

      # just to make sure
      disallowedReferences = [ drv ];
    };

in
  rehash
