# UKI (Unified Kernel Image) generation library
#
# Builds a UKI containing:
# - Linux kernel
# - Initrd with installer agent
# - Kernel command line
# - OS release info
{
  lib,
  pkgs,
  ...
}:
let
  inherit (pkgs.stdenv.hostPlatform) efiArch;

  # INI format for ukify config
  format = pkgs.formats.ini { };
in
{
  # Build a minimal UKI for the installer
  #
  # Arguments:
  # - name: Name of the UKI (used for filename)
  # - kernel: Linux kernel package
  # - initrd: Initrd derivation
  # - cmdline: Kernel command line string
  # - osRelease: Optional os-release file path
  # - version: Optional version string
  buildUKI =
    {
      name ? "nixos-boot",
      kernel,
      initrd,
      cmdline ? "",
      osRelease ? null,
      version ? null,
    }:
    let
      ukifyConfig = format.generate "ukify.conf" {
        UKI = {
          Linux = "${kernel}/${kernel.file or "bzImage"}";
          Initrd = "${initrd}";
          Cmdline = cmdline;
          Stub = "${pkgs.systemd}/lib/systemd/boot/efi/linux${efiArch}.efi.stub";
          Uname = kernel.modDirVersion or kernel.version or "unknown";
          EFIArch = efiArch;
        }
        // lib.optionalAttrs (osRelease != null) { OSRelease = "@${osRelease}"; };
      };

      versionSuffix = lib.optionalString (version != null) "_${version}";
      filename = "${name}${versionSuffix}.efi";
    in
    pkgs.runCommand filename
      {
        nativeBuildInputs = [ pkgs.buildPackages.systemdUkify ];
        passthru = {
          inherit
            filename
            kernel
            initrd
            cmdline
            ;
        };
      }
      ''
        mkdir -p $out
        ukify build \
          --config=${ukifyConfig} \
          --output="$out/${filename}"
      '';

  # Build a minimal initrd with the installer agent
  #
  # This creates a small initrd focused on:
  # - Network boot
  # - Disk partitioning
  # - Nix store fetch
  buildInstallerInitrd =
    {
      installerPackage,
      extraPackages ? [ ],
      extraModules ? [ ],
    }:
    let
      # Use makeInitrdNG for modern initrd
      initrd = pkgs.makeInitrdNG {
        compressor = "zstd";
        contents = [
          # The installer binary
          {
            source = "${installerPackage}/bin/nixos-boot-installer";
            target = "/init";
          }
          # Basic shell for debugging
          {
            source = "${pkgs.busybox}/bin/busybox";
            target = "/bin/busybox";
          }
        ];
      };
    in
    "${initrd}/initrd";

  # Generate an os-release file for the installer
  mkOsRelease =
    {
      name ? "NixOS Boot Installer",
      version ? "0.1.0",
      id ? "nixos-boot",
    }:
    pkgs.writeText "os-release" ''
      NAME="${name}"
      ID=${id}
      VERSION="${version}"
      VERSION_ID="${version}"
      PRETTY_NAME="${name} ${version}"
    '';
}
