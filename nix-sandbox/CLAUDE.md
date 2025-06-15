# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

```bash
# Build the project
cargo build

# Build release version
cargo build --release

# Run tests
cargo test

# Run with debug logging
RUST_LOG=debug cargo run -- enter

# Install binary locally
sudo cp target/release/nix-sandbox /usr/local/bin/

# Enter the development shell (uses flake.nix)
nix develop

# Run cargo test and clippy to verify code quality
cargo test
cargo clippy
```

## Architecture Overview

This is a Rust CLI tool that creates secure, isolated development environments using Nix. The architecture follows a modular design:

- **CLI Layer** (`src/cli.rs`): Handles command parsing and dispatching (enter, list, clean)
- **Session Management** (`src/session.rs`): Manages Git workspaces and branch-based sessions
- **Environment Detection** (`src/environment.rs`): Detects `flake.nix` or `devenv.nix` and generates cache keys
- **Sandboxing** (`src/sandbox/`): OS-specific sandboxing implementations
  - Linux: Uses `bubblewrap` for filesystem isolation
  - macOS: Uses `sandbox-exec` for security restrictions
- **Configuration** (`src/config.rs`): Manages cache directories and settings

## Key Design Patterns

- **Platform-specific code**: Sandboxing logic is separated by OS in `src/sandbox/linux.rs` and `src/sandbox/macos.rs`
- **Environment types**: Supports both Nix flakes (`flake.nix`) and devenv (`devenv.nix`) with different shell commands
- **Git integration**: Creates separate workspaces per branch when session names are provided
- **Caching**: Uses SHA256 hashes of environment files and modification times for cache invalidation

## Security Model

The sandbox restricts filesystem access to:
- Project directory (full access)
- `/nix/store` (read-only)
- Nix daemon socket (for builds)
- Essential devices (`/dev/null`, `/dev/random`)

All other host filesystem access is denied by default.

## Dependencies

- Uses `clap` for CLI parsing with derive macros
- `tokio` for async runtime
- `tracing` for structured logging
- Platform-specific `nix` crate features for system calls
- `bubblewrap` required on Linux systems