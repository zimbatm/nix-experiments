# Automated boot test for the test UKI
#
# This test boots the UKI in QEMU and verifies it reaches the shell prompt
{ pkgs, perSystem, ... }:
let
  uki = perSystem.self.test-uki;
  ovmf = pkgs.OVMF.fd;

  # Timeout in seconds
  timeout = 60;
in
pkgs.runCommand "boot-test"
  {
    nativeBuildInputs = [
      pkgs.qemu_kvm
      pkgs.expect
    ];
    # This test needs KVM, mark it appropriately
    requiredSystemFeatures = [ "kvm" ];
  }
  ''
    echo "Running boot test for ${uki}/${uki.filename}"

    # Create virtual disk
    qemu-img create -f qcow2 disk.qcow2 1G

    # Copy OVMF vars (needs to be writable)
    cp ${ovmf}/FV/OVMF_VARS.fd OVMF_VARS.fd
    chmod +w OVMF_VARS.fd

    # Create expect script for automated testing
    cat > boot-test.exp << 'EOF'
    set timeout ${toString timeout}

    spawn qemu-system-x86_64 \
      -m 512M \
      -enable-kvm \
      -cpu host \
      -drive if=pflash,format=raw,readonly=on,file=${ovmf}/FV/OVMF_CODE.fd \
      -drive if=pflash,format=raw,file=OVMF_VARS.fd \
      -drive file=disk.qcow2,format=qcow2,if=virtio \
      -kernel ${uki}/${uki.filename} \
      -nic none \
      -serial mon:stdio \
      -display none

    # Wait for the test success message
    expect {
      "Test successful - system booted!" {
        puts "\n\nPASS: Boot test succeeded!"
      }
      timeout {
        puts "\n\nFAIL: Timeout waiting for boot"
        exit 1
      }
      eof {
        puts "\n\nFAIL: QEMU exited unexpectedly"
        exit 1
      }
    }

    # Shutdown cleanly
    send "\x01x"
    expect eof
    EOF

    # Run the test
    expect boot-test.exp

    mkdir -p $out
    echo "Boot test passed" > $out/result
  ''
