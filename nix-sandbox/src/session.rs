use anyhow::Result;
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::config::Config;
use crate::constants::git;
use crate::error::SandboxError;

/// Information about a git repository
#[derive(Debug, Clone)]
struct GitRepositoryInfo {
    project_name: String,
}

#[derive(Debug, Clone)]
pub struct Session {
    name: String,
    project_dir: PathBuf,
    git_branch: Option<String>,
}

impl Session {
    pub fn new_in_place(project_dir: &Path) -> Result<Self> {
        let name = project_dir
            .file_name()
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
                    String::from_utf8(output.stdout)
                        .ok()
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
        match self.find_git_repository(current_dir) {
            Some(git_info) => self.create_git_session(name, current_dir, &git_info),
            None => Session::new_in_place(current_dir),
        }
    }

    /// Find git repository information for the given directory
    fn find_git_repository(&self, current_dir: &Path) -> Option<GitRepositoryInfo> {
        let output = Command::new("git")
            .args(git::REV_PARSE_TOPLEVEL)
            .current_dir(current_dir)
            .output()
            .ok()?;

        if !output.status.success() {
            return None;
        }

        let git_root = String::from_utf8(output.stdout).ok()?;
        let git_root = git_root.trim();
        let project_name = Path::new(git_root)
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("project");

        Some(GitRepositoryInfo {
            project_name: project_name.to_string(),
        })
    }

    /// Create a git-based session with worktree
    fn create_git_session(
        &self,
        name: &str,
        current_dir: &Path,
        git_info: &GitRepositoryInfo,
    ) -> Result<Session> {
        let session_dir = self
            .config
            .sessions_dir
            .join(format!("{}-{}", git_info.project_name, name));

        if !session_dir.exists() {
            self.create_worktree(current_dir, &session_dir, name)?;
        }

        Ok(Session {
            name: format!("{}-{}", git_info.project_name, name),
            project_dir: session_dir,
            git_branch: Some(name.to_string()),
        })
    }

    /// Create a git worktree for the session
    fn create_worktree(
        &self,
        current_dir: &Path,
        session_dir: &Path,
        branch_name: &str,
    ) -> Result<()> {
        // Try to create worktree with new branch
        let status = Command::new("git")
            .args([
                git::WORKTREE_ADD,
                git::ADD,
                &session_dir.to_string_lossy(),
                git::NEW_BRANCH_FLAG,
                branch_name,
            ])
            .current_dir(current_dir)
            .status()?;

        if status.success() {
            return Ok(());
        }

        // If that failed, try without -b flag (branch might already exist)
        let status = Command::new("git")
            .args([
                git::WORKTREE_ADD,
                git::ADD,
                &session_dir.to_string_lossy(),
                branch_name,
            ])
            .current_dir(current_dir)
            .status()?;

        if status.success() {
            Ok(())
        } else {
            Err(SandboxError::GitError(format!(
                "Failed to create worktree for branch: {}",
                branch_name
            ))
            .into())
        }
    }

    pub fn list_sessions(&self) -> Result<Vec<Session>> {
        let mut sessions = Vec::new();

        if self.config.sessions_dir.exists() {
            for entry in std::fs::read_dir(&self.config.sessions_dir)? {
                let entry = entry?;
                let path = entry.path();

                if path.is_dir() {
                    let name = path
                        .file_name()
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

    #[test]
    fn test_session_manager_new() {
        let temp_dir = TempDir::new().unwrap();
        let config = Config {
            sessions_dir: temp_dir.path().join("sessions"),
            cache_dir: temp_dir.path().join("cache"),
        };

        let manager = SessionManager::new(&config).unwrap();
        // Just verify it doesn't panic and returns something
        assert_eq!(manager.config.sessions_dir, config.sessions_dir);
    }

    #[test]
    fn test_find_git_repository_none_for_non_git_dir() {
        let temp_dir = TempDir::new().unwrap();
        let config = Config {
            sessions_dir: temp_dir.path().join("sessions"),
            cache_dir: temp_dir.path().join("cache"),
        };

        let manager = SessionManager::new(&config).unwrap();
        let git_info = manager.find_git_repository(temp_dir.path());

        assert!(git_info.is_none());
    }

    #[test]
    fn test_session_name_and_git_branch() {
        let temp_dir = TempDir::new().unwrap();
        let session = Session {
            name: "test-session".to_string(),
            project_dir: temp_dir.path().to_path_buf(),
            git_branch: Some("feature-branch".to_string()),
        };

        assert_eq!(session.name(), "test-session");
        assert_eq!(session.git_branch(), Some("feature-branch"));
        assert_eq!(session.project_dir(), temp_dir.path());
    }

    #[test]
    fn test_session_without_git_branch() {
        let temp_dir = TempDir::new().unwrap();
        let session = Session {
            name: "no-git-session".to_string(),
            project_dir: temp_dir.path().to_path_buf(),
            git_branch: None,
        };

        assert_eq!(session.name(), "no-git-session");
        assert_eq!(session.git_branch(), None);
    }

    #[test]
    fn test_get_current_git_branch_none_for_non_git() {
        let temp_dir = TempDir::new().unwrap();
        let branch = Session::get_current_git_branch(temp_dir.path());

        assert!(branch.is_none());
    }

    #[test]
    fn test_create_or_get_session_non_git_directory() {
        let temp_dir = TempDir::new().unwrap();
        std::fs::create_dir_all(temp_dir.path().join("sessions")).unwrap();

        let config = Config {
            sessions_dir: temp_dir.path().join("sessions"),
            cache_dir: temp_dir.path().join("cache"),
        };

        let manager = SessionManager::new(&config).unwrap();
        let session = manager
            .create_or_get_session("test-session", temp_dir.path())
            .unwrap();

        // Should create in-place session for non-git directory
        assert_eq!(session.project_dir(), temp_dir.path());
        assert!(session.git_branch().is_none());
    }

    #[test]
    fn test_list_sessions_empty() {
        let temp_dir = TempDir::new().unwrap();
        let sessions_dir = temp_dir.path().join("sessions");
        std::fs::create_dir_all(&sessions_dir).unwrap();

        let config = Config {
            sessions_dir,
            cache_dir: temp_dir.path().join("cache"),
        };

        let manager = SessionManager::new(&config).unwrap();
        let sessions = manager.list_sessions().unwrap();

        assert!(sessions.is_empty());
    }

    #[test]
    fn test_list_sessions_with_directories() {
        let temp_dir = TempDir::new().unwrap();
        let sessions_dir = temp_dir.path().join("sessions");
        std::fs::create_dir_all(&sessions_dir).unwrap();

        // Create some session directories
        std::fs::create_dir_all(sessions_dir.join("session1")).unwrap();
        std::fs::create_dir_all(sessions_dir.join("session2")).unwrap();

        let config = Config {
            sessions_dir: sessions_dir.clone(),
            cache_dir: temp_dir.path().join("cache"),
        };

        let manager = SessionManager::new(&config).unwrap();
        let sessions = manager.list_sessions().unwrap();

        assert_eq!(sessions.len(), 2);
        let session_names: Vec<&str> = sessions.iter().map(|s| s.name()).collect();
        assert!(session_names.contains(&"session1"));
        assert!(session_names.contains(&"session2"));
    }
}
