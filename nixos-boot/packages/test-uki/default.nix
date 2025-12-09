# Test UKI for development
#
# This creates a minimal bootable UKI for testing the build process
# and QEMU boot framework.
{
  pkgs,
  lib ? pkgs.lib,
  linuxPackages ? pkgs.linuxPackages_latest,
  ...
}:
let
  # Use static busybox so we don't need glibc
  busyboxStatic = pkgs.pkgsStatic.busybox;

  # Build initrd with proper structure
  initrdContents = pkgs.runCommand "initrd-contents" { } ''
        mkdir -p $out/bin $out/sbin $out/dev $out/proc $out/sys $out/tmp $out/run

        # Copy static busybox
        cp ${busyboxStatic}/bin/busybox $out/bin/busybox
        chmod +x $out/bin/busybox

        # Create symlinks for all busybox applets we need
        for cmd in sh ash ls cat echo mount mkdir ln uname date poweroff reboot sleep; do
          ln -sf busybox $out/bin/$cmd
        done
        ln -sf ../bin/busybox $out/sbin/init

        # Create init script - use heredoc without leading spaces
        cat > $out/init << 'INIT_EOF'
    #!/bin/sh
    # Mount essential filesystems
    mount -t proc proc /proc
    mount -t sysfs sysfs /sys
    mount -t devtmpfs devtmpfs /dev

    echo "====================================="
    echo "  NixOS Boot Installer Test Image"
    echo "====================================="
    echo ""
    echo "Kernel: $(uname -r)"
    echo "Date: $(date 2>/dev/null || echo 'unavailable')"
    echo ""
    echo "Test successful - system booted!"
    echo ""
    echo "Dropping to shell..."
    exec /bin/sh
    INIT_EOF
        chmod +x $out/init
  '';

  # Create the initrd cpio archive
  initrd = pkgs.runCommand "initrd" { } ''
    mkdir -p $out
    cd ${initrdContents}
    find . -print0 | ${pkgs.cpio}/bin/cpio -o0H newc | ${pkgs.zstd}/bin/zstd > $out/initrd
  '';

  osRelease = pkgs.writeText "os-release" ''
    NAME="NixOS Boot Test"
    ID=nixos-boot-test
    VERSION="0.1.0"
    VERSION_ID="0.1.0"
    PRETTY_NAME="NixOS Boot Test Image 0.1.0"
  '';

  inherit (pkgs.stdenv.hostPlatform) efiArch;

  ukifyConfig = (pkgs.formats.ini { }).generate "ukify.conf" {
    UKI = {
      Linux = "${linuxPackages.kernel}/${linuxPackages.kernel.file or "bzImage"}";
      Initrd = "${initrd}/initrd";
      # Put ttyS0 last so it becomes the primary console for userspace output
      Cmdline = "console=tty0 console=ttyS0 loglevel=7";
      Stub = "${pkgs.systemd}/lib/systemd/boot/efi/linux${efiArch}.efi.stub";
      Uname = linuxPackages.kernel.modDirVersion;
      OSRelease = "@${osRelease}";
      EFIArch = efiArch;
    };
  };

  filename = "nixos-boot-test.efi";
in
pkgs.runCommand filename
  {
    nativeBuildInputs = [ pkgs.systemdUkify ];
    passthru = {
      inherit filename;
      kernel = linuxPackages.kernel;
      inherit initrd osRelease;
    };
  }
  ''
    mkdir -p $out
    ukify build \
      --config=${ukifyConfig} \
      --output="$out/${filename}"
  ''
