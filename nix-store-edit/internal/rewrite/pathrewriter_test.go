package rewrite

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEngine_applyRewrites(t *testing.T) {
	// Create a test directory structure
	tempDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := []byte("This references /nix/store/abc123-test and /nix/store/def456-other")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test symlink
	symlinkPath := filepath.Join(tempDir, "test.link")
	if err := os.Symlink("/nix/store/abc123-test/bin/program", symlinkPath); err != nil {
		t.Fatalf("Failed to create test symlink: %v", err)
	}

	// Create engine with test rewrites
	engine := NewEngine()
	engine.recordRewrite("/nix/store/abc123-test", "/nix/store/xyz789-test")
	engine.recordRewrite("/nix/store/def456-other", "/nix/store/ghi012-other")

	// Apply rewrites
	if err := engine.applyRewrites(tempDir); err != nil {
		t.Fatalf("applyRewrites failed: %v", err)
	}

	// Check file was rewritten
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read rewritten file: %v", err)
	}

	if !bytes.Contains(newContent, []byte("xyz789")) {
		t.Error("File was not rewritten with new hash")
	}
	if !bytes.Contains(newContent, []byte("ghi012")) {
		t.Error("File was not rewritten with second new hash")
	}
	if bytes.Contains(newContent, []byte("abc123")) {
		t.Error("Old hash still present in file")
	}

	// Check symlink was updated
	newTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read updated symlink: %v", err)
	}

	expectedTarget := "/nix/store/xyz789-test/bin/program"
	if newTarget != expectedTarget {
		t.Errorf("Symlink target = %s, want %s", newTarget, expectedTarget)
	}
}

func TestEngine_rewriteFile(t *testing.T) {
	// Create a test file
	tempFile := filepath.Join(t.TempDir(), "test.sh")
	content := []byte(`#!/nix/store/abc123-bash/bin/bash
echo "Using /nix/store/def456-coreutils/bin/echo"
`)
	if err := os.WriteFile(tempFile, content, 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create engine with rewrites
	engine := NewEngine()
	engine.recordRewrite("/nix/store/abc123-bash", "/nix/store/xyz789-bash")
	engine.recordRewrite("/nix/store/def456-coreutils", "/nix/store/ghi012-coreutils")

	// Rewrite file
	if err := engine.rewriteFile(tempFile); err != nil {
		t.Fatalf("rewriteFile failed: %v", err)
	}

	// Check result
	newContent, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read rewritten file: %v", err)
	}

	expected := `#!/nix/store/xyz789-bash/bin/bash
echo "Using /nix/store/ghi012-coreutils/bin/echo"
`
	if string(newContent) != expected {
		t.Errorf("File content = %q, want %q", string(newContent), expected)
	}

	// Check permissions were preserved
	info, err := os.Stat(tempFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("File permissions = %o, want %o", info.Mode().Perm(), 0755)
	}
}

func TestEngine_rewriteSymlink(t *testing.T) {
	tempDir := t.TempDir()

	// Create test cases
	tests := []struct {
		name     string
		target   string
		expected string
		rewrites map[string]string
	}{
		{
			name:     "full path rewrite",
			target:   "/nix/store/abc123-test/bin/program",
			expected: "/nix/store/xyz789-test/bin/program",
			rewrites: map[string]string{
				"/nix/store/abc123-test": "/nix/store/xyz789-test",
			},
		},
		{
			name:     "partial path with hash",
			target:   "../abc123-test/lib/library.so",
			expected: "../xyz789-test/lib/library.so",
			rewrites: map[string]string{
				"/nix/store/abc123-test": "/nix/store/xyz789-test",
			},
		},
		{
			name:     "no matching rewrite",
			target:   "/usr/bin/env",
			expected: "/usr/bin/env",
			rewrites: map[string]string{
				"/nix/store/abc123-test": "/nix/store/xyz789-test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create symlink
			linkPath := filepath.Join(tempDir, tt.name)
			if err := os.Symlink(tt.target, linkPath); err != nil {
				t.Fatalf("Failed to create symlink: %v", err)
			}

			// Create engine with rewrites
			engine := NewEngine()
			for old, new := range tt.rewrites {
				engine.recordRewrite(old, new)
			}

			// Rewrite symlink
			if err := engine.rewriteSymlink(linkPath); err != nil {
				t.Fatalf("rewriteSymlink failed: %v", err)
			}

			// Check result
			newTarget, err := os.Readlink(linkPath)
			if err != nil {
				t.Fatalf("Failed to read symlink: %v", err)
			}

			if newTarget != tt.expected {
				t.Errorf("Symlink target = %s, want %s", newTarget, tt.expected)
			}
		})
	}
}

func TestEngine_extractPath_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=1 to run.")
	}

	// This test would require actual Nix store paths
	engine := NewEngine()
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "extracted")

	// Would need a real store path here
	storePath := "/nix/store/some-real-path"

	err := engine.extractPath(storePath, destPath)
	if err != nil {
		t.Fatalf("extractPath failed: %v", err)
	}

	// Verify extraction
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Destination path was not created")
	}
}
