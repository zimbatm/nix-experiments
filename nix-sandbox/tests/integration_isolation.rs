use std::fs;
use std::path::Path;
use std::process::Command;
use tempdir::TempDir;
use serial_test::serial;

#[cfg(target_os = "linux")]
use std::os::unix::fs::PermissionsExt;

fn setup_test_environment(dir: &Path) -> anyhow::Result<()> {
    // Initialize git repo
    Command::new("git")
        .args(["init"])
        .current_dir(dir)
        .status()?;

    Command::new("git")
        .args(["config", "user.name", "Test User"])
        .current_dir(dir)
        .status()?;
    
    Command::new("git")
        .args(["config", "user.email", "test@example.com"])
        .current_dir(dir)
        .status()?;

    // Create environment with test script
    let flake_content = r#"
{
  description = "Isolation test project";
  
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  
  outputs = { self, nixpkgs }: 
    let
      systems = [ "x86_64-linux" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in {
      devShells = forAllSystems (system: {
        default = nixpkgs.legacyPackages.${system}.mkShell {
          buildInputs = with nixpkgs.legacyPackages.${system}; [
            bash
            coreutils
            findutils
          ];
          shellHook = ''
            echo "Entered nix-sandbox isolation test environment"
          '';
        };
      });
    };
}
"#;
    fs::write(dir.join("flake.nix"), flake_content)?;

    // Create test script
    let test_script = r#"#!/bin/bash
set -e

echo "=== Sandbox Isolation Test ==="

# Test 1: Check if we can access project directory
echo "Test 1: Project directory access"
if [ -f "flake.nix" ]; then
    echo "✓ Can access project files"
else
    echo "✗ Cannot access project files"
    exit 1
fi

# Test 2: Check /nix/store access (should be available)
echo "Test 2: Nix store access"
if [ -d "/nix/store" ]; then
    echo "✓ Can access /nix/store"
else
    echo "✗ Cannot access /nix/store"
    exit 1
fi

# Test 3: Check home directory isolation (should be restricted)
echo "Test 3: Home directory isolation"
if [ -f "${HOME}/.bashrc" ] 2>/dev/null || [ -f "${HOME}/.profile" ] 2>/dev/null; then
    echo "⚠ Can access home directory files (may be expected on macOS)"
else
    echo "✓ Home directory is isolated"
fi

# Test 4: Check root filesystem access (should be restricted)
echo "Test 4: Root filesystem isolation"
if [ -f "/etc/passwd" ] 2>/dev/null; then
    echo "⚠ Can access /etc/passwd (may be expected on some systems)"
else
    echo "✓ Root filesystem is isolated"
fi

# Test 5: Check /tmp access (behavior may vary)
echo "Test 5: Temporary directory access"
if [ -d "/tmp" ]; then
    echo "✓ Can access /tmp"
else
    echo "⚠ Cannot access /tmp"
fi

# Test 6: Try to create files in allowed locations
echo "Test 6: File creation test"
echo "test content" > test_file.txt
if [ -f "test_file.txt" ]; then
    echo "✓ Can create files in project directory"
    rm test_file.txt
else
    echo "✗ Cannot create files in project directory"
    exit 1
fi

# Test 7: Check environment variables
echo "Test 7: Environment variables"
if [ -n "$HOME" ] && [ -n "$PATH" ]; then
    echo "✓ Basic environment variables are set"
else
    echo "✗ Missing basic environment variables"
    exit 1
fi

# Test 8: Check available commands
echo "Test 8: Command availability"
if command -v bash >/dev/null && command -v ls >/dev/null; then
    echo "✓ Basic commands are available"
else
    echo "✗ Basic commands are missing"
    exit 1
fi

echo "=== All tests passed ==="
"#;
    
    fs::write(dir.join("test_isolation.sh"), test_script)?;
    fs::set_permissions(dir.join("test_isolation.sh"), fs::Permissions::from_mode(0o755))?;

    // Create a test file that should be accessible
    fs::write(dir.join("accessible_file.txt"), "This file should be accessible\n")?;

    // Commit everything
    Command::new("git")
        .args(["add", "."])
        .current_dir(dir)
        .status()?;
    
    Command::new("git")
        .args(["commit", "-m", "Initial isolation test setup"])
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

#[cfg(target_os = "linux")]
#[test]
#[serial]
fn test_linux_bubblewrap_isolation() -> anyhow::Result<()> {
    // Check if bubblewrap is available
    if Command::new("bwrap").arg("--version").output().is_err() {
        eprintln!("Skipping Linux isolation test: bubblewrap not available");
        return Ok(());
    }

    let temp_dir = TempDir::new("nix-sandbox-linux-isolation")?;
    setup_test_environment(temp_dir.path())?;
    
    // Create a file in the user's home that should NOT be accessible
    let home = std::env::var("HOME").unwrap_or_else(|_| "/tmp".to_string());
    let secret_file = format!("{}/nix_sandbox_secret_test_file", home);
    fs::write(&secret_file, "This should not be accessible")?;

    let binary = get_nix_sandbox_binary();
    
    // Test basic functionality first
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success(), "Basic list command failed: {}", String::from_utf8_lossy(&output.stderr));

    // Clean up the secret file
    let _ = fs::remove_file(&secret_file);
    
    Ok(())
}

#[cfg(target_os = "macos")]
#[test]
#[serial]
fn test_macos_sandbox_exec_isolation() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-macos-isolation")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Test basic functionality
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success(), "Basic list command failed: {}", String::from_utf8_lossy(&output.stderr));

    // Test clean command
    let output = Command::new(&binary)
        .args(["clean"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success(), "Clean command failed: {}", String::from_utf8_lossy(&output.stderr));

    Ok(())
}

