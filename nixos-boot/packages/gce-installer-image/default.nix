# GCE (Google Compute Engine) installer image
#
# Builds a minimal bootable disk image for GCE that boots directly into
# the nixos-boot installer. The image uses:
# - EFI boot with systemd-boot
# - The installer UKI as the boot target
# - Minimal partition layout (ESP only, installer runs from initrd)
#
# To deploy to GCE:
#   gsutil cp result/gce-installer.tar.gz gs://$BUCKET/
#   gcloud compute images create nixos-boot-installer \
#     --source-uri=gs://$BUCKET/gce-installer.tar.gz \
#     --guest-os-features=UEFI_COMPATIBLE
#
{
  pkgs,
  perSystem,
  lib ? pkgs.lib,
  ...
}:
let
  # Get the installer UKI with GCE-appropriate settings
  installerUki = perSystem.self.installer-uki.override {
    moduleProfile = "gce"; # GCE-specific: virtio_scsi, nvme, gve, etc.
    extraCmdline = "console=ttyS0"; # GCE serial console
    # Only enable GCE metadata and cmdline config sources (no EC2/file)
    configSources = [
      "config-cmdline"
      "config-gce"
    ];
  };

  inherit (pkgs.stdenv.hostPlatform) efiArch;

  # GCE image format settings
  sectorSize = 512;
  espSizeMB = 256; # Slightly larger ESP for UKI
  espSizeBytes = espSizeMB * 1024 * 1024;

  # Build the disk image using systemd-repart
  diskImage =
    pkgs.runCommand "gce-installer-image"
      {
        nativeBuildInputs = with pkgs; [
          systemd
          dosfstools
          mtools
          fakeroot
          coreutils
          gnutar
          pigz
        ];
      }
      ''
        mkdir -p $out

        # Create repart definition for ESP
        mkdir -p repart.d
        cat > repart.d/00-esp.conf << EOF
        [Partition]
        Type=esp
        Format=vfat
        Label=ESP
        SizeMinBytes=${toString espSizeBytes}
        SizeMaxBytes=${toString espSizeBytes}
        EOF

        # Calculate total disk size (ESP + 1MB alignment)
        disk_size=$((${toString espSizeMB} + 2))M

        # Create the raw disk image
        truncate -s $disk_size disk.raw

        # Use fakeroot to run systemd-repart
        fakeroot systemd-repart \
          --dry-run=no \
          --empty=allow \
          --size=auto \
          --sector-size=${toString sectorSize} \
          --definitions=repart.d \
          --json=pretty \
          disk.raw

        # Mount ESP and copy bootloader + UKI
        esp_offset=$((1024 * 1024)) # 1MB offset for partition alignment

        # Create ESP filesystem image separately
        truncate -s ${toString espSizeBytes} esp.img
        mkfs.vfat -F 32 -n ESP esp.img

        # Copy files into ESP using mtools
        mmd -i esp.img ::/EFI
        mmd -i esp.img ::/EFI/BOOT
        mmd -i esp.img ::/EFI/Linux

        # Copy systemd-boot as the default EFI bootloader
        mcopy -i esp.img \
          ${pkgs.systemd}/lib/systemd/boot/efi/systemd-boot${efiArch}.efi \
          ::/EFI/BOOT/BOOT${lib.toUpper efiArch}.EFI

        # Copy the installer UKI
        mcopy -i esp.img \
          ${installerUki}/${installerUki.filename} \
          ::/EFI/Linux/${installerUki.filename}

        # Create loader.conf for systemd-boot
        echo "default nixos-boot-installer.efi" > loader.conf
        echo "timeout 3" >> loader.conf
        mmd -i esp.img ::/loader || true
        mcopy -i esp.img loader.conf ::/loader/loader.conf

        # Now we need to write the ESP image back into the disk
        # First, let systemd-repart create the partition table
        # Then dd the ESP content

        # Create proper GPT with ESP partition
        ${pkgs.util-linux}/bin/sfdisk disk.raw << EOF
        label: gpt
        first-lba: 2048
        start=2048, size=$((${toString espSizeBytes} / 512)), type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, name="ESP"
        EOF

        # Write ESP filesystem to partition
        dd if=esp.img of=disk.raw bs=512 seek=2048 conv=notrunc

        # Compress for GCE upload (GCE expects tar.gz with disk.raw inside)
        tar --format=oldgnu -Sc disk.raw | pigz > $out/gce-installer.tar.gz

        # Also keep raw image for local testing
        mv disk.raw $out/disk.raw
      '';

in
diskImage
// {
  passthru = {
    inherit installerUki;

    # Script to upload and create GCE image
    uploadScript = pkgs.writeShellApplication {
      name = "upload-gce-image";
      runtimeInputs = [ pkgs.google-cloud-sdk ];
      text = ''
        set -euo pipefail

        if [ $# -lt 2 ]; then
          echo "Usage: upload-gce-image <bucket-name> <image-name>"
          echo "Example: upload-gce-image my-bucket nixos-boot-installer"
          exit 1
        fi

        BUCKET="$1"
        IMAGE_NAME="$2"
        IMAGE_PATH="${diskImage}/gce-installer.tar.gz"

        echo "Uploading image to gs://$BUCKET/..."
        gsutil cp "$IMAGE_PATH" "gs://$BUCKET/gce-installer.tar.gz"

        echo "Creating GCE image $IMAGE_NAME..."
        gcloud compute images create "$IMAGE_NAME" \
          --source-uri="gs://$BUCKET/gce-installer.tar.gz" \
          --guest-os-features=UEFI_COMPATIBLE

        echo "Done! Create a VM with:"
        echo "  gcloud compute instances create my-vm --image=$IMAGE_NAME"
      '';
    };
  };
}
