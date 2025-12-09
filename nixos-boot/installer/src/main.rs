//! NixOS Boot Installer Agent
//!
//! A minimal installer that runs from an initrd/UKI to bootstrap a NixOS system.
//!
//! Configuration sources (in order of priority):
//! 1. Command line config file (--config)
//! 2. Kernel command line parameters (nixos-boot.*)
//! 3. File-based user-data (cloud-init style)
//! 4. Cloud metadata services (GCE, EC2)
//! 5. Interactive console configuration

use anyhow::{Context, Result};
use clap::Parser;
use tracing::{error, info};

mod config;
mod network;
mod partition;
mod progress;
mod store;

use config::InstallerConfig;

#[derive(Parser, Debug)]
#[command(name = "nixos-boot-installer")]
#[command(about = "Minimal NixOS installer agent")]
struct Args {
    /// Path to configuration file (optional, will probe for config sources)
    #[arg(short, long)]
    config: Option<std::path::PathBuf>,

    /// Dry run mode - don't make any changes
    #[arg(long)]
    dry_run: bool,

    /// Target disk device (e.g., /dev/sda, /dev/nvme0n1)
    #[arg(short, long)]
    disk: Option<String>,

    /// Only partition and format the disk, skip store operations
    #[arg(long)]
    partition_only: bool,
}

/// Mount essential filesystems needed for init
fn mount_essential_filesystems() -> Result<()> {
    use std::fs;
    use std::process::Command;

    // Create mount points if they don't exist
    for dir in &["/proc", "/sys", "/dev", "/run", "/tmp"] {
        fs::create_dir_all(dir).ok();
    }

    // Mount proc
    let status = Command::new("/bin/mount")
        .args(["-t", "proc", "proc", "/proc"])
        .status();
    if status.is_err() || !status.unwrap().success() {
        // Try without /bin prefix (might be in PATH or busybox symlink)
        Command::new("mount")
            .args(["-t", "proc", "proc", "/proc"])
            .status()
            .context("Failed to mount /proc")?;
    }

    // Mount sysfs
    Command::new("mount")
        .args(["-t", "sysfs", "sysfs", "/sys"])
        .status()
        .context("Failed to mount /sys")?;

    // Mount devtmpfs
    Command::new("mount")
        .args(["-t", "devtmpfs", "devtmpfs", "/dev"])
        .status()
        .context("Failed to mount /dev")?;

    // Mount tmpfs on /run
    Command::new("mount")
        .args(["-t", "tmpfs", "tmpfs", "/run"])
        .status()
        .context("Failed to mount /run")?;

    Ok(())
}

/// Check if we're running as PID 1 (init)
fn is_init() -> bool {
    std::process::id() == 1
}

/// Check if we're in non-interactive mode (e.g., for automated testing)
/// This is determined by the kernel cmdline parameter nixos-boot.noninteractive=true
fn is_noninteractive() -> bool {
    if let Ok(cmdline) = std::fs::read_to_string("/proc/cmdline") {
        cmdline.contains("nixos-boot.noninteractive=true")
            || cmdline.contains("nixos-boot.noninteractive=1")
    } else {
        false
    }
}

/// Print a banner when starting as init
fn print_banner() {
    // ANSI escape codes for colors
    const CYAN: &str = "\x1b[36m";
    const WHITE: &str = "\x1b[97m";
    const RESET: &str = "\x1b[0m";

    println!();
    println!("{CYAN}    _   _ _       ___  ____    ____              _   {RESET}");
    println!("{CYAN}   | \\ | (_)_  __/ _ \\/ ___|  | __ )  ___   ___ | |_ {RESET}");
    println!("{CYAN}   |  \\| | \\ \\/ / | | \\___ \\  |  _ \\ / _ \\ / _ \\| __|{RESET}");
    println!("{CYAN}   | |\\  | |>  <| |_| |___) | | |_) | (_) | (_) | |_ {RESET}");
    println!("{CYAN}   |_| \\_|_/_/\\_\\\\___/|____/  |____/ \\___/ \\___/ \\__|{RESET}");
    println!();
    println!("{WHITE}              Universal NixOS Installer{RESET}");
    println!("{WHITE}                    v0.1.0{RESET}");
    println!();
}

