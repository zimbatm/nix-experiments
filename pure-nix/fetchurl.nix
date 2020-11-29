{ url
, hash ? "" # an SRI hash
, executable ? false
, name ? baseNameOf (toString url)
, meta ? { }
, passthru ? { }
}:
let
  drv = derivation {
    system = "builtin";
    builder = "builtin:fetchurl";

    inherit name url executable;

    # New-style output content requirements.
    outputHash = hash;
    outputHashMode = if executable then "recursive" else "flat";

    # No need to double the amount of network traffic
    preferLocalBuild = true;

    impureEnvVars = [
      # We borrow these environment variables from the caller to allow
      # easy proxy configuration.  This is impure, but a fixed-output
      # derivation like fetchurl is allowed to do so since its result is
      # by definition pure.
      "http_proxy"
      "https_proxy"
      "ftp_proxy"
      "all_proxy"
      "no_proxy"
    ];

    # To make "nix-prefetch-url" work.
    urls = [ url ];
  };
in
drv // { inherit meta passthru; }
