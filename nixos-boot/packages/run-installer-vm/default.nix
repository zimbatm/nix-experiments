# Script to run a UKI in QEMU
#
# This is a generic VM runner that can be used with any UKI.
# Override `uki` to run different images.
{
  pkgs,
  perSystem,
  lib ? pkgs.lib,
  writeShellScriptBin ? pkgs.writeShellScriptBin,
  # Override these to customize the VM
  uki ? perSystem.self.installer-uki,
  name ? "run-installer-vm",
  ...
}:
let
  ovmf = pkgs.OVMF.fd;
  qemuBin = "${pkgs.qemu_kvm}/bin/qemu-system-x86_64";
in
writeShellScriptBin name ''
  set -euo pipefail

  echo "NixOS Boot Installer VM"
  echo "======================="
  echo ""
  echo "UKI: ${uki}/${uki.filename}"
  echo ""

  # Create a temporary directory for VM state
  vm_dir=$(mktemp -d)
  trap 'rm -rf "$vm_dir"' EXIT

  # Create the virtual disk
  ${pkgs.qemu}/bin/qemu-img create -f qcow2 "$vm_dir/disk.qcow2" 10G

  # Copy OVMF vars (needs to be writable)
  cp ${ovmf}/FV/OVMF_VARS.fd "$vm_dir/OVMF_VARS.fd"
  chmod +w "$vm_dir/OVMF_VARS.fd"

  echo "Starting QEMU..."
  echo "Press Ctrl-A X to exit"
  echo ""

  # Check if KVM is available
  kvm_args=""
  if [ -w /dev/kvm ]; then
    kvm_args="-enable-kvm -cpu host"
    echo "Using KVM acceleration"
  else
    echo "KVM not available, using software emulation (slower)"
  fi

  # Use -serial mon:stdio to multiplex monitor and serial on stdio
  # Use -display none instead of -nographic to avoid monitor conflict
  exec ${qemuBin} \
    -m 2G \
    $kvm_args \
    -drive if=pflash,format=raw,readonly=on,file=${ovmf}/FV/OVMF_CODE.fd \
    -drive if=pflash,format=raw,file="$vm_dir/OVMF_VARS.fd" \
    -drive file="$vm_dir/disk.qcow2",format=qcow2,if=virtio \
    -kernel ${uki}/${uki.filename} \
    -nic user,model=virtio-net-pci \
    -serial mon:stdio \
    -display none
''
