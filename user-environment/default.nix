{ writeText, lib }:
# Build a user environment purely with nix.
#
# The original implementation is a mix of C++ and nix code.
#
# See https://github.com/nixos/nix/blob/f4b94958543138671bc3641fc126589a5cffb24b/src/nix-env/user-env.cc
#
# TODO:
# * also add the drvPath if the keepDerivations nix settings is set
# * support "disabled" mode that breaks nix-env?
# * allow to pass old environment and extend it
# * remove the use of writeText. builtins.toFile forbits the use of references
#   to derivations, which makes it impossible to create exactly the same
#   manifest file as `nix-env`.
#
# Arguments:
# * derivations: a list of derivations
{
  # A list of derivations to install
  derivations
}:
let
  stdlib = import ../nix-stdlib;
  toNix = stdlib.toNix;
in
# Supporting code
with builtins;
let
  # Generate a nix-env compatible manifest.nix file
  genManifest = drv:
    let
      outputs =
        drv.meta.outputsToInstall or
          # install the first output
          [ (head drv.outputs) ];

      base = {
        inherit (drv) meta name outPath system type;
        out = { inherit (drv) outPath; };
        inherit outputs;
      };

      toOut = name: {
        outPath = drv.${name}.outPath;
      };

      outs = lib.genAttrs outputs toOut;
    in
    base // outs;

  writeManifest = derivations:
    writeText "env-manifest.nix" (
      toNix (map genManifest derivations)
    );
in
derivation {
  name = "user-environment";
  system = "builtin";
  builder = "builtin:buildenv";
  manifest = writeManifest derivations;

  # !!! grmbl, need structured data for passing this in a clean way.
  derivations =
    map
      (d:
        [
          (d.meta.active or "true")
          (d.meta.priority or 5)
          (builtins.length d.outputs)
        ] ++ map (output: builtins.getAttr output d) d.outputs)
      derivations;

  # Building user environments remotely just causes huge amounts of
  # network traffic, so don't do that.
  preferLocalBuild = true;

  # Also don't bother substituting.
  allowSubstitutes = false;
}
