package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNixStoreOperations tests basic Nix store operations with custom store
func TestNixStoreOperations(t *testing.T) {
	// Skip if Nix is not available
	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("nix-store not found in PATH")
	}
	
	t.Run("custom store initialization", func(t *testing.T) {
		tempDir := t.TempDir()
		storeDir := filepath.Join(tempDir, "nix", "store")
		
		// Create necessary directories
		must(t, os.MkdirAll(storeDir, 0755))
		must(t, os.MkdirAll(filepath.Join(tempDir, "nix", "var", "nix", "db"), 0755))
		
		// Try to initialize the store
		cmd := exec.Command("nix-store", "--store", tempDir, "--init")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("nix-store --init output: %s", output)
			t.Fatalf("Failed to initialize store: %v", err)
		}
		
		// Verify store was created
		dbPath := filepath.Join(tempDir, "nix", "var", "nix", "db", "db.sqlite")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("Store database was not created")
		}
	})
	
	t.Run("add file to custom store", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Initialize store
		cmd := exec.Command("nix-store", "--store", tempDir, "--init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to initialize store: %v", err)
		}
		
		// Create a test file
		testFile := filepath.Join(tempDir, "test.txt")
		must(t, os.WriteFile(testFile, []byte("test content"), 0644))
		
		// Add to store
		cmd = exec.Command("nix-store", "--store", tempDir, "--add", testFile)
		output, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				t.Logf("stderr: %s", exitErr.Stderr)
			}
			t.Fatalf("Failed to add file to store: %v", err)
		}
		
		storePath := strings.TrimSpace(string(output))
		t.Logf("Added file to store: %s", storePath)
		
		// The returned path has /nix/store prefix, but it's actually in our custom store
		// We need to adjust the path
		expectedPath := filepath.Join(tempDir, strings.TrimPrefix(storePath, "/"))
		t.Logf("Expected path in custom store: %s", expectedPath)
		
		// Verify it exists in the custom store
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			// List what's actually in the store
			storeDir := filepath.Join(tempDir, "nix", "store")
			entries, _ := os.ReadDir(storeDir)
			t.Logf("Store contents:")
			for _, e := range entries {
				t.Logf("  %s", e.Name())
			}
			t.Error("Store path does not exist in custom store")
		}
	})
	
	t.Run("query references", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Initialize store
		cmd := exec.Command("nix-store", "--store", tempDir, "--init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to initialize store: %v", err)
		}
		
		// Create and add a test file
		testFile := filepath.Join(tempDir, "test.txt")
		must(t, os.WriteFile(testFile, []byte("test content"), 0644))
		
		cmd = exec.Command("nix-store", "--store", tempDir, "--add", testFile)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}
		
		storePath := strings.TrimSpace(string(output))
		
		// Query references (should be empty for a simple file)
		cmd = exec.Command("nix-store", "--store", tempDir, "--query", "--references", storePath)
		output, err = cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				t.Logf("stderr: %s", exitErr.Stderr)
			}
			t.Fatalf("Failed to query references: %v", err)
		}
		
		t.Logf("References: %s", output)
	})
}