package integration_test

import (
	"os"
	"testing"
)

// skipIfNoNix skips the test if nix is not available
func skipIfNoNix(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("Skipping test: /nix/store not available")
	}
}

// isRealNixAvailable checks if we have a real Nix installation
func isRealNixAvailable() bool {
	_, err := os.Stat("/nix/store")
	return err == nil
}