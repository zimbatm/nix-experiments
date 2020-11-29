self: super: {
  nix-src = super.python3Packages.callPackage ./nix-src { };

  #####

  fetchSrc = super.callPackage ./fetchSrc.nix { fetchers = super; };
  ;

  fetchers = {
    github = { url, sha256 }:

      ;
      }


      }
      }
