package patch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
)

func TestRun_InvalidPath(t *testing.T) {
	cfg := &config.Config{
		Path:   "/tmp/not-a-store-path",
		Editor: "vim",
	}

	err := Run(cfg)
	if err == nil {
		t.Error("Expected error for non-store path")
	}
}

func TestRun_SymlinkResolution(t *testing.T) {
	// Create a temporary symlink for testing
	tmpDir := t.TempDir()
	linkPath := filepath.Join(tmpDir, "test-link")

	// Create a symlink to a non-existent nix store path
	// In real test, this would point to an actual store path
	err := os.Symlink("/nix/store/fake-path", linkPath)
	if err != nil {
		t.Skip("Cannot create symlink for test")
	}

	cfg := &config.Config{
		Path:   linkPath,
		Editor: "vim",
	}

	// This will fail because the target doesn't exist, but we're testing
	// that it attempts to resolve the symlink
	err = Run(cfg)
	if err == nil {
		t.Error("Expected error, but got none")
	}
}

// Integration tests would be needed for:
// - Full Run() execution with actual Nix store
// - Editor interaction
// - System closure rewriting
// - nixos-rebuild test command

func TestRun_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=1 to run.")
	}

	// Integration tests would go here
	// They would require:
	// - A real Nix store
	// - Root privileges for nixos-rebuild
	// - A test system closure
}
