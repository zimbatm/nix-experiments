# Example

This shows an example usage of `devenv-light`.

## Usage

Run `FLAKE_ROOT=$PWD nix run .#devenv` to enter the developer shell.

Run `FLAKE_ROOT=$PWD nix run .#devenv -- echo OK` to run `echo OK` inside of the developer
shell.

Or use `./devenv.sh`, a small wrapper that shorten those commands (imagine this is the installable wrapper).

```
Usage: FLAKE_ROOT=$PWD nix run .#devenv -- [<cmd> [<args...>]
```
