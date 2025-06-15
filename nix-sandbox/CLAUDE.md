# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

```bash
# Build the project
cargo build

# Build release version
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
# Run `nix flake check -L` instead to get better logs
nix flake check

# Run with debug logging
RUST_LOG=debug cargo run -- enter

# Install binary locally
sudo cp target/release/nix-sandbox /usr/local/bin/

# Enter the development shell (uses flake.nix)
nix develop

# Run cargo test and clippy to verify code quality
cargo test
cargo clippy

# Format code with nix formatter
nix fmt

# Run full Nix flake checks (includes VM tests)
# Note: This requires the binary to be built in the Nix store
nix flake check
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

## Testing Infrastructure

The project has comprehensive testing across multiple layers:

### Integration Tests (tests/)

- **19 comprehensive Rust integration tests** covering all major functionality
- `tests/integration_sandbox.rs` (5 tests): Sandbox creation, environment detection, cache keys
- `tests/integration_git.rs` (7 tests): Git worktree operations, branch isolation, edge cases
- `tests/integration_isolation.rs` (7 tests): Platform-specific sandboxing, security boundaries
- All tests use temporary directories and `serial_test` for isolation
- Tests automatically build the binary and handle cross-platform differences

### NixOS Integration Tests (checks/)

- `checks/integration-test.nix`: Basic functionality testing with real Nix environment
- `checks/vm-test.nix`: Full system testing in isolated NixOS VM with bubblewrap
- Run with `nix flake check` for complete VM-based validation

### Test Development Notes

- Integration tests require the binary to be built first (`cargo build --release`)
- Tests handle missing dependencies gracefully (e.g., bubblewrap on non-Linux systems)
- Use `RUST_LOG=debug` with test commands for detailed debugging
- All integration tests pass and provide comprehensive coverage of core functionality