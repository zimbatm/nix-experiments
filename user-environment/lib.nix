{ writeText }:
# Supporting code
with builtins;
rec {
  toNix = import ./toNix.nix;

  genAttrs = names: f:
    listToAttrs (map (n: { name = n; value = f n; }) names);

  buildEnv = import <nix/buildenv.nix>;

  genManifest = drv:
    let
      outputs = drv.meta.outputsToInstall or [ "out" ];

      base = {
        inherit (drv) meta name outPath system type;
        out = { inherit (drv) outPath; };
        inherit outputs;
      };

      toOut = name: {
        "${name}" = {
          outPath = drv.${name}.outPath;
        };
      };

      outs = genAttrs outputs toOut;
    in
      base // outs;

  asManifest = drvs: map genManifest drvs;

  writeManifest = derivations:
    writeText "env-manifest.nix" (
      toNix (
        asManifest
          derivations
      )
    );
}

