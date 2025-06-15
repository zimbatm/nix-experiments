pub mod linux;
pub mod macos;

use anyhow::Result;

use crate::config::Config;
use crate::environment::Environment;
use crate::error::SandboxError;
use crate::session::Session;

pub struct Sandbox {
    config: Config,
    session: Session,
    environment: Environment,
}

impl Sandbox {
    pub fn new(config: &Config, session: &Session, environment: &Environment) -> Result<Self> {
        Ok(Sandbox {
            config: config.clone(),
            session: session.clone(),
            environment: environment.clone(),
        })
    }
    
    pub async fn enter(&self) -> Result<()> {
        #[cfg(target_os = "linux")]
        {
            linux::enter_sandbox(&self.session, &self.environment).await
        }
        
        #[cfg(target_os = "macos")]
        {
            macos::enter_sandbox(&self.session, &self.environment).await
        }
        
        #[cfg(not(any(target_os = "linux", target_os = "macos")))]
        {
            Err(SandboxError::UnsupportedOS(std::env::consts::OS.to_string()).into())
        }
    }
}