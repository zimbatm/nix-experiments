{ pkgs }:
pkgs.mkShell {
  packages = [
    # Rust toolchain
    pkgs.cargo
    pkgs.rustc
    pkgs.rust-analyzer
    pkgs.clippy
    pkgs.rustfmt

    # Build tools
    pkgs.pkg-config

    # Testing
    pkgs.qemu_kvm
    pkgs.OVMF

    # Debugging
    pkgs.gdb

    # Deploy
    pkgs.google-cloud-sdk
  ];

  env = {
    RUST_BACKTRACE = "1";
  };

  shellHook = ''
    echo "nixos-boot development shell"
  '';
}
