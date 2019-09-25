# rehash - Nix CAS for a single drv

**STATUS: success!**

## Motivation

I was looking at [this PR that adds `builtins.intern` to
Nix](https://github.com/NixOS/nix/pull/1502) and wanted to better understand
the implications.

## Intro

Nix always rebuilds derivations whenever any of the inputs has changed. This
is an awesome property because it solves cache invalidation. Completely. Let
me say this again; Nix solves one of the 3 hardest problems in the computing
industry. Ahem :)

    A -> B -> C

Let's say that A, B and C are derivations. Whenever A changes, both B and C
will rebuild. If B changes, only C will rebuild. Here we have a list but
usually the structure is like a tree where A has B', B''... as dependents and
it also goes much deeper.

Now let's imagine that C is a very expensive operations. And when B rebuilds,
it usually spits the same output. C gets the same input but which has a
different name. So in that case Nix will always rebuild. It solves the cache
invalidation but at the expense of more computational overhead.

This project introduces `rehash`. The goal is to avoid rebuilding C if the
output of B is the same.

    A -> B -> rehash(B)-> C

For that we take B, read it's content into a new derivation, and forget where
that data came from.

## Example usage

[$ example.nix](example.nix)
```
let
  pkgs = import <nixpkgs> {};
  rehash = pkgs.callPackage ./. {};
  runCommand = pkgs.runCommand;

  packageA = runCommand "package-a" {} ''
    echo CONTENT > $out
  '';

  packageA' = runCommand "package-a" {} ''
    # the build instructions have changed but the output is the same
    echo CONTENT > $out
  '';

  mkPackageB = { packageA }:
    runCommand "package-b" {} ''
      # depends on package-a
      echo ${packageA} > $out
    '';
in
  rec {
    inherit packageA packageA';

    withoutRehash = {
      packageB = mkPackageB { packageA = packageA; };
      packageB' = mkPackageB { packageA = packageA'; };
    };

    withRehash = {
      packageB = mkPackageB { packageA = rehash packageA; };
      packageB' = mkPackageB { packageA = rehash packageA'; };
    };

    test =
      assert withRehash.packageB.outPath == withRehash.packageB'.outPath;
      true
      ;
  }
```
Package A *needs* to be built before, since Nix doesn't know about the
dependency anymore.
`$ nix-build example.nix -A "packageA" -A "packageA'"`
```
/nix/store/gnaf8p8s8qk68f18f1pqy8cks2la6crc-package-a
/nix/store/dn59yhpf9c9djxfsa1ghkik2vh4g1bmn-package-a
```
These two outputs should be the same:
`$ nix-instantiate example.nix -A withRehash.packageB`
```
/nix/store/imfcfww8z34w7cmzlrl8kaxsiz6b7rzl-package-b.drv
```
`$ nix-instantiate example.nix -A "withRehash.packageB'"`
```
/nix/store/imfcfww8z34w7cmzlrl8kaxsiz6b7rzl-package-b.drv
```
## TODO

See if `builtins.seq` could be used to force the build of packageA.

## Known issues

This is not the existential store, it's just a hack.

The input drv **must** be realized into the store before instantiating the
rehashed drv. This is a bit cumbersome because it requires to keep track of
that. Otherwise you will see an error such as:

> error: getting attributes of path '/nix/store/gnaf8p8s8qk68f18f1pqy8cks2la6crc-package-a': No such file or directory

Rebuild will happen if the drv name changes.

There are also a few dependencies like stdenv, default-builder and bash that
will still cause a rebuild.

This approach doesn't work if the input drv contains self-references because
it's content will always be different between rebuilds.
