# Fetch doggy, fetch!

This project is trying to solve fetching and updating of package sources for
nix. Never copy a sha256 again!

nixpkgs has a collection of fetchers such as `pkgs.fetchurl`,
`pkgs.fetchFromGitHub`, `pkgs.fetchGit`, ...
Then nix itself has another set of fetchers such as `builtins.fetchurl`,
`builtins.fetchTarball`, `builtins.fetchGit`, ...

All those fetchers have slightly different behaviours and outputs. Wouldn't it
be great if they all behaved the same?

## Design goals

* use SRI hashes everywhere
* often the fetcher knows about the package version and should re-export it
* often the fetcher knows about the package homepage and should re-export it
* it should be possible to build a generic updater tool
* a fetcher should optionally support the `meta` attribute to attach extra
  meta-data.
* ideally, the user can select build-time or eval-time as an independent
  attribute.
* all fetchers should look like derivations even if they are eval-time
* support for mirrors
* empty SRI hash should default to AAAAAA...

Hopefully one day this will be available in nixpkgs.

## Updater

package metadata

versions list. Paginated. Eg:
* git tags
* commit ids
* ???

url template (thanks niv). Map versions to URLs.

update strategy. Eg:
* follow releases
*



