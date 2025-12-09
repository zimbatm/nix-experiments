//! Nix store operations - fetching closures and installing

use anyhow::{Context, Result};
use std::fs;
use tokio::process::Command;
use tracing::{debug, info, warn};

use crate::config::InstallerConfig;

/// Initialize the nix store on the target filesystem
async fn init_nix_store(target_root: &str) -> Result<()> {
    let store_path = format!("{}/nix/store", target_root);
    let var_path = format!("{}/nix/var/nix", target_root);

    // Create necessary directories
    fs::create_dir_all(&store_path).context("Failed to create /nix/store")?;
    fs::create_dir_all(format!("{}/db", var_path)).context("Failed to create /nix/var/nix/db")?;
    fs::create_dir_all(format!("{}/gcroots", var_path))
        .context("Failed to create /nix/var/nix/gcroots")?;
    fs::create_dir_all(format!("{}/profiles", var_path))
        .context("Failed to create /nix/var/nix/profiles")?;
    fs::create_dir_all(format!("{}/temproots", var_path))
        .context("Failed to create /nix/var/nix/temproots")?;
    fs::create_dir_all(format!("{}/userpool", var_path))
        .context("Failed to create /nix/var/nix/userpool")?;

    info!("Initialized nix store structure at {}", target_root);
    Ok(())
}

/// Write nix configuration for substituters
fn write_nix_conf(config: &InstallerConfig) -> Result<()> {
    let mut conf = String::new();

    // Configure substituters
    if !config.binary_caches.is_empty() {
        conf.push_str(&format!(
            "substituters = {}\n",
            config.binary_caches.join(" ")
        ));
    }

    // Configure trusted public keys
    if !config.signing_keys.is_empty() {
        conf.push_str(&format!(
            "trusted-public-keys = {}\n",
            config.signing_keys.join(" ")
        ));
    }

    // Common settings for initrd environment
    conf.push_str("sandbox = false\n");
    conf.push_str("require-sigs = false\n");
    conf.push_str("experimental-features = nix-command flakes\n");

    // Write to /etc/nix/nix.conf
    fs::create_dir_all("/etc/nix").context("Failed to create /etc/nix")?;
    fs::write("/etc/nix/nix.conf", conf).context("Failed to write /etc/nix/nix.conf")?;

    info!("Wrote nix configuration to /etc/nix/nix.conf");
    Ok(())
}

/// Fetch the system closure from binary caches and install to target store
///
/// Uses nix-store --realise with --store /mnt to:
/// 1. Fetch the closure from substituters
/// 2. Write to /mnt/nix/store
pub async fn fetch_closure(config: &InstallerConfig) -> Result<()> {
    let closure = config
        .system_closure
        .as_ref()
        .context("No system closure specified")?;

    info!("Fetching closure: {}", closure);

    // Initialize the target nix store structure
    init_nix_store("/mnt").await?;

    // Write nix configuration (substituters, keys)
    write_nix_conf(config)?;

    // Build the substituters string
    let substituters = config.binary_caches.join(" ");

    // Use nix-store --realise with --store /mnt to fetch the closure
    // This fetches from substituters and writes to /mnt/nix/store
    info!("Realising closure from substituters...");

    let mut cmd = Command::new("nix-store");
    cmd.args([
        "--store", "/mnt",
        "--extra-substituters", &substituters,
        "--realise", closure,
    ]);

    // Add trusted public keys if configured
    if !config.signing_keys.is_empty() {
        let keys = config.signing_keys.join(" ");
        cmd.args(["--option", "trusted-public-keys", &keys]);
    }

    debug!("Running: {:?}", cmd);
    let output = cmd.output().await.context("Failed to run nix-store --realise")?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        let stdout = String::from_utf8_lossy(&output.stdout);
        warn!("nix-store stdout: {}", stdout);
        warn!("nix-store stderr: {}", stderr);
        anyhow::bail!("nix-store --realise failed: {}", stderr);
    }

    info!("Closure fetched successfully");

    // Now set up the system profile using nix-env
    let profile_path = "/mnt/nix/var/nix/profiles/system";
    info!("Setting system profile: {}", profile_path);

    let status = Command::new("nix-env")
        .args([
            "--store", "/mnt",
            "-p", profile_path,
            "--set", closure,
        ])
        .status()
        .await
        .context("Failed to run nix-env")?;

    if !status.success() {
        anyhow::bail!("Failed to set system profile");
    }

    info!("System profile set successfully");
    Ok(())
}

