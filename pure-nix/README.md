# A bunch of nix functions that work without nixpkgs

## fetchurl.nix

Uses `builtin:fetchurl` to fetch urls.

## mkUserEnvironment.nix

Creates a nix-env compatible profile purely with Nix code.

## toNix.nix

A small library that converts Nix data to a string. It's the equivalent of
`builtins.toJSON` but for Nix code.

## writeText.nix

Writes a string to the `/nix/store`, contrary to `builtins.toFile`.
