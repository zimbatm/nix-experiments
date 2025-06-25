package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/archive"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

func TestContentBasedHashing(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	
	// Create store instance for tests
	s := store.New(env.tempDir) // Pass root directory, not store directory

	t.Run("archive.Create generates new hash for modified content", func(t *testing.T) {
		// Create a simple store item
		originalContent := "Hello, World!"
		item := env.CreateStoreItem("test-package", originalContent)
		
		// Create a temporary directory with modified content
		tempDir := t.TempDir()
		modifiedPath := filepath.Join(tempDir, "test-package")
		modifiedContent := "Hello, Modified World!"
		err := os.WriteFile(modifiedPath, []byte(modifiedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create modified file: %v", err)
		}

		// Create archive with original path but modified content
		narData, expectedPath, err := archive.CreateWithStore(item, modifiedPath, s)
		if err != nil {
			t.Fatalf("Failed to create archive: %v", err)
		}

		// Import the archive and check the path
		importedPath, err := s.Import(narData)
		if err != nil {
			t.Fatalf("Failed to import archive: %v", err)
		}

		// The imported path should match the expected path
		if importedPath != expectedPath {
			t.Errorf("Imported path doesn't match expected: got %s, expected %s", importedPath, expectedPath)
		}

		// The imported path should be different from the original
		if importedPath == item {
			t.Errorf("Expected new store path for modified content, got same path: %s", importedPath)
		}

		// Verify the imported path contains our package name
		if !strings.Contains(importedPath, "test-package") {
			t.Errorf("Expected imported path to contain 'test-package', got: %s", importedPath)
		}

		// Verify the content was imported correctly
		importedContent, err := os.ReadFile(importedPath)
		if err != nil {
			t.Fatalf("Failed to read imported file: %v", err)
		}
		if string(importedContent) != modifiedContent {
			t.Errorf("Content mismatch: expected %q, got %q", modifiedContent, string(importedContent))
		}
	})

	t.Run("archive.Create uses same hash for unchanged content", func(t *testing.T) {
		// Create a store item
		content := "Unchanged content"
		item := env.CreateStoreItem("unchanged-package", content)

		// Create archive with same path (no modifications)
		narData, expectedPath, err := archive.CreateWithStore(item, item, s)
		if err != nil {
			t.Fatalf("Failed to create archive: %v", err)
		}

		// Import should return the same path
		importedPath, err := s.Import(narData)
		if err != nil {
			t.Fatalf("Failed to import archive: %v", err)
		}

		if importedPath != expectedPath {
			t.Errorf("Imported path doesn't match expected: got %s, expected %s", importedPath, expectedPath)
		}
		
		if importedPath != item {
			t.Errorf("Expected same store path for unchanged content, got different path: original=%s, imported=%s", item, importedPath)
		}
	})

	t.Run("content-based hash is deterministic", func(t *testing.T) {
		// Create identical content in two different temp directories
		content := "Deterministic content test"
		
		tempDir1 := t.TempDir()
		path1 := filepath.Join(tempDir1, "deterministic-package")
		err := os.WriteFile(path1, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file 1: %v", err)
		}

		tempDir2 := t.TempDir()
		path2 := filepath.Join(tempDir2, "deterministic-package")
		err = os.WriteFile(path2, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file 2: %v", err)
		}

		// Create a dummy store path for the original
		dummyOriginal := filepath.Join(env.storeDir, "dummy-hash-deterministic-package")
		
		// Create archives from both paths
		narData1, expectedPath1, err := archive.CreateWithStore(dummyOriginal, path1, s)
		if err != nil {
			t.Fatalf("Failed to create archive 1: %v", err)
		}

		narData2, expectedPath2, err := archive.CreateWithStore(dummyOriginal, path2, s)
		if err != nil {
			t.Fatalf("Failed to create archive 2: %v", err)
		}
		
		// Expected paths should be the same (deterministic)
		if expectedPath1 != expectedPath2 {
			t.Errorf("Expected paths not deterministic: %s vs %s", expectedPath1, expectedPath2)
		}

		// Import both archives
		imported1, err := s.Import(narData1)
		if err != nil {
			t.Fatalf("Failed to import archive 1: %v", err)
		}

		imported2, err := s.Import(narData2)
		if err != nil {
			t.Fatalf("Failed to import archive 2: %v", err)
		}

		// They should have the same store path (deterministic hash)
		if imported1 != imported2 {
			t.Errorf("Expected deterministic hash, got different paths: %s vs %s", imported1, imported2)
		}
	})

	t.Run("GenerateContentHash produces valid nixbase32 hash", func(t *testing.T) {
		// Test the GenerateContentHash function directly
		testData := []byte("Test NAR content for hashing")
		hash := store.GenerateContentHash(testData)

		// Verify hash format (nixbase32 uses specific character set)
		validChars := "0123456789abcdfghijklmnpqrsvwxyz"
		for _, c := range hash {
			if !strings.ContainsRune(validChars, c) {
				t.Errorf("Invalid character in nixbase32 hash: %c", c)
			}
		}

		// Verify hash length (20 bytes in base32 = 32 characters)
		if len(hash) != 32 {
			t.Errorf("Expected hash length 32, got %d", len(hash))
		}

		// Verify determinism
		hash2 := store.GenerateContentHash(testData)
		if hash != hash2 {
			t.Errorf("Hash not deterministic: %s vs %s", hash, hash2)
		}

		// Verify different content produces different hash
		differentData := []byte("Different NAR content")
		differentHash := store.GenerateContentHash(differentData)
		if hash == differentHash {
			t.Errorf("Different content produced same hash")
		}
	})

	t.Run("archive with modified directories", func(t *testing.T) {
		// Create a directory structure
		nar := NewMockNARArchive()
		nar.AddFile("bin/app", []byte("#!/bin/sh\necho 'v1.0'"))
		nar.AddFile("lib/config.json", []byte(`{"version": "1.0"}`))
		item := env.CreateStoreItemWithNAR("dir-package", nar)

		// Create modified directory structure
		tempDir := t.TempDir()
		modifiedDir := filepath.Join(tempDir, "dir-package")
		
		// Create modified structure
		binDir := filepath.Join(modifiedDir, "bin")
		libDir := filepath.Join(modifiedDir, "lib")
		err := os.MkdirAll(binDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create bin dir: %v", err)
		}
		err = os.MkdirAll(libDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create lib dir: %v", err)
		}
		
		// Write modified files
		err = os.WriteFile(filepath.Join(binDir, "app"), []byte("#!/bin/sh\necho 'v2.0'"), 0755)
		if err != nil {
			t.Fatalf("Failed to write app: %v", err)
		}
		err = os.WriteFile(filepath.Join(libDir, "config.json"), []byte(`{"version": "2.0"}`), 0644)
		if err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		// Create archive with modifications
		narData, expectedPath, err := archive.CreateWithStore(item, modifiedDir, s)
		if err != nil {
			t.Fatalf("Failed to create archive: %v", err)
		}

		// Import and verify new path
		importedPath, err := s.Import(narData)
		if err != nil {
			t.Fatalf("Failed to import archive: %v", err)
		}

		if importedPath != expectedPath {
			t.Errorf("Imported path doesn't match expected: got %s, expected %s", importedPath, expectedPath)
		}
		
		if importedPath == item {
			t.Errorf("Expected new store path for modified directory, got same path: %s", importedPath)
		}

		// Verify the imported directory structure
		appContent, err := os.ReadFile(filepath.Join(importedPath, "bin", "app"))
		if err != nil {
			t.Fatalf("Failed to read imported app: %v", err)
		}
		if !bytes.Contains(appContent, []byte("v2.0")) {
			t.Errorf("Expected modified app content with v2.0, got: %s", appContent)
		}
	})
}