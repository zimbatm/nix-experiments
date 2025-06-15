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
    /// Execute a command in the sandbox environment
    Exec {
        /// Session name or branch name (optional)
        session: Option<String>,
        /// Command to execute
        #[arg(required = true)]
        command: String,
        /// Arguments for the command
        #[arg(trailing_var_arg = true)]
        args: Vec<String>,
    },
    /// List active sessions
    List,
    /// Clean up cached environments
    Clean,
}

fn setup_sandbox(session_name: Option<String>) -> Result<(Config, Session, Environment, Sandbox)> {
    let config = Config::load()?;
    let current_dir = std::env::current_dir()?;

    // Initialize session
    let session_mgr = SessionManager::new(&config)?;
    let session = if let Some(name) = session_name {
        session_mgr.create_or_get_session(&name, &current_dir)?
    } else {
        Session::new_in_place(&current_dir)?
    };

    // Detect environment
    let env = Environment::detect(session.project_dir())?;
    info!("Detected environment type: {:?}", env.env_type());

    // Create sandbox
    let sandbox = Sandbox::new(&config, &session, &env)?;

    Ok((config, session, env, sandbox))
}

pub async fn handle_enter(session_name: Option<String>) -> Result<()> {
    let (_config, session, _env, sandbox) = setup_sandbox(session_name)?;

    info!("Entering sandbox for: {}", session.project_dir().display());
    sandbox.enter().await?;

    Ok(())
}

pub async fn handle_exec(
    session_name: Option<String>,
    command: String,
    args: Vec<String>,
) -> Result<()> {
    let (_config, _session, _env, sandbox) = setup_sandbox(session_name)?;

    info!("Executing in sandbox: {} {:?}", command, args);
    sandbox.exec(command, args).await?;

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
