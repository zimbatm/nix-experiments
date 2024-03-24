{ pkgs ? null }:
with builtins;
let
  defaultHash = hash:
    if hash == "" then "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" else hash;

  /*** fetch - the universal fetcher

    fetch is designed to take only pure data in and work with a variety of type
    of sources.
    */
  fetch =
    { url
    , # Name of the file. If empty, use the basename of `url`.
      name ? ""
    , # a SRI-hash. Only sha256 hashes are supported for now.
      hash ? ""
    , meta ? { }
    , passthru ? { }
    }@attrs:
    let
      fetchFn =
        fetchers."${type}"
          or (throw "fetcher ${type} not found");
      fetchAttrs = (removeAttrs attrs [ "type" ]) // {
        # include the default values
        inherit passthru meta;
        hash = defaultHash hash;
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

  # a version of fetchurl that ressembles more <nix/fetchurl.nix>
  fetchurl = with builtins;
    { url
    , name ? baseNameOf url
    , hash ? ""
    , unpack ? false
    , ...
    }:
    (if unpack then fetchTarball else fetchurl)
      {
        inherit name url;
        hash = defaultHash hash;
      };

  # all fetchers can be converted to their outPath, just like derivations
  mkFetcher = attrs: outPath: attrs // {
    __toString = self: "${toString self.outPath}";
    outputs = [ "out" ];
    outPath = outPath;
  };

in
fetch