/// Print an error banner
fn print_error_banner(error_msg: &str) {
    // ANSI escape codes for colors
    const RED: &str = "\x1b[31m";
    const YELLOW: &str = "\x1b[33m";
    const RESET: &str = "\x1b[0m";

    println!();
    println!("{RED}    ____  ____  ____  ____  ____ {RESET}");
    println!("{RED}   | ___||  _ \\|  _ \\/ __ \\|  _ \\{RESET}");
    println!("{RED}   | |__ | |_) | |_) | |  | | |_) |{RESET}");
    println!("{RED}   |  __||  _ <|  _ <| |  | |  _ < {RESET}");
    println!("{RED}   | |___| | \\ | | \\ | |__| | | \\ \\{RESET}");
    println!("{RED}   |_____|_|  \\|_|  \\_\\____/|_|  \\_\\{RESET}");
    println!();
    println!("{YELLOW}   Oops! Something went wrong during installation.{RESET}");
    println!();
    println!("{RED}   Error: {error_msg}{RESET}");
    println!();
}

/// Display menu and get user choice
fn show_menu() -> MenuChoice {
    use std::io::{self, Write};

    // ANSI escape codes
    const GREEN: &str = "\x1b[32m";
    const YELLOW: &str = "\x1b[33m";
    const CYAN: &str = "\x1b[36m";
    const RESET: &str = "\x1b[0m";

    println!("{CYAN}   What would you like to do?{RESET}");
    println!();
    println!("   {GREEN}[1]{RESET} Retry installation");
    println!("   {GREEN}[2]{RESET} Drop to shell");
    println!("   {GREEN}[3]{RESET} Power off");
    println!("   {GREEN}[4]{RESET} Reboot");
    println!();
    print!("{YELLOW}   Enter choice [1-4]: {RESET}");
    io::stdout().flush().ok();

    let mut input = String::new();
    if io::stdin().read_line(&mut input).is_err() {
        return MenuChoice::Shell;
    }

    match input.trim() {
        "1" => MenuChoice::Retry,
        "2" => MenuChoice::Shell,
        "3" => MenuChoice::PowerOff,
        "4" => MenuChoice::Reboot,
        _ => {
            println!("   Invalid choice, dropping to shell...");
            MenuChoice::Shell
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq)]
enum MenuChoice {
    Retry,
    Shell,
    PowerOff,
    Reboot,
}

/// Drop to a shell for debugging
fn drop_to_shell() {
    use std::process::Command;

    println!();
    println!("   Dropping to shell for debugging...");
    println!("   Type 'exit' to return to menu.");
    println!();

    // Try sh first, then ash (busybox)
    let shell = if std::path::Path::new("/bin/sh").exists() {
        "/bin/sh"
    } else if std::path::Path::new("/bin/ash").exists() {
        "/bin/ash"
    } else {
        eprintln!("   No shell found!");
        return;
    };

    let _ = Command::new(shell).status();
}

/// Power off the system
fn power_off() {
    println!("   Powering off...");
    let _ = std::process::Command::new("poweroff").arg("-f").status();
}

/// Reboot the system
fn reboot() {
    println!("   Rebooting...");
    let _ = std::process::Command::new("reboot").arg("-f").status();
}

#[tokio::main]
async fn main() -> Result<()> {
    // Check if we're running as init (PID 1)
    let running_as_init = is_init();

    if running_as_init {
        print_banner();
        println!("   Running as init (PID 1)");
        println!();

        // Mount essential filesystems first
        if let Err(e) = mount_essential_filesystems() {
            print_error_banner(&format!("Failed to mount filesystems: {}", e));
            loop {
                match show_menu() {
                    MenuChoice::Retry => continue,
                    MenuChoice::Shell => drop_to_shell(),
                    MenuChoice::PowerOff => power_off(),
                    MenuChoice::Reboot => reboot(),
                }
            }
        }
        println!("   Mounted essential filesystems");
        println!();
    }

    // Initialize logging
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive(tracing::Level::INFO.into()),
        )
        .init();

    // Main installation loop
    loop {
        let result = run_installer().await;

        match result {
            Ok(()) => {
                if running_as_init {
                    println!();
                    println!("   Installation complete!");
                    println!("   The system will reboot in 5 seconds...");
                    std::thread::sleep(std::time::Duration::from_secs(5));
                    reboot();
                }
                return Ok(());
            }
            Err(ref e) => {
                error!("Installation failed: {:?}", e);

                if running_as_init {
                    print_error_banner(&format!("{:#}", e));

                    // Check if we're in non-interactive mode (kernel cmdline parameter)
                    // In this case, exit with error instead of showing menu
                    if is_noninteractive() {
                        println!();
                        println!("   Non-interactive mode detected, exiting with error...");
                        println!("   E2E-TEST-FAILURE");
                        println!();
                        // Power off so the VM exits
                        power_off();
                        return Err(anyhow::anyhow!("Installation failed (non-interactive)"));
                    }

                    match show_menu() {
                        MenuChoice::Retry => {
                            println!();
                            println!("   Retrying installation...");
                            println!();
                            continue;
                        }
                        MenuChoice::Shell => {
                            drop_to_shell();
                            continue;
                        }
                        MenuChoice::PowerOff => {
                            power_off();
                            // Should never reach here
                            return Err(anyhow::anyhow!("Power off failed"));
                        }
                        MenuChoice::Reboot => {
                            reboot();
                            // Should never reach here
                            return Err(anyhow::anyhow!("Reboot failed"));
                        }
                    }
                } else {
                    return result;
                }
            }
        }
    }
}

