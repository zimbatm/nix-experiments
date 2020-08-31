# We want to have two versions of the fetchers, one that is pure and one that
# uses nixpkgs
{ path }:
let
  fetch = import ./eval-fetch.nix;

  fetchOrPath = value:
    if builtins.typeOf value == "set" then
      fetch value
    else
      toString value;

  sources =
    if builtins.isAttrs path then
      path
    else
      import "${toString path}";
in
builtins.mapAttrs (_: fetchOrPath) sources