/// Install the bootloader on the target system
pub async fn install_bootloader(config: &InstallerConfig) -> Result<()> {
    let closure = config
        .system_closure
        .as_ref()
        .context("No system closure specified")?;

    info!("Installing bootloader");

    // The system profile was already set by fetch_closure using nix-env --store /mnt
    // Now we need to run the bootloader installation script in a chroot.
    // This is how nixos-install does it.

    // First, bind mount essential filesystems into the chroot
    for (src, dst) in &[
        ("/proc", "/mnt/proc"),
        ("/sys", "/mnt/sys"),
        ("/dev", "/mnt/dev"),
        ("/run", "/mnt/run"),
    ] {
        fs::create_dir_all(dst).ok();
        let status = Command::new("mount")
            .args(["--bind", src, dst])
            .status()
            .await;
        if let Err(e) = status {
            warn!("Failed to bind mount {} to {}: {}", src, dst, e);
        }
    }

    // Set up /etc in the target following nixos-install approach:
    // 1. Create /etc as a regular directory (NOT a symlink to the store!)
    // 2. Touch /etc/NIXOS to mark it as a NixOS installation
    // 3. Create /etc/mtab -> /proc/mounts for grub/bootloader
    let mnt_etc = "/mnt/etc";

    // Ensure /etc exists as a directory
    fs::create_dir_all(mnt_etc).context("Failed to create /mnt/etc")?;

    // Touch the NIXOS marker file
    let nixos_marker = format!("{}/NIXOS", mnt_etc);
    fs::write(&nixos_marker, "").context("Failed to create /mnt/etc/NIXOS")?;
    info!("Created NixOS marker at {}", nixos_marker);

    // Create /etc/mtab symlink to /proc/mounts (needed by some bootloader installers)
    let mtab_link = format!("{}/mtab", mnt_etc);
    let _ = fs::remove_file(&mtab_link);
    std::os::unix::fs::symlink("/proc/mounts", &mtab_link)
        .context("Failed to create /mnt/etc/mtab symlink")?;
    info!("Created /etc/mtab symlink");

    // Create /etc/os-release symlink to the system's etc/os-release
    // bootctl install requires ID or IMAGE_ID fields to be set
    let os_release_link = format!("{}/os-release", mnt_etc);
    let _ = fs::remove_file(&os_release_link);
    let os_release_target = format!("{}/etc/os-release", closure);
    std::os::unix::fs::symlink(&os_release_target, &os_release_link)
        .context("Failed to create /mnt/etc/os-release symlink")?;
    info!("Created /etc/os-release symlink -> {}", os_release_target);

    // Read the switch-to-configuration wrapper to find the install-bootloader script path
    // The script exports INSTALL_BOOTLOADER pointing to the actual installation script
    // Note: The closure was copied to /mnt, so we need to read from there
    let switch_script = format!("/mnt{}/bin/switch-to-configuration", closure);
    let script_content = fs::read_to_string(&switch_script)
        .context("Failed to read switch-to-configuration script")?;

    // Parse the INSTALL_BOOTLOADER export from the wrapper script
    let install_bootloader_path = script_content
        .lines()
        .find(|line| line.contains("INSTALL_BOOTLOADER="))
        .and_then(|line| {
            // Extract the path from export INSTALL_BOOTLOADER='...'
            line.split('\'')
                .nth(1)
                .map(|s| s.to_string())
        })
        .context("Could not find INSTALL_BOOTLOADER in switch-to-configuration")?;

    info!("Found bootloader installer: {}", install_bootloader_path);

    // The install-systemd-boot.sh script (which calls systemd-boot Python script)
    // needs the default config path as an argument.
    // This is the path to the NixOS system configuration.
    info!("Running NixOS bootloader installation script");

    let status = Command::new("chroot")
        .args(["/mnt", &install_bootloader_path, closure])
        .env("NIXOS_INSTALL_BOOTLOADER", "1")
        .status()
        .await
        .context("Failed to run bootloader installation script")?;

    if !status.success() {
        anyhow::bail!("Bootloader installation failed");
    }

    info!("Bootloader installed successfully");
    Ok(())
}
