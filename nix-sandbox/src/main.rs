use anyhow::Result;
use clap::Parser;
use tracing_subscriber::EnvFilter;

mod cache;
mod cli;
mod config;
mod constants;
mod environment;
mod error;
mod sandbox;
mod session;

use crate::cli::Commands;

#[derive(Parser)]
#[command(name = "nix-sandbox")]
#[command(about = "Secure, reproducible development environments using Nix", long_about = None)]
#[command(version)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env())
        .init();

    let cli = Cli::parse();

    match cli.command {
        Commands::Enter { session } => {
            cli::handle_enter(session).await?;
        }
        Commands::List => {
            cli::handle_list().await?;
        }
        Commands::Clean => {
            cli::handle_clean().await?;
        }
    }

    Ok(())
}