async fn run_installer() -> Result<()> {
    use progress::{output, step, Step};

    let args = Args::parse();

    output("NixOS Boot Installer starting...");

    // Load configuration from various sources (may use network for cloud metadata)
    // Note: Network setup (module loading and DHCP) is handled by the init script
    let config = load_config(&args).await?;

    if args.dry_run {
        output("Dry run mode enabled - no changes will be made");
        output(&format!("Configuration loaded:\n{:#?}", config));
        return Ok(());
    }

    // Verify connectivity to binary caches before proceeding
    step(Step::Network, "Verifying cache connectivity...");
    network::verify_cache_connectivity(&config).await?;
    step(Step::Network, "Connected to binary cache");

    // Step 2: Detect and select target disk
    // Note: Storage modules are loaded by the init script before we run
    step(Step::DiskDetection, "Scanning for disks...");
    let target_disk = if let Some(disk) = args.disk.as_deref() {
        step(Step::DiskDetection, &format!("Using CLI disk: {}", disk));
        disk.to_string()
    } else if let Some(disk) = config.target_disk.as_deref() {
        step(Step::DiskDetection, &format!("Using config disk: {}", disk));
        disk.to_string()
    } else {
        let disks = partition::detect_disks();

        if disks.is_empty() {
            anyhow::bail!("No suitable disks found for installation");
        }

        partition::print_disk_list(&disks);

        match partition::select_disk(&disks) {
            Some(disk) => {
                step(
                    Step::DiskDetection,
                    &format!("Selected: {} ({})", disk.device, disk.model),
                );
                disk.device.clone()
            }
            None => anyhow::bail!("No suitable disk found for installation"),
        }
    };

    // Step 3: Partition the disk
    step(Step::Partition, &format!("Partitioning {}...", target_disk));
    partition::partition_disk(&target_disk, &config).await?;
    step(Step::Partition, "Complete");

    // Step 4: Mount the partitions
    step(Step::Mount, "Mounting filesystems...");
    partition::mount_target(&target_disk).await?;
    step(Step::Mount, "Mounted at /mnt");

    // If partition-only mode, stop here
    if args.partition_only || config.partition_only {
        output("Partition-only mode: disk partitioned and mounted at /mnt");
        return Ok(());
    }

    // Step 5: Fetch and install the system closure
    step(Step::FetchClosure, "Downloading NixOS system...");
    store::fetch_closure(&config).await?;
    step(Step::FetchClosure, "Complete");

    // Step 6: Install bootloader and finalize
    step(Step::InstallBootloader, "Installing bootloader...");
    store::install_bootloader(&config).await?;
    step(Step::InstallBootloader, "Complete");

    step(Step::Complete, "System ready!");

    Ok(())
}

async fn load_config(args: &Args) -> Result<InstallerConfig> {
    info!("Searching for configuration...");

    // Log which configuration sources are enabled at compile time
    let mut enabled_sources: Vec<&str> = vec!["cli-file"];
    #[cfg(feature = "config-cmdline")]
    enabled_sources.push("cmdline");
    #[cfg(feature = "config-file")]
    enabled_sources.push("file");
    #[cfg(feature = "config-gce")]
    enabled_sources.push("gce");
    #[cfg(feature = "config-ec2")]
    enabled_sources.push("ec2");
    #[cfg(feature = "config-interactive")]
    enabled_sources.push("interactive");

    info!("Enabled configuration sources: {}", enabled_sources.join(", "));

    // Try explicit config file first
    if let Some(config_path) = &args.config {
        info!("[cli-file] Trying config file: {:?}", config_path);
        let content = tokio::fs::read_to_string(config_path)
            .await
            .context("Failed to read config file")?;
        info!("[cli-file] SUCCESS - Loaded config from {:?}", config_path);
        return serde_json::from_str(&content).context("Failed to parse config file");
    }

    // Try kernel command line
    #[cfg(feature = "config-cmdline")]
    {
        info!("[cmdline] Trying kernel command line parameters...");
        match config::from_cmdline().await {
            Ok(config) => {
                info!("[cmdline] SUCCESS - Loaded config from kernel command line");
                return Ok(config);
            }
            Err(e) => {
                info!("[cmdline] Not found: {}", e);
            }
        }
    }

    // Try file-based user-data (cloud-init style)
    #[cfg(feature = "config-file")]
    {
        info!("[file] Trying file-based user-data...");
        match config::from_userdata_file().await {
            Ok(config) => {
                info!("[file] SUCCESS - Loaded config from file-based user-data");
                return Ok(config);
            }
            Err(e) => {
                info!("[file] Not found: {}", e);
            }
        }
    }

    // Try GCE metadata service (network should already be up from run_installer)
    #[cfg(feature = "config-gce")]
    {
        info!("[gce] Trying GCE metadata service...");
        match config::from_gce_metadata().await {
            Ok(config) => {
                info!("[gce] SUCCESS - Loaded config from GCE metadata service");
                return Ok(config);
            }
            Err(e) => {
                info!("[gce] Not found: {}", e);
            }
        }
    }

    // Try EC2 metadata service
    #[cfg(feature = "config-ec2")]
    {
        info!("[ec2] Trying EC2 metadata service (IMDSv2)...");
        match config::from_ec2_metadata().await {
            Ok(config) => {
                info!("[ec2] SUCCESS - Loaded config from EC2 metadata service");
                return Ok(config);
            }
            Err(e) => {
                info!("[ec2] Not found: {}", e);
            }
        }
    }

    // No configuration found - start web UI for interactive configuration
    #[cfg(feature = "config-interactive")]
    {
        info!("[interactive] No automatic configuration found, starting interactive mode");
        return interactive_config().await;
    }

    #[cfg(not(feature = "config-interactive"))]
    {
        anyhow::bail!("No configuration found and interactive mode is disabled")
    }
}

/// Interactive configuration via console
#[cfg(feature = "config-interactive")]
async fn interactive_config() -> Result<InstallerConfig> {
    use std::io::{self, BufRead, Write};

    info!("[interactive] Starting console-based configuration");

    println!();
    println!("   =============================================");
    println!("   Interactive NixOS Boot Configuration");
    println!("   =============================================");
    println!();

    let mut config = InstallerConfig::default();
    let stdin = io::stdin();
    let mut stdout = io::stdout();

    // Prompt for system closure
    print!("   Enter NixOS system closure path (e.g., /nix/store/...): ");
    stdout.flush().ok();

    let mut closure = String::new();
    stdin.lock().read_line(&mut closure)?;
    let closure = closure.trim().to_string();

    if !closure.is_empty() {
        config.system_closure = Some(closure.clone());
        println!("   Using closure: {}", closure);
    } else {
        println!("   No closure specified - will need to be provided later");
    }

    // Prompt for binary cache (optional, has default)
    print!("   Enter binary cache URL (press Enter for default cache.nixos.org): ");
    stdout.flush().ok();

    let mut cache = String::new();
    stdin.lock().read_line(&mut cache)?;
    let cache = cache.trim().to_string();

    if !cache.is_empty() {
        config.binary_caches = vec![cache.clone(), "https://cache.nixos.org".to_string()];
        println!("   Using caches: {}, https://cache.nixos.org", cache);
    } else {
        println!("   Using default cache: https://cache.nixos.org");
    }

    println!();

    Ok(config)
}
