use anyhow::Result;
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::config::Config;
use crate::constants::git;
use crate::error::SandboxError;

#[derive(Debug, Clone)]
pub struct Session {
    name: String,
    project_dir: PathBuf,
    git_branch: Option<String>,
}

impl Session {
    pub fn new_in_place(project_dir: &Path) -> Result<Self> {
        let name = project_dir.file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("default")
            .to_string();
            
        let git_branch = Self::get_current_git_branch(project_dir);
        
        Ok(Session {
            name,
            project_dir: project_dir.to_path_buf(),
            git_branch,
        })
    }
    
    pub fn name(&self) -> &str {
        &self.name
    }
    
    pub fn project_dir(&self) -> &Path {
        &self.project_dir
    }
    
    pub fn git_branch(&self) -> Option<&str> {
        self.git_branch.as_deref()
    }
    
    fn get_current_git_branch(dir: &Path) -> Option<String> {
        Command::new("git")
            .args(git::BRANCH_SHOW_CURRENT)
            .current_dir(dir)
            .output()
            .ok()
            .and_then(|output| {
                if output.status.success() {
                    String::from_utf8(output.stdout).ok()
                        .map(|s| s.trim().to_string())
                        .filter(|s| !s.is_empty())
                } else {
                    None
                }
            })
    }
}

pub struct SessionManager {
    config: Config,
}

impl SessionManager {
    pub fn new(config: &Config) -> Result<Self> {
        Ok(SessionManager {
            config: config.clone(),
        })
    }
    
    pub fn create_or_get_session(&self, name: &str, current_dir: &Path) -> Result<Session> {
        // Check if we're in a git repository
        let git_root = Command::new("git")
            .args(git::REV_PARSE_TOPLEVEL)
            .current_dir(current_dir)
            .output();
            
        if let Ok(output) = git_root {
            if output.status.success() {
                let git_root = String::from_utf8(output.stdout)?;
                let git_root = git_root.trim();
                let project_name = Path::new(git_root).file_name()
                    .and_then(|n| n.to_str())
                    .unwrap_or("project");
                
                let session_dir = self.config.sessions_dir.join(format!("{}-{}", project_name, name));
                
                if !session_dir.exists() {
                    // Create new worktree
                    let status = Command::new("git")
                        .args([git::WORKTREE_ADD, git::ADD, &session_dir.to_string_lossy(), git::NEW_BRANCH_FLAG, name])
                        .current_dir(current_dir)
                        .status()?;
                        
                    if !status.success() {
                        // Try without -b flag if branch already exists
                        let status = Command::new("git")
                            .args([git::WORKTREE_ADD, git::ADD, &session_dir.to_string_lossy(), name])
                            .current_dir(current_dir)
                            .status()?;
                            
                        if !status.success() {
                            return Err(SandboxError::GitError(format!("Failed to create worktree for branch: {}", name)).into());
                        }
                    }
                }
                
                Ok(Session {
                    name: format!("{}-{}", project_name, name),
                    project_dir: session_dir,
                    git_branch: Some(name.to_string()),
                })
            } else {
                // Not in a git repository
                Session::new_in_place(current_dir)
            }
        } else {
            // Git command failed
            Session::new_in_place(current_dir)
        }
    }
    
    pub fn list_sessions(&self) -> Result<Vec<Session>> {
        let mut sessions = Vec::new();
        
        if self.config.sessions_dir.exists() {
            for entry in std::fs::read_dir(&self.config.sessions_dir)? {
                let entry = entry?;
                let path = entry.path();
                
                if path.is_dir() {
                    let name = path.file_name()
                        .and_then(|n| n.to_str())
                        .unwrap_or("unknown")
                        .to_string();
                    
                    let git_branch = Session::get_current_git_branch(&path);
                    
                    sessions.push(Session {
                        name,
                        project_dir: path,
                        git_branch,
                    });
                }
            }
        }
        
        Ok(sessions)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;
    
    #[test]
    fn test_session_new_in_place() {
        let temp_dir = TempDir::new().unwrap();
        let session = Session::new_in_place(temp_dir.path()).unwrap();
        
        assert_eq!(session.project_dir(), temp_dir.path());
        assert!(session.git_branch().is_none());
    }
}