# This file contains all the developer environment tools
{ system ? builtins.currentSystem }:
let
  pkgs = import ./nix { inherit system; };

  inherit (pkgs)
    mkProfile
    sources
    ;

  bats-helpers = pkgs.runCommand "bats-helpers" {} ''
    mkdir -p $out/bin
    cat <<'HELPERS' > $out/bin/bats-helpers
    #!${pkgs.stdenv.shell}
    if [[ ''${BASH_SOURCE[0]} = "$0" ]]; then
      echo "$0"
      exit
    fi
    load '${sources.bats-support}/load.bash'
    load '${sources.bats-assert}/load.bash'
    load '${sources.bats-file}/load.bash'
    HELPERS
    chmod +x $out/bin/bats-helpers
  '';
in
mkProfile {
  packages = with pkgs; [
    go
    bats
    bats-helpers
    #cachix
    file
    findutils
    niv
    shellcheck
    #shfmt
    # (mkLazyBin {
    #   drv = niv;
    #   nixSrc = ./.;
    #   nixAttr = "niv";
    # })
    # (mkLazyBin {
    #   drv = cachix;
    #   nixSrc = ./.;
    #   nixAttr = "cachix";
    # })
  ];
  profile = ''
    export GO111MODULE=on
    export NIX_PATH=nixpkgs=${pkgs.path}
    export PATH=$PROFILE_ROOT/bin:$PATH
    unset GOPATH GOROOT
  '';
}
