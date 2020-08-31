# nix-path - manage your NIX_PATH

Status: *experimental*

When managing a monorepo, it often happens that one wants to load external
sources. Having them scattered all over the repo can make it quickly difficult
to handle and update them.

This project proposes to put them all in a top-level nix-path.nix file and
a wrapper that extends the NIX_PATH with their values. 

It also does a few other related things like:

* define a better interface for the fetchers
* define an interface for updaters, with common ones included

## Dependencies

* `bash` >= 4
* `gnugrep`
* `jq`
* `nix` >= 2.2

## Out of scope

This project doesn't try to handle constraint solving; this should be
implemented at a higher level. Or reduce the number of dependencies that you
have?

## Related topics

### IFD

Eval vs build time: if one wants to avoid Import From Derivation (IFD), it is
better to fetch the sources at evaluation time.

### Universal fetcher

It would be nice to have a unified interface for the fetchers. It would be
nice to be able to interchangeably swap and eval fetcher with a
derivation fetcher.

### Access credentials

Some 3rd party sources might require user-defined credentials. In that case
the eval fetchers are superior as they all happen on the client side.

## Composable tools

This project is part of the composable tools distribution, where we follow the
UNIX philosophy of building small composable tools.

## TODO

* finalize the universal fetcher interface
* implement derivation fetchers with the same interface
* design the updater interface
* make a derivation out of this project, pin `jq`
