# Minimal bootable UKI with the Rust installer
#
# This creates a bootable UKI with:
# - Static busybox as init
# - Module loading via kmod
# - DHCP via busybox udhcpc
# - The Rust installer binary
#
# Configuration sources can be selected via configSources:
# - config-cmdline: Kernel command line parameters (nixos-boot.*)
# - config-gce: Google Compute Engine metadata service
# - config-ec2: AWS EC2 instance metadata service (IMDSv2)
# - config-file: Cloud-init style file-based user-data
# - config-interactive: Interactive console/web UI configuration
{
  pkgs,
  perSystem,
  lib ? pkgs.lib,
  # Module profile: "minimal" (virtio only), "gce", "server", "desktop"
  moduleProfile ? "minimal",
  # Extra kernel cmdline parameters (for testing)
  extraCmdline ? "",
  # Configuration source features to enable
  configSources ? [
    "config-cmdline"
    "config-gce"
    "config-ec2"
    "config-file"
    "config-interactive"
  ],
  ...
}:
let
  # Get the static installer with the specified config sources
  installer = perSystem.self.nixos-boot-installer-static.override {
    inherit configSources;
  };

  # Use standard kernel
  kernelPackages = pkgs.linuxPackages_latest;
  kernel = kernelPackages.kernel;

  # Static busybox for init and utilities
  busyboxStatic = pkgs.pkgsStatic.busybox;

  # Define module profiles for different deployment targets
  # Only list "root" modules - dependencies are automatically resolved by makeModulesClosure
  # Note: Bus drivers (like virtio_pci) must be listed explicitly - they're not dependencies
  #       of device drivers, they're needed for device discovery
  # Note: NLS codepage modules (nls_cp437, nls_iso8859_1) must be listed for FAT/vfat mount
  moduleProfiles = {
    # Minimal: virtio only, for QEMU/KVM/cloud VMs
    minimal = [
      "af_packet"
      "virtio_pci" # Bus driver for virtio device discovery
      "virtio_net"
      "virtio_blk"
      "virtio_console"
      "ext4"
      "vfat"
      "nls_cp437" # Required for FAT codepage support
      "nls_iso8859_1" # Required for FAT iocharset support
    ];

    # GCE: Google Compute Engine specific modules
    gce = [
      "af_packet"
      "virtio_pci"
      "virtio_scsi"
      "sd_mod" # SCSI disk driver - creates /dev/sd* device nodes
      "nvme"
      "virtio_net"
      "gve"
      "virtio_console"
      "virtio_balloon"
      "ext4"
      "vfat"
      "nls_cp437"
      "nls_iso8859_1"
    ];

    # Server: virtio + common server/cloud hardware
    server = [
      "af_packet"
      "virtio_pci"
      "virtio_blk"
      "virtio_scsi"
      "sd_mod" # SCSI disk driver - creates /dev/sd* device nodes
      "virtio_net"
      "virtio_console"
      "e1000"
      "e1000e"
      "igb"
      "ixgbe"
      "gve"
      "nvme"
      "ahci"
      "ext4"
      "vfat"
      "nls_cp437"
      "nls_iso8859_1"
    ];

    # Desktop/USB: broad hardware support
    desktop = [
      "virtio_pci"
      "virtio_blk"
      "virtio_net"
      "virtio_console"
      "xhci_pci"
      "ehci_pci"
      "uhci_hcd"
      "usbhid"
      "usb_storage"
      "sd_mod"
      "nvme"
      "ahci"
      "e1000"
      "e1000e"
      "r8169"
      "iwlwifi"
      "iwlmvm"
      "ath9k"
      "rtw88_pci"
      "ext4"
      "vfat"
      "nls_cp437"
      "nls_iso8859_1"
    ];
  };

  selectedModules = moduleProfiles.${moduleProfile} or moduleProfiles.minimal;

  # Empty firmware directory for makeModulesClosure
  emptyFirmware = pkgs.runCommand "empty-firmware" { } ''
    mkdir -p $out/lib/firmware
  '';

  # Use makeModulesClosure to automatically resolve dependencies
  # This copies only needed modules and generates proper modules.dep
  modulesClosure = pkgs.makeModulesClosure {
    kernel = kernel.modules;
    rootModules = selectedModules;
    firmware = emptyFirmware;
    allowMissing = true; # Some modules may be built-in
  };

  # Static nix for store operations
  nixStatic = builtins.fetchClosure {
    fromStore = "https://cache.nixos.org";
    fromPath = /nix/store/vy6a19illla21fmm9shgzgrwxqgpra5j-nix-static-x86_64-unknown-linux-musl-2.28.5;
    inputAddressed = true;
  };

  # Static partitioning tools
  utilLinuxStatic = pkgs.pkgsStatic.util-linux;
  e2fsprogsStatic = pkgs.pkgsStatic.e2fsprogs;
  dosfstoolsStatic = pkgs.pkgsStatic.dosfstools;

  # Build initrd contents
  initrdContents =
    pkgs.runCommand "installer-initrd-contents"
      { }
      ''
            mkdir -p $out/{bin,sbin,lib,etc,dev,proc,sys,tmp,run,var,root,mnt}

            # Create necessary symlinks
            ln -sf ../run $out/var/run

            # Copy static busybox
            cp ${busyboxStatic}/bin/busybox $out/bin/busybox
            chmod +x $out/bin/busybox

            # Create symlinks for all busybox applets we need
            for cmd in sh ash ls cat echo mount mkdir ln rm cp mv grep sed awk sleep \
                       dmesg ps kill hostname uname date reboot poweroff ip udhcpc \
                       mknod chroot switch_root head tail tr ping wget; do
              ln -sf busybox $out/bin/$cmd
            done

            # Copy the installer binary
            cp ${installer}/bin/nixos-boot-installer $out/bin/nixos-boot-installer
            chmod +x $out/bin/nixos-boot-installer

            # Copy static nix for store operations
            cp ${nixStatic}/bin/nix $out/bin/nix
            chmod +x $out/bin/nix
            ln -sf nix $out/bin/nix-store
            ln -sf nix $out/bin/nix-env
            ln -sf nix $out/bin/nix-build

            # Copy static partitioning tools (skip if already exists as busybox symlink)
            for tool in sfdisk lsblk blkid blockdev wipefs; do
              if [ -f ${utilLinuxStatic}/bin/$tool ] && [ ! -e $out/bin/$tool ]; then
                cp ${utilLinuxStatic}/bin/$tool $out/bin/
                chmod +x $out/bin/$tool
              fi
            done

            # From e2fsprogs: mkfs.ext4
            cp ${e2fsprogsStatic}/bin/mke2fs $out/bin/mke2fs
            chmod +x $out/bin/mke2fs
            ln -sf mke2fs $out/bin/mkfs.ext4
            ln -sf mke2fs $out/bin/mkfs.ext3

            # From dosfstools: mkfs.vfat
            cp ${dosfstoolsStatic}/sbin/mkfs.fat $out/sbin/mkfs.fat
            chmod +x $out/sbin/mkfs.fat
            ln -sf mkfs.fat $out/sbin/mkfs.vfat

            # Copy kmod for module loading (static)
            cp ${pkgs.pkgsStatic.kmod}/bin/kmod $out/bin/kmod
            chmod +x $out/bin/kmod
            ln -sf kmod $out/bin/modprobe
            ln -sf kmod $out/sbin/modprobe
            ln -sf kmod $out/bin/lsmod
            ln -sf kmod $out/bin/insmod
            ln -sf kmod $out/bin/rmmod
            ln -sf kmod $out/bin/depmod

            # Copy kernel modules from makeModulesClosure (includes dependencies + modules.dep)
            echo "Copying kernel modules from modulesClosure for profile: ${moduleProfile}"
            cp -r ${modulesClosure}/lib/modules $out/lib/
            echo "Modules copied with automatic dependency resolution"

            # Create /etc files
            echo "root:x:0:0:root:/root:/bin/sh" > $out/etc/passwd
            echo "root:x:0:" > $out/etc/group

            # Copy CA certificates for SSL/TLS (needed for HTTPS caches)
            mkdir -p $out/etc/ssl/certs
            cp ${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt $out/etc/ssl/certs/ca-bundle.crt
            ln -sf ca-bundle.crt $out/etc/ssl/certs/ca-certificates.crt

            # Create udhcpc script to configure the interface
            mkdir -p $out/usr/share/udhcpc
            cat > $out/usr/share/udhcpc/default.script << 'UDHCPC_EOF'
        #!/bin/sh
        # udhcpc script to configure network interface
        # Supports both standard DHCP and GCE's special /32 subnet with link-local gateway

        log() {
            echo "[udhcpc] $*"
        }

        # Convert subnet mask to CIDR prefix length
        mask_to_cidr() {
            local mask="$1"
            local cidr=0
            local octet
            for octet in $(echo "$mask" | tr '.' ' '); do
                case "$octet" in
                    255) cidr=$((cidr + 8)) ;;
                    254) cidr=$((cidr + 7)) ;;
                    252) cidr=$((cidr + 6)) ;;
                    248) cidr=$((cidr + 5)) ;;
                    240) cidr=$((cidr + 4)) ;;
                    224) cidr=$((cidr + 3)) ;;
                    192) cidr=$((cidr + 2)) ;;
                    128) cidr=$((cidr + 1)) ;;
                    0) ;;
                esac
            done
            echo "$cidr"
        }

        log "Action: $1, interface: $interface"

        case "$1" in
            deconfig)
                log "Deconfiguring interface"
                ip addr flush dev "$interface"
                ip link set "$interface" up
                ;;
            renew|bound)
                log "Configuring: ip=$ip subnet=$subnet router=$router dns=$dns"

                # Convert subnet mask to CIDR
                if [ -n "$subnet" ]; then
                    prefix=$(mask_to_cidr "$subnet")
                else
                    prefix=24
                fi
                log "Calculated prefix: /$prefix"

                ip addr flush dev "$interface"
                ip addr add "$ip/$prefix" dev "$interface"
                log "Address configured: $ip/$prefix"

                if [ -n "$router" ]; then
                    # GCE uses /32 subnets with a link-local gateway (169.254.1.1)
                    # For /32, we need to add a route to the gateway first
                    if [ "$prefix" = "32" ]; then
                        log "Detected /32 subnet (GCE-style), adding route to gateway first"
                        ip route add "$router" dev "$interface" scope link
                    fi
                    ip route add default via "$router" dev "$interface"
                    if [ $? -eq 0 ]; then
                        log "Default route added via $router"
                    else
                        log "ERROR: Failed to add default route via $router"
                        # Try GCE's link-local gateway as fallback
                        log "Trying GCE link-local gateway 169.254.1.1"
                        ip route add 169.254.1.1 dev "$interface" scope link
                        ip route add default via 169.254.1.1 dev "$interface"
                    fi
                else
                    log "No router provided, trying GCE link-local gateway"
                    ip route add 169.254.1.1 dev "$interface" scope link
                    ip route add default via 169.254.1.1 dev "$interface"
                fi

                if [ -n "$dns" ]; then
                    : > /etc/resolv.conf
                    for ns in $dns; do
                        echo "nameserver $ns" >> /etc/resolv.conf
                        log "Added nameserver: $ns"
                    done
                fi

                log "Configuration complete"
                log "Routes:"
                ip route show
                ;;
        esac
        UDHCPC_EOF
            chmod +x $out/usr/share/udhcpc/default.script

            # Create init script
            cat > $out/init << 'INIT_EOF'
        #!/bin/sh
        # NixOS Boot Installer Init Script

        set -e

        log() {
            echo "[init] $*"
        }

        # Mount essential filesystems
        log "Mounting essential filesystems..."
        mount -t proc proc /proc
        mount -t sysfs sysfs /sys
        mount -t devtmpfs devtmpfs /dev
        mkdir -p /dev/pts /dev/shm
        mount -t devpts devpts /dev/pts
        mount -t tmpfs tmpfs /dev/shm
        mount -t tmpfs tmpfs /run
        mount -t tmpfs tmpfs /tmp

        echo "====================================="
        echo "  NixOS Boot Installer"
        echo "====================================="
        echo ""
        echo "Kernel: $(uname -r)"
        echo "Date: $(date 2>/dev/null || echo 'unavailable')"
        echo ""

        # Load kernel modules
        log "Loading kernel modules..."
        INIT_EOF

            # Add module loading commands to init script
            ${lib.concatMapStringsSep "\n" (mod: ''
              echo "log \"Loading ${mod}...\"" >> $out/init
              echo "if modprobe ${mod} 2>&1; then log \"  ${mod} loaded\"; else log \"  WARNING: ${mod} failed\"; fi" >> $out/init
            '') selectedModules}

            cat >> $out/init << 'INIT_EOF'
        log "Module loading complete."
        echo ""

        # Show loaded modules
        log "Loaded modules:"
        lsmod | head -20
        echo ""

        # Wait for devices to settle
        log "Waiting for devices to settle..."
        sleep 2

        # Show available block devices
        log "Block devices:"
        ls -la /dev/sd* /dev/nvme* /dev/vd* 2>/dev/null || log "  No block devices found yet"
        echo ""

        # Find and configure network interfaces
        log "Configuring network..."
        log "Available network interfaces:"
        ls -la /sys/class/net/
        echo ""

        for iface in $(ls /sys/class/net/ | grep -v lo); do
            log "Processing interface: $iface"
            log "  Link status before:"
            ip link show "$iface" 2>&1 || true
            log "  Bringing up $iface..."
            ip link set "$iface" up
            sleep 2
            log "  Link status after:"
            ip link show "$iface" 2>&1 || true
            log "  Running DHCP on $iface (verbose)..."
            udhcpc -v -n -q -f -s /usr/share/udhcpc/default.script -i "$iface" 2>&1
            dhcp_result=$?
            log "  DHCP result: $dhcp_result"
            if [ $dhcp_result -eq 0 ]; then
                log "DHCP successful on $iface"
                break
            fi
        done

        echo ""
        log "Network configuration complete."
        echo ""

        # Show detailed network status
        log "Network addresses:"
        ip addr show 2>&1
        echo ""

        log "Routing table:"
        ip route show 2>&1
        echo ""

        log "DNS configuration (/etc/resolv.conf):"
        cat /etc/resolv.conf 2>/dev/null || log "  No resolv.conf"
        echo ""

        # Test network connectivity
        log "Testing network connectivity..."
        log "  Pinging metadata service (169.254.169.254)..."
        if ping -c 1 -W 2 169.254.169.254 2>&1; then
            log "  Metadata service reachable"
        else
            log "  WARNING: Metadata service unreachable"
        fi
        echo ""

        # Run the installer
        log "Starting NixOS Boot Installer..."
        echo ""
        exec /bin/nixos-boot-installer

        # Fallback to shell on failure
        log "Installer exited. Dropping to shell..."
        exec /bin/sh
        INIT_EOF

            chmod +x $out/init
      '';

  # Create the initrd cpio archive
  initrd =
    pkgs.runCommand "installer-initrd"
      {
        nativeBuildInputs = [
          pkgs.cpio
          pkgs.zstd
        ];
      }
      ''
        mkdir -p $out
        cd ${initrdContents}
        find . -print0 | cpio -o0H newc | zstd > $out/initrd
      '';

  osRelease = pkgs.writeText "os-release" ''
    NAME="NixOS Boot Installer"
    ID=nixos-boot-installer
    VERSION="0.1.0"
    VERSION_ID="0.1.0"
    PRETTY_NAME="NixOS Boot Installer 0.1.0"
  '';

  inherit (pkgs.stdenv.hostPlatform) efiArch;

  ukifyConfig = (pkgs.formats.ini { }).generate "ukify.conf" {
    UKI = {
      Linux = "${kernel}/${kernel.file or "bzImage"}";
      Initrd = "${initrd}/initrd";
      Cmdline = "console=tty0 console=ttyS0 loglevel=7${
        lib.optionalString (extraCmdline != "") " ${extraCmdline}"
      }";
      Stub = "${pkgs.systemd}/lib/systemd/boot/efi/linux${efiArch}.efi.stub";
      Uname = kernel.modDirVersion;
      OSRelease = "@${osRelease}";
      EFIArch = efiArch;
    };
  };

  filename = "nixos-boot-installer.efi";
in
pkgs.runCommand filename
  {
    nativeBuildInputs = [ pkgs.systemdUkify ];
    passthru = {
      inherit
        filename
        kernel
        moduleProfile
        extraCmdline
        configSources
        ;
      inherit initrd osRelease installer;
    };
  }
  ''
    mkdir -p $out
    ukify build \
      --config=${ukifyConfig} \
      --output="$out/${filename}"
  ''
