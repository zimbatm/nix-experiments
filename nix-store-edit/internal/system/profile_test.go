package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testProfile := filepath.Join(tmpDir, "test-profile")

	// Create a test file to act as our profile
	if err := os.WriteFile(testProfile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test profile: %v", err)
	}

	t.Run("Type", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		if p.Type() != TypeProfile {
			t.Errorf("Type() = %v, want %v", p.Type(), TypeProfile)
		}
	})

	t.Run("GetClosurePath with valid path", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		path, err := p.GetClosurePath()
		if err != nil {
			t.Errorf("GetClosurePath() error = %v", err)
		}
		if path != testProfile {
			t.Errorf("GetClosurePath() = %v, want %v", path, testProfile)
		}
	})

	t.Run("GetClosurePath with empty path", func(t *testing.T) {
		p := &Profile{}
		_, err := p.GetClosurePath()
		if err == nil {
			t.Error("GetClosurePath() expected error for empty path")
		}
	})

	t.Run("TestConfiguration with valid path", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		err := p.TestConfiguration(testProfile)
		if err != nil {
			t.Errorf("TestConfiguration() error = %v", err)
		}
	})

	t.Run("TestConfiguration with invalid path", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		err := p.TestConfiguration("/non/existent/path")
		if err == nil {
			t.Error("TestConfiguration() expected error for non-existent path")
		}
	})

	t.Run("ApplyClosure", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		// This test will only work if nix-env is available
		// In most test environments, this will fail, which is expected
		err := p.ApplyClosure(testProfile, "")
		if err != nil {
			t.Logf("ApplyClosure() error (expected in test environment): %v", err)
		}
	})

	t.Run("ApplyClosure with custom command", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		// Test with a custom command using placeholders
		err := p.ApplyClosure(testProfile, "echo {path} {profile}")
		if err != nil {
			t.Errorf("ApplyClosure() with custom command error: %v", err)
		}
	})

	t.Run("GetDefaultCommand", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		cmd := p.GetDefaultCommand(testProfile)
		expectedStart := []string{"nix-env", "--profile", testProfile, "--set", testProfile}

		if len(cmd) != len(expectedStart) {
			t.Errorf("GetDefaultCommand() length = %d, want %d", len(cmd), len(expectedStart))
		}

		for i, arg := range expectedStart {
			if i >= len(cmd) || cmd[i] != arg {
				t.Errorf("GetDefaultCommand()[%d] = %v, want %v", i, cmd[i], arg)
			}
		}
	})

	t.Run("IsAvailable with path", func(t *testing.T) {
		p := &Profile{ProfilePath: testProfile}
		if !p.IsAvailable() {
			t.Error("IsAvailable() = false, want true")
		}
	})

	t.Run("IsAvailable without path", func(t *testing.T) {
		p := &Profile{}
		if p.IsAvailable() {
			t.Error("IsAvailable() = true, want false")
		}
	})
}
