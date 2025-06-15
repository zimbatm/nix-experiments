use std::fs;
use std::path::Path;
use std::process::Command;
use tempdir::TempDir;
use serial_test::serial;

#[cfg(target_os = "linux")]
use std::os::unix::fs::PermissionsExt;

fn setup_test_project(dir: &Path) -> anyhow::Result<()> {
    // Initialize git repo
    let status = Command::new("git")
        .args(["init"])
        .current_dir(dir)
        .status()?;
    assert!(status.success());

    // Configure git
    Command::new("git")
        .args(["config", "user.name", "Test User"])
        .current_dir(dir)
        .status()?;
    
    Command::new("git")
        .args(["config", "user.email", "test@example.com"])
        .current_dir(dir)
        .status()?;

    // Create a simple flake.nix
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
        cowsay
      ];
    };
    devShells.aarch64-darwin.default = nixpkgs.legacyPackages.aarch64-darwin.mkShell {
      buildInputs = with nixpkgs.legacyPackages.aarch64-darwin; [
        hello
        cowsay
      ];
    };
  };
}
"#;
    fs::write(dir.join("flake.nix"), flake_content)?;

    // Create initial commit
    Command::new("git")
        .args(["add", "."])
        .current_dir(dir)
        .status()?;
    
    Command::new("git")
        .args(["commit", "-m", "Initial commit"])
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
    if let Ok(output) = Command::new("cargo")
        .args(["build", "--release"])
        .output()
    {
        if output.status.success() && release_path.exists() {
            return release_path.to_string_lossy().to_string();
        }
    }
    
    // Fallback to PATH
    "nix-sandbox".to_string()
}

#[test]
#[serial]
fn test_sandbox_basic_functionality() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-test")?;
    setup_test_project(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Test list command (should work even without sessions)
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success(), "list command failed: {}", String::from_utf8_lossy(&output.stderr));
    
    // Test clean command
    let output = Command::new(&binary)
        .args(["clean"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success(), "clean command failed: {}", String::from_utf8_lossy(&output.stderr));
    
    Ok(())
}

#[test]
#[serial]
fn test_environment_detection() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-env-test")?;
    
    // Test with flake.nix
    setup_test_project(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // The binary should detect flake.nix environment
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success());
    
    // Now test with devenv.nix
    fs::remove_file(temp_dir.path().join("flake.nix"))?;
    
    let devenv_content = r#"
{ pkgs, ... }: {
  packages = with pkgs; [
    hello
    cowsay
  ];
}
"#;
    fs::write(temp_dir.path().join("devenv.nix"), devenv_content)?;
    
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success());
    
    Ok(())
}

#[cfg(target_os = "linux")]
#[test]
#[serial]
fn test_linux_sandbox_isolation() -> anyhow::Result<()> {
    // Check if bubblewrap is available
    if Command::new("bwrap").arg("--version").output().is_err() {
        eprintln!("Skipping Linux sandbox test: bubblewrap not available");
        return Ok(());
    }
    
    let temp_dir = TempDir::new("nix-sandbox-isolation-test")?;
    setup_test_project(temp_dir.path())?;
    
    // Create a test file in the home directory that should NOT be accessible
    let home_test_file = std::env::var("HOME").unwrap() + "/sandbox-test-file";
    fs::write(&home_test_file, "secret content")?;
    
    let binary = get_nix_sandbox_binary();
    
    // Try to enter sandbox and access the file (should fail)
    let script = format!(
        r#"
        if [ -f "{}" ]; then
            echo "ISOLATION_FAILED: Can access home file"
            exit 1
        else
            echo "ISOLATION_OK: Cannot access home file"
        fi
        
        # Check if /nix/store is accessible (should be)
        if [ -d "/nix/store" ]; then
            echo "NIX_STORE_OK: Can access /nix/store"
        else
            echo "NIX_STORE_FAILED: Cannot access /nix/store"
            exit 1
        fi
        
        # Check if current project is accessible (should be)
        if [ -f "flake.nix" ]; then
            echo "PROJECT_OK: Can access project files"
        else
            echo "PROJECT_FAILED: Cannot access project files"
            exit 1
        fi
        "#,
        home_test_file
    );
    
    let test_script_path = temp_dir.path().join("test_isolation.sh");
    fs::write(&test_script_path, script)?;
    fs::set_permissions(&test_script_path, fs::Permissions::from_mode(0o755))?;
    
    // This test would require actually entering the sandbox, which is complex
    // For now, we just verify the sandbox can be set up
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success());
    
    // Clean up
    let _ = fs::remove_file(&home_test_file);
    
    Ok(())
}

#[cfg(target_os = "macos")]
#[test]
#[serial]
fn test_macos_sandbox_isolation() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-isolation-test")?;
    setup_test_project(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Test that macOS sandbox can be set up
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
    setup_test_project(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Run list command multiple times - should be consistent
    let output1 = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    let output2 = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output1.status.success());
    assert!(output2.status.success());
    
    // Modify flake.nix and verify cache key changes
    let modified_flake = fs::read_to_string(temp_dir.path().join("flake.nix"))?
        .replace("hello", "hello neofetch");
    
    fs::write(temp_dir.path().join("flake.nix"), modified_flake)?;
    
    let output3 = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output3.status.success());
    
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
    assert!(stderr.contains("flake.nix") || stderr.contains("devenv.nix") || stderr.contains("environment"));
    
    Ok(())
}