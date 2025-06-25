# A derivation with dependencies
{ pkgs ? import <nixpkgs> {} }:

let
  config = pkgs.writeTextDir "etc/app-config" ''
    database_host = localhost
    database_port = 5432
  '';
  
  script = pkgs.writeScriptBin "app" ''
    #!${pkgs.bash}/bin/bash
    echo "Loading config from ${config}"
    cat ${config}
  '';
in
pkgs.symlinkJoin {
  name = "app-with-config";
  paths = [ script config ];
}