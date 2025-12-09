//! Disk partitioning using systemd-repart

use anyhow::{Context, Result};
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use tokio::process::Command;
use tracing::{debug, info};

use crate::config::InstallerConfig;

/// Information about a detected disk
#[derive(Debug, Clone)]
pub struct DiskInfo {
    /// Device path (e.g., /dev/sda)
    pub device: String,
    /// Size in bytes
    pub size_bytes: u64,
    /// Model/name of the disk
    pub model: String,
    /// Whether this appears to be a removable device (USB stick, etc.)
    pub removable: bool,
    /// Whether this is an NVMe device
    pub is_nvme: bool,
    /// Whether this is a virtio block device
    pub is_virtio: bool,
}

/// Detect all available block devices
pub fn detect_disks() -> Vec<DiskInfo> {
    let mut disks = Vec::new();

    // Read from /sys/block for all block devices
    let block_dir = Path::new("/sys/block");
    if let Ok(entries) = fs::read_dir(block_dir) {
        for entry in entries.flatten() {
            let name = entry.file_name().to_string_lossy().to_string();

            // Skip loop devices, RAM disks, and device mapper
            if name.starts_with("loop")
                || name.starts_with("ram")
                || name.starts_with("dm-")
                || name.starts_with("sr")
            {
                continue;
            }

            let device_path = format!("/dev/{}", name);
            let sys_path = entry.path();

            // Get disk size
            let size_bytes = fs::read_to_string(sys_path.join("size"))
                .ok()
                .and_then(|s| s.trim().parse::<u64>().ok())
                .map(|sectors| sectors * 512) // Sector size is 512 bytes
                .unwrap_or(0);

            // Skip very small devices (< 1GB)
            if size_bytes < 1_000_000_000 {
                continue;
            }

            // Check if removable
            let removable = fs::read_to_string(sys_path.join("removable"))
                .map(|s| s.trim() == "1")
                .unwrap_or(false);

            // Get model name
            let model = fs::read_to_string(sys_path.join("device/model"))
                .map(|s| s.trim().to_string())
                .unwrap_or_else(|_| {
                    // For virtio devices, try vendor
                    fs::read_to_string(sys_path.join("device/vendor"))
                        .map(|s| s.trim().to_string())
                        .unwrap_or_else(|_| "Unknown".to_string())
                });

            let is_nvme = name.starts_with("nvme");
            let is_virtio = name.starts_with("vd");

            disks.push(DiskInfo {
                device: device_path,
                size_bytes,
                model,
                removable,
                is_nvme,
                is_virtio,
            });
        }
    }

    // Sort disks by preference:
    // 1. NVMe devices first (fastest)
    // 2. Virtio devices (fast in VMs)
    // 3. Non-removable before removable
    // 4. Larger disks before smaller
    disks.sort_by(|a, b| {
        // NVMe first
        if a.is_nvme != b.is_nvme {
            return b.is_nvme.cmp(&a.is_nvme);
        }
        // Virtio second
        if a.is_virtio != b.is_virtio {
            return b.is_virtio.cmp(&a.is_virtio);
        }
        // Non-removable before removable
        if a.removable != b.removable {
            return a.removable.cmp(&b.removable);
        }
        // Larger disks first
        b.size_bytes.cmp(&a.size_bytes)
    });

    disks
}

/// Select the best disk for installation
pub fn select_disk(disks: &[DiskInfo]) -> Option<&DiskInfo> {
    if disks.is_empty() {
        return None;
    }

    // After sorting, the first disk is the best choice
    let selected = &disks[0];

    // Log the selection rationale
    let size_gb = selected.size_bytes / 1_000_000_000;
    let disk_type = if selected.is_nvme {
        "NVMe"
    } else if selected.is_virtio {
        "VirtIO"
    } else if selected.removable {
        "removable"
    } else {
        "standard"
    };

    info!(
        "Selected {} ({}, {} GB, {}) as installation target",
        selected.device, selected.model, size_gb, disk_type
    );

    Some(selected)
}

/// Format disk size as human-readable string
pub fn format_size(bytes: u64) -> String {
    if bytes >= 1_000_000_000_000 {
        format!("{:.1} TB", bytes as f64 / 1_000_000_000_000.0)
    } else if bytes >= 1_000_000_000 {
        format!("{:.1} GB", bytes as f64 / 1_000_000_000.0)
    } else if bytes >= 1_000_000 {
        format!("{:.1} MB", bytes as f64 / 1_000_000.0)
    } else {
        format!("{} bytes", bytes)
    }
}

/// Print a list of available disks for user information
pub fn print_disk_list(disks: &[DiskInfo]) {
    println!();
    println!("   Available disks:");
    for (i, disk) in disks.iter().enumerate() {
        let size = format_size(disk.size_bytes);
        let removable = if disk.removable { " (removable)" } else { "" };
        println!(
            "   [{}] {} - {} ({}){}",
            i + 1,
            disk.device,
            disk.model,
            size,
            removable
        );
    }
    println!();
}

