# Demo NixOS system for testing nixos-boot installer
#
# This is a minimal NixOS configuration that can be installed by the
# nixos-boot installer. It's designed for GCE but works on any UEFI system.
#
# Build: nix build .#demo-system
# Push to cache: nix copy --to s3://your-cache?profile=... $(nix build .#demo-system --print-out-paths)
#
{
  pkgs,
  lib ? pkgs.lib,
  ...
}:
let
  # Build a minimal NixOS system using the eval-config approach
  nixos = import "${pkgs.path}/nixos/lib/eval-config.nix" {
    system = pkgs.stdenv.hostPlatform.system;
    modules = [
      (
        {
          config,
          pkgs,
          lib,
          modulesPath,
          ...
        }:
        {
          imports = [
            "${modulesPath}/profiles/minimal.nix"
            "${modulesPath}/profiles/headless.nix"
            "${modulesPath}/profiles/qemu-guest.nix"
          ];

          # Basic system info
          system.stateVersion = "24.11";
          networking.hostName = "nixos-demo";

          # Boot configuration for UEFI
          boot.loader.systemd-boot.enable = true;
          boot.loader.efi.canTouchEfiVariables = true;

          # Use latest kernel
          boot.kernelPackages = pkgs.linuxPackages_latest;

          # Serial console for cloud environments
          boot.kernelParams = [
            "console=ttyS0"
            "console=tty0"
          ];

          # GCE/cloud virtio modules
          boot.initrd.kernelModules = [ "virtio_scsi" ];
          boot.kernelModules = [
            "virtio_pci"
            "virtio_net"
          ];

          # Root filesystem - will be set up by installer
          fileSystems."/" = {
            device = "/dev/disk/by-label/nixos";
            fsType = "ext4";
          };
          fileSystems."/boot" = {
            device = "/dev/disk/by-label/ESP";
            fsType = "vfat";
          };

          # SSH access
          services.openssh = {
            enable = true;
            settings = {
              PasswordAuthentication = false;
              PermitRootLogin = "prohibit-password";
            };
          };

          # Nix settings
          nix.settings = {
            experimental-features = [
              "nix-command"
              "flakes"
            ];
            trusted-users = [
              "root"
              "@wheel"
            ];
          };

          # Basic packages
          environment.systemPackages = with pkgs; [
            vim
            curl
            htop
            git
          ];

          # Create a demo user
          users.users.demo = {
            isNormalUser = true;
            extraGroups = [ "wheel" ];
            # Empty password for demo - DO NOT USE IN PRODUCTION
            initialHashedPassword = "";
          };

          # Allow demo user to sudo without password (for demo only)
          security.sudo.wheelNeedsPassword = false;

          # Firewall
          networking.firewall = {
            enable = true;
            allowedTCPPorts = [ 22 ];
          };

          # Use systemd-networkd for networking
          networking.useNetworkd = true;
          networking.useDHCP = false;
          systemd.network = {
            enable = true;
            networks."10-ens" = {
              matchConfig.Name = "ens* eth*";
              networkConfig = {
                DHCP = "yes";
                IPv6AcceptRA = true;
              };
            };
          };
        }
      )
    ];
  };
in
pkgs.runCommand "demo-system"
  {
    passthru = {
      inherit (nixos) config;
      toplevel = nixos.config.system.build.toplevel;
      # The closure path that needs to be passed to the installer
      closure = nixos.config.system.build.toplevel;
    };
  }
  ''
      mkdir -p $out

      # Create a symlink to the system toplevel
      ln -s ${nixos.config.system.build.toplevel} $out/toplevel

      # Write the store path for easy reference
      echo "${nixos.config.system.build.toplevel}" > $out/closure-path

      # Write a simple info file
      cat > $out/README <<EOF
    Demo NixOS System
    =================

    System closure: ${nixos.config.system.build.toplevel}

    To install this system with nixos-boot:
    1. Push to your binary cache:
       nix copy --to s3://your-cache ${nixos.config.system.build.toplevel}

    2. Boot a machine with the nixos-boot installer and provide:
       - system_closure: ${nixos.config.system.build.toplevel}
       - binary_caches: ["https://your-cache.example.com", "https://cache.nixos.org"]
    EOF
  ''
