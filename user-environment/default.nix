{ writeText }@attrs:
# Build a user environment purely with nix.
#
# The original implementation is a mix of C++ and nix code.
#
# See https://github.com/nixos/nix/blob/f4b94958543138671bc3641fc126589a5cffb24b/src/nix-env/user-env.cc
#
# TODO:
# * also add the drvPath if the keepDerivations nix settings is set
# * support "disabled" mode that breaks nix-env?
# * remove the use of writeText. builtins.toFile forbits the use of references
#   to derivations, which makes it impossible to create exactly the same
#   manifest file as `nix-env`.
#
# Arguments:
# * derivations: a list of derivations
with (import ./lib.nix attrs);
{
  # A list of derivations to install
  derivations
}:
buildEnv {
  inherit derivations;
  manifest = writeManifest derivations;
}
