# TODO: rebuild this
{ path, system ? builtins.currentSystem }:
let
  # sha1 + base32 encoding
  STORE_HASH_LEN = 32;
  # +1 for the / after the storeDir
  STORE_DIR_LEN = builtins.stringLength builtins.storeDir + 1;

  # Returns a store path out of a non-store path
  storePath =
    builtins.path { name = "source"; path = path; };

  # Return the hash of a store path
  storePathHash = storePath:
    builtins.substring STORE_DIR_LEN STORE_HASH_LEN storePath;

  # Returns the name of a store path
  storePathName = storePath:
    builtins.substring (STORE_DIR_LEN + STORE_HASH_LEN) 99999 storePath;

  # The XDG_CACHE dir for nixpkgs-snapshot
  cacheDir =
    let
      env = builtins.getEnv "XDG_CACHE_HOME";
      dir = if env == "" then toString ~/.config else env;
    in
    "${env}/nixpkgs-snapshot/${storePathHash storePath}";

  # this is used by the cli to force the storePath to be written
  # to the /nix/store.
  #
  # TODO: use `nix-instantiate --eval --read-write-mode` instead ?
  info =
    let
      data = builtins.toJSON {
        inherit cacheDir storePath;
      };
    in
    derivation {
      inherit system;
      name = "path-data.json";
      builder = "/bin/sh";
      args = [ "-c" "echo '${data}' > $out" ];
    };

  # Load a nix folder that contains a default.nix. First check in the
  # nixpkgs-snapshot cache dir if it has been evaluated already. Otherwise
  # load the original.
  import =
    let
      pathImport = import path;
    in
    if builtins.pathExists cacheDir then
      let
        toFakeDrv = attr: type:
          assert type == "symlink";
          rec {
            type = "derivation";
            name = storePathName outPath;
            # FIXME: drvPath
            outPath = builtins.storePath "${toString cacheDir}/${attr}";
          };
        # FIXME: make the construct lazy?
        # FIXME: build the recursive tree if the name contains a dot
        data = builtins.readDir cacheDir;
      in
      builtins.mapAttrs toFakeDrv data
    else if builtins.isAttrs pathImport then
    # the ideal scenario
      pathImport
    else if builtins.isFunction pathImport then
    # if it's a function, assume it's nixpkgs and make it pure.
      pathImport
        {
          config = { };
          overlays = [ ];
        }
    else
      throw "${builtins.typeOf pathImport} is not supported";
in
{
  inherit
    # this is used by the CLI
    info
    # this is used by the nix code
    import
    ;
}
