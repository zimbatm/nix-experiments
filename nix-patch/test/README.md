# nix-patch Integration Tests

This directory contains integration tests and utilities for testing nix-patch with a custom Nix store that doesn't interfere with your system Nix installation.

## Test Environment Setup

The `setup_test_env.sh` script creates an isolated test environment with:
- Custom store directory (`/tmp/nix-patch-test/nix/store`)
- Custom profile directory (`/tmp/nix-patch-test/nix/var/nix/profiles`)
- Mock store items for testing various scenarios
- Environment configuration script

### Quick Start

1. **Create test environment:**
   ```bash
   ./test/setup_test_env.sh setup
   ```

2. **Configure your shell:**
   ```bash
   source /tmp/nix-patch-test/env.sh
   ```

3. **Run nix-patch with test profile:**
   ```bash
   # List available store items
   ./test/setup_test_env.sh list

   # Edit a file in the test store
   nix-patch --store $TEST_STORE_ROOT --system=profile --profile=$TEST_PROFILE \
     /tmp/nix-patch-test/nix/store/*-config-1.0/etc/app.conf
   ```

4. **Clean up when done:**
   ```bash
   ./test/setup_test_env.sh cleanup
   ```

## Running Go Tests

The integration tests use the custom store approach:

```bash
# Run all integration tests
go test ./internal/integration/...

# Run specific test suites
go test ./internal/integration/integration_test.go
go test ./internal/integration/nar_test.go
go test ./internal/integration/profile_test.go

# Run with verbose output
go test -v ./internal/integration/...

# Run specific test
go test -v -run TestBasicFileEdit ./internal/integration/...
```

## Test Scenarios Covered

### Basic Operations
- Edit text files in custom store
- Dry-run mode verification
- Timeout handling
- Error scenarios (non-existent files, outside store)

### Complex Scenarios
- Files with dependencies requiring rewrites
- Circular dependency handling
- Binary file rewrites with embedded store paths
- Concurrent edits to same closure

### Profile-Specific Tests
- Profile system detection with custom store
- Multiple profile generations
- Broken and circular symlinks
- Deeply nested profiles
- Profiles with spaces in paths

### Edge Cases
- Empty files
- Very large files
- Read-only permissions
- Various editor commands

## Test Store Structure

The test environment creates mock store items:

```
/tmp/nix-patch-test/nix/store/
├── <hash>-config-1.0/          # Configuration package
│   └── etc/app.conf
├── <hash>-scripts-1.0/         # Scripts package
│   └── bin/hello.sh
├── <hash>-myapp-2.0/           # App with dependencies
│   ├── bin/myapp
│   └── share/myapp/references
└── <hash>-complex-1.0/         # Complex package
    ├── bin/binary              # Mock ELF with store paths
    ├── bin/config-link         # Symlink
    ├── etc/config.ini
    └── share/doc/README.md
```

## Safety Features

- All test operations use a custom store directory
- Never touches system `/nix/store`
- Cleanup only removes test directories under `/tmp/nix-patch-test*`
- Environment variables are properly isolated

## Debugging Tips

1. **Inspect test store:**
   ```bash
   find /tmp/nix-patch-test/nix/store -type f -name "*.conf" | xargs cat
   ```

2. **Check profile structure:**
   ```bash
   ls -la /tmp/nix-patch-test/nix/var/nix/profiles/
   ```

3. **Verify environment:**
   ```bash
   source /tmp/nix-patch-test/env.sh
   echo $TEST_STORE_ROOT  # Should show test root path
   ```

4. **Test with verbose logging:**
   ```bash
   nix-patch --store $TEST_STORE_ROOT --verbose --dry-run --system=profile \
     --profile=$TEST_PROFILE <store-path>
   ```

## Writing New Tests

When adding new integration tests:

1. Use `NewTestEnvironment(t)` to create isolated environment
2. Always call `defer env.Cleanup()`
3. Create store items with proper structure using helper methods
4. Test both success and failure scenarios
5. Verify the test doesn't touch system Nix paths

Example test structure:
```go
func TestNewScenario(t *testing.T) {
    env := NewTestEnvironment(t)
    defer env.Cleanup()
    
    t.Run("specific case", func(t *testing.T) {
        // Create test data
        item := env.CreateStoreItem("test", "content")
        env.CreateProfileWithClosure(filepath.Dir(item))
        
        // Run test
        cfg := &config.Config{
            Path:        item,
            Editor:      "sed -i 's/old/new/g'",
            SystemType:  "profile",
            ProfilePath: env.profile,
            StoreRoot:   env.tempDir,  // Use test store root
            // ...
        }
        
        err := patch.Run(cfg)
        // Verify results
    })
}
```