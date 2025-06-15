use anyhow::Result;
use std::path::PathBuf;

use crate::constants::{app_dirs, env_vars};
use crate::error::SandboxError;

#[derive(Debug, Clone)]
pub struct Config {
    pub sessions_dir: PathBuf,
    pub cache_dir: PathBuf,
}

impl Config {
    pub fn load() -> Result<Self> {
        let state_dir = if let Ok(dir) = std::env::var(env_vars::XDG_STATE_HOME) {
            PathBuf::from(dir).join(app_dirs::APP_NAME)
        } else {
            dirs::home_dir()
                .ok_or_else(|| {
                    SandboxError::ConfigError("Could not determine home directory".into())
                })?
                .join(app_dirs::LOCAL_STATE_PATH)
                .join(app_dirs::APP_NAME)
        };

        let sessions_dir = state_dir.join(app_dirs::SESSIONS);
        let cache_dir = state_dir.join(app_dirs::CACHE);

        // Create directories if they don't exist
        std::fs::create_dir_all(&sessions_dir)?;
        std::fs::create_dir_all(&cache_dir)?;

        Ok(Config {
            sessions_dir,
            cache_dir,
        })
    }
}
