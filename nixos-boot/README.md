# NixOS Boot

A universal NixOS installer that makes NixOS trivial to install on any platform.

## Status

**Work in Progress** - This project is in early development.

### Completed

- [x] Rust installer agent with configuration parsing from kernel command line
- [x] UKI (Unified Kernel Image) generation using systemd-ukify
- [x] QEMU VM test framework with UEFI boot
- [x] Static musl build of installer for initrd use
- [x] Installer runs as init (PID 1) with filesystem mounting

### In Progress

- [ ] Networking support (kernel module loading + DHCP)

### Planned

- [ ] Test systemd-repart integration on virtual disk
- [ ] Build ISO image format
- [ ] Cloud-init user-data support (EC2 IMDS)
- [ ] Nix store operations (fetch closures from binary cache)
- [ ] QR code fallback configuration
- [ ] SecureBoot support (Microsoft signed)
- [ ] Multiple boot targets: PXE, USB, AMI, GCE, Docker

## Architecture

The installer is designed as a minimal bootable image that:

1. Boots from UKI (Unified Kernel Image) containing kernel + initrd
2. Runs a Rust-based installer agent as init
3. Reads configuration from kernel command line or cloud-init user-data
4. Sets up networking via DHCP
5. Partitions disk using systemd-repart
6. Fetches Nix closures from binary cache
7. Installs the target NixOS system

## Development

### Prerequisites

- Nix with flakes enabled
- KVM support recommended for faster VM testing

### Building

```bash
# Build the installer UKI
nix build .#installer-uki

# Build the static Rust installer
nix build .#nixos-boot-installer-static
```

### Testing

```bash
# Run the installer in a QEMU VM
nix run .#run-installer-vm

# Run the minimal test UKI (busybox shell)
nix run .#run-test-vm
```

Press `Ctrl-A X` to exit QEMU.

### Project Structure

```
.
├── installer/           # Rust installer agent
│   ├── src/
│   │   ├── main.rs     # Entry point and init logic
│   │   ├── config.rs   # Configuration parsing
│   │   ├── network.rs  # Network setup
│   │   ├── partition.rs # Disk partitioning
│   │   └── store.rs    # Nix store operations
│   └── Cargo.toml
├── packages/
│   ├── nixos-boot-installer/        # Dynamic Rust build
│   ├── nixos-boot-installer-static/ # Static musl build
│   ├── installer-uki/               # Production UKI
│   ├── test-uki/                    # Test UKI (busybox)
│   ├── run-installer-vm/            # VM runner script
│   └── run-test-vm/                 # Test VM runner
└── flake.nix
```

## Configuration

The installer reads configuration from the kernel command line:

- `nixos.closure=<store-path>` - Nix store path of the system closure
- `nixos.binary_cache=<url>` - Binary cache URL (can be repeated)
- `nixos.signing_key=<key>` - Public key for binary cache verification
- `nixos.target_disk=<device>` - Target disk for installation

## See also

TODO: take a closer look

* https://github.com/DeterminateSystems/nix-netboot-serve/
* https://git.mbosch.me/linus/snowboot/
* https://github.com/dep-sys/nix-dabei

## License

MIT
