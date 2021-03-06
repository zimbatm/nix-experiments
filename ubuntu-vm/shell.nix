let
  pkgs = import (builtins.fetchTarball { url = "channel:nixos-20.09"; }) { };
in
pkgs.mkShell {
  buildInputs = with pkgs; [
    (pkgs.python.withPackages (p: [ p.pexpect ]))
    pkgs.qemu
  ];
}
