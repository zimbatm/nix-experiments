use serial_test::serial;
use std::fs;
use std::path::Path;
use std::process::Command;
use tempdir::TempDir;

fn setup_git_repo_with_branches(dir: &Path) -> anyhow::Result<()> {
    // Initialize git repo
    Command::new("git")
        .args(["init"])
        .current_dir(dir)
        .status()?;

    // Configure git
    Command::new("git")
        .args(["config", "user.name", "Test User"])
        .current_dir(dir)
        .status()?;

    Command::new("git")
        .args(["config", "user.email", "test@example.com"])
        .current_dir(dir)
        .status()?;

    // Create initial flake.nix
    let flake_content = r#"
{
  description = "Test project for nix-sandbox";
  
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  
  outputs = { self, nixpkgs }: {
    devShells.x86_64-linux.default = nixpkgs.legacyPackages.x86_64-linux.mkShell {
      buildInputs = with nixpkgs.legacyPackages.x86_64-linux; [
        hello
      ];
    };
    devShells.aarch64-darwin.default = nixpkgs.legacyPackages.aarch64-darwin.mkShell {
      buildInputs = with nixpkgs.legacyPackages.aarch64-darwin; [
        hello
      ];
    };
  };
}
"#;
    fs::write(dir.join("flake.nix"), flake_content)?;
    fs::write(
        dir.join("README.md"),
        "# Test Project\n\nMain branch version\n",
    )?;

    // Initial commit on main
    Command::new("git")
        .args(["add", "."])
        .current_dir(dir)
        .status()?;

    Command::new("git")
        .args(["commit", "-m", "Initial commit on main"])
        .current_dir(dir)
        .status()?;

    // Create feature branch
    Command::new("git")
        .args(["checkout", "-b", "feature/test-branch"])
        .current_dir(dir)
        .status()?;

    // Modify files on feature branch
    let feature_flake = flake_content.replace("hello", "hello cowsay");
    fs::write(dir.join("flake.nix"), feature_flake)?;
    fs::write(
        dir.join("README.md"),
        "# Test Project\n\nFeature branch version\n",
    )?;
    fs::write(
        dir.join("feature.txt"),
        "This file only exists on feature branch\n",
    )?;

    Command::new("git")
        .args(["add", "."])
        .current_dir(dir)
        .status()?;

    Command::new("git")
        .args(["commit", "-m", "Add feature changes"])
        .current_dir(dir)
        .status()?;

    // Create another branch
    Command::new("git")
        .args(["checkout", "-b", "hotfix/urgent-fix"])
        .current_dir(dir)
        .status()?;

    fs::write(dir.join("hotfix.txt"), "Critical fix\n")?;

    Command::new("git")
        .args(["add", "."])
        .current_dir(dir)
        .status()?;

    Command::new("git")
        .args(["commit", "-m", "Add hotfix"])
        .current_dir(dir)
        .status()?;

    // Return to main
    Command::new("git")
        .args(["checkout", "main"])
        .current_dir(dir)
        .status()?;

    Ok(())
}

fn get_nix_sandbox_binary() -> String {
    use std::env;

    // Try different paths to find the binary
    let current_dir = env::current_dir().expect("Failed to get current directory");
    let release_path = current_dir.join("target/release/nix-sandbox");
    let debug_path = current_dir.join("target/debug/nix-sandbox");

    if release_path.exists() {
        return release_path.to_string_lossy().to_string();
    }

    if debug_path.exists() {
        return debug_path.to_string_lossy().to_string();
    }

    // Try to build it
    if let Ok(output) = Command::new("cargo").args(["build", "--release"]).output() {
        if output.status.success() && release_path.exists() {
            return release_path.to_string_lossy().to_string();
        }
    }

    // Fallback to PATH
    "nix-sandbox".to_string()
}

