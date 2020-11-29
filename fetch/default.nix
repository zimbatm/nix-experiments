{ pkgs ? null }:
with builtins;
let
  /*** fetch - the universal fetcher

    fetch is designed to take only pure data in and work with a variety of type
    of sources.
    */
  fetch =
    { type ? "url"
    , # Name of the file. If empty, use the basename of `url`.
      name ? ""
    , # a SRI-hash. Only sha256 hashes are supported for now.
      hash
    , meta ? { }
    , passthru ? { }
    , ...
    }@attrs:
    let
      fetchFn =
        fetchers."${type}"
          or (throw "fetcher ${type} not found");
      fetchAttrs = (removeAttrs attrs [ "type" ]) // {
        # include the default values
        inherit hash passthru meta;
      };
    in
    fetchFn fetchAttrs;

  # a map of all the supported fetchers
  #
  # TODO: make this extensible?
  builtinFetchers = with builtins; {
    url = { url, unpack ? false, hash, meta, passthru }@attrs:
      assert (typeOf url == "string");
      mkFetcher attrs (fetchurl attrs);

    github = { owner, repo, ref ? rev, rev, hash, meta, passthru }@attrs:
      mkFetcher attrs
        (
          fetchurl {
            name = "${owner}-${repo}-${rev}";
            url = "https://github.com/${owner}/${repo}/archive/${ref}.tar.gz";
            unpack = true;
            hash = hash;
          }
        ) // {
        meta = meta // {
          homepage = "https://github.com/${owner}/${repo}";
        };
      };

    git = { url, ref ? rev, rev ? "HEAD", hash, meta, passthru }@attrs:
      mkFetcher attrs (
        fetchGit {
          name = baseNameOf url;
          inherit url ref rev;
        }
      );
  };

  pkgsFetchers = with pkgs; { };

  # extract the sha256 of a SRI-hash
  getSHA256 = sriHash:
    if (substring 0 7 sriHash) == "sha256-" then
    # assuming 1000 > length of sriHash
      substring 7 1000 sriHash
    else
      throw "expected sha256 SRI hash, got ${sriHash}";

  # a version of fetchurl that ressembles more <nix/fetchurl.nix>
  fetchurl = with builtins;
    { url
    , name ? baseNameOf url
    , hash ? null
    , unpack ? false
    , ...
    }:
    (if unpack then fetchTarball else fetchurl)
      (
        { inherit name url; }
        // (
          if hash == null then { } else {
            sha256 = getSHA256 hash;
          }
        )
      );

  # all fetchers can be converted to their outPath, just like derivations
  mkFetcher = attrs: outPath: attrs // {
    __toString = self: "${toString self.outPath}";
    outPath = outPath;
  };

in
fetch
