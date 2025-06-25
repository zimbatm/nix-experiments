# Simple derivations with dependencies for testing
# This uses coreutils which should be available in test environment
{ pkgs ? import <nixpkgs> {} }:

let
  inherit (pkgs) runCommand;
  
  # Base config file
  config = runCommand "app-config" {} ''
    cat > $out << EOF
    database_host = localhost
    database_port = 5432
    EOF
  '';
  
  # Script that depends on config
  script = runCommand "app-script" { inherit config; } ''
    mkdir -p $out/bin
    cat > $out/bin/app << EOF
    #!/bin/sh
    echo "Config location: ${config}"
    cat ${config}
    EOF
    chmod +x $out/bin/app
  '';
in
# Bundle both together
runCommand "app-bundle" { inherit script config; } ''
  mkdir -p $out
  ln -s ${script}/bin $out/bin
  ln -s ${config} $out/config
''