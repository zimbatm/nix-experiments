//! Network connectivity verification
//!
//! Network setup (module loading and DHCP) is handled by the init script
//! before this installer runs. This module only verifies connectivity to
//! binary caches.

use anyhow::Result;
use std::error::Error;
use std::net::ToSocketAddrs;
use tracing::info;

use crate::config::InstallerConfig;

/// Try to resolve a hostname to verify DNS is working
fn test_dns_resolution(hostname: &str) {
    info!("Testing DNS resolution for {}", hostname);
    match format!("{}:443", hostname).to_socket_addrs() {
        Ok(addrs) => {
            let addrs: Vec<_> = addrs.collect();
            info!("  DNS resolved {} to {:?}", hostname, addrs);
        }
        Err(e) => {
            info!("  DNS resolution failed for {}: {}", hostname, e);
        }
    }
}

/// Test raw TCP connectivity to an IP address (no DNS involved)
fn test_tcp_connectivity(addr: &str) {
    use std::net::TcpStream;
    use std::time::Duration;

    info!("Testing TCP connectivity to {}...", addr);
    match TcpStream::connect_timeout(
        &addr.parse().expect("invalid address"),
        Duration::from_secs(5),
    ) {
        Ok(_) => info!("  TCP connection to {} succeeded", addr),
        Err(e) => info!("  TCP connection to {} failed: {}", addr, e),
    }
}

/// Verify connectivity to at least one binary cache
pub async fn verify_cache_connectivity(config: &InstallerConfig) -> Result<()> {
    // Check /etc/resolv.conf contents
    match std::fs::read_to_string("/etc/resolv.conf") {
        Ok(content) => {
            info!("/etc/resolv.conf contents:\n{}", content);
        }
        Err(e) => {
            info!("Could not read /etc/resolv.conf: {}", e);
        }
    }

    // Test raw TCP connectivity first (no DNS)
    // 8.8.8.8:53 is Google's public DNS
    test_tcp_connectivity("8.8.8.8:53");
    // 151.101.2.217:443 is one of Fastly's IPs (used by cache.nixos.org)
    test_tcp_connectivity("151.101.2.217:443");

    // Then check DNS resolution
    info!("Checking DNS resolution...");
    test_dns_resolution("cache.nixos.org");
    test_dns_resolution("numtide.cachix.org");

    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(5))
        .build()?;

    for cache in &config.binary_caches {
        let nix_cache_info = format!("{}/nix-cache-info", cache);
        info!("Checking connectivity to {}", nix_cache_info);

        match client.get(&nix_cache_info).send().await {
            Ok(resp) if resp.status().is_success() => {
                info!("Connected to {}", cache);
                return Ok(());
            }
            Ok(resp) => {
                info!("Got status {} from {}", resp.status(), cache);
            }
            Err(e) => {
                info!("Failed to connect to {}: {}", cache, e);
                // More detailed error info
                if e.is_timeout() {
                    info!("  Error type: timeout");
                }
                if e.is_connect() {
                    info!("  Error type: connection error");
                }
                if e.is_request() {
                    info!("  Error type: request error");
                }
                if let Some(source) = e.source() {
                    info!("  Source: {}", source);
                }
            }
        }
    }

    anyhow::bail!("Could not reach any binary cache")
}
