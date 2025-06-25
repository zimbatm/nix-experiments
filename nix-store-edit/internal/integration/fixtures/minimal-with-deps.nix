# Minimal derivation with dependencies but no nixpkgs
{ system ? builtins.currentSystem }:
let
  # Base config file
  config = derivation {
    inherit system;
    name = "app-config";
    builder = "/bin/sh";
    args = [ "-c" ''
      echo "database_host = localhost" > $out
      echo "database_port = 5432" >> $out
    '' ];
  };
  
  # Script that depends on config
  script = derivation {
    inherit system;
    name = "app-script";
    builder = "/bin/sh";
    args = [ "-c" ''
      echo "#!/bin/sh" > $out
      echo "echo 'Config location: ${config}'" >> $out
      echo "echo 'Config contents: (see ${config})'" >> $out
    '' ];
  };
in
# Bundle both together with proper directory structure
derivation {
  inherit system;
  name = "app-bundle";
  builder = "/bin/sh";
  args = [ "-c" ''
    mkdir -p $out
    cp ${config} $out/app-config
    cp ${script} $out/app-script
  '' ];
}
