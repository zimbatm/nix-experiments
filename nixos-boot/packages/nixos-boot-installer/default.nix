{
  pkgs,
  lib ? pkgs.lib,
  rustPlatform ? pkgs.rustPlatform,
  pkg-config ? pkgs.pkg-config,
  openssl ? pkgs.openssl,
  ...
}:
rustPlatform.buildRustPackage {
  pname = "nixos-boot-installer";
  version = "0.1.0";

  src = lib.fileset.toSource {
    root = ../../installer;
    fileset = lib.fileset.unions [
      ../../installer/Cargo.toml
      ../../installer/Cargo.lock
      ../../installer/src
    ];
  };

  cargoLock = {
    lockFile = ../../installer/Cargo.lock;
  };

  nativeBuildInputs = [ pkg-config ];
  buildInputs = [ openssl ];

  meta = {
    description = "Minimal NixOS installer agent for UKI-based deployments";
    license = lib.licenses.mit;
    mainProgram = "nixos-boot-installer";
  };
}
