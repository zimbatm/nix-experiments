package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/patch"
)

// MockNARArchive creates a mock NAR archive for testing
type MockNARArchive struct {
	entries map[string][]byte
}

// NewMockNARArchive creates a new mock NAR archive
func NewMockNARArchive() *MockNARArchive {
	return &MockNARArchive{
		entries: make(map[string][]byte),
	}
}

// AddFile adds a file to the mock NAR
func (m *MockNARArchive) AddFile(path string, content []byte) {
	m.entries[path] = content
}

// CreateStoreItemWithNAR creates a proper store item with NAR structure
func (e *TestEnvironment) CreateStoreItemWithNAR(name string, nar *MockNARArchive) string {
	// Calculate a proper store hash
	hasher := sha256.New()
	for path, content := range nar.entries {
		hasher.Write([]byte(path))
		hasher.Write(content)
	}
	hash := hex.EncodeToString(hasher.Sum(nil))[:32]
	
	itemPath := filepath.Join(e.storeDir, fmt.Sprintf("%s-%s", hash, name))
	
	// Create the store item directory structure
	for path, content := range nar.entries {
		fullPath := filepath.Join(itemPath, path)
		must(e.t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		must(e.t, os.WriteFile(fullPath, content, 0644))
	}
	
	// Create a .nar.gz file (simplified)
	narPath := itemPath + ".nar.gz"
	if err := createMockNARFile(narPath, nar); err != nil {
		e.t.Fatalf("Failed to create NAR file: %v", err)
	}
	
	return itemPath
}

// createMockNARFile creates a mock compressed NAR file
func createMockNARFile(path string, nar *MockNARArchive) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()
	
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()
	
	// Write NAR magic and version (simplified)
	header := &tar.Header{
		Name: "nix-archive-1",
		Mode: 0644,
		Size: 0,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	
	// Write entries
	for path, content := range nar.entries {
		header := &tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tarWriter.Write(content); err != nil {
			return err
		}
	}
	
	return nil
}

func TestNARRewriting(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	t.Run("rewrite store paths in NAR", func(t *testing.T) {
		// Create first store item
		nar1 := NewMockNARArchive()
		nar1.AddFile("bin/script", []byte("#!/bin/sh\necho 'Hello from store'"))
		nar1.AddFile("etc/config", []byte("path=/nix/store/old-hash-dependency/bin/tool"))
		item1 := env.CreateStoreItemWithNAR("package-v1", nar1)
		
		// Create dependency that will be referenced
		nar2 := NewMockNARArchive()
		nar2.AddFile("bin/tool", []byte("#!/bin/sh\necho 'I am a tool'"))
		oldDep := env.CreateStoreItemWithNAR("dependency", nar2)
		
		// Create a package that references the dependency
		nar3 := NewMockNARArchive()
		nar3.AddFile("bin/app", []byte(fmt.Sprintf("#!/bin/sh\nexec %s/bin/tool \"$@\"", oldDep)))
		nar3.AddFile("share/config", []byte(fmt.Sprintf("toolpath=%s/bin/tool\nversion=1.0", oldDep)))
		item3 := env.CreateStoreItemWithNAR("app-package", nar3)
		
		env.CreateProfileWithClosure(item3, item1, oldDep)
		
		// Edit the config file
		configPath := filepath.Join(item1, "etc/config")
		cfg := &config.Config{
			Path:        configPath,
			Editor:      "sed -i 's/old-hash/new-hash/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			// This test demonstrates the rewriting scenario
			// In practice, it would create new store paths with updated references
			t.Logf("NAR rewrite test result: %v", err)
		}
	})
	
	t.Run("handle binary file rewrites", func(t *testing.T) {
		// Create a mock binary file with embedded store paths
		binaryContent := []byte{
			0x7f, 0x45, 0x4c, 0x46, // ELF header
			0x00, 0x00, 0x00, 0x00,
		}
		// Embed a store path in the binary
		storePath := "/nix/store/abcdef-package/lib/library.so"
		binaryContent = append(binaryContent, []byte(storePath)...)
		binaryContent = append(binaryContent, 0x00, 0x00, 0x00, 0x00)
		
		nar := NewMockNARArchive()
		nar.AddFile("bin/program", binaryContent)
		item := env.CreateStoreItemWithNAR("binary-package", nar)
		
		env.CreateProfileWithClosure(item)
		
		// Try to edit the binary (this should handle binary rewriting)
		binaryPath := filepath.Join(item, "bin/program")
		cfg := &config.Config{
			Path:        binaryPath,
			Editor:      "true", // No-op editor for binary
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
			Force:       true, // Force binary editing
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Binary rewrite test result: %v", err)
		}
	})
}

func TestStorePathValidation(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	tests := []struct {
		name      string
		storePath string
		valid     bool
	}{
		{
			name:      "valid store path",
			storePath: "/nix/store/abcdef1234567890abcdef1234567890-package-1.0",
			valid:     true,
		},
		{
			name:      "invalid hash length",
			storePath: "/nix/store/abc-package",
			valid:     false,
		},
		{
			name:      "missing package name",
			storePath: "/nix/store/abcdef1234567890abcdef1234567890",
			valid:     false,
		},
		{
			name:      "wrong prefix",
			storePath: "/usr/store/abcdef1234567890abcdef1234567890-package",
			valid:     false,
		},
		{
			name:      "custom store prefix",
			storePath: env.storeDir + "/abcdef1234567890abcdef1234567890-package",
			valid:     true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Adjust the path for custom store
			testPath := strings.Replace(tt.storePath, "/nix/store", env.storeDir, 1)
			
			// Create a dummy file at this path if it should be valid
			if tt.valid {
				dir := filepath.Dir(testPath)
				if filepath.Base(dir) != filepath.Base(env.storeDir) {
					must(t, os.MkdirAll(dir, 0755))
					must(t, os.WriteFile(testPath, []byte("content"), 0644))
				}
			}
			
			// Test validation logic would go here
			// This is a placeholder for the actual validation
			t.Logf("Testing store path: %s", testPath)
		})
	}
}

