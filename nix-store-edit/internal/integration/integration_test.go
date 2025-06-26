package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/patch"
)

// TestEnvironment encapsulates a custom Nix environment for testing
type TestEnvironment struct {
	t          *testing.T
	tempDir    string
	storeDir   string
	profileDir string
	profile    string
}

// NewTestEnvironment creates an isolated Nix environment for testing
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	tempDir := t.TempDir()

	env := &TestEnvironment{
		t:          t,
		tempDir:    tempDir,
		storeDir:   filepath.Join(tempDir, "nix", "store"),
		profileDir: filepath.Join(tempDir, "nix", "var", "nix", "profiles"),
		profile:    filepath.Join(tempDir, "nix", "var", "nix", "profiles", "test-profile"),
	}

	// Create directories for the Nix store
	must(t, os.MkdirAll(env.storeDir, 0755))
	must(t, os.MkdirAll(env.profileDir, 0755))
	must(t, os.MkdirAll(filepath.Join(tempDir, "nix", "var", "nix", "db"), 0755))
	must(t, os.MkdirAll(filepath.Join(tempDir, "nix", "var", "log", "nix"), 0755))

	// Initialize a minimal Nix store database
	// Create an empty Nix database file (this is a simplified version)
	dbPath := filepath.Join(tempDir, "nix", "var", "nix", "db", "db.sqlite")
	_, err := os.Create(dbPath)
	must(t, err)

	// Initialize the store with some basic derivations
	env.initializeStore()

	return env
}

// BuildDerivation builds a Nix derivation and returns the output path
func (e *TestEnvironment) BuildDerivation(nixFile string) string {
	// Get the fixtures directory
	fixturesDir := filepath.Join("fixtures", nixFile)

	// Build the derivation with our custom store
	cmd := exec.Command("nix-build", "--store", e.tempDir, fixturesDir, "--no-out-link")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			e.t.Fatalf("Failed to build derivation %s: %v\nStderr: %s", nixFile, err, exitErr.Stderr)
		}
		e.t.Fatalf("Failed to build derivation %s: %v", nixFile, err)
	}

	// The output is the store path (last line of output)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	storePath := lines[len(lines)-1]

	// Convert /nix/store path to our custom store path
	if strings.HasPrefix(storePath, "/nix/store/") {
		relativePath := strings.TrimPrefix(storePath, "/nix/store/")
		customPath := filepath.Join(e.storeDir, relativePath)
		// Always return the custom store path for test consistency
		return customPath
	}

	return storePath
}

// CreateStoreItem creates a simple text file derivation for backwards compatibility
func (e *TestEnvironment) CreateStoreItem(name, content string) string {
	// Create a temporary nix file that produces the desired content
	tmpNix := filepath.Join(e.tempDir, fmt.Sprintf("%s.nix", name))
	nixContent := fmt.Sprintf(`
derivation {
  name = "%s";
  system = builtins.currentSystem;
  builder = "/bin/sh";
  args = [ "-c" "echo -n '%s' > $out" ];
}
`, name, content)
	must(e.t, os.WriteFile(tmpNix, []byte(nixContent), 0644))

	// Build it with the custom store
	cmd := exec.Command("nix-build", "--store", e.tempDir, tmpNix, "--no-out-link")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			e.t.Fatalf("Failed to build store item: %v\nStderr: %s", err, exitErr.Stderr)
		}
		e.t.Fatalf("Failed to build store item: %v", err)
	}

	// Clean up temp file
	_ = os.Remove(tmpNix)

	// The output is the store path (last line of output)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	storePath := lines[len(lines)-1]

	// Convert /nix/store path to our custom store path
	if strings.HasPrefix(storePath, "/nix/store/") {
		relativePath := strings.TrimPrefix(storePath, "/nix/store/")
		customPath := filepath.Join(e.storeDir, relativePath)
		// Always return the custom store path for test consistency
		return customPath
	}

	return storePath
}

// CreateComplexStoreStructure creates a more complex store item with multiple files
func (e *TestEnvironment) CreateComplexStoreStructure(name string) string {
	hash := fmt.Sprintf("%032x", time.Now().UnixNano())
	itemPath := filepath.Join(e.storeDir, fmt.Sprintf("%s-%s", hash[:32], name))

	// Create directory structure
	must(e.t, os.MkdirAll(filepath.Join(itemPath, "bin"), 0755))
	must(e.t, os.MkdirAll(filepath.Join(itemPath, "etc"), 0755))
	must(e.t, os.MkdirAll(filepath.Join(itemPath, "lib"), 0755))

	// Create files
	must(e.t, os.WriteFile(filepath.Join(itemPath, "bin", "program"), []byte("#!/bin/sh\necho 'Hello'"), 0755))
	must(e.t, os.WriteFile(filepath.Join(itemPath, "etc", "config.conf"), []byte("setting=value"), 0644))
	must(e.t, os.WriteFile(filepath.Join(itemPath, "lib", "library.so"), []byte("binary content"), 0644))

	return itemPath
}

// CreateProfileWithClosure creates a test profile pointing to store items
func (e *TestEnvironment) CreateProfileWithClosure(items ...string) {
	if len(items) == 0 {
		e.t.Fatal("At least one store item required for profile")
	}

	// Remove existing profile if it exists
	_ = os.Remove(e.profile)

	// Get the directory of the first item to use as profile target
	profileTarget := items[0]
	if strings.Contains(profileTarget, "/file.txt") {
		profileTarget = filepath.Dir(profileTarget)
	}

	// Create a simple closure by symlinking to the first item
	must(e.t, os.Symlink(profileTarget, e.profile))
}

// Cleanup cleans up environment variables and makes store items writable for deletion
func (e *TestEnvironment) Cleanup() {
	// Make store items writable so they can be deleted
	if _, err := os.Stat(e.storeDir); err == nil {
		filepath.Walk(e.storeDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// Make everything writable
			os.Chmod(path, 0755)
			return nil
		})
	}
}

// CreateConfig creates a config with test environment settings
func (e *TestEnvironment) CreateConfig() *config.Config {
	return &config.Config{
		Timeout:   30 * time.Second,
		StoreRoot: e.tempDir, // Use the temp directory as the store root
	}
}

// Test scenarios

func TestBasicFileEdit(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	t.Run("edit text file in custom store", func(t *testing.T) {
		// Build a minimal file derivation (no nixpkgs needed)
		simpleFile := env.BuildDerivation("minimal.nix")
		env.CreateProfileWithClosure(simpleFile)

		cfg := env.CreateConfig()
		cfg.Path = simpleFile
		cfg.Editor = "sed -i 's/original/modified/g'" // Use sed as editor for testing
		cfg.SystemType = "profile"
		cfg.ProfilePath = env.profile
		cfg.DryRun = false

		err := patch.Run(cfg)
		if err != nil {
			t.Fatalf("Failed to edit file: %v", err)
		}

		// The edit should have created a new store path with modified content
	})

	t.Run("dry-run doesn't modify store", func(t *testing.T) {
		// Build a simple file
		filePath := env.CreateStoreItem("config", "do not modify")
		originalContent := mustReadFile(t, filePath)
		env.CreateProfileWithClosure(filePath)

		cfg := env.CreateConfig()
		cfg.Path = filePath
		cfg.Editor = "sed -i 's/not/MODIFIED/g'"
		cfg.SystemType = "profile"
		cfg.ProfilePath = env.profile
		cfg.DryRun = true

		err := patch.Run(cfg)
		if err != nil {
			t.Fatalf("Dry-run failed: %v", err)
		}

		// Verify content unchanged
		currentContent := mustReadFile(t, filePath)
		if currentContent != originalContent {
			t.Errorf("Dry-run modified file: was %q, now %q", originalContent, currentContent)
		}
	})
}

func TestComplexRewriteScenarios(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	t.Run("edit file with dependencies", func(t *testing.T) {
		// For now, create a simple file that contains references to test dependency handling
		content := `Config file
References: /nix/store/abc123-somelib
References: /nix/store/def456-otherlib`
		configFile := env.CreateStoreItem("config-with-refs", content)
		env.CreateProfileWithClosure(configFile)

		cfg := env.CreateConfig()
		cfg.Path = configFile
		cfg.Editor = "sed -i 's|abc123|xyz789|g'"
		cfg.SystemType = "profile"
		cfg.ProfilePath = env.profile
		cfg.DryRun = false

		err := patch.Run(cfg)
		if err != nil {
			t.Fatalf("Failed to edit file with dependencies: %v", err)
		}

		// The system should have created a new closure with updated references
	})

	t.Run("complex dependency handling", func(t *testing.T) {
		// Build a test fixture with complex dependency graph
		complexPath := env.BuildDerivation("minimal-complex-deps.nix")
		env.CreateProfileWithClosure(complexPath)

		// The bundle file itself contains the references
		configFile := complexPath

		cfg := env.CreateConfig()
		cfg.Path = configFile
		cfg.Editor = "sed -i 's/Config A/Configuration A/g'"
		cfg.SystemType = "profile"
		cfg.ProfilePath = env.profile
		cfg.DryRun = false

		// Should handle complex dependency graphs gracefully
		err := patch.Run(cfg)
		if err != nil {
			t.Fatalf("Failed to handle complex deps: %v", err)
		}
	})
}

