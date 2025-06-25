# A simple derivation that creates a text file
{ pkgs ? import <nixpkgs> {} }:

pkgs.writeText "simple-config" ''
  original content
''