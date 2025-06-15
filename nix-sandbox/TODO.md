# TODO - Nix Sandbox Implementation

## Core Implementation
- [x] Create Rust project structure with Cargo.toml
- [x] Implement environment detection (flake.nix/devenv.nix)
- [x] Implement filesystem sandboxing for Linux (using bubblewrap)
- [x] Implement filesystem sandboxing for macOS (using sandbox-exec)
- [x] Implement Git-aware session management
- [x] Create CLI interface with enter command
- [ ] Add environment caching mechanism
- [ ] Add file watching for environment changes

## Testing
- [x] Add unit tests for environment detection
- [x] Add unit tests for session management
- [x] Add unit tests for cache key generation
- [x] Add integration tests for sandbox creation (19 comprehensive tests)
  - Run with: `cargo test --test integration_sandbox --test integration_git --test integration_isolation`
  - Individual suites: `cargo test --test [integration_sandbox|integration_git|integration_isolation]`
  - NixOS VM tests: `nix flake check` (includes checks/integration-test.nix and checks/vm-test.nix)
- [x] Add integration tests for Git worktree operations
- [x] Test sandbox isolation on Linux (bubblewrap)
- [x] Test sandbox isolation on macOS (sandbox-exec)

## Features to Complete
- [ ] Implement proper environment caching to speed up re-entry
- [ ] Add file watcher to detect changes to flake.nix/devenv.nix
- [ ] Implement prompt to reload when environment files change
- [ ] Add support for per-project configuration overrides
- [ ] Add support for exposing specific host paths (e.g., ~/.gitconfig)
- [ ] Improve error messages and user feedback
- [ ] Add progress indicators for long-running operations

## Documentation
- [x] Write comprehensive README.md
- [x] Add usage examples
- [x] Document security model
- [ ] Document configuration options
- [ ] Add troubleshooting guide

## Build and Distribution
- [ ] Set up CI/CD pipeline
- [ ] Create release binaries for Linux and macOS
- [ ] Create installation script
- [ ] Package for Nix
- [ ] Package for Homebrew (macOS)

## Code Refactoring (Technical Debt)
- [x] Create constants module for magic strings and hard-coded values
- [x] Break down complex functions (Environment::cache_key, SessionManager::create_or_get_session)
- [ ] Extract duplicate code patterns (path building, git commands, file hashing)
- [ ] Create missing abstractions (command execution, Git repository, structured shell commands)
- [ ] Unify error handling with custom Result type
- [ ] Improve type safety (structured ShellCommand instead of string parsing)
- [ ] Standardize naming conventions (remove inconsistent get_ prefixes)
- [ ] Separate concerns in Config::load() method
- [ ] Create Git utility module for repository operations
- [ ] Extract command execution abstraction for sandbox and session management

## Future Enhancements (v2+)
- [ ] Add support for shells other than bash
- [ ] Implement secrets management
- [ ] Add IDE integration
- [ ] Support for global tooling overlays
- [ ] Windows support (WSL2)
- [ ] Performance profiling and optimization
- [ ] Support for devcontainer.json
- [ ] Network isolation options
- [ ] Resource limits (CPU, memory)