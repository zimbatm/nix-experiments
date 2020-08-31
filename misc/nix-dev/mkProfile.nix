{ buildEnv
, lib
, devenv
, writeText
, pkgs
}:
{
  # packages to add to the profile
  packages ? []
  # environment variables to load
, profile ? ""
  # only useful in the project
, withDevenv ? true
  # a set of nix modules to configure the profile
, modules ? []
}@args:
let
  profileText = writeText "extra-profile" profile;

  paths =
    lib.optional withDevenv devenv
    ++ packages
  ;

  baseModule = { config, lib, ... }:
    with lib;
    let
      cfg = config.profile;
    in
      {
        options = {
          profile.packages = mkOption {
            description = "The set of packages to appear in the profile";
            type = types.listOf types.package;
            default = [];
          };

          profile.path = mkOption {
            internal = true;
            type = types.package;
          };
        };

        config = {
          profile.path = buildEnv {
            name = "nix-profile";

            paths = cfg.packages ++ paths;

            postBuild = ''
              mkdir -p $out/etc
              cat <<ETC_PROFILE > $out/etc/profile
              export PROFILE_ROOT=$out

              # add path to the environment
              export PATH=$PROFILE_ROOT/bin:\$PATH

              # load profiles
              for filename in "$PROFILE_ROOT/etc/profile.d/"*.sh; do
                [ -e "\$filename" ] || continue
                # shellcheck disable=SC1090
                . "\$filename"
              done

              # extra profile
              source "${profileText}"
              ETC_PROFILE
            '';

            meta = {
              description = "Environment of packages installed through mkProfile";
            };
          };
        };
      };

  out = lib.evalModules {
    specialArgs = {};
    modules = [ baseModule ] ++ modules;
    args = { inherit pkgs; };
  };
in
out.config.profile.path
