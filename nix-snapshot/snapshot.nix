# TODO: handle multiple outputs
{ path, system ? builtins.currentSystem }:
let
  # copy the nix code into the store
  storePath = "${path}";

  data = import storePath;

  data2 =
    if builtins.isFunction data then
      # assume it's nixpkgs and make it pure
      data { config = {}; overlays = []; }
    else if builtins.isAttrs data then data
    else throw "importing '${builtins.typeOf data}' from ${toString path} is not supported";

  # create a snapshot that can later be restored
  snapshot =
    let
      toNixDrv = name: drv:
      # Discard context for drvPath, otherwise all the build dependencies
      # come along.
        ''
          "${name}" = {
            name = "${drv.name}";
            type = "derivation";
            drvPath = "${builtins.unsafeDiscardStringContext drv.drvPath}";
            outPath = "${drv.outPath}";
          };
        '';

      drvs = builtins.concatStringsSep "" (
        builtins.attrValues
          (builtins.mapAttrs toNixDrv data2)
      );

      # get back the context on the outPath so it gets propagated properly.
      nixCode = ''
        let
          drvs = {
          ${drvs}
          };

          toDrv = _: drv: drv // {
            outPath = builtins.appendContext drv.outPath {
              "''${drv.drvPath}" = { outputs = [ "out" ]; };
            };
          };
        in
          builtins.mapAttrs toDrv drvs
      '';
    in
      derivation {
        inherit system;
        name = "snapshot.nix";
        builder = "/bin/sh";
        args = [ "-ec" ". $buildScriptPath" ];
        nixCode = nixCode;
        buildScript = ''
          # Pure POSIX-sh cat. Only works if there are non-NUL characters.
          cat() {
            while IFS= read -r line <&3; do
              printf '%s\n' "$line"
            done 3< "$1"
          }
          cat "$nixCodePath" > "$out"
        '';
        passAsFile = [ "nixCode" "buildScript" ];
      };
in
snapshot
