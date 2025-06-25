# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

nix-store-edit is a "heretic tool" that allows direct editing of files in the Nix store. It creates mutable copies of store paths, allows editing, and then creates new store paths with the changes, rewriting system closures to use the new paths.

## Development Commands

### Building
```bash
# Build with Nix (recommended)
nix build

# Build with Go directly
go build -o nix-store-edit

# Enter development shell with all dependencies
nix develop
```

### Testing
```bash
# Run all tests
go test ./...

# Run all tests with verbose output
go test -v ./...

# Run integration tests specifically
go test ./internal/integration/...

# Run a specific test
go test -v -run TestBasicFileEdit ./internal/integration/...
```

### Linting
```bash
# Run golangci-lint (available in nix develop shell)
golangci-lint run

# Run go vet
go vet ./...

# Format code
go fmt ./...
```

## Architecture

### Core Components

1. **cmd/root.go** - CLI interface, flag parsing, and command execution entry point
2. **internal/patch/** - Main patching logic that orchestrates the edit workflow
3. **internal/store/** - Nix store operations including path validation, dependencies, and trusted user checks
4. **internal/rewrite/** - Store path rewriting logic for updating references after edits
5. **internal/system/** - System detection and integration (NixOS, nix-darwin, home-manager, profile)
6. **internal/nar/** - NAR archive extraction (currently uses cp due to go-nix limitations)
7. **internal/archive/** - Archive operations for creating new store items
8. **internal/errors/** - Custom error types with exit codes

### Key Workflows

1. **File Editing Flow**:
   - Validate store path and user permissions
   - Extract store item to temporary location
   - Open in editor
   - Create new store item with changes
   - Rewrite dependent store paths
   - Update system configuration

2. **System Integration**:
   - Detects system type (NixOS, nix-darwin, home-manager, or profile)
   - Each system has its own update mechanism
   - Supports dry-run mode for safety

### Safety Features

- Requires trusted user status for Nix operations
- Validates all paths are within Nix store
- Supports dry-run mode to preview changes
- Comprehensive error handling with specific exit codes
- Integration tests use isolated test environment to avoid touching system store

### Testing Strategy

- Unit tests alongside source files (*_test.go)
- Integration tests in internal/integration/ with isolated Nix store
- Test environment setup is done in pure Go (no shell scripts)
- Tests cover edge cases like circular dependencies, binary rewrites, and various system types
- NEVER write mocks.

### Integration Test Limitations

The integration tests currently have limitations when working with custom Nix stores:

1. **Store Registration**: Test items must be properly registered in the Nix database using `nix-store --add`
2. **Dependency Analysis**: `nix why-depends` requires real store paths with actual dependencies
3. **Custom Store Paths**: When using `--store` flag, Nix commands expect standard `/nix/store/` paths, not custom store paths
4. **Test Isolation**: Tests create a temporary store directory but need proper Nix database initialization

For full integration testing, consider:
- Using `nix-store --add` to properly register test files
- Creating real derivations with dependencies for testing `nix why-depends`
- Testing against a real Nix store when possible (with appropriate safeguards)

**Current Approach**: 
- Integration tests use `nix-build --store` to create derivations in a temporary custom store
- Tests use a custom store to avoid modifying the real `/nix/store`
- Test fixtures in `internal/integration/fixtures/` contain minimal Nix expressions
- Path translation between custom store paths and standard `/nix/store/` paths is handled automatically

**Test Fixtures**:
- `minimal.nix` - Simple text file derivation (no nixpkgs needed)
- `minimal-with-deps.nix` - Derivation with dependency relationships  
- `minimal-complex-deps.nix` - Complex dependency graph test case
- `with-dependencies.nix` - Uses nixpkgs for more complex scenarios

**Custom Store Integration**:
When using `--store` flag with a custom store:
1. `nix-build --store /path` builds derivations and stores outputs in `/path/nix/store/`
2. Nix commands (`nix why-depends`, `nix-store --query`) expect standard `/nix/store/` paths
3. The store implementation automatically converts between custom and standard paths
4. NAR archives must use standard paths for `nix-store --import` to work

**Known Limitations**:
- Some fixtures require nixpkgs availability
- Complex derivations with many dependencies can be slow to build in tests