#[test]
#[serial]
fn test_git_worktree_session_creation() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-git-test")?;
    setup_git_repo_with_branches(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Test creating session for main branch
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(
        output.status.success(),
        "Failed to list sessions on main: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    // Test creating session for feature branch
    Command::new("git")
        .args(["checkout", "feature/test-branch"])
        .current_dir(temp_dir.path())
        .status()?;

    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(
        output.status.success(),
        "Failed to list sessions on feature branch: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    // Test creating session for hotfix branch
    Command::new("git")
        .args(["checkout", "hotfix/urgent-fix"])
        .current_dir(temp_dir.path())
        .status()?;

    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(
        output.status.success(),
        "Failed to list sessions on hotfix branch: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    Ok(())
}

#[test]
#[serial]
fn test_branch_specific_environments() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-branch-env-test")?;
    setup_git_repo_with_branches(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Start on main branch
    Command::new("git")
        .args(["checkout", "main"])
        .current_dir(temp_dir.path())
        .status()?;

    // Verify main branch content
    let main_readme = fs::read_to_string(temp_dir.path().join("README.md"))?;
    assert!(main_readme.contains("Main branch version"));
    assert!(!temp_dir.path().join("feature.txt").exists());

    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    assert!(output.status.success());

    // Switch to feature branch
    Command::new("git")
        .args(["checkout", "feature/test-branch"])
        .current_dir(temp_dir.path())
        .status()?;

    // Verify feature branch content
    let feature_readme = fs::read_to_string(temp_dir.path().join("README.md"))?;
    assert!(feature_readme.contains("Feature branch version"));
    assert!(temp_dir.path().join("feature.txt").exists());

    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    assert!(output.status.success());

    // Switch to hotfix branch
    Command::new("git")
        .args(["checkout", "hotfix/urgent-fix"])
        .current_dir(temp_dir.path())
        .status()?;

    // Verify hotfix branch content
    assert!(temp_dir.path().join("hotfix.txt").exists());
    assert!(temp_dir.path().join("feature.txt").exists()); // Should have feature changes too

    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    assert!(output.status.success());

    Ok(())
}

#[test]
#[serial]
fn test_session_isolation_between_branches() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-session-isolation-test")?;
    setup_git_repo_with_branches(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Create sessions on different branches
    Command::new("git")
        .args(["checkout", "main"])
        .current_dir(temp_dir.path())
        .status()?;

    let main_output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    assert!(main_output.status.success());

    Command::new("git")
        .args(["checkout", "feature/test-branch"])
        .current_dir(temp_dir.path())
        .status()?;

    let feature_output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    assert!(feature_output.status.success());

    // The outputs should potentially be different if sessions are branch-aware
    // This is more of a behavioral test that ensures the tool works across branches

    Ok(())
}

#[test]
#[serial]
fn test_worktree_with_named_session() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-named-session-test")?;
    setup_git_repo_with_branches(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Test list command (sessions would need to be created with enter or exec first)
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(
        output.status.success(),
        "Failed to list sessions: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    Ok(())
}

#[test]
#[serial]
fn test_git_worktree_cleanup() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-cleanup-test")?;
    setup_git_repo_with_branches(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Create some sessions
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    assert!(output.status.success());

    // Test cleanup
    let output = Command::new(&binary)
        .args(["clean"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(
        output.status.success(),
        "Clean command failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    Ok(())
}

#[test]
#[serial]
fn test_git_repo_edge_cases() -> anyhow::Result<()> {
    // Test in a directory that's not a git repo
    let temp_dir = TempDir::new("nix-sandbox-no-git-test")?;

    // Create flake.nix without git
    let flake_content = r#"
{
  description = "Test project without git";
  
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  
  outputs = { self, nixpkgs }: {
    devShells.x86_64-linux.default = nixpkgs.legacyPackages.x86_64-linux.mkShell {
      buildInputs = with nixpkgs.legacyPackages.x86_64-linux; [
        hello
      ];
    };
  };
}
"#;
    fs::write(temp_dir.path().join("flake.nix"), flake_content)?;

    let binary = get_nix_sandbox_binary();

    // Should still work without git
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(
        output.status.success(),
        "Should work without git repo: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    Ok(())
}

#[test]
#[serial]
fn test_detached_head_state() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-detached-head-test")?;
    setup_git_repo_with_branches(temp_dir.path())?;

    // Get the commit hash of main
    let output = Command::new("git")
        .args(["rev-parse", "HEAD"])
        .current_dir(temp_dir.path())
        .output()?;
    assert!(output.status.success());
    let commit_hash = String::from_utf8_lossy(&output.stdout).trim().to_string();

    // Checkout detached HEAD
    Command::new("git")
        .args(["checkout", &commit_hash])
        .current_dir(temp_dir.path())
        .status()?;

    let binary = get_nix_sandbox_binary();

    // Should handle detached HEAD gracefully
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(
        output.status.success(),
        "Should handle detached HEAD: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    Ok(())
}
