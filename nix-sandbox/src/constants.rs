/// System paths and constants used throughout the application
pub mod paths {
    /// The root path for the Nix store
    pub const NIX_STORE: &str = "/nix/store/";

    /// Path to the Nix daemon socket
    pub const NIX_DAEMON_SOCKET: &str = "/nix/var/nix/daemon-socket/socket";

    /// NixOS system binaries path (symlinks that resolve to /nix/store)
    pub const _NIXOS_SYSTEM_PATH: &str = "/run/current-system/sw/";

    /// System-wide Nix configuration directory
    pub const NIX_SYSTEM_CONFIG: &str = "/etc/nix";

    /// User Nix configuration directory (relative to HOME)
    pub const NIX_USER_CONFIG_REL: &str = ".config/nix";
}

/// Environment file names and extensions
pub mod environment {
    /// Nix flake configuration file
    pub const FLAKE_NIX: &str = "flake.nix";

    /// Nix flake lock file
    pub const FLAKE_LOCK: &str = "flake.lock";

    /// Devenv configuration file
    pub const DEVENV_NIX: &str = "devenv.nix";

    /// Devenv lock file
    pub const DEVENV_LOCK: &str = "devenv.lock";
}

/// Binary names used for environment management
pub mod binaries {
    /// The main Nix binary
    pub const NIX: &str = "nix";

    /// Legacy nix-shell binary (usually symlinks to nix)
    pub const NIX_SHELL: &str = "nix-shell";

    /// Bubblewrap binary for Linux sandboxing
    pub const BUBBLEWRAP: &str = "bwrap";

    /// macOS sandbox-exec binary
    #[cfg(target_os = "macos")]
    pub const SANDBOX_EXEC: &str = "sandbox-exec";
}

/// Sandbox configuration constants
pub mod sandbox {
    /// Default hostname for sandboxed environments
    pub const HOSTNAME: &str = "nix-sandbox";

    /// Default user name in sandboxed environments
    pub const USER: &str = "sandbox";

    /// Default terminal type
    pub const DEFAULT_TERM: &str = "xterm";
}

/// Bubblewrap command line arguments
pub mod bubblewrap {
    /// Die when parent process dies
    pub const DIE_WITH_PARENT: &str = "--die-with-parent";

    /// Unshare all namespaces
    pub const UNSHARE_ALL: &str = "--unshare-all";

    /// Share network namespace with host
    pub const SHARE_NET: &str = "--share-net";

    /// Set hostname
    pub const HOSTNAME: &str = "--hostname";

    /// Set working directory
    pub const CHDIR: &str = "--chdir";

    /// Create /dev directory
    pub const DEV: &str = "--dev";

    /// Bind mount device file
    pub const DEV_BIND: &str = "--dev-bind";

    /// Create /proc filesystem
    pub const PROC: &str = "--proc";

    /// Create tmpfs filesystem
    pub const TMPFS: &str = "--tmpfs";

    /// Bind mount directory (read-write)
    pub const BIND: &str = "--bind";

    /// Bind mount directory (read-only)
    pub const RO_BIND: &str = "--ro-bind";

    /// Create directory
    pub const DIR: &str = "--dir";

    /// Separator for command arguments
    pub const COMMAND_SEPARATOR: &str = "--";
}

/// Device files that need to be accessible in sandbox
pub mod devices {
    /// Null device
    pub const NULL: &str = "/dev/null";

    /// Zero device
    pub const ZERO: &str = "/dev/zero";

    /// Random device
    pub const RANDOM: &str = "/dev/random";

    /// Urandom device
    pub const URANDOM: &str = "/dev/urandom";

    /// TTY device
    pub const TTY: &str = "/dev/tty";
}

/// Filesystem paths used in sandboxing
pub mod filesystem {
    /// Device directory
    pub const DEV_DIR: &str = "/dev";

    /// Process filesystem
    pub const PROC_DIR: &str = "/proc";

    /// Temporary directory
    pub const TMP_DIR: &str = "/tmp";

    /// Nix store directory
    #[cfg(target_os = "macos")]
    pub const NIX_STORE_DIR: &str = "/nix/store";
}

/// Git command arguments and branch operations
pub mod git {
    /// Show current branch command
    pub const BRANCH_SHOW_CURRENT: &[&str] = &["branch", "--show-current"];

    /// Show repository root command
    pub const REV_PARSE_TOPLEVEL: &[&str] = &["rev-parse", "--show-toplevel"];

    /// Worktree add command
    pub const WORKTREE_ADD: &str = "worktree";
    pub const ADD: &str = "add";

    /// Create new branch flag
    pub const NEW_BRANCH_FLAG: &str = "-b";
}

/// Nix command arguments for different environment types
pub mod nix_commands {
    /// Nix develop command with impure flag
    pub const DEVELOP_IMPURE: &[&str] = &["develop", "--impure"];
}

/// macOS sandbox profile flags
#[cfg(target_os = "macos")]
pub mod macos_sandbox {
    /// Profile file flag
    pub const PROFILE_FLAG: &str = "-f";
}

/// Environment variables used in sandbox
pub mod env_vars {
    /// Home directory environment variable
    pub const HOME: &str = "HOME";

    /// User environment variable
    pub const USER: &str = "USER";

    /// Terminal type environment variable
    pub const TERM: &str = "TERM";

    /// XDG state home directory
    pub const XDG_STATE_HOME: &str = "XDG_STATE_HOME";
}

/// Directory names for application state
pub mod app_dirs {
    /// Main application directory name
    pub const APP_NAME: &str = "nix-sandbox";

    /// Sessions subdirectory
    pub const SESSIONS: &str = "sessions";

    /// Cache subdirectory
    pub const CACHE: &str = "cache";

    /// Local state directory path relative to home
    pub const LOCAL_STATE_PATH: &str = ".local/state";
}