func TestErrorScenarios(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	t.Run("edit non-existent file", func(t *testing.T) {
		cfg := &config.Config{
			Path:        filepath.Join(env.storeDir, "nonexistent"),
			Editor:      "vim",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}

		err := patch.Run(cfg)
		if err == nil {
			t.Fatal("Expected error when editing non-existent file")
		}
	})

	t.Run("edit outside store", func(t *testing.T) {
		// Create a file outside the store
		outsideFile := filepath.Join(env.tempDir, "outside.txt")
		must(t, os.WriteFile(outsideFile, []byte("outside content"), 0644))

		cfg := &config.Config{
			Path:        outsideFile,
			Editor:      "vim",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}

		err := patch.Run(cfg)
		if err == nil {
			t.Fatal("Expected error when editing file outside store")
		}
	})

	t.Run("timeout handling", func(t *testing.T) {
		filePath := env.CreateStoreItem("config", "content")
		env.CreateProfileWithClosure(filepath.Dir(filePath))

		cfg := &config.Config{
			Path:        filePath,
			Editor:      "sleep 10", // Command that takes too long
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     1 * time.Second,
		}

		err := patch.Run(cfg)
		if err == nil {
			t.Fatal("Expected timeout error")
		}
	})
}

func TestEdgeCases(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	t.Run("edit empty file", func(t *testing.T) {
		filePath := env.CreateStoreItem("empty", "")
		env.CreateProfileWithClosure(filepath.Dir(filePath))

		cfg := &config.Config{
			Path:        filePath,
			Editor:      "echo 'new content' > ",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}

		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Empty file edit result: %v", err)
		}
	})

	t.Run("edit symlink", func(t *testing.T) {
		// Create a target file
		targetPath := env.CreateStoreItem("target", "target content")

		// Create a symlink to it
		linkDir := env.CreateComplexStoreStructure("links")
		linkPath := filepath.Join(linkDir, "link")
		must(t, os.Symlink(targetPath, linkPath))

		env.CreateProfileWithClosure(linkDir, filepath.Dir(targetPath))

		cfg := &config.Config{
			Path:        linkPath,
			Editor:      "sed -i 's/target/modified/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}

		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Symlink edit result: %v", err)
		}
	})

	t.Run("very large file", func(t *testing.T) {
		// Create a large file (10MB)
		largeContent := strings.Repeat("large content line\n", 500000)
		filePath := env.CreateStoreItem("large", largeContent)
		env.CreateProfileWithClosure(filepath.Dir(filePath))

		cfg := &config.Config{
			Path:        filePath,
			Editor:      "head -n 10 > ", // Just keep first 10 lines
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}

		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Large file edit result: %v", err)
		}
	})
}

func TestActivationScenarios(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	t.Run("custom activation command", func(t *testing.T) {
		filePath := env.CreateStoreItem("config", "content")
		env.CreateProfileWithClosure(filepath.Dir(filePath))

		// Create a script to use as activation command
		activationScript := filepath.Join(env.tempDir, "activate.sh")
		must(t, os.WriteFile(activationScript, []byte("#!/bin/sh\necho 'Activated' > "+filepath.Join(env.tempDir, "activated.txt")), 0755))

		cfg := &config.Config{
			Path:              filePath,
			Editor:            "sed -i 's/content/modified/g'",
			SystemType:        "profile",
			ProfilePath:       env.profile,
			ActivationCommand: activationScript,
			DryRun:            false,
			Timeout:           30 * time.Second,
		}

		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Custom activation result: %v", err)
		}

		// Check if activation ran
		if _, err := os.Stat(filepath.Join(env.tempDir, "activated.txt")); err == nil {
			t.Log("Activation command executed successfully")
		}
	})

	t.Run("activation failure handling", func(t *testing.T) {
		filePath := env.CreateStoreItem("config", "content")
		env.CreateProfileWithClosure(filepath.Dir(filePath))

		cfg := &config.Config{
			Path:              filePath,
			Editor:            "sed -i 's/content/modified/g'",
			SystemType:        "profile",
			ProfilePath:       env.profile,
			ActivationCommand: "false", // Command that always fails
			DryRun:            false,
			Timeout:           30 * time.Second,
		}

		err := patch.Run(cfg)
		if err == nil {
			t.Fatal("Expected error from failed activation command")
		}
	})
}

// Helper functions

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	must(t, err)
	return string(content)
}

// initializeStore sets up the Nix store database
func (e *TestEnvironment) initializeStore() {
	// The store is initialized when we first use nix-build with --store flag
	// No need to manually create items here
}
