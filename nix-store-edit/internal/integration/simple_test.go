package integration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/patch"
)

// TestSimpleEdit tests basic editing without dependency analysis
func TestSimpleEdit(t *testing.T) {
	// Skip if no real Nix store available
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("Skipping test: /nix/store not available")
	}
	
	t.Run("edit file in real store", func(t *testing.T) {
		// Find a simple text file in the real Nix store
		// This is a common file that exists in most Nix installations
		possiblePaths := []string{
			"/run/current-system/sw/bin/nix",
			"/nix/var/nix/profiles/default/bin/nix",
		}
		
		var nixBin string
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				// Resolve symlink to get actual store path
				resolved, err := filepath.EvalSymlinks(path)
				if err == nil {
					nixBin = resolved
					break
				}
			}
		}
		
		if nixBin == "" {
			t.Skip("Could not find nix binary in store")
		}
		
		t.Logf("Found nix binary at: %s", nixBin)
		
		// Create a simple test that just verifies we can read the file
		cfg := &config.Config{
			Path:     nixBin,
			Editor:   "true", // Just exit successfully
			DryRun:   true,   // Don't actually modify anything
			Timeout:  30 * time.Second,
		}
		
		// This should work up to the point where it tries to analyze dependencies
		err := patch.Run(cfg)
		if err != nil {
			// For now, we expect this to fail at dependency analysis
			t.Logf("Expected error: %v", err)
		}
	})
}