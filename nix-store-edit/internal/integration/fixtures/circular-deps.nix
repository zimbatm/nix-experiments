# Test case for circular dependencies (which Nix prevents at build time)
{ pkgs ? import <nixpkgs> {} }:

let
  # Create two configs that reference each other's paths
  # Note: Nix will prevent actual circular dependencies, but we can
  # create a structure where both items exist in the same closure
  
  configA = pkgs.writeText "config-a" ''
    name = "Config A"
    partner = "config-b"
  '';
  
  configB = pkgs.writeText "config-b" ''
    name = "Config B" 
    partner = "config-a"
  '';
  
  # A script that uses both configs
  mainScript = pkgs.writeScriptBin "circular-test" ''
    #!${pkgs.bash}/bin/bash
    echo "Config A location: ${configA}"
    echo "Config B location: ${configB}"
    cat ${configA}
    cat ${configB}
  '';
in
pkgs.symlinkJoin {
  name = "circular-test-env";
  paths = [ mainScript configA configB ];
}