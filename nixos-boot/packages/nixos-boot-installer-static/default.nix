# Static build of the installer for use in initrd
#
# Uses musl for static linking, producing a single binary with no dependencies
#
# Configuration sources can be enabled/disabled via cargoFeatures:
# - config-cmdline: Kernel command line parameters (nixos-boot.*)
# - config-gce: Google Compute Engine metadata service
# - config-ec2: AWS EC2 instance metadata service (IMDSv2)
# - config-file: Cloud-init style file-based user-data
# - config-interactive: Interactive console/web UI configuration
{
  pkgs,
  lib ? pkgs.lib,
  # Configuration source features to enable
  # By default, all are enabled. Override this for platform-specific builds.
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
  # Use the static musl toolchain
  pkgsStatic = pkgs.pkgsStatic;
in
pkgsStatic.rustPlatform.buildRustPackage {
  pname = "nixos-boot-installer-static";
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

  # Disable default features and enable only the selected ones
  buildNoDefaultFeatures = true;
  buildFeatures = configSources;

  # For static builds, we use rustls (already configured in Cargo.toml)
  # No need for openssl
  nativeBuildInputs = [ pkgs.pkg-config ];

  # Static musl build
  CARGO_BUILD_TARGET = "x86_64-unknown-linux-musl";

  meta = {
    description = "Minimal NixOS installer agent for UKI-based deployments (static build)";
    license = lib.licenses.mit;
    mainProgram = "nixos-boot-installer";
    platforms = [ "x86_64-linux" ];
  };
}
