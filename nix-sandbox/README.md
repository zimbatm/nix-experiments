# nix-sandbox

Secure, reproducible development environments using Nix.

## Overview

`nix-sandbox` provides safe-by-default development environments that are portable and instant to enter. It creates isolated sandboxes for each project, restricting code execution to the project scope while maintaining access to Nix for reproducible builds.

## Features

- **Security**: Restricts filesystem access to project directory only
- **Reproducibility**: Uses Nix flakes or devenv.nix for consistent environments
- **Performance**: Caches environments for instant re-entry
- **Git Integration**: Supports separate workspaces per branch

## Requirements

- Nix with flakes enabled
- Git
- Linux: `bubblewrap` (bwrap)
- macOS: `sandbox-exec` (included with macOS)

## Installation

```bash
# Clone and build
git clone https://github.com/zimbatm/nix-experiments.git
cd nix-experiments/nix-sandbox
cargo build --release

# Install binary
sudo cp target/release/nix-sandbox /usr/local/bin/
```

## Usage

```bash
# Enter sandbox in current directory
nix-sandbox enter

# Enter sandbox for a specific branch (creates separate workspace)
nix-sandbox enter feature-branch

# List active sessions
nix-sandbox list

# Clean cached environments
nix-sandbox clean
```

## How It Works

1. **Environment Detection**: Looks for `flake.nix` or `devenv.nix` in the project
2. **Sandboxing**: Uses OS-native sandboxing (bubblewrap on Linux, sandbox-exec on macOS)
3. **Filesystem Isolation**:
   - Full access to project directory
   - Read-only access to `/nix/store`
   - Access to Nix daemon socket for builds
   - No access to rest of host filesystem
4. **Git Workspaces**: When branch name provided, creates separate Git worktree

## Security Model

The sandbox denies all host access by default, only allowing:

- Project directory (read/write)
- Nix store (read-only)
- Nix daemon socket (for builds)
- Network access (for Nix daemon)
- Essential devices (/dev/null, /dev/random, etc.)

## Development

```bash
# Build the project
cargo build --release

# Run all tests (unit + integration)
cargo test

# Run integration tests only (19 comprehensive tests)
cargo test --test integration_sandbox --test integration_git --test integration_isolation

# Run specific integration test suites
cargo test --test integration_sandbox    # Sandbox creation & functionality (5 tests)
cargo test --test integration_git        # Git worktree operations (7 tests)
cargo test --test integration_isolation  # Platform-specific isolation (7 tests)

# Run NixOS integration tests (VM-based testing)
nix flake check

# Run with logging
RUST_LOG=debug cargo run -- enter

# Check code quality
cargo clippy
```

### Integration Test Coverage

The project includes multiple test layers:

**Rust Integration Tests** (19 tests):

- **Sandbox Creation** (`tests/integration_sandbox.rs`):

  - Basic functionality (list, clean commands)
  - Environment detection (flake.nix, devenv.nix)
  - Cache key generation and invalidation
  - Error handling for missing environments
  - Linux-specific isolation with bubblewrap

- **Git Operations** (`tests/integration_git.rs`):

  - Multi-branch repository setup and switching
  - Branch-specific environment isolation
  - Named session support with worktrees
  - Git cleanup operations
  - Edge cases (non-git repos, detached HEAD)

- **Security Isolation** (`tests/integration_isolation.rs`):
  - Platform-specific sandboxing (Linux/macOS)
  - Filesystem access validation
  - Environment variable handling
  - Network accessibility testing
  - Concurrent sandbox operations

**NixOS Integration Tests** (`checks/`):

- **Basic Integration** (`integration-test.nix`): Tests binary functionality with real Nix environment
- **VM Testing** (`vm-test.nix`): Full system testing in isolated NixOS VM with bubblewrap

## License

MIT
