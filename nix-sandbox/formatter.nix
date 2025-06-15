{
  pkgs,
  flake,
  inputs,
}:
let
  mod = inputs.treefmt-nix.lib.evalModule pkgs {
    projectRootFile = "flake.nix";

    programs = {
      nixfmt.enable = true;
      deadnix.enable = true;
      prettier.enable = true;
      statix.enable = true;
      shellcheck.enable = true;
      rustfmt.enable = true;
    };

    settings = {
      global.excludes = [
        "LICENSE"
        # unsupported extensions
        "*.{gif,png,svg,tape,mts,lock,mod,sum,env,gitignore}"
      ];

      formatter = {
        deadnix = {
          priority = 1;
        };

        statix = {
          priority = 2;
        };

        nixfmt = {
          priority = 3;
        };
      };
    };
  };
in
mod.config.build.wrapper
// {
  passthru.tests.check = mod.config.build.check flake;
}
