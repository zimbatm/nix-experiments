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
# Run tests
cargo test

# Run with logging
RUST_LOG=debug cargo run -- enter
```

## License

MIT