# End-to-end test script for nixos-boot installer
#
# This creates a script that:
# 1. Builds a minimal NixOS system closure
# 2. Starts Harmonia to serve the closure locally
# 3. Boots the installer VM configured to pull from Harmonia
# 4. The installer fetches and installs the system
#
# Run: nix build .#e2e-test && ./result/bin/run-e2e-test
{
  pkgs,
  perSystem,
  lib ? pkgs.lib,
  writeShellScriptBin ? pkgs.writeShellScriptBin,
  ...
}:
let
  # Build a truly minimal NixOS system for testing (faster to build)
  minimalSystem = import "${pkgs.path}/nixos/lib/eval-config.nix" {
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
          ];

          # Minimal config
          system.stateVersion = "24.11";
          networking.hostName = "test-target";

          # UEFI boot
          boot.loader.systemd-boot.enable = true;
          boot.loader.efi.canTouchEfiVariables = true;

          # Virtio for QEMU
          boot.kernelPackages = pkgs.linuxPackages_latest;
          boot.initrd.kernelModules = [
            "virtio_pci"
            "virtio_blk"
            "virtio_net"
          ];

          # Serial console
          boot.kernelParams = [
            "console=ttyS0"
            "console=tty0"
          ];

          # Filesystems (will be created by installer)
          fileSystems."/" = {
            device = "/dev/disk/by-label/nixos";
            fsType = "ext4";
          };
          fileSystems."/boot" = {
            device = "/dev/disk/by-label/ESP";
            fsType = "vfat";
          };

          # No extra services for minimal size
          services.openssh.enable = false;
          documentation.enable = false;
          documentation.man.enable = false;
          documentation.nixos.enable = false;

          # Mark successful boot
          systemd.services.boot-success = {
            description = "Signal successful boot";
            wantedBy = [ "multi-user.target" ];
            after = [ "multi-user.target" ];
            script = ''
              # Write directly to serial console so it appears in the test log
              {
                echo ""
                echo "=========================================="
                echo "  E2E-TEST-SUCCESS"
                echo "  NixOS installed and booted successfully!"
                echo "=========================================="
                echo ""
              } > /dev/ttyS0
              # Shutdown after success
              ${pkgs.systemd}/bin/systemctl poweroff
            '';
            serviceConfig = {
              Type = "oneshot";
              RemainAfterExit = true;
            };
          };
        }
      )
    ];
  };

  systemClosure = minimalSystem.config.system.build.toplevel;

  # Fixed port for Harmonia - UKIs embed cmdline at build time, so we can't use dynamic ports
  harmoniaPort = 15123;

  # Create installer UKI with embedded cmdline pointing to our test cache
  # Note: UKIs don't support QEMU's -append flag, cmdline must be embedded at build time
  installerUki = perSystem.self.installer-uki.override {
    moduleProfile = "minimal";
    configSources = [ "config-cmdline" ];
    # 10.0.2.2 is the host from QEMU user-mode networking guest perspective
    # nixos-boot.noninteractive=true makes installer exit on error instead of showing menu
    extraCmdline = "nixos-boot.closure=${systemClosure} nixos-boot.cache=http://10.0.2.2:${toString harmoniaPort},https://cache.nixos.org nixos-boot.disk=/dev/vda nixos-boot.noninteractive=true";
  };

  ovmf = pkgs.OVMF.fd;
  harmonia = pkgs.harmonia;
  qemu = pkgs.qemu_kvm;
