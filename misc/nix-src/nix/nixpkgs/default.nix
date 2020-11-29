let
  fetchers = {
    # Light version
    fetchFromGitHub = { owner, repo, rev, sha256, meta }:
      let
        homepage = "https://github.com/${owner}/${repo}";
        url = "${homepage}/archive/${rev}.tar.gz";
      in
      builtins.fetchTarball { inherit url sha256; };
  };

  fetchSrc = import ../fetchSrc.nix { inherit fetchers; };
in
fetchSrc ./default.src.json
