nix-src - generic fetcher and pinning tool
==========================================

**STATUS: ALPHA**

Updating sources and sha256 is a big part of the nix development
lifecycle. This tool can be used to automate that process. Never
copy-and-paste sha256 again and automate nixpkgs pinning updates to
get the last security updates.

Compatible with Nix 2.0

Problem
-------

Let's start with a simple package:

```nix
{ stdenv, fetchFromGitHub }:
stdenv.mkDerivation rec {
  name = "foo-${version}";
  version = "0.1.0";

  src = fetchFromGitHub {
    owner = "myuser";
    repo = "foo";
    rev = "v${version}";
    sha256 = "<sha256>";
  };
}
```

Over time, multiple tasks have to be performed to keep the package fresh.

* find out if there is an update available
* update the `version` and `sha256` variables accordingly.

Usually the version is updated first, then touch the sha256 and run `nix-build` or if you are lucky `nix-prefetch-url -A foo.src` to find the new sha256. Then finally go back into the code to update it.

In some cases there are multiple sources for multiple architectures and the task becomes more complicated.

Proposal
--------

Update the nix code to:

```nix
{ stdenv, fetchSrc }:
stdenv.mkDerivation rec {
  src = fetchSrc ./default.src.json;
  name = "foo-${src.meta.version}";
}
```

The accompanying JSON would look like this:

```json
{
  "fetcher": "fetchFromGitHub",
  "src": {
    "owner": "myuser",
    "repo": "foo",
    "rev": "<sha1>",
    "sha256": "<sha256>",
    "meta": {
      "version": "0.1.0"
    }
  }
}
```

Nixpkgs pinning
---------------

Another problem is to pin nixpkgs and do regular updates for security releases. In that case the tool provides a pure and dependency less boilerplate default.nix:

```nix
# TODO: nixpkgs pinning script
```

The accompanying JSON would look like this:

```json
{
  "type": "github",
  "updater": {
    "type": "branch",
    "owner": "nixos",
    "repo": "nixpkgs-channels",
    "branch": "nixos-17.09"
  },
  "version": "<commit-date>",
  "src": {
    "url": "<url>",
    "sha256": "<sha256>"
  }
}
```

Composable (multiple architectures)
-----------------------------------

And finally, some sources are custom.

```json
{
  "type": "custom",
  "updater": {
    "type": "script",
    "path": "./myupdater"
  },
  "version": "<version>",
  "src": {
    "type": "arch",
    "x86_64-linux": {
      "url": "https://...",
      "sha256": "<sha256>"
    }
  }
}
```

Nix fetcher interface
-----------------

A fetcher is a function that takes a number of arguments including a sha256 argument and returns the fetched source as a derivation output.

The fetcher MUST pass through the meta attributes (potentially mixed by it's own).

fetchSrc interface
------------------

```
fetchSrc path => derivation { version, homepage }
```

TODO: what happens for arch selection, is it possible to access the other archs?

Usage
-----

    Usage:
      nix-src [options] <command>
      nix-src init [options]
      nix-src list [options]
      nix-src update [options]
      nix-src select <version|--latest>
      nix-src help | -h | --help

    Commands:
      help
      init
      list
      update

    Options:
      --path <path>       Path to .nix file
      --repo <owner/repo>
      --ref <branch>
      --method <pure | nixpkgs>
      -h --help  Show this screen.

Examples
--------

    nix-src update

TODO
----

Recursive update scenarios. Go to the top of the tree and update all the packages.

License
-------

Copyright 2018 zimbatm and contributors. Licenses under the ISC. See LICENSE.txt