func TestMultipleProfileTypes(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	t.Run("user profile type", func(t *testing.T) {
		// Create a user-profile-like structure
		userProfile := filepath.Join(env.profileDir, "per-user", "testuser", "profile")
		must(t, os.MkdirAll(filepath.Dir(userProfile), 0755))
		
		item := env.CreateStoreItem("user-package", "user config")
		must(t, os.Symlink(filepath.Dir(item), userProfile))
		
		cfg := &config.Config{
			Path:        item,
			Editor:      "sed -i 's/user/USER/g'",
			SystemType:  "profile",
			ProfilePath: userProfile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("User profile test result: %v", err)
		}
	})
	
	t.Run("system profile type", func(t *testing.T) {
		// Create a system-profile-like structure
		systemProfile := filepath.Join(env.profileDir, "system")
		
		nar := NewMockNARArchive()
		nar.AddFile("etc/nixos/configuration.nix", []byte("{ config, pkgs, ... }: { }"))
		nar.AddFile("bin/switch-to-configuration", []byte("#!/bin/sh\necho 'Switching'"))
		item := env.CreateStoreItemWithNAR("nixos-system", nar)
		
		must(t, os.Symlink(item, systemProfile))
		
		cfg := &config.Config{
			Path:        filepath.Join(item, "etc/nixos/configuration.nix"),
			Editor:      "sed -i 's/{}/{ boot.loader.grub.enable = true; }/g'",
			SystemType:  "profile",
			ProfilePath: systemProfile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("System profile test result: %v", err)
		}
	})
}

func TestConcurrentEdits(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	t.Run("multiple users editing same closure", func(t *testing.T) {
		// Create a shared item
		item := env.CreateStoreItem("shared-config", "shared=true\nvalue=1")
		env.CreateProfileWithClosure(filepath.Dir(item))
		
		// Simulate concurrent edits
		errors := make(chan error, 2)
		
		go func() {
			cfg := &config.Config{
				Path:        item,
				Editor:      "sed -i 's/value=1/value=2/g'",
				SystemType:  "profile",
				ProfilePath: env.profile,
				DryRun:      false,
				Timeout:     30 * time.Second,
			}
			errors <- patch.Run(cfg)
		}()
		
		go func() {
			cfg := &config.Config{
				Path:        item,
				Editor:      "sed -i 's/shared=true/shared=false/g'",
				SystemType:  "profile",
				ProfilePath: env.profile,
				DryRun:      false,
				Timeout:     30 * time.Second,
			}
			errors <- patch.Run(cfg)
		}()
		
		// Collect results
		err1 := <-errors
		err2 := <-errors
		
		t.Logf("Concurrent edit 1: %v", err1)
		t.Logf("Concurrent edit 2: %v", err2)
		
		// At least one should succeed, but both might fail due to conflicts
	})
}