# devenv - the bash IDE :)

devenv is an integrated solution to manage project dependencies.

## Features

* bootstrap: nix version checker
* bootstrap: nix profile
* mkProfile
* mkLazyBin
* direnv setup
* don't touch the user $HOME

## `./env.sh` interface

A project or folder containing may contain a `./env.sh` file. That file
contains the shell code that can be used to configure the developer environment
of that project or folder when sourced with bash.

`source ./env.sh` should only export environment variables and limit it's
polluting of function, variables and aliases namespaces.

Alternatively, `./env.sh <command> <args>...` can be used to load the
environment and exec the given command with arguments into it.

Minimal implementation:
`./env.sh`
```sh
#!/usr/bin/env bash
# Usage: ./env.sh [<command> <args>...]
env_root=$(dirname "${BASH_SOURCE[0]}")

# configure the environment here
export PATH=$env_root/bin:

# if not being sourced
if ! (return &>/dev/null); then
  if [[ $# -gt 0 ]]; then
    # run the given command
    exec "$@"
  else
    echo "$0: missing command" >&2
    exit 1
  fi
fi
```

## `./fmt.sh` interface

A project root might contain a `./fmt.sh` command. When executed, that command
will format *all* of the code of the project, using whatever code formatter it
wants to use.

This command should take less than a second to execute. Code formatting
isn't such a difficult task that it should take longer on normal-sized
repositories.

In cases where a file is unparseable, `./fmt.sh` may exit with a non-zero exit
status.

TODO: define the output format

## `./test.sh` interface

TODO

## Notes

* requires Nix to be installed globally
* TODO: check if the user is trusted-user or not
* TODO: support for remote builders
* add an example folder to separate usage


default.nix with ./.

don't like bash script. env.sh is too big. => create a binary

direnv env.sh ->

## TODO

* nix binary cache

* env.ps1
* lazy bin nix eval caching
* self-upgrade
* per project bootstrap
* bootstrap nix installer
* tests
* NixOS module system
* init function to get started
* docker dev environment
* docker remote builder
* hydra integration
* cache auto-GC
* overrides when the user wants to use their own binaries
* integrate with LSP
* integrate with lorri
* integrate with niv

## Related projects

* https://github.com/tweag/lorri
* https://github.com/nmattia/niv
* https://github.com/mozilla/release-services/blob/master/please
* http://testanything.org/ - Test Anything Protocol
