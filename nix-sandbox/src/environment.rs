use anyhow::Result;
use std::path::{Path, PathBuf};
use std::time::SystemTime;
use sha2::{Sha256, Digest};

use crate::constants::{binaries, environment, nix_commands, paths};
use crate::error::SandboxError;

#[derive(Debug, Clone, PartialEq)]
pub enum EnvironmentType {
    Flake,
    Devenv,
}

#[derive(Debug, Clone)]
pub struct Environment {
    project_dir: PathBuf,
    env_type: EnvironmentType,
    resolved_binary: PathBuf,
}

impl Environment {
    pub fn detect(project_dir: &Path) -> Result<Self> {
        let flake_path = project_dir.join(environment::FLAKE_NIX);
        let devenv_path = project_dir.join(environment::DEVENV_NIX);
        
        let env_type = if flake_path.exists() {
            EnvironmentType::Flake
        } else if devenv_path.exists() {
            EnvironmentType::Devenv
        } else {
            return Err(SandboxError::NoEnvironmentFound(project_dir.to_path_buf()).into());
        };
        
        let resolved_binary = Self::resolve_binary(&env_type)?;
        
        Ok(Environment {
            project_dir: project_dir.to_path_buf(),
            env_type,
            resolved_binary,
        })
    }
    
    pub fn project_dir(&self) -> &Path {
        &self.project_dir
    }
    
    pub fn env_type(&self) -> &EnvironmentType {
        &self.env_type
    }
    
    pub fn shell_command(&self) -> String {
        let binary_path = self.resolved_binary.to_string_lossy();
        match self.env_type {
            EnvironmentType::Flake => format!("{} {}", binary_path, nix_commands::DEVELOP_IMPURE.join(" ")),
            EnvironmentType::Devenv => binary_path.to_string(),
        }
    }
    
    pub fn resolved_binary(&self) -> &Path {
        &self.resolved_binary
    }
    
    fn resolve_binary(env_type: &EnvironmentType) -> Result<PathBuf> {
        let binary_name = match env_type {
            EnvironmentType::Flake => binaries::NIX,
            EnvironmentType::Devenv => binaries::NIX_SHELL,
        };
        
        // Use which to resolve the binary path
        let mut resolved_path = which::which(binary_name)
            .map_err(|_| SandboxError::BinaryNotFound(binary_name.to_string()))?;
        
        // Follow symlinks until we get the actual binary
        loop {
            match std::fs::read_link(&resolved_path) {
                Ok(target) => {
                    // If the target is relative, resolve it relative to the symlink's directory
                    if target.is_absolute() {
                        resolved_path = target;
                    } else {
                        if let Some(parent) = resolved_path.parent() {
                            resolved_path = parent.join(target);
                        }
                    }
                }
                Err(_) => break, // Not a symlink or can't read it, we're done
            }
        }
        
        // Verify the final resolved binary is in /nix/store
        let path_str = resolved_path.to_string_lossy();
        if !path_str.starts_with(paths::NIX_STORE) {
            return Err(SandboxError::BinaryNotInNixStore {
                binary: binary_name.to_string(),
                path: resolved_path,
            }.into());
        }
        
        Ok(resolved_path)
    }
    
    pub fn cache_key(&self) -> Result<String> {
        let mut hasher = Sha256::new();
        
        match self.env_type {
            EnvironmentType::Flake => {
                let flake_path = self.project_dir.join(environment::FLAKE_NIX);
                let lock_path = self.project_dir.join(environment::FLAKE_LOCK);
                
                let flake_mtime = std::fs::metadata(&flake_path)?
                    .modified()?
                    .duration_since(SystemTime::UNIX_EPOCH)?
                    .as_secs();
                hasher.update(flake_mtime.to_le_bytes());
                
                if lock_path.exists() {
                    let lock_mtime = std::fs::metadata(&lock_path)?
                        .modified()?
                        .duration_since(SystemTime::UNIX_EPOCH)?
                        .as_secs();
                    hasher.update(lock_mtime.to_le_bytes());
                }
            }
            EnvironmentType::Devenv => {
                let devenv_path = self.project_dir.join(environment::DEVENV_NIX);
                let lock_path = self.project_dir.join(environment::DEVENV_LOCK);
                
                let devenv_mtime = std::fs::metadata(&devenv_path)?
                    .modified()?
                    .duration_since(SystemTime::UNIX_EPOCH)?
                    .as_secs();
                hasher.update(devenv_mtime.to_le_bytes());
                
                if lock_path.exists() {
                    let lock_mtime = std::fs::metadata(&lock_path)?
                        .modified()?
                        .duration_since(SystemTime::UNIX_EPOCH)?
                        .as_secs();
                    hasher.update(lock_mtime.to_le_bytes());
                }
            }
        }
        
        Ok(hex::encode(hasher.finalize()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;
    use std::fs;
    
    #[test]
    fn test_detect_flake_environment() {
        let temp_dir = TempDir::new().unwrap();
        let flake_path = temp_dir.path().join(environment::FLAKE_NIX);
        fs::write(&flake_path, "{}").unwrap();
        
        let env = Environment::detect(temp_dir.path()).unwrap();
        assert_eq!(env.env_type(), &EnvironmentType::Flake);
        assert!(env.shell_command().ends_with(" develop --impure"));
    }
    
    #[test]
    fn test_detect_devenv_environment() {
        let temp_dir = TempDir::new().unwrap();
        let devenv_path = temp_dir.path().join(environment::DEVENV_NIX);
        fs::write(&devenv_path, "{}").unwrap();
        
        let env = Environment::detect(temp_dir.path()).unwrap();
        assert_eq!(env.env_type(), &EnvironmentType::Devenv);
        let shell_cmd = env.shell_command();
        // nix-shell resolves to the nix binary in /nix/store
        assert!(shell_cmd.contains("/nix/store/") && shell_cmd.ends_with("/bin/nix"));
    }
    
    #[test]
    fn test_detect_no_environment() {
        let temp_dir = TempDir::new().unwrap();
        
        let result = Environment::detect(temp_dir.path());
        assert!(result.is_err());
    }
    
    #[test]
    fn test_cache_key_changes_with_file_modification() {
        let temp_dir = TempDir::new().unwrap();
        let flake_path = temp_dir.path().join(environment::FLAKE_NIX);
        fs::write(&flake_path, "{}").unwrap();
        
        let env = Environment::detect(temp_dir.path()).unwrap();
        let key1 = env.cache_key().unwrap();
        
        // Modify the file - sleep at least 1 second to ensure mtime changes
        std::thread::sleep(std::time::Duration::from_secs(1));
        fs::write(&flake_path, "{modified}").unwrap();
        
        let env2 = Environment::detect(temp_dir.path()).unwrap();
        let key2 = env2.cache_key().unwrap();
        
        assert_ne!(key1, key2);
    }
}