/// Partition the target disk using sfdisk
///
/// Creates a GPT partition table with:
/// - Partition 1: EFI System Partition (512MB, vfat)
/// - Partition 2: Root partition (rest of disk, ext4)
pub async fn partition_disk(disk: &str, _config: &InstallerConfig) -> Result<()> {
    info!("Partitioning disk: {}", disk);

    // First, wipe any existing partition table
    info!("Wiping existing partition table...");
    let status = Command::new("wipefs")
        .args(["--all", "--force", disk])
        .status()
        .await
        .context("Failed to run wipefs")?;

    if !status.success() {
        anyhow::bail!("wipefs failed");
    }

    // Create GPT partition table with sfdisk
    // Format: label type, then partition definitions
    let sfdisk_script = r#"label: gpt

# EFI System Partition (512MB)
size=512M, type=uefi, name="ESP"

# Root partition (rest of disk)
type=linux, name="nixos"
"#;

    info!("Creating partition table with sfdisk...");
    let mut cmd = Command::new("sfdisk");
    cmd.arg(disk);
    cmd.stdin(std::process::Stdio::piped());

    debug!("Running: sfdisk {} with script:\n{}", disk, sfdisk_script);

    let mut child = cmd.spawn().context("Failed to spawn sfdisk")?;

    if let Some(mut stdin) = child.stdin.take() {
        use tokio::io::AsyncWriteExt;
        stdin
            .write_all(sfdisk_script.as_bytes())
            .await
            .context("Failed to write to sfdisk stdin")?;
    }

    let status = child.wait().await.context("Failed to wait for sfdisk")?;

    if !status.success() {
        anyhow::bail!("sfdisk failed to create partition table");
    }

    // Wait for kernel to re-read partition table
    tokio::time::sleep(std::time::Duration::from_millis(500)).await;

    // Tell kernel to re-read partition table
    let _ = Command::new("blockdev")
        .args(["--rereadpt", disk])
        .status()
        .await;

    // Wait for partitions to appear
    tokio::time::sleep(std::time::Duration::from_millis(500)).await;

    // Determine partition naming scheme
    let part_prefix = if disk.contains("nvme") || disk.contains("mmcblk") {
        format!("{}p", disk)
    } else {
        disk.to_string()
    };

    let esp_part = format!("{}1", part_prefix);
    let root_part = format!("{}2", part_prefix);

    // Format ESP partition as vfat
    info!("Formatting ESP partition ({}) as vfat...", esp_part);
    let status = Command::new("/sbin/mkfs.vfat")
        .args(["-F", "32", "-n", "ESP", &esp_part])
        .status()
        .await
        .context("Failed to run mkfs.vfat")?;

    if !status.success() {
        anyhow::bail!("mkfs.vfat failed");
    }

    // Format root partition as ext4
    info!("Formatting root partition ({}) as ext4...", root_part);
    let status = Command::new("mkfs.ext4")
        .args(["-L", "nixos", "-F", &root_part])
        .status()
        .await
        .context("Failed to run mkfs.ext4")?;

    if !status.success() {
        anyhow::bail!("mkfs.ext4 failed");
    }

    info!("Disk partitioning complete");
    Ok(())
}

/// Mount the target partitions for installation
pub async fn mount_target(disk: &str) -> Result<()> {
    let target = "/mnt";

    // Determine partition naming scheme
    let part_prefix = if disk.contains("nvme") || disk.contains("mmcblk") {
        format!("{}p", disk)
    } else {
        disk.to_string()
    };

    // Mount root partition
    let root_part = format!("{}2", part_prefix);
    info!("Mounting {} to {}", root_part, target);

    // Wait for partition device to appear (udev may be slow)
    let root_path = std::path::Path::new(&root_part);
    for i in 0..20 {
        if root_path.exists() {
            debug!("Partition {} appeared after {} attempts", root_part, i + 1);
            break;
        }
        if i == 19 {
            anyhow::bail!("Partition device {} did not appear after waiting", root_part);
        }
        debug!("Waiting for {} to appear (attempt {}/20)...", root_part, i + 1);
        tokio::time::sleep(std::time::Duration::from_millis(250)).await;
    }

    // Ensure mount point exists
    info!("Creating mount point {}", target);

    // Check if mount point already exists
    let target_path = std::path::Path::new(target);
    if target_path.exists() {
        info!("Mount point {} already exists", target);
    } else {
        info!("Mount point {} does not exist, creating...", target);
        tokio::fs::create_dir_all(target)
            .await
            .context("Failed to create mount point")?;
    }

    // Verify mount point exists and log its metadata
    if !target_path.exists() {
        anyhow::bail!("Mount point {} does not exist after creation", target);
    }

    // Check mount point metadata
    match std::fs::metadata(target) {
        Ok(meta) => {
            info!("Mount point {} exists, is_dir={}, permissions={:o}",
                  target, meta.is_dir(), meta.permissions().mode());
        }
        Err(e) => {
            info!("Cannot get metadata for {}: {}", target, e);
        }
    }

    // Also verify device exists
    let device_path = std::path::Path::new(&root_part);
    if !device_path.exists() {
        anyhow::bail!("Device {} does not exist", root_part);
    }
    info!("Device {} exists", root_part);

    let status = Command::new("mount")
        .args(["-t", "ext4", &root_part, target])
        .status()
        .await
        .context("Failed to mount root partition")?;

    if !status.success() {
        anyhow::bail!("Failed to mount root partition");
    }

    // Create and mount boot/ESP
    let esp_mount = format!("{}/boot", target);
    tokio::fs::create_dir_all(&esp_mount)
        .await
        .context("Failed to create boot mount point")?;

    let esp_part = format!("{}1", part_prefix);
    info!("Mounting {} to {}", esp_part, esp_mount);

    let status = Command::new("mount")
        .args(["-t", "vfat", &esp_part, &esp_mount])
        .status()
        .await
        .context("Failed to mount ESP")?;

    if !status.success() {
        anyhow::bail!("Failed to mount ESP");
    }

    info!("All partitions mounted at {}", target);
    Ok(())
}