in
writeShellScriptBin "run-e2e-test" ''
    set -euo pipefail

    echo "========================================"
    echo "  NixOS Boot End-to-End Test"
    echo "========================================"
    echo ""
    echo "System closure: ${systemClosure}"
    echo "Installer UKI:  ${installerUki}/${installerUki.filename}"
    echo ""

    # Create temporary directory for test artifacts
    test_dir=$(mktemp -d)
    at_exit() {
      echo "Cleaning up...";
      kill $harmonia_pid 2>/dev/null || true;
      rm -rf "$test_dir"
    }
    trap at_exit EXIT

    # Use fixed port for Harmonia (UKIs embed cmdline at build time)
    harmonia_port=${toString harmoniaPort}
    echo "Using port $harmonia_port for Harmonia cache server"

    # Start Harmonia cache server
    echo ""
    echo "Starting Harmonia cache server..."
    export NIX_STORE=/nix/store

    # Create harmonia config
    # Bind to 0.0.0.0 so QEMU guest can reach it via 10.0.2.2 (host from guest perspective)
    cat > "$test_dir/harmonia.toml" <<EOF
  bind = "0.0.0.0:$harmonia_port"
  priority = 50
  EOF

    echo "Harmonia config:"
    cat "$test_dir/harmonia.toml"
    echo ""

    # Harmonia uses CONFIG_FILE env var, not --config flag
    CONFIG_FILE="$test_dir/harmonia.toml" ${harmonia}/bin/harmonia &
    harmonia_pid=$!
    echo "Harmonia started with PID $harmonia_pid"

    # Wait for Harmonia to be ready
    echo "Waiting for Harmonia to be ready..."
    for i in $(seq 1 30); do
      if ${pkgs.curl}/bin/curl -s "http://127.0.0.1:$harmonia_port/nix-cache-info" > /dev/null 2>&1; then
        echo "Harmonia is ready!"
        break
      fi
      if [ $i -eq 30 ]; then
        echo "ERROR: Harmonia failed to start"
        exit 1
      fi
      sleep 0.5
    done

    # Show cache info
    echo ""
    echo "Cache info:"
    ${pkgs.curl}/bin/curl -s "http://127.0.0.1:$harmonia_port/nix-cache-info"
    echo ""

    # Test that the closure is accessible
    closure_hash=$(basename "${systemClosure}" | cut -d- -f1)
    echo "Testing closure availability (hash: $closure_hash)..."
    if ${pkgs.curl}/bin/curl -sf "http://127.0.0.1:$harmonia_port/$closure_hash.narinfo" > /dev/null; then
      echo "Closure is available in cache!"
    else
      echo "WARNING: Closure narinfo not directly accessible (may need signing)"
    fi
    echo ""

    # Create virtual disk
    echo "Creating virtual disk..."
    ${qemu}/bin/qemu-img create -f qcow2 "$test_dir/disk.qcow2" 30G

    # Copy OVMF vars (needs to be writable)
    cp ${ovmf}/FV/OVMF_VARS.fd "$test_dir/OVMF_VARS.fd"
    chmod +w "$test_dir/OVMF_VARS.fd"

    echo "Installer UKI has embedded cmdline with nixos-boot parameters"
    echo ""

    # Check if KVM is available
    kvm_args=""
    if [ -w /dev/kvm ]; then
      kvm_args="-enable-kvm -cpu host"
      echo "Using KVM acceleration"
    else
      echo "WARNING: KVM not available, using software emulation (will be slow)"
    fi

    echo ""
    echo "========================================"
    echo "  Phase 1: Running Installer"
    echo "========================================"
    echo ""
    echo "Press Ctrl-A X to abort"
    echo ""

    # Run the installer VM
    # Note: UKIs have cmdline embedded at build time, -append is not supported
    # Use --foreground to allow proper terminal I/O with timeout
    # Capture output to check for failure markers
    installer_log="$test_dir/installer.log"
    timeout --foreground 600 ${qemu}/bin/qemu-system-x86_64 \
      -m 4G \
      $kvm_args \
      -drive if=pflash,format=raw,readonly=on,file=${ovmf}/FV/OVMF_CODE.fd \
      -drive if=pflash,format=raw,file="$test_dir/OVMF_VARS.fd" \
      -drive file="$test_dir/disk.qcow2",format=qcow2,if=virtio \
      -kernel ${installerUki}/${installerUki.filename} \
      -nic user,model=virtio-net-pci,hostfwd=tcp::10022-:22 \
      -serial mon:stdio \
      -display none \
      -no-reboot 2>&1 | tee "$installer_log" || {
        echo ""
        echo "Installer VM exited (this is expected after installation)"
      }

    # Check for installation failure
    if grep -q "E2E-TEST-FAILURE" "$installer_log"; then
      echo ""
      echo "========================================"
      echo "  INSTALLATION FAILED"
      echo "========================================"
      echo ""
      echo "The installer encountered an error. Check the log above for details."
      exit 1
    fi

    echo ""
    echo "========================================"
    echo "  Phase 2: Booting Installed System"
    echo "========================================"
    echo ""
    echo "Starting the installed system to verify..."
    echo "Press Ctrl-A X to abort"
    echo ""

    # Boot the installed system
    boot_log="$test_dir/boot.log"
    timeout --foreground 300 ${qemu}/bin/qemu-system-x86_64 \
      -m 4G \
      $kvm_args \
      -drive if=pflash,format=raw,readonly=on,file=${ovmf}/FV/OVMF_CODE.fd \
      -drive if=pflash,format=raw,file="$test_dir/OVMF_VARS.fd" \
      -drive file="$test_dir/disk.qcow2",format=qcow2,if=virtio \
      -nic user,model=virtio-net-pci \
      -serial mon:stdio \
      -display none \
      -no-reboot 2>&1 | tee "$boot_log" || {
        echo ""
        echo "Installed system exited"
      }

    # Check for successful boot
    if grep -q "E2E-TEST-SUCCESS" "$boot_log"; then
      echo ""
      echo "========================================"
      echo "  E2E TEST PASSED"
      echo "========================================"
      echo ""
      echo "NixOS was installed and booted successfully!"
      exit 0
    else
      echo ""
      echo "========================================"
      echo "  E2E TEST FAILED"
      echo "========================================"
      echo ""
      echo "The installed system did not boot successfully."
      echo "Expected to see 'E2E-TEST-SUCCESS' in the boot output."
      exit 1
    fi
''
