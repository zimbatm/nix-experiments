# Evaluate `release.nix' like Hydra would.  Too bad nix-instantiate
# can't to do this.
let
  inherit (builtins)
    getEnv
    isAttrs
    mapAttrs
    tryEval
    ;

  # imported from lib
  isDerivation = x: isAttrs x && x ? type && x.type == "derivation";

  trace = if getEnv "VERBOSE" == "1" then builtins.trace else (x: y: y);

  rel = import <release> { };

  # Add the ‘recurseForDerivations’ attribute to ensure that
  # nix-instantiate recurses into nested attribute sets.
  recurse = path: attrs:
    if (tryEval attrs).success then
      if isDerivation attrs
      then
        if (tryEval attrs.drvPath).success
        then { inherit (attrs) name drvPath; }
        else { failed = true; }
      else { recurseForDerivations = true; } // mapAttrs (n: v:
        let path' = path ++ [ n ]; in trace path' (recurse path' v)) attrs
    else { };

in
recurse [ ] rel
