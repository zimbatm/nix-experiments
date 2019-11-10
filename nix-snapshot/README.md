# nix-snapshot

When using nix in a developer environment, nixpkgs doesn't change a lot. But
it takes a long time to evaluate.

If we can assume that:

* the nix source path doesn't change often
* the nix source path is self-contained
* only outputs are consumed later on

The it's possible to cache the outputs by using the source path as the hash
key.

Ideal to use with incremental rebuilders.

## Brain dump

The outputs need to be recorded as GC roots. How to GC the cache?

## Usage

```
Usage: nix-snapshot <path> [<attribute>...]
```


## TODO

* restore the cache dir
* create a CLI
* create a lib to import from the cache dir

* is it possible to use the nix store as a cache store so that `nix-store
    --gc` is also handled for this project?
