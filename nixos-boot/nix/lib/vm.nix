# QEMU VM test framework for nixos-boot
#
# Provides utilities for:
# - Running UKIs in QEMU (UEFI mode)
# - Creating virtual disks
# - Automated testing
{
  lib,
  pkgs,
  ...
}:
let
  # Path to OVMF firmware
  ovmf = pkgs.OVMF.fd;
in
{
  # Create a script to run a UKI in QEMU
  #
  # Arguments:
  # - uki: The UKI derivation from buildUKI
  # - memory: RAM size (default "2G")
  # - diskSize: Virtual disk size (default "10G")
  # - extraQemuArgs: Additional QEMU arguments
  # - serial: Enable serial console (default true)
  mkRunScript =
    {
      uki,
      memory ? "2G",
      diskSize ? "10G",
      extraQemuArgs ? [ ],
      serial ? true,
      graphics ? false,
      enableKVM ? true,
    }:
    let
      qemuBin =
        if enableKVM then
          "${pkgs.qemu_kvm}/bin/qemu-system-x86_64"
        else
          "${pkgs.qemu}/bin/qemu-system-x86_64";

      serialArgs = lib.optionals serial [
        "-serial"
        "stdio"
        "-append"
        "console=ttyS0"
      ];

      graphicsArgs =
        if graphics then
          [ ]
        else
          [
            "-nographic"
            "-vga"
            "none"
          ];

      kvmArgs = lib.optionals enableKVM [
        "-enable-kvm"
        "-cpu"
        "host"
      ];
    in
    pkgs.writeShellScript "run-nixos-boot-vm" ''
      set -euo pipefail

      # Create a temporary directory for VM state
      vm_dir=$(mktemp -d)
      trap 'rm -rf "$vm_dir"' EXIT

      # Create the virtual disk
      ${pkgs.qemu}/bin/qemu-img create -f qcow2 "$vm_dir/disk.qcow2" ${diskSize}

      # Copy OVMF vars (needs to be writable)
      cp ${ovmf}/FV/OVMF_VARS.fd "$vm_dir/OVMF_VARS.fd"
      chmod +w "$vm_dir/OVMF_VARS.fd"

      echo "Starting QEMU with UKI: ${uki}/${uki.filename}"
      echo "Virtual disk: $vm_dir/disk.qcow2 (${diskSize})"
      echo "Press Ctrl-A X to exit"
      echo

      exec ${qemuBin} \
        -m ${memory} \
        ${lib.concatStringsSep " " kvmArgs} \
        -drive if=pflash,format=raw,readonly=on,file=${ovmf}/FV/OVMF_CODE.fd \
        -drive if=pflash,format=raw,file="$vm_dir/OVMF_VARS.fd" \
        -drive file="$vm_dir/disk.qcow2",format=qcow2,if=virtio \
        -kernel ${uki}/${uki.filename} \
        -nic user,model=virtio-net-pci \
        ${lib.concatStringsSep " " serialArgs} \
        ${lib.concatStringsSep " " graphicsArgs} \
        ${lib.concatStringsSep " " extraQemuArgs}
    '';

  # Create a test that boots the UKI and runs assertions
  #
  # This is for automated testing - boots the VM and checks output
  mkBootTest =
    {
      name,
      uki,
      timeout ? 120,
      expectedOutput ? [ ],
      script ? "",
    }:
    pkgs.runCommand "nixos-boot-test-${name}"
      {
        nativeBuildInputs = [
          pkgs.qemu_kvm
          pkgs.expect
        ];
        passthru = {
          inherit uki;
        };
      }
      ''
        mkdir -p $out

        # Create virtual disk
        qemu-img create -f qcow2 disk.qcow2 10G

        # Copy OVMF vars
        cp ${ovmf}/FV/OVMF_VARS.fd OVMF_VARS.fd
        chmod +w OVMF_VARS.fd

        # Create expect script for automated testing
        cat > test.exp << 'EXPECT_EOF'
        set timeout ${toString timeout}

        spawn qemu-system-x86_64 \
          -m 2G \
          -enable-kvm \
          -cpu host \
          -drive if=pflash,format=raw,readonly=on,file=${ovmf}/FV/OVMF_CODE.fd \
          -drive if=pflash,format=raw,file=OVMF_VARS.fd \
          -drive file=disk.qcow2,format=qcow2,if=virtio \
          -kernel ${uki}/${uki.filename} \
          -nic user,model=virtio-net-pci \
          -serial stdio \
          -nographic \
          -vga none

        ${lib.concatMapStringsSep "\n" (pattern: ''
          expect {
            "${pattern}" { puts "PASS: Found '${pattern}'" }
            timeout { puts "FAIL: Timeout waiting for '${pattern}'"; exit 1 }
            eof { puts "FAIL: EOF before '${pattern}'"; exit 1 }
          }
        '') expectedOutput}

        ${script}

        # Send shutdown command
        send "\x01x"
        expect eof
        EXPECT_EOF

        # Run the test
        expect test.exp | tee $out/test.log

        echo "Test ${name} passed!" > $out/result
      '';

  # Create a bootable disk image with the UKI
  #
  # This creates a GPT disk with ESP containing the UKI
  mkBootableImage =
    {
      uki,
      size ? "512M",
      name ? "nixos-boot.img",
    }:
    pkgs.runCommand name
      {
        nativeBuildInputs = [
          pkgs.dosfstools
          pkgs.mtools
          pkgs.util-linux
        ];
      }
      ''
        # Create the disk image
        truncate -s ${size} $out

        # Create GPT partition table with ESP
        ${pkgs.util-linux}/bin/sfdisk $out << EOF
        label: gpt
        type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, size=+
        EOF

        # Get the partition offset
        offset=$(${pkgs.util-linux}/bin/sfdisk -J $out | ${pkgs.jq}/bin/jq '.partitiontable.partitions[0].start * 512')

        # Create FAT32 filesystem in the partition
        truncate -s $((${size} - 1048576)) esp.img
        mkfs.vfat -F 32 esp.img

        # Create EFI directory structure and copy UKI
        mmd -i esp.img ::EFI
        mmd -i esp.img ::EFI/BOOT
        mcopy -i esp.img ${uki}/${uki.filename} ::EFI/BOOT/BOOTX64.EFI

        # Write the ESP to the disk image
        dd if=esp.img of=$out bs=512 seek=$((offset / 512)) conv=notrunc
      '';
}
