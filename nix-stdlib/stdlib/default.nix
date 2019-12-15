let
  types = import ./types;
  stdlib = types // {
    asserts = import ./asserts.nix stdlib;
    generic = import ./generic.nix stdlib;
    imports = import ./imports.nix stdlib;
    impure = import ./impure stdlib;
    toNix = import ./toNix.nix stdlib;
  };
in
stdlib
