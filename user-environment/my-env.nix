let
  pkgs = import <nixpkgs> { };
  mkUserEnvironment = pkgs.callPackage ./. { };
in
mkUserEnvironment {
  derivations = [
    # Put the packages that you want in your user environment here
    pkgs.rclone # bin-first
    pkgs.git
    pkgs.groff
    pkgs.vim
  ];
}
