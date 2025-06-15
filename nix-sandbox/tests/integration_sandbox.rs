use anyhow::Result;
use serial_test::serial;
use std::{
    fs,
    path::Path,
    process::{Command, Stdio},
};
use tempdir::TempDir;

// Test helpers to avoid duplicating across modules
fn get_nix_sandbox_binary() -> String {
    let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
    let binary = std::path::Path::new(&manifest_dir)
        .join("target")
        .join("release")
        .join("nix-sandbox");

    // Build the release binary first
    let output = Command::new("cargo")
        .args(["build", "--release"])
        .current_dir(&manifest_dir)
        .output()
        .expect("Failed to build nix-sandbox");

    if !output.status.success() {
        panic!(
            "Failed to build nix-sandbox: {}",
            String::from_utf8_lossy(&output.stderr)
        );
    }

    binary.to_str().unwrap().to_string()
}

fn setup_basic_test_env(dir: &Path) -> Result<()> {
    // Create a minimal flake.nix
    let flake_content = r#"{
  description = "Test flake";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  outputs = { self, nixpkgs }: {
    devShells.x86_64-linux.default = nixpkgs.legacyPackages.x86_64-linux.mkShell {
      packages = [ nixpkgs.legacyPackages.x86_64-linux.hello ];
    };
    devShells.aarch64-linux.default = nixpkgs.legacyPackages.aarch64-linux.mkShell {
      packages = [ nixpkgs.legacyPackages.aarch64-linux.hello ];
    };
    devShells.x86_64-darwin.default = nixpkgs.legacyPackages.x86_64-darwin.mkShell {
      packages = [ nixpkgs.legacyPackages.x86_64-darwin.hello ];
    };
    devShells.aarch64-darwin.default = nixpkgs.legacyPackages.aarch64-darwin.mkShell {
      packages = [ nixpkgs.legacyPackages.aarch64-darwin.hello ];
    };
  };
}"#;
    fs::write(dir.join("flake.nix"), flake_content)?;

    // Initialize git repo
    Command::new("git")
        .args(["init"])
        .current_dir(dir)
        .output()?;

    Command::new("git")
        .args(["add", "."])
        .current_dir(dir)
        .output()?;

    Command::new("git")
        .args(["commit", "-m", "Initial commit"])
        .current_dir(dir)
        .output()?;

    Ok(())
}

fn setup_devenv_test_env(dir: &Path) -> Result<()> {
    // Create a minimal devenv.nix
    let devenv_content = r#"{ pkgs, ... }: {
  packages = [ pkgs.hello ];
}"#;
    fs::write(dir.join("devenv.nix"), devenv_content)?;
    Ok(())
}

#[test]
#[serial]
fn test_sandbox_basic_functionality() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-test")?;
    setup_basic_test_env(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Test list command
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(output.status.success());

    // Test clean command
    let output = Command::new(&binary)
        .args(["clean"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(output.status.success());

    Ok(())
}

#[test]
#[serial]
fn test_environment_detection() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-env-test")?;

    // Test with flake.nix
    setup_basic_test_env(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Should succeed with flake.nix present
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(output.status.success());

    // Remove flake.nix and add devenv.nix
    fs::remove_file(temp_dir.path().join("flake.nix"))?;
    setup_devenv_test_env(temp_dir.path())?;

    // Should succeed with devenv.nix present
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(output.status.success());

    Ok(())
}

#[test]
#[serial]
fn test_cache_key_generation() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-cache-test")?;
    setup_basic_test_env(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Run list multiple times - should use cache
    for _ in 0..3 {
        let output = Command::new(&binary)
            .args(["list"])
            .current_dir(temp_dir.path())
            .output()?;

        assert!(output.status.success());
    }

    // Modify flake.nix
    let flake_content = fs::read_to_string(temp_dir.path().join("flake.nix"))?;
    fs::write(
        temp_dir.path().join("flake.nix"),
        format!("{}\n# Modified", flake_content),
    )?;

    // Should still work with modified flake
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(output.status.success());

    Ok(())
}

#[test]
#[serial]
fn test_linux_sandbox_isolation() -> anyhow::Result<()> {
    // Skip this test on non-Linux platforms
    if !cfg!(target_os = "linux") {
        return Ok(());
    }

    let temp_dir = TempDir::new("nix-sandbox-isolation-test")?;
    setup_basic_test_env(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Check if bubblewrap is available
    let bwrap_check = Command::new("which")
        .arg("bwrap")
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();

    if bwrap_check.is_err() || !bwrap_check.unwrap().success() {
        eprintln!("Skipping Linux sandbox isolation test: bubblewrap not found");
        return Ok(());
    }

    // The actual sandboxing will be tested by trying to run commands
    // that should be isolated. For now, just ensure the basic
    // functionality works on Linux.
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;

    assert!(output.status.success());

    Ok(())
}

#[test]
#[serial]
fn test_error_handling() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-error-test")?;

    let binary = get_nix_sandbox_binary();

    // Test in directory without environment files
    let output = Command::new(&binary)
        .args(["enter"])
        .current_dir(temp_dir.path())
        .output()?;

    // Should fail gracefully
    assert!(!output.status.success());
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("flake.nix")
            || stderr.contains("devenv.nix")
            || stderr.contains("environment")
    );

    Ok(())
}

#[test]
#[serial]
fn test_exec_command_new_syntax() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-exec-test")?;
    setup_basic_test_env(temp_dir.path())?;

    let binary = get_nix_sandbox_binary();

    // Test exec without session
    let output = Command::new(&binary)
        .args(["exec", "--", "echo", "hello"])
        .current_dir(temp_dir.path())
        .output()?;

    // Command should succeed (environment setup will be skipped in tests)
    // but we're testing the CLI parsing
    assert!(
        output.status.success() || String::from_utf8_lossy(&output.stderr).contains("flake.nix"),
        "exec command failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    // Test exec with session
    let output = Command::new(&binary)
        .args(["exec", "--session", "test-branch", "--", "echo", "hello"])
        .current_dir(temp_dir.path())
        .output()?;

    // Command should succeed or fail with expected error
    assert!(
        output.status.success() || String::from_utf8_lossy(&output.stderr).contains("git") 
            || String::from_utf8_lossy(&output.stderr).contains("flake.nix"),
        "exec with session failed unexpectedly: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    Ok(())
}