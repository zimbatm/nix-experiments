pub mod linux;
pub mod macos;

use anyhow::Result;
use std::collections::HashMap;
use std::process::Command;
use tracing::info;

use crate::cache::EnvironmentCache;
use crate::config::Config;
use crate::environment::Environment;
use crate::error::SandboxError;
use crate::session::Session;

pub struct Sandbox {
    _config: Config,
    session: Session,
    environment: Environment,
    cache: EnvironmentCache,
}

impl Sandbox {
    pub fn new(config: &Config, session: &Session, environment: &Environment) -> Result<Self> {
        let cache = EnvironmentCache::new(config.clone());

        Ok(Sandbox {
            _config: config.clone(),
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
            self.build_and_cache_environment().await?
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

    pub async fn exec(&self, command: String, args: Vec<String>) -> Result<()> {
        // Check if we have a valid cached environment
        let cached_env = self.cache.get_cached_environment(&self.environment)?;

        let environment_vars = if let Some(cached_metadata) = cached_env {
            info!("Using cached environment");
            cached_metadata.environment_vars
        } else {
            info!("Building new environment (no valid cache found)");
            self.build_and_cache_environment().await?
        };

        #[cfg(target_os = "linux")]
        {
            linux::exec_in_sandbox(&self.session, &self.environment, &environment_vars, command, args).await
        }

        #[cfg(target_os = "macos")]
        {
            macos::exec_in_sandbox(&self.session, &self.environment, &environment_vars, command, args).await
        }

        #[cfg(not(any(target_os = "linux", target_os = "macos")))]
        {
            Err(SandboxError::UnsupportedOS(std::env::consts::OS.to_string()).into())
        }
    }

    async fn build_and_cache_environment(&self) -> Result<HashMap<String, String>> {
        info!("Building Nix environment...");

        let shell_command = self.environment.shell_command();
        let _shell_args: Vec<&str> = shell_command.split_whitespace().collect();

        // Use nix-instantiate or nix print-dev-env to get environment without entering shell
        let env_output =
            if self.environment.env_type() == &crate::environment::EnvironmentType::Flake {
                self.get_flake_environment().await?
            } else {
                self.get_devenv_environment().await?
            };

        // Parse environment variables from output
        let environment_vars = self.parse_environment_output(&env_output)?;

        // Extract Nix store paths from PATH and other relevant variables
        let nix_store_paths = self.extract_nix_store_paths(&environment_vars);

        // Cache the environment
        self.cache
            .store_cache(&self.environment, nix_store_paths, environment_vars.clone())?;

        info!("Environment built and cached successfully");
        Ok(environment_vars)
    }

    async fn get_flake_environment(&self) -> Result<String> {
        // Use nix print-dev-env with JSON output for clean parsing
        let output = Command::new(self.environment.resolved_binary())
            .args(["print-dev-env", "--json", "--impure"])
            .current_dir(self.environment.project_dir())
            .output()?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(SandboxError::SandboxSetupError(format!(
                "Failed to get flake environment: {}",
                stderr
            ))
            .into());
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
                "Failed to get devenv environment: {}",
                stderr
            ))
            .into());
        }

        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    }

    fn parse_environment_output(&self, output: &str) -> Result<HashMap<String, String>> {
        let mut env_vars = HashMap::new();

        // Check if this is JSON output from nix print-dev-env
        if output.trim_start().starts_with('{') {
            // Parse JSON format
            let json: serde_json::Value = serde_json::from_str(output)?;
            
            if let Some(variables) = json.get("variables").and_then(|v| v.as_object()) {
                for (key, var_obj) in variables {
                    if let Some(value) = var_obj.get("value").and_then(|v| v.as_str()) {
                        env_vars.insert(key.clone(), value.to_string());
                    }
                }
            }
        } else {
            // Parse simple KEY=VALUE pairs from env command output
            for line in output.lines() {
                if line.is_empty() {
                    continue;
                }

                // Parse KEY=VALUE
                if let Some(eq_pos) = line.find('=') {
                    let key = &line[..eq_pos];
                    let value = &line[eq_pos + 1..];
                    env_vars.insert(key.to_string(), value.to_string());
                }
            }
        }

        Ok(env_vars)
    }

    fn extract_nix_store_paths(&self, env_vars: &HashMap<String, String>) -> Vec<String> {
        let mut store_paths = std::collections::HashSet::new();

        // Check PATH for nix store paths
        if let Some(path) = env_vars.get("PATH") {
            for path_entry in path.split(':') {
                if let Some(stripped) = path_entry.strip_prefix("/nix/store/") {
                    // Extract the store path (everything up to the first / after /nix/store/hash-)
                    if let Some(end_pos) = stripped.find('/') {
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
                    if let Some(stripped) = path_entry.strip_prefix("/nix/store/") {
                        if let Some(end_pos) = stripped.find('/') {
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
