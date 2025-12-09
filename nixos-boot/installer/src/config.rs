//! Configuration loading from various sources
//!
//! This module supports multiple configuration sources, each gated by a Cargo feature:
//! - `config-cmdline`: Kernel command line parameters (nixos-boot.*)
//! - `config-file`: Cloud-init style file-based user-data
//! - `config-ec2`: AWS EC2 instance metadata service (IMDSv2)
//! - `config-gce`: Google Compute Engine metadata service

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use tracing::debug;

/// Main installer configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InstallerConfig {
    /// Nix binary cache URLs to fetch from
    #[serde(default = "default_caches")]
    pub binary_caches: Vec<String>,

    /// Public keys for verifying store paths
    #[serde(default)]
    pub signing_keys: Vec<String>,

    /// The system closure store path to install
    pub system_closure: Option<String>,

    /// Disk partitioning configuration
    #[serde(default)]
    pub partitioning: PartitionConfig,

    /// Only partition and format the disk, skip store operations
    #[serde(default)]
    pub partition_only: bool,

    /// Target disk device (e.g., /dev/vda)
    #[serde(default)]
    pub target_disk: Option<String>,
}

impl Default for InstallerConfig {
    fn default() -> Self {
        Self {
            binary_caches: default_caches(),
            signing_keys: Vec::new(),
            system_closure: None,
            partitioning: PartitionConfig::default(),
            partition_only: false,
            target_disk: None,
        }
    }
}

fn default_caches() -> Vec<String> {
    vec!["https://cache.nixos.org".to_string()]
}

/// Disk partitioning configuration for systemd-repart
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PartitionConfig {
    /// Use the entire disk (wipe existing partitions)
    #[serde(default = "default_true")]
    pub wipe: bool,

    /// Enable encryption (LUKS)
    #[serde(default)]
    pub encrypt: bool,
}

fn default_true() -> bool {
    true
}

/// Load configuration from kernel command line
#[cfg(feature = "config-cmdline")]
pub async fn from_cmdline() -> Result<InstallerConfig> {
    debug!("Trying kernel command line configuration source");
    let cmdline = tokio::fs::read_to_string("/proc/cmdline")
        .await
        .context("Failed to read /proc/cmdline")?;

    parse_cmdline(&cmdline)
}

/// Parse kernel command line parameters
#[cfg(feature = "config-cmdline")]
fn parse_cmdline(cmdline: &str) -> Result<InstallerConfig> {
    let mut config = InstallerConfig::default();
    let mut found_any = false;

    for param in cmdline.split_whitespace() {
        if let Some(value) = param.strip_prefix("nixos-boot.closure=") {
            config.system_closure = Some(value.to_string());
            found_any = true;
        } else if let Some(value) = param.strip_prefix("nixos-boot.cache=") {
            config.binary_caches = value.split(',').map(String::from).collect();
            found_any = true;
        } else if let Some(value) = param.strip_prefix("nixos-boot.key=") {
            config.signing_keys.push(value.to_string());
            found_any = true;
        } else if param == "nixos-boot.encrypt" {
            config.partitioning.encrypt = true;
            found_any = true;
        } else if param == "nixos-boot.partition-only" {
            config.partition_only = true;
            found_any = true;
        } else if let Some(value) = param.strip_prefix("nixos-boot.disk=") {
            config.target_disk = Some(value.to_string());
            found_any = true;
        }
    }

    if found_any {
        Ok(config)
    } else {
        anyhow::bail!("No nixos-boot.* parameters found in cmdline")
    }
}

/// Load configuration from cloud-init user-data files
#[cfg(feature = "config-file")]
pub async fn from_userdata_file() -> Result<InstallerConfig> {
    debug!("Trying file-based user-data configuration source");
    // Try common cloud-init user-data locations
    let paths = [
        "/run/cloud-init/user-data",
        "/var/lib/cloud/instance/user-data.txt",
    ];

    for path in paths {
        debug!("Checking for user-data at {}", path);
        if let Ok(content) = tokio::fs::read_to_string(path).await {
            // Try to parse as JSON directly
            if let Ok(config) = serde_json::from_str(&content) {
                return Ok(config);
            }
        }
    }

    anyhow::bail!("No file-based user-data found")
}

/// Fetch configuration from EC2 instance metadata service (IMDSv2)
#[cfg(feature = "config-ec2")]
pub async fn from_ec2_metadata() -> Result<InstallerConfig> {
    debug!("Trying EC2 metadata configuration source");
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(2))
        .build()?;

    // IMDSv2 - get token first
    let token = client
        .put("http://169.254.169.254/latest/api/token")
        .header("X-aws-ec2-metadata-token-ttl-seconds", "60")
        .send()
        .await?
        .text()
        .await?;

    let userdata = client
        .get("http://169.254.169.254/latest/user-data")
        .header("X-aws-ec2-metadata-token", &token)
        .send()
        .await?
        .text()
        .await?;

    serde_json::from_str(&userdata).context("Failed to parse EC2 user-data as JSON")
}

/// Fetch configuration from GCE instance metadata service
#[cfg(feature = "config-gce")]
pub async fn from_gce_metadata() -> Result<InstallerConfig> {
    debug!("Trying GCE metadata configuration source");
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(2))
        .build()?;

    // GCE metadata requires the Metadata-Flavor header
    // Use IP address (169.254.169.254) instead of hostname to avoid DNS resolution issues
    // in early boot. GCE's metadata service is available at the same link-local IP as AWS.
    let userdata = client
        .get("http://169.254.169.254/computeMetadata/v1/instance/attributes/user-data")
        .header("Metadata-Flavor", "Google")
        .send()
        .await?
        .text()
        .await?;

    serde_json::from_str(&userdata).context("Failed to parse GCE user-data as JSON")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    #[cfg(feature = "config-cmdline")]
    fn test_parse_cmdline_basic() {
        let cmdline = "console=ttyS0 nixos-boot.closure=/nix/store/abc-nixos root=/dev/sda1";
        let config = parse_cmdline(cmdline).unwrap();
        assert_eq!(
            config.system_closure,
            Some("/nix/store/abc-nixos".to_string())
        );
    }

    #[test]
    #[cfg(feature = "config-cmdline")]
    fn test_parse_cmdline_multiple_caches() {
        let cmdline = "nixos-boot.cache=https://cache1.example.com,https://cache2.example.com";
        let config = parse_cmdline(cmdline).unwrap();
        assert_eq!(config.binary_caches.len(), 2);
    }

    #[test]
    #[cfg(feature = "config-cmdline")]
    fn test_parse_cmdline_no_params() {
        let cmdline = "console=ttyS0 root=/dev/sda1";
        assert!(parse_cmdline(cmdline).is_err());
    }
}
