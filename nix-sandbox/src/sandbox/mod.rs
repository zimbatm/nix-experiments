pub mod linux;
pub mod macos;

use anyhow::Result;
use std::collections::HashMap;
use std::process::Command;
use tracing::{info, warn};

use crate::cache::EnvironmentCache;
use crate::config::Config;
use crate::environment::Environment;
use crate::error::SandboxError;
use crate::session::Session;

pub struct Sandbox {
    config: Config,
    session: Session,
    environment: Environment,
    cache: EnvironmentCache,
}

impl Sandbox {
    pub fn new(config: &Config, session: &Session, environment: &Environment) -> Result<Self> {
        let cache = EnvironmentCache::new(config.clone());
        
        Ok(Sandbox {
            config: config.clone(),
            session: session.clone(),
            environment: environment.clone(),
            cache,
        })
    }
    
    pub async fn enter(&self) -> Result<()> {
        // Check if we have a valid cached environment
        let cached_env = self.cache.get_cached_environment(&self.environment)?;
        
        let environment_vars = if let Some(cached_metadata) = cached_env {
            info!("Using cached environment");
            cached_metadata.environment_vars
        } else {
            info!("Building new environment (no valid cache found)");
            let env_vars = self.build_and_cache_environment().await?;
            env_vars
        };
        
        #[cfg(target_os = "linux")]
        {
            linux::enter_sandbox(&self.session, &self.environment, &environment_vars).await
        }
        
        #[cfg(target_os = "macos")]
        {
            macos::enter_sandbox(&self.session, &self.environment, &environment_vars).await
        }
        
        #[cfg(not(any(target_os = "linux", target_os = "macos")))]
        {
            Err(SandboxError::UnsupportedOS(std::env::consts::OS.to_string()).into())
        }
    }
    
    async fn build_and_cache_environment(&self) -> Result<HashMap<String, String>> {
        info!("Building Nix environment...");
        
        let shell_command = self.environment.shell_command();
        let shell_args: Vec<&str> = shell_command.split_whitespace().collect();
        
        // Use nix-instantiate or nix print-dev-env to get environment without entering shell
        let env_output = if self.environment.env_type() == &crate::environment::EnvironmentType::Flake {
            self.get_flake_environment().await?
        } else {
            self.get_devenv_environment().await?
        };
        
        // Parse environment variables from output
        let environment_vars = self.parse_environment_output(&env_output)?;
        
        // Extract Nix store paths from PATH and other relevant variables
        let nix_store_paths = self.extract_nix_store_paths(&environment_vars);
        
        // Cache the environment
        self.cache.store_cache(&self.environment, nix_store_paths, environment_vars.clone())?;
        
        info!("Environment built and cached successfully");
        Ok(environment_vars)
    }
    
    async fn get_flake_environment(&self) -> Result<String> {
        let output = Command::new(self.environment.resolved_binary())
            .args(["print-dev-env", "--impure"])
            .current_dir(self.environment.project_dir())
            .output()?;
        
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(SandboxError::SandboxSetupError(format!(
                "Failed to get flake environment: {}", stderr
            )).into());
        }
        
        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    }
    
    async fn get_devenv_environment(&self) -> Result<String> {
        let output = Command::new(self.environment.resolved_binary())
            .args(["--run", "env"])
            .current_dir(self.environment.project_dir())
            .output()?;
        
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(SandboxError::SandboxSetupError(format!(
                "Failed to get devenv environment: {}", stderr
            )).into());
        }
        
        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    }
    
    fn parse_environment_output(&self, output: &str) -> Result<HashMap<String, String>> {
        let mut env_vars = HashMap::new();
        
        // Parse bash-style export statements or simple KEY=VALUE pairs
        for line in output.lines() {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            
            // Handle "export KEY=VALUE" format
            let line = if line.starts_with("export ") {
                &line[7..] // Skip "export "
            } else {
                line
            };
            
            // Handle "declare -x KEY=VALUE" format
            let line = if line.starts_with("declare -x ") {
                &line[11..] // Skip "declare -x "
            } else {
                line
            };
            
            // Parse KEY=VALUE
            if let Some(eq_pos) = line.find('=') {
                let key = line[..eq_pos].trim();
                let value = line[eq_pos + 1..].trim();
                
                // Remove quotes if present
                let value = if (value.starts_with('"') && value.ends_with('"')) ||
                              (value.starts_with('\'') && value.ends_with('\'')) {
                    &value[1..value.len()-1]
                } else {
                    value
                };
                
                env_vars.insert(key.to_string(), value.to_string());
            }
        }
        
        Ok(env_vars)
    }
    
    fn extract_nix_store_paths(&self, env_vars: &HashMap<String, String>) -> Vec<String> {
        let mut store_paths = std::collections::HashSet::new();
        
        // Check PATH for nix store paths
        if let Some(path) = env_vars.get("PATH") {
            for path_entry in path.split(':') {
                if path_entry.starts_with("/nix/store/") {
                    // Extract the store path (everything up to the first / after /nix/store/hash-)
                    if let Some(end_pos) = path_entry[11..].find('/') {
                        let store_path = &path_entry[..11 + end_pos];
                        store_paths.insert(store_path.to_string());
                    }
                }
            }
        }
        
        // Check other common variables that might contain nix store paths
        for var in ["LD_LIBRARY_PATH", "PKG_CONFIG_PATH", "CMAKE_PREFIX_PATH"] {
            if let Some(value) = env_vars.get(var) {
                for path_entry in value.split(':') {
                    if path_entry.starts_with("/nix/store/") {
                        if let Some(end_pos) = path_entry[11..].find('/') {
                            let store_path = &path_entry[..11 + end_pos];
                            store_paths.insert(store_path.to_string());
                        }
                    }
                }
            }
        }
        
        store_paths.into_iter().collect()
    }
}