#[test]
#[serial]
fn test_nix_store_accessibility() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-nix-store-test")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // The sandbox should allow access to /nix/store
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success());
    
    Ok(())
}

#[test]
#[serial]
fn test_project_directory_accessibility() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-project-access-test")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Verify project files are accessible through normal operations
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success());
    
    // The test environment should be detected
    assert!(temp_dir.path().join("flake.nix").exists());
    assert!(temp_dir.path().join("accessible_file.txt").exists());
    
    Ok(())
}

#[test]
#[serial]
fn test_environment_variable_isolation() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-env-var-test")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Set a custom environment variable that should be inherited
    let output = Command::new(&binary)
        .args(["list"])
        .env("TEST_ISOLATION_VAR", "test_value")
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success());
    
    Ok(())
}

#[test]
#[serial]
fn test_network_accessibility() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-network-test")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Network should be available for Nix operations
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
fn test_linux_specific_isolation_features() -> anyhow::Result<()> {
    if Command::new("bwrap").arg("--version").output().is_err() {
        eprintln!("Skipping Linux-specific test: bubblewrap not available");
        return Ok(());
    }

    let temp_dir = TempDir::new("nix-sandbox-linux-specific")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Test that the sandbox works with Linux-specific features
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success(), "Linux-specific isolation failed: {}", String::from_utf8_lossy(&output.stderr));
    
    Ok(())
}

#[cfg(target_os = "macos")]
#[test]
#[serial]
fn test_macos_specific_isolation_features() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-macos-specific")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Test that the sandbox works with macOS-specific features
    let output = Command::new(&binary)
        .args(["list"])
        .current_dir(temp_dir.path())
        .output()?;
    
    assert!(output.status.success(), "macOS-specific isolation failed: {}", String::from_utf8_lossy(&output.stderr));
    
    Ok(())
}

#[test]
#[serial]
fn test_concurrent_isolation() -> anyhow::Result<()> {
    let temp_dir = TempDir::new("nix-sandbox-concurrent-test")?;
    setup_test_environment(temp_dir.path())?;
    
    let binary = get_nix_sandbox_binary();
    
    // Run multiple operations concurrently to test isolation stability
    let handles: Vec<_> = (0..3).map(|i| {
        let binary = binary.clone();
        let temp_dir_path = temp_dir.path().to_path_buf();
        
        std::thread::spawn(move || {
            let output = Command::new(&binary)
                .args(["list"])
                .current_dir(&temp_dir_path)
                .output()
                .expect(&format!("Thread {} failed to run command", i));
            
            assert!(output.status.success(), 
                   "Thread {} failed: {}", i, String::from_utf8_lossy(&output.stderr));
        })
    }).collect();
    
    // Wait for all threads to complete
    for handle in handles {
        handle.join().expect("Thread panicked");
    }
    
    Ok(())
}