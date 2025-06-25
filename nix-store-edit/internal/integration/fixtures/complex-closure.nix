# A more complex derivation with multiple levels of dependencies
{ pkgs ? import <nixpkgs> {} }:

let
  # Base library
  baseLib = pkgs.writeText "base-lib" ''
    export BASE_VERSION="1.0"
  '';
  
  # Middle layer that depends on base
  middleLib = pkgs.writeTextDir "lib/middle.sh" ''
    source ${baseLib}
    export MIDDLE_VERSION="2.0"
  '';
  
  # Application that depends on middle (and transitively on base)
  app = pkgs.writeScriptBin "complex-app" ''
    #!${pkgs.bash}/bin/bash
    source ${middleLib}/lib/middle.sh
    echo "App using base version: $BASE_VERSION"
    echo "App using middle version: $MIDDLE_VERSION"
  '';
  
  # Configuration that references the app
  config = pkgs.writeText "app.conf" ''
    app_path = ${app}/bin/complex-app
    lib_path = ${middleLib}
  '';
in
pkgs.buildEnv {
  name = "complex-system";
  paths = [ app config ];
}