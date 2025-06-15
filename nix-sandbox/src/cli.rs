use anyhow::Result;
use clap::Subcommand;
use tracing::info;

use crate::cache::EnvironmentCache;
use crate::config::Config;
use crate::environment::Environment;
use crate::sandbox::Sandbox;
use crate::session::{Session, SessionManager};

#[derive(Subcommand)]
pub enum Commands {
    /// Enter a sandbox environment
    Enter {
        /// Session name or branch name (optional)
        session: Option<String>,
    },
    /// List active sessions
    List,
    /// Clean up cached environments
    Clean,
}

pub async fn handle_enter(session_name: Option<String>) -> Result<()> {
    let config = Config::load()?;
    let current_dir = std::env::current_dir()?;

    // Initialize session
    let session_mgr = SessionManager::new(&config)?;
    let session = if let Some(name) = session_name {
        session_mgr.create_or_get_session(&name, &current_dir)?
    } else {
        Session::new_in_place(&current_dir)?
    };

    info!("Entering sandbox for: {}", session.project_dir().display());

    // Detect environment
    let env = Environment::detect(session.project_dir())?;
    info!("Detected environment type: {:?}", env.env_type());

    // Create and enter sandbox
    let sandbox = Sandbox::new(&config, &session, &env)?;
    sandbox.enter().await?;

    Ok(())
}

pub async fn handle_list() -> Result<()> {
    let config = Config::load()?;
    let session_mgr = SessionManager::new(&config)?;

    let sessions = session_mgr.list_sessions()?;

    if sessions.is_empty() {
        info!("No active sessions");
    } else {
        info!("Active sessions:");
        for session in sessions {
            println!(
                "  - {} (branch: {})",
                session.name(),
                session.git_branch().unwrap_or("none")
            );
        }
    }

    Ok(())
}

pub async fn handle_clean() -> Result<()> {
    let config = Config::load()?;
    let cache = EnvironmentCache::new(config);

    info!("Cleaning stale caches (older than 7 days)...");
    cache.cleanup_stale_caches(7)?;
    info!("Cache cleanup completed");

    Ok(())
}
