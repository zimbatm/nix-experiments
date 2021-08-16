let
  pkgs = import (builtins.fetchTarball { url = "channel:nixos-21.05"; }) { };
  inherit (pkgs) runCommand;
in
rec {
  image = pkgs.fetchurl {
    url = "https://mirrors.xtom.com/osdn//android-x86/69704/android-x86-8.1-r6.iso";
    hash = "sha256-9YrtTRlhOG7zl0FJycdK+N27RgSn5PV3KlvzfXgODu4=";
  };

  config = {
    cpus = 2;
    memory = "8G";
    disk = "32G";
  };

  runVM = pkgs.writeShellScript "runVM" ''
    #
    # Starts the VM
    #
    set -euo pipefail

    export PATH=${pkgs.qemu}/bin
    image=androidx86_hda.img

    if [[ ! -f "$image" ]]; then
      qemu-img create -f qcow2 "$image" ${toString config.disk}
    fi

    args=(
      -enable-kvm
      -m ${toString config.memory}
      -smp ${toString config.cpus}
      -cpu host
      -device ES1370
      -device virtio-mouse-pci
      -device virtio-keyboard-pci
      -serial mon:stdio
      -boot menu=on
      -net nic
      -net user,hostfwd=tcp::5555-:22
      # -device virtio-vga,virgl=on
      # -display gtk,gl=on
      -hda "$image"
      -cdrom ${image}
    )

    set -x
    exec qemu-system-x86_64 "''${args[@]}" "$@"
  '';
}
