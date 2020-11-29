let
  src = <nix_file>;

  # using scopedImport, replace readFile with
  # implementations which will log files and paths they see.
  #
  # Note that scopedImport is not memoized, contrary to import
  overrides = {
    import = scopedImport overrides;
    scopedImport = x: builtins.scopedImport (overrides // x);
    builtins = builtins
      // {
      readFile = file: builtins.trace "evaluating file '${toString file}'" (builtins.readFile file);
      # TODO: add readDir
    }
    ;
  };

  imported =
    let
      raw = overrides.scopedImport overrides src;
    in
    if (builtins.isFunction raw)
    then raw { }
    else raw;
in
#imported
overrides.scopedImport overrides src
