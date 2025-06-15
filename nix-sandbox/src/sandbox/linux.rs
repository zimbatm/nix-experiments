use anyhow::Result;
use std::collections::HashMap;
use std::os::unix::process::CommandExt;
use std::process::Command;
use tracing::info;

use crate::constants::{binaries, bubblewrap, devices, env_vars, filesystem, paths, sandbox};
use crate::environment::Environment;
use crate::error::SandboxError;
use crate::sandbox::prepare_sandbox_env_vars;
use crate::session::Session;

pub async fn enter_sandbox(
    session: &Session,
    environment: &Environment,
    environment_vars: &HashMap<String, String>,
) -> Result<()> {
    // Check if bubblewrap is installed
    let bwrap_path = which::which(binaries::BUBBLEWRAP).map_err(|_| {
        SandboxError::SandboxSetupError("bubblewrap (bwrap) is not installed".into())
    })?;
    info!("Found bwrap at: {:?}", bwrap_path);

    let project_dir = session.project_dir();
    let shell_command = environment.shell_command();

    info!("Using bubblewrap to create sandbox");
    info!("Shell command: {}", shell_command);

    // Build bubblewrap command
    let mut cmd = Command::new(&bwrap_path);

    // Basic setup
    cmd.args([
        bubblewrap::DIE_WITH_PARENT,
        bubblewrap::UNSHARE_ALL,
        bubblewrap::SHARE_NET, // Allow network for Nix daemon
        bubblewrap::HOSTNAME,
        sandbox::HOSTNAME,
    ]);

    // Mount minimal /dev
    cmd.args([
        bubblewrap::DEV,
        filesystem::DEV_DIR,
        bubblewrap::DEV_BIND,
        devices::NULL,
        devices::NULL,
        bubblewrap::DEV_BIND,
        devices::ZERO,
        devices::ZERO,
        bubblewrap::DEV_BIND,
        devices::RANDOM,
        devices::RANDOM,
        bubblewrap::DEV_BIND,
        devices::URANDOM,
        devices::URANDOM,
        bubblewrap::DEV_BIND,
        devices::TTY,
        devices::TTY,
    ]);

    // Mount /proc and /tmp
    cmd.args([
        bubblewrap::PROC,
        filesystem::PROC_DIR,
        bubblewrap::TMPFS,
        filesystem::TMP_DIR,
    ]);

    // Bind mount project directory with full access
    cmd.args([
        bubblewrap::BIND,
        &project_dir.to_string_lossy(),
        &project_dir.to_string_lossy(),
    ]);

    // Bind mount entire /nix directory (writable for daemon socket and temp files)
    cmd.args([bubblewrap::BIND, "/nix", "/nix"]);

    // Bind mount system Nix configuration (read-only)
    // This exposes /etc/nix with system-wide Nix settings like nix.conf
    if std::path::Path::new(paths::NIX_SYSTEM_CONFIG).exists() {
        cmd.args([
            bubblewrap::RO_BIND,
            paths::NIX_SYSTEM_CONFIG,
            paths::NIX_SYSTEM_CONFIG,
        ]);
    }

    // Bind mount user Nix configuration (read-only)
    // This exposes ~/.config/nix with user-specific Nix settings
    if let Ok(home_dir) = std::env::var(env_vars::HOME) {
        let user_nix_config = std::path::Path::new(&home_dir).join(paths::NIX_USER_CONFIG_REL);
        if user_nix_config.exists() {
            cmd.args([
                bubblewrap::RO_BIND,
                &user_nix_config.to_string_lossy(),
                &user_nix_config.to_string_lossy(),
            ]);
        }
    }

    // Set working directory
    cmd.args([bubblewrap::CHDIR, &project_dir.to_string_lossy()]);

    // Add all environment variables
    let env_vars = prepare_sandbox_env_vars(project_dir, environment_vars);
    for (key, value) in &env_vars {
        cmd.env(key, value);
    }

    // Execute the shell command (use bash to ensure proper environment)
    // Get bash from the Nix environment or fall back to system bash
    let bash_path = environment_vars
        .get("BASH")
        .and_then(|path| {
            if std::path::Path::new(path).exists() {
                Some(path.as_str())
            } else {
                None
            }
        })
        .unwrap_or_else(|| {
            // Try common locations
            if std::path::Path::new("/run/current-system/sw/bin/bash").exists() {
                "/run/current-system/sw/bin/bash"
            } else if std::path::Path::new("/usr/bin/bash").exists() {
                "/usr/bin/bash"
            } else {
                // Fall back to /bin/bash and hope for the best
                "/bin/bash"
            }
        });

    cmd.args([bubblewrap::COMMAND_SEPARATOR, bash_path, "-c"]);
    cmd.arg(&shell_command);

    info!("Bash path: {}", bash_path);

    // Replace the current process
    Err(cmd.exec().into())
}

