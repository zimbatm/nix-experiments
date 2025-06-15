use thiserror::Error;
use std::path::PathBuf;

#[derive(Error, Debug)]
pub enum SandboxError {
    #[error("No environment definition found (neither flake.nix nor devenv.nix) in {0}")]
    NoEnvironmentFound(PathBuf),
    
    #[error("Git operation failed: {0}")]
    GitError(String),
    
    #[error("Sandbox setup failed: {0}")]
    SandboxSetupError(String),
    
    #[error("Unsupported operating system: {0}")]
    UnsupportedOS(String),
    
    #[error("Session error: {0}")]
    SessionError(String),
    
    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),
    
    #[error("Configuration error: {0}")]
    ConfigError(String),
    
    #[error("Binary '{0}' not found in PATH")]
    BinaryNotFound(String),
    
    #[error("Binary '{binary}' resolves to '{path}' which is not in /nix/store")]
    BinaryNotInNixStore {
        binary: String,
        path: PathBuf,
    },
    
    #[error("Cache operation '{operation}' failed for {path}: {source}")]
    CacheError {
        operation: String,
        path: PathBuf,
        source: anyhow::Error,
    },
}