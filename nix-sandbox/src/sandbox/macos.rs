use anyhow::Result;
use std::collections::HashMap;
use std::io::Write;
use std::os::unix::process::CommandExt;
use std::process::Command;
use tempfile::NamedTempFile;
use tracing::info;

use crate::constants::{binaries, devices, env_vars, filesystem, macos_sandbox, paths};
use crate::environment::Environment;
use crate::sandbox::prepare_sandbox_env_vars;
use crate::session::Session;

pub async fn enter_sandbox(
    session: &Session,
    environment: &Environment,
    environment_vars: &HashMap<String, String>,
) -> Result<()> {
    let project_dir = session.project_dir();
    let shell_command = environment.shell_command();

    info!("Creating macOS sandbox profile");

    // Create sandbox profile
    let mut profile_file = NamedTempFile::new()?;
    // Get user's home directory for Nix config path
    let home_dir = std::env::var(env_vars::HOME).unwrap_or_else(|_| "/tmp".to_string());
    let user_nix_config = std::path::Path::new(&home_dir).join(paths::NIX_USER_CONFIG_REL);

    let profile_content = format!(
        r#"
(version 1)
(deny default)

; Allow stdio
(allow file-read* file-write* (literal "{}") (literal "{}") (literal "{}"))
(allow file-read* file-write* (regex #"^/dev/fd/"))
(allow file-read* file-write* (regex #"^/dev/pts/"))
(allow file-read* file-write* (regex #"^/dev/tty"))

; Allow network access (for Nix daemon)
(allow network*)

; Allow Nix store read access
(allow file-read* (regex #"^{}"))

; Allow Nix daemon socket access
(allow file-read* file-write* (literal "{}"))

; Allow Nix configuration access (system-wide and user-specific)
(allow file-read* (regex #"^{}/"))
(allow file-read* (regex #"^{}"))

; Allow project directory full access
(allow file-read* file-write* (regex #"^{}/"))

; Allow temp directory access
(allow file-read* file-write* (regex #"^/tmp/"))
(allow file-read* file-write* (regex #"^/var/folders/"))
(allow file-read* file-write* (regex #"^/private/tmp/"))
(allow file-read* file-write* (regex #"^/private/var/folders/"))

; Allow process operations
(allow process*)
(allow signal* (target self))
(allow signal* (target children))
(allow sysctl-read)

; Allow mach operations
(allow mach*)
(allow mach-lookup)

; Additional system operations
(allow system-socket)
(allow system-fsctl)
(allow system-fcntl)

; Allow file metadata operations
(allow file-read-metadata)
(allow file-read-xattr)
(allow file-write-xattr)

; Allow IPC
(allow ipc-posix*)
"#,
        devices::NULL,
        devices::RANDOM,
        devices::URANDOM,
        filesystem::NIX_STORE_DIR,
        paths::NIX_DAEMON_SOCKET,
        paths::NIX_SYSTEM_CONFIG,
        user_nix_config.to_string_lossy(),
        project_dir.to_string_lossy()
    );

    profile_file.write_all(profile_content.as_bytes())?;
    profile_file.flush()?;

    // Build sandbox-exec command
    let mut cmd = Command::new(binaries::SANDBOX_EXEC);
    cmd.args([
        macos_sandbox::PROFILE_FLAG,
        profile_file.path().to_str().unwrap(),
    ]);
    cmd.current_dir(project_dir);

    // Add all environment variables
    let env_vars = prepare_sandbox_env_vars(project_dir, environment_vars);
    for (key, value) in &env_vars {
        cmd.env(key, value);
    }

    // Add shell command (use bash to ensure proper environment)
    cmd.args(["/bin/bash", "-c", &shell_command]);

    // Replace the current process
    Err(cmd.exec().into())
}

pub async fn exec_in_sandbox(
    session: &Session,
    _environment: &Environment,
    environment_vars: &HashMap<String, String>,
    command: String,
    args: Vec<String>,
) -> Result<()> {
    let project_dir = session.project_dir();

    info!("Creating macOS sandbox profile for exec");

    // Create sandbox profile
    let mut profile_file = NamedTempFile::new()?;
    // Get user's home directory for Nix config path
    let home_dir = std::env::var(env_vars::HOME).unwrap_or_else(|_| "/tmp".to_string());
    let user_nix_config = std::path::Path::new(&home_dir).join(paths::NIX_USER_CONFIG_REL);

    let profile_content = format!(
        r#"
(version 1)
(deny default)

; Allow stdio
(allow file-read* file-write* (literal "{}") (literal "{}") (literal "{}"))
(allow file-read* file-write* (regex #"^/dev/fd/"))
(allow file-read* file-write* (regex #"^/dev/pts/"))
(allow file-read* file-write* (regex #"^/dev/tty"))

; Allow network access (for Nix daemon)
(allow network*)

; Allow Nix store read access
(allow file-read* (regex #"^{}"))

; Allow Nix daemon socket access
(allow file-read* file-write* (literal "{}"))

; Allow Nix configuration access (system-wide and user-specific)
(allow file-read* (literal "{}"))
(allow file-read* (regex #"^{}"))

; Allow project directory full access
(allow file-read* file-write* file-write-create file-write-unlink (regex #"^{}"))

; Allow basic system calls
(allow signal)
(allow system*)
(allow process*)
(allow mach*)
(allow sysctl-read)

; Allow file system operations in project directory
(allow file-ioctl)
(allow file-read-metadata)
(allow file-write-metadata)
(allow file-read-xattr)
(allow file-write-xattr)

; Allow IPC
(allow ipc-posix*)
"#,
        devices::NULL,
        devices::RANDOM,
        devices::URANDOM,
        filesystem::NIX_STORE_DIR,
        paths::NIX_DAEMON_SOCKET,
        paths::NIX_SYSTEM_CONFIG,
        user_nix_config.to_string_lossy(),
        project_dir.to_string_lossy()
    );

    profile_file.write_all(profile_content.as_bytes())?;
    profile_file.flush()?;

    // Build sandbox-exec command
    let mut cmd = Command::new(binaries::SANDBOX_EXEC);
    cmd.args([
        macos_sandbox::PROFILE_FLAG,
        profile_file.path().to_str().unwrap(),
    ]);
    cmd.current_dir(project_dir);

    // Add all environment variables
    let env_vars = prepare_sandbox_env_vars(project_dir, environment_vars);
    for (key, value) in &env_vars {
        cmd.env(key, value);
    }

    // Add the command and its arguments
    cmd.arg(&command);
    for arg in args {
        cmd.arg(arg);
    }

    // Replace the current process
    Err(cmd.exec().into())
}
