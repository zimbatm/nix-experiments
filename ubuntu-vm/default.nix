let
  pkgs = import (builtins.fetchTarball { url = "channel:nixos-19.09"; }) {};
  inherit (pkgs) runCommand;

  img_orig = "ubuntu-18.04-server-cloudimg-amd64.img";
in
rec {
  image = pkgs.fetchurl {
    url = "https://cloud-images.ubuntu.com/releases/18.04/release/${img_orig}";
    hash = "sha256-pyDDQGbc5VIRNMDvpj1STFP0DGjbJM8WHXWTVqJKrQ4=";
  };

  # This is the cloud-init config
  cloudInit = {
    ssh_authorized_keys = [
      (builtins.readFile ./vagrant.pub)
    ];
    password = "ubuntu";
    chpasswd = {
      list = [
        "root:root"
        "ubuntu:ubuntu"
      ];
      expire = false;
    };
    ssh_pwauth = true;
    mounts = [
      [ "hostshare" "/mnt" "9p" "defaults,trans=virtio,version=9p2000.L" ]
    ];
  };

  # Generate the initial user data disk. This containst extra configuration
  # for the VM.
  userdata = runCommand
    "userdata.qcow2"
    { buildInputs = [ pkgs.cloud-utils pkgs.yj pkgs.qemu ]; }
    ''
      {
        echo '#cloud-config'
        echo '${builtins.toJSON cloudInit}' | yj -jy
      } > cloud-init.yaml
      cloud-localds userdata.raw cloud-init.yaml
      qemu-img convert -p -f raw userdata.raw -O qcow2 "$out"
    '';

  runVM = pkgs.writeShellScript "runVM" ''
    #
    # Starts the VM with the given system image
    #
    set -euo pipefail
    image=$1
    userdata=$2
    shift 2

    args=(
        -drive "file=$image,format=qcow2"
        -drive "file=$userdata,format=qcow2"
        -enable-kvm
        -m 2G
        -nographic
        -serial mon:stdio
        -smp 2
        -device "rtl8139,netdev=net0"
        -netdev "user,id=net0,hostfwd=tcp:127.0.0.1:10022-:22"
    )

    set -x
    exec ${pkgs.qemu}/bin/qemu-system-x86_64 "''${args[@]}" "$@"
  '';

  sshClient = pkgs.writeShellScript "sshVM" ''
    sshKey=$(mktemp)
    trap 'rm $sshKey' EXIT
    cp ${./vagrant} "$sshKey"
    chmod 0600 "$sshKey"
    ssh -i "$sshKey" ubuntu@127.0.0.1 -p 10022 "$@"
  '';

  # Prepare the VM snapshot for faster resume.
  prepare = runCommand "prepare"
    { buildInputs = [ pkgs.qemu (pkgs.python.withPackages (p: [ p.pexpect ])) ]; }
    ''
      export LANG=C.UTF-8
      export LC_ALL=C.UTF-8

      # copy the images to work on them
      cp --reflink=auto "${image}" disk.qcow2
      cp --reflink=auto "${userdata}" userdata.qcow2
      chmod +w disk.qcow2 userdata.qcow2

      # Make some room on the root image
      qemu-img resize disk.qcow2 +64G

      # Run the automated installer
      python ${./prepare.py} ${runVM} disk.qcow2 userdata.qcow2

      # At this point the disk should have a named snapshot
      qemu-img snapshot -l disk.qcow2 | grep prepare

      mkdir $out
      mv disk.qcow2 userdata.qcow2 $out/

      cat <<WRAP > $out/runVM
      #!${pkgs.stdenv.shell}
      set -euo pipefail
      # Setup the VM configuration on boot
      cp --reflink=auto "$out/disk.qcow2" disk.qcow2
      cp --reflink=auto "$out/userdata.qcow2" userdata.qcow2
      chmod +w disk.qcow2 userdata.qcow2

      # And finally boot qemu with a bunch of arguments
      args=(
        #-loadvm prepare
        #-vga virtio
        # Share the nix folder with the guest
        -virtfs "local,security_model=passthrough,id=fsdev0,path=\$PWD,readonly,mount_tag=hostshare"
      )

      echo "Starting VM."
      echo "To login: ubuntu / ubuntu"
      echo "To quit: type 'Ctrl+a c' then 'quit'"
      echo "Press enter in a few seconds"
      exec ${runVM} disk.qcow2 userdata.qcow2 "\''${args[@]}"
      WRAP
      chmod +x $out/runVM
    '';

  # TODO: actually inject the installer, boot the VM and run some test
  /*
  test = runCommand "test"
    { __noChroot = true; buildInputs = [ pkgs.curl ]; }
    ''
      curl 1.1.1.1 > $out
    '';
  */
}
