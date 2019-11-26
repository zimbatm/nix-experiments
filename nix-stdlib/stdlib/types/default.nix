let
  types = {
    bools = import ./bools.nix types;
    floats = import ./floats.nix types;
    ints = import ./ints.nix types;
    lambdas = import ./lambdas.nix types;
    lists = import ./lists.nix types;
    nulls = import ./nulls.nix types;
    paths = import ./paths.nix types;
    sets = import ./sets.nix types;
    strings = import ./strings.nix types;
  };
in
types
