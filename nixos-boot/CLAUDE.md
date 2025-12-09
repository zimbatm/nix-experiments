# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

nixos-boot is a minimal NixOS installer agent designed for UKI (Unified Kernel Image) based deployments. It runs from an initrd as PID 1 to bootstrap a NixOS system from bare metal.

The project combines Rust (installer agent) with Nix (packaging, building, testing).

## Build Commands

```bash
# Enter development shell
nix develop

# Build packages
nix build .#packages.x86_64-linux.nixos-boot-installer        # Dynamic linked
nix build .#packages.x86_64-linux.nixos-boot-installer-static # Static musl build
nix build .#packages.x86_64-linux.installer-uki               # Full bootable UKI
nix build .#packages.x86_64-linux.test-uki                    # Test UKI

# Run in QEMU
nix run .#packages.x86_64-linux.run-installer-vm
nix run .#packages.x86_64-linux.run-test-vm

# Run checks (require KVM)
nix build .#checks.x86_64-linux.boot-test
nix build .#checks.x86_64-linux.installer-build
```

## Rust Development

```bash
cd installer
cargo build
cargo test
cargo fmt
cargo clippy
```

## Architecture

**Boot Flow:**
1. Kernel boots UKI with initrd containing the Rust installer
2. Installer runs as PID 1, mounts /proc, /sys, /dev, /run, /tmp
3. Loads config from: kernel cmdline -> cloud-init -> EC2 metadata
4. Sets up network (DHCP), partitions disk (systemd-repart)
5. Fetches NixOS closure from binary cache, installs bootloader
6. Drops to shell on failure for debugging

**Nix/Rust Integration:**
- Rust compiles to static binary via `pkgsStatic` for minimal initrd
- Nix creates UKI with binary + busybox + kernel modules using `systemd-ukify`
- QEMU testing framework in `nix/lib/vm.nix`

**Key Directories:**
- `installer/` - Rust source (main.rs, config.rs, network.rs, partition.rs, store.rs)
- `packages/` - Nix package definitions
- `nix/lib/` - Reusable Nix library functions (UKI building, VM testing)
- `checks/` - Automated tests

## Rust Code Structure

- `main.rs` - Entry point, PID 1 logic, filesystem mounting
- `config.rs` - Config parsing from kernel cmdline, cloud-init, EC2 metadata
- `network.rs` - Network setup, DHCP, kernel module loading
- `partition.rs` - systemd-repart integration, mounting
- `store.rs` - Nix store operations, closure fetching

## Testing

Tests require KVM. The `boot-test.nix` check creates a VM, boots the UKI, and waits for success message.

Use `--dry-run` flag for config inspection without side effects.