pub async fn exec_in_sandbox(
    session: &Session,
    environment: &Environment,
    environment_vars: &HashMap<String, String>,
    command: String,
    args: Vec<String>,
) -> Result<()> {
    // Check if bubblewrap is installed
    let bwrap_path = which::which(binaries::BUBBLEWRAP).map_err(|_| {
        SandboxError::SandboxSetupError("bubblewrap (bwrap) is not installed".into())
    })?;
    info!("Found bwrap at: {:?}", bwrap_path);

    let project_dir = session.project_dir();

    info!(
        "Using bubblewrap to execute command: {} {:?}",
        command, args
    );

    // Build bubblewrap command
    let mut cmd = Command::new(&bwrap_path);

    // Basic setup
    cmd.args([
        bubblewrap::DIE_WITH_PARENT,
        bubblewrap::UNSHARE_ALL,
        bubblewrap::SHARE_NET, // Allow network for Nix daemon
        bubblewrap::HOSTNAME,
        sandbox::HOSTNAME,
    ]);

    // Mount minimal /dev
    cmd.args([
        bubblewrap::DEV,
        filesystem::DEV_DIR,
        bubblewrap::DEV_BIND,
        devices::NULL,
        devices::NULL,
        bubblewrap::DEV_BIND,
        devices::ZERO,
        devices::ZERO,
        bubblewrap::DEV_BIND,
        devices::URANDOM,
        devices::URANDOM,
        bubblewrap::DEV_BIND,
        devices::RANDOM,
        devices::RANDOM,
    ]);

    // Create /tmp
    cmd.args([bubblewrap::TMPFS, filesystem::TMP_DIR]);

    // Mount /nix/store read-only
    cmd.args([bubblewrap::RO_BIND, paths::NIX_STORE, paths::NIX_STORE]);

    // Mount nix daemon socket if it exists
    let nix_daemon_socket = std::path::Path::new(paths::NIX_DAEMON_SOCKET);
    if let Some(parent) = nix_daemon_socket.parent() {
        if parent.exists() {
            cmd.args([
                bubblewrap::RO_BIND,
                &parent.to_string_lossy(),
                &parent.to_string_lossy(),
            ]);
        }
    }

    // Mount project directory read-write
    cmd.args([
        bubblewrap::BIND,
        &project_dir.to_string_lossy(),
        &project_dir.to_string_lossy(),
    ]);

    // Mount git directory if it exists (for the original project)
    if let Some(git_dir) = environment.project_dir().parent().and_then(|p| p.parent()) {
        let original_git_dir = git_dir.join(".git");
        if original_git_dir.exists() {
            cmd.args([
                bubblewrap::RO_BIND,
                &original_git_dir.to_string_lossy(),
                &original_git_dir.to_string_lossy(),
            ]);
        }
    }

    // Mount user nix config if it exists
    let home_dir = std::env::var("HOME").unwrap_or_else(|_| "/tmp".to_string());
    let user_nix_config = std::path::Path::new(&home_dir).join(".config/nix");
    if user_nix_config.exists() {
        // Create parent directory
        if let Some(parent) = user_nix_config.parent() {
            cmd.args([bubblewrap::DIR, &parent.to_string_lossy()]);
            cmd.args([
                bubblewrap::RO_BIND,
                &user_nix_config.to_string_lossy(),
                &user_nix_config.to_string_lossy(),
            ]);
        }
    }

    // Set working directory
    cmd.args([bubblewrap::CHDIR, &project_dir.to_string_lossy()]);

    // Add all environment variables
    let env_vars = prepare_sandbox_env_vars(project_dir, environment_vars);
    for (key, value) in &env_vars {
        cmd.env(key, value);
    }

    // Execute the command directly
    cmd.args([bubblewrap::COMMAND_SEPARATOR, &command]);
    for arg in args {
        cmd.arg(arg);
    }

    // Replace the current process
    Err(cmd.exec().into())
}
