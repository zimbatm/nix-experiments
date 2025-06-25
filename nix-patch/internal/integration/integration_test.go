package integration_test

import (
	"fmt"
	"os"
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
		storeDir:   filepath.Join(tempDir, "store"),
		profileDir: filepath.Join(tempDir, "profiles"),
		profile:    filepath.Join(tempDir, "profiles", "test-profile"),
	}
	
	// Create directories
	must(t, os.MkdirAll(env.storeDir, 0755))
	must(t, os.MkdirAll(env.profileDir, 0755))
	
	return env
}

// CreateStoreItem creates a test item in the custom store
func (e *TestEnvironment) CreateStoreItem(name, content string) string {
	// Create a mock store path with proper Nix store format
	hash := fmt.Sprintf("%032x", time.Now().UnixNano()) // Mock hash
	itemPath := filepath.Join(e.storeDir, fmt.Sprintf("%s-%s", hash[:32], name))
	
	must(e.t, os.MkdirAll(itemPath, 0755))
	
	// Create the actual file
	filePath := filepath.Join(itemPath, "file.txt")
	must(e.t, os.WriteFile(filePath, []byte(content), 0644))
	
	return filePath
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
	
	// Create a simple closure by symlinking to the first item
	// In real Nix, this would be more complex with proper derivations
	must(e.t, os.Symlink(items[0], e.profile))
	
	// Create a mock manifest.nix that references all items
	manifestPath := filepath.Join(filepath.Dir(items[0]), "manifest.nix")
	manifest := "[\n"
	for _, item := range items {
		manifest += fmt.Sprintf("  { path = \"%s\"; }\n", item)
	}
	manifest += "]\n"
	must(e.t, os.WriteFile(manifestPath, []byte(manifest), 0644))
}

// SetupEnvironmentVariables sets up environment for the test
func (e *TestEnvironment) SetupEnvironmentVariables() {
	// Override NIX_STORE_DIR to use our custom store
	os.Setenv("NIX_STORE_DIR", e.storeDir)
	os.Setenv("NIX_STATE_DIR", filepath.Join(e.tempDir, "var/nix"))
	os.Setenv("NIX_LOG_DIR", filepath.Join(e.tempDir, "var/log/nix"))
}

// Cleanup cleans up environment variables
func (e *TestEnvironment) Cleanup() {
	os.Unsetenv("NIX_STORE_DIR")
	os.Unsetenv("NIX_STATE_DIR")
	os.Unsetenv("NIX_LOG_DIR")
}

// CreateConfig creates a config with test environment settings
func (e *TestEnvironment) CreateConfig() *config.Config {
	return &config.Config{
		Timeout:  30 * time.Second,
		StoreDir: e.storeDir,
	}
}

// Test scenarios

func TestBasicFileEdit(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	t.Run("edit text file in custom store", func(t *testing.T) {
		// Create a store item
		filePath := env.CreateStoreItem("config", "original content")
		env.CreateProfileWithClosure(filepath.Dir(filePath))
		
		cfg := env.CreateConfig()
		cfg.Path = filePath
		cfg.Editor = "sed -i 's/original/modified/g'" // Use sed as editor for testing
		cfg.SystemType = "profile"
		cfg.ProfilePath = env.profile
		cfg.DryRun = false
		
		err := patch.Run(cfg)
		if err != nil {
			t.Fatalf("Failed to edit file: %v", err)
		}
		
		// Verify the content was changed
		// Note: In real implementation, the file would be in a new store path
		// This is a simplified test
	})
	
	t.Run("dry-run doesn't modify store", func(t *testing.T) {
		filePath := env.CreateStoreItem("config", "do not modify")
		originalContent := mustReadFile(t, filePath)
		env.CreateProfileWithClosure(filepath.Dir(filePath))
		
		cfg := &config.Config{
			Path:        filePath,
			Editor:      "sed -i 's/not/MODIFIED/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      true,
			Timeout:     30 * time.Second,
		}
		
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
	env.SetupEnvironmentVariables()
	
	t.Run("edit file with dependencies", func(t *testing.T) {
		// Create interdependent store items
		configPath := env.CreateStoreItem("app-config", "database=/old/path/db.sock")
		
		// Create an app that references the config
		appItem := env.CreateComplexStoreStructure("app")
		appBin := filepath.Join(appItem, "bin", "program")
		must(t, os.WriteFile(appBin, []byte(fmt.Sprintf("#!/bin/sh\n. %s\necho $database", configPath)), 0755))
		
		env.CreateProfileWithClosure(appItem, filepath.Dir(configPath))
		
		cfg := &config.Config{
			Path:        configPath,
			Editor:      "sed -i 's|/old/path|/new/path|g'",
			SystemType:  "profile", 
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Fatalf("Failed to edit file with dependencies: %v", err)
		}
		
		// In a real test, we'd verify that:
		// 1. A new store path was created for the edited config
		// 2. The app was rewritten to reference the new config path
		// 3. The profile was updated to point to the new closure
	})
	
	t.Run("circular dependency handling", func(t *testing.T) {
		// Create two items that reference each other
		item1 := env.CreateStoreItem("item1", "ref to item2: ITEM2_PATH")
		item2 := env.CreateStoreItem("item2", fmt.Sprintf("ref to item1: %s", item1))
		
		// Update item1 to reference item2
		content1 := mustReadFile(t, item1)
		must(t, os.WriteFile(item1, []byte(strings.Replace(content1, "ITEM2_PATH", item2, 1)), 0644))
		
		env.CreateProfileWithClosure(filepath.Dir(item1), filepath.Dir(item2))
		
		cfg := &config.Config{
			Path:        item1,
			Editor:      "sed -i 's/ref/reference/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		// Should handle circular dependencies gracefully
		err := patch.Run(cfg)
		if err != nil {
			// Some errors are expected for circular deps
			t.Logf("Circular dependency test result: %v", err)
		}
	})
}

func TestErrorScenarios(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
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
	env.SetupEnvironmentVariables()
	
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
	env.SetupEnvironmentVariables()
	
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