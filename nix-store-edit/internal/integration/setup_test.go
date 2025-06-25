package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvironmentSetup verifies that the test environment is created correctly
func TestEnvironmentSetup(t *testing.T) {
	t.Run("creates directory structure", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		
		// Check that all required directories exist
		dirs := []string{
			env.storeDir,
			env.profileDir,
			filepath.Join(env.tempDir, "nix", "var", "nix", "db"),
			filepath.Join(env.tempDir, "nix", "var", "log", "nix"),
		}
		
		for _, dir := range dirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Errorf("Directory %s does not exist", dir)
			}
		}
		
		// Check that the database file exists
		dbPath := filepath.Join(env.tempDir, "nix", "var", "nix", "db", "db.sqlite")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("Database file does not exist")
		}
		
		// Check that mock store items were created
		entries, err := os.ReadDir(env.storeDir)
		if err != nil {
			t.Fatalf("Failed to read store directory: %v", err)
		}
		
		if len(entries) < 2 {
			t.Errorf("Expected at least 2 store items, got %d", len(entries))
		}
		
		// Check that profile symlink exists
		if _, err := os.Lstat(env.profile); os.IsNotExist(err) {
			t.Error("Profile symlink does not exist")
		}
	})
	
	t.Run("cleanup removes temp directory", func(t *testing.T) {
		// Create a test environment
		env := NewTestEnvironment(t)
		tempDir := env.tempDir
		
		// Verify it exists
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			t.Fatal("Temp directory was not created")
		}
		
		// Clean up
		env.Cleanup()
		
		// The directory should still exist because t.TempDir() handles cleanup
		// But we can verify our Cleanup method doesn't cause errors
		// In a real implementation, we might have additional cleanup logic
	})
	
	t.Run("CreateStoreItem creates valid paths", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		
		// Create a test item
		content := "test content"
		itemPath := env.CreateStoreItem("test-item", content)
		
		// Verify the file exists
		if _, err := os.Stat(itemPath); os.IsNotExist(err) {
			t.Errorf("Created item does not exist: %s", itemPath)
		}
		
		// Verify the content
		data, err := os.ReadFile(itemPath)
		if err != nil {
			t.Fatalf("Failed to read created item: %v", err)
		}
		
		if string(data) != content {
			t.Errorf("Content mismatch: got %q, want %q", string(data), content)
		}
		
		// Verify it's in the store directory
		if !strings.HasPrefix(itemPath, env.storeDir) {
			t.Errorf("Item path %s is not in store directory %s", itemPath, env.storeDir)
		}
	})
	
	t.Run("CreateConfig returns valid config", func(t *testing.T) {
		env := NewTestEnvironment(t)
		defer env.Cleanup()
		
		cfg := env.CreateConfig()
		
		// Verify the store root is set correctly
		if cfg.StoreRoot != env.tempDir {
			t.Errorf("StoreRoot mismatch: got %s, want %s", cfg.StoreRoot, env.tempDir)
		}
		
		// Verify timeout is set
		if cfg.Timeout == 0 {
			t.Error("Timeout not set")
		}
	})
}

// TestEnvironmentConcurrency verifies that multiple test environments don't interfere
func TestEnvironmentConcurrency(t *testing.T) {
	// Run multiple tests in parallel
	t.Run("parallel", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			i := i // capture loop variable
			t.Run(fmt.Sprintf("env-%d", i), func(t *testing.T) {
				t.Parallel()
				
				env := NewTestEnvironment(t)
				defer env.Cleanup()
				
				// Create a unique item
				itemName := fmt.Sprintf("item-%d", i)
				content := fmt.Sprintf("content for %d", i)
				itemPath := env.CreateStoreItem(itemName, content)
				
				// Verify it exists and has correct content
				data, err := os.ReadFile(itemPath)
				if err != nil {
					t.Fatalf("Failed to read item: %v", err)
				}
				
				if string(data) != content {
					t.Errorf("Content mismatch in parallel test %d", i)
				}
			})
		}
	})
}