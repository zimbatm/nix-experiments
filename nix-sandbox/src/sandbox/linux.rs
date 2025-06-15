use anyhow::Result;
use std::process::Command;
use std::os::unix::process::CommandExt;
use tracing::info;

use crate::constants::{binaries, bubblewrap, devices, env_vars, filesystem, paths, sandbox};
use crate::environment::Environment;
use crate::error::SandboxError;
use crate::session::Session;

pub async fn enter_sandbox(session: &Session, environment: &Environment) -> Result<()> {
    // Check if bubblewrap is installed
    which::which(binaries::BUBBLEWRAP)
        .map_err(|_| SandboxError::SandboxSetupError("bubblewrap (bwrap) is not installed".into()))?;
    
    let project_dir = session.project_dir();
    let shell_command = environment.shell_command();
    let shell_args: Vec<&str> = shell_command.split_whitespace().collect();
    
    info!("Using bubblewrap to create sandbox");
    
    // Build bubblewrap command
    let mut cmd = Command::new(binaries::BUBBLEWRAP);
    
    // Basic setup
    cmd.args([
        bubblewrap::DIE_WITH_PARENT,
        bubblewrap::UNSHARE_ALL,
        bubblewrap::SHARE_NET, // Allow network for Nix daemon
        bubblewrap::HOSTNAME, sandbox::HOSTNAME,
    ]);
    
    // Mount minimal /dev
    cmd.args([
        bubblewrap::DEV, filesystem::DEV_DIR,
        bubblewrap::DEV_BIND, devices::NULL, devices::NULL,
        bubblewrap::DEV_BIND, devices::ZERO, devices::ZERO,
        bubblewrap::DEV_BIND, devices::RANDOM, devices::RANDOM,
        bubblewrap::DEV_BIND, devices::URANDOM, devices::URANDOM,
        bubblewrap::DEV_BIND, devices::TTY, devices::TTY,
    ]);
    
    // Mount /proc and /tmp
    cmd.args([
        bubblewrap::PROC, filesystem::PROC_DIR,
        bubblewrap::TMPFS, filesystem::TMP_DIR,
    ]);
    
    // Bind mount project directory with full access
    cmd.args([
        bubblewrap::BIND, &project_dir.to_string_lossy(), &project_dir.to_string_lossy(),
    ]);
    
    // Bind mount Nix store (read-only)
    cmd.args([
        bubblewrap::RO_BIND, filesystem::NIX_STORE_DIR, filesystem::NIX_STORE_DIR,
    ]);
    
    // Bind mount Nix daemon socket
    if std::path::Path::new(paths::NIX_DAEMON_SOCKET).exists() {
        cmd.args([
            bubblewrap::BIND, paths::NIX_DAEMON_SOCKET_DIR, paths::NIX_DAEMON_SOCKET_DIR,
        ]);
    }
    
    // Bind mount system Nix configuration (read-only)
    // This exposes /etc/nix with system-wide Nix settings like nix.conf
    if std::path::Path::new(paths::NIX_SYSTEM_CONFIG).exists() {
        cmd.args([
            bubblewrap::RO_BIND, paths::NIX_SYSTEM_CONFIG, paths::NIX_SYSTEM_CONFIG,
        ]);
    }
    
    // Bind mount user Nix configuration (read-only)
    // This exposes ~/.config/nix with user-specific Nix settings
    if let Ok(home_dir) = std::env::var(env_vars::HOME) {
        let user_nix_config = std::path::Path::new(&home_dir).join(paths::NIX_USER_CONFIG_REL);
        if user_nix_config.exists() {
            cmd.args([
                bubblewrap::RO_BIND, &user_nix_config.to_string_lossy(), &user_nix_config.to_string_lossy(),
            ]);
        }
    }
    
    // Set working directory
    cmd.args([
        bubblewrap::CHDIR, &project_dir.to_string_lossy(),
    ]);
    
    // Add environment variables
    cmd.env(env_vars::HOME, project_dir);
    cmd.env(env_vars::USER, sandbox::USER);
    cmd.env(env_vars::TERM, std::env::var(env_vars::TERM).unwrap_or_else(|_| sandbox::DEFAULT_TERM.to_string()));
    
    // Execute the shell command
    cmd.args([bubblewrap::COMMAND_SEPARATOR, shell_args[0]]);
    cmd.args(&shell_args[1..]);
    
    // Replace the current process
    Err(cmd.exec().into())
}