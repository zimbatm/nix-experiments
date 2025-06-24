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
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/system"
)

// TestProfileSystemIntegration tests the profile system type specifically
func TestProfileSystemIntegration(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	t.Run("profile system detection", func(t *testing.T) {
		// Create a profile
		item := env.CreateStoreItem("test-package", "content")
		env.CreateProfileWithClosure(filepath.Dir(item))
		
		// Test system detection with custom store
		sys := &system.Profile{ProfilePath: env.profile}
		if !sys.IsAvailable() {
			t.Error("Profile system should be available")
		}
		
		closurePath, err := sys.GetClosurePath()
		if err != nil {
			t.Errorf("Failed to get closure path: %v", err)
		}
		
		if !strings.Contains(closurePath, env.storeDir) {
			t.Errorf("Closure path should be in custom store: %s", closurePath)
		}
	})
	
	t.Run("profile generations", func(t *testing.T) {
		// Create multiple generations
		var items []string
		for i := 0; i < 3; i++ {
			content := fmt.Sprintf("generation %d content", i+1)
			item := env.CreateStoreItem(fmt.Sprintf("gen-%d", i+1), content)
			items = append(items, filepath.Dir(item))
		}
		
		// Create profile generations
		for i, item := range items {
			genProfile := fmt.Sprintf("%s-%d-link", env.profile, i+1)
			must(t, os.Symlink(item, genProfile))
			
			// Update current profile
			must(t, os.Remove(env.profile))
			must(t, os.Symlink(item, env.profile))
			
			// Test editing in this generation
			cfg := &config.Config{
				Path:        filepath.Join(item, "file.txt"),
				Editor:      fmt.Sprintf("sed -i 's/generation %d/GENERATION %d/g'", i+1, i+1),
				SystemType:  "profile",
				ProfilePath: env.profile,
				DryRun:      false,
				Timeout:     30 * time.Second,
			}
			
			err := patch.Run(cfg)
			if err != nil {
				t.Logf("Generation %d edit result: %v", i+1, err)
			}
		}
	})
}

// TestProfileEdgeCases tests edge cases specific to profile handling
func TestProfileEdgeCases(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	t.Run("broken profile symlink", func(t *testing.T) {
		// Create a broken symlink
		brokenTarget := filepath.Join(env.tempDir, "nonexistent")
		must(t, os.Symlink(brokenTarget, env.profile))
		
		cfg := &config.Config{
			Path:        "/some/path",
			Editor:      "vim",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err == nil {
			t.Fatal("Expected error with broken profile symlink")
		}
	})
	
	t.Run("circular profile symlinks", func(t *testing.T) {
		// Create circular symlinks
		link1 := filepath.Join(env.profileDir, "link1")
		link2 := filepath.Join(env.profileDir, "link2")
		
		must(t, os.Symlink(link2, link1))
		must(t, os.Symlink(link1, link2))
		
		cfg := &config.Config{
			Path:        "/some/path",
			Editor:      "vim",
			SystemType:  "profile",
			ProfilePath: link1,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err == nil {
			t.Fatal("Expected error with circular symlinks")
		}
	})
	
	t.Run("deeply nested profile", func(t *testing.T) {
		// Create a chain of symlinks
		current := env.CreateStoreItem("base", "content")
		currentDir := filepath.Dir(current)
		
		for i := 0; i < 10; i++ {
			link := filepath.Join(env.profileDir, fmt.Sprintf("nested-%d", i))
			must(t, os.Symlink(currentDir, link))
			currentDir = link
		}
		
		// Final profile points to last link
		must(t, os.Symlink(currentDir, env.profile))
		
		cfg := &config.Config{
			Path:        current,
			Editor:      "sed -i 's/content/CONTENT/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Deeply nested profile result: %v", err)
		}
	})
	
	t.Run("profile with spaces in path", func(t *testing.T) {
		// Create profile with spaces
		spaceProfile := filepath.Join(env.profileDir, "my profile with spaces")
		item := env.CreateStoreItem("space-test", "content with spaces")
		must(t, os.Symlink(filepath.Dir(item), spaceProfile))
		
		cfg := &config.Config{
			Path:        item,
			Editor:      "sed -i 's/with spaces/WITHOUT_SPACES/g'",
			SystemType:  "profile",
			ProfilePath: spaceProfile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Profile with spaces result: %v", err)
		}
	})
}

// TestProfilePermissions tests permission-related scenarios
func TestProfilePermissions(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission tests when running as root")
	}
	
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	t.Run("read-only profile directory", func(t *testing.T) {
		// Create profile and make directory read-only
		item := env.CreateStoreItem("readonly-test", "content")
		env.CreateProfileWithClosure(filepath.Dir(item))
		
		// Make profile directory read-only
		must(t, os.Chmod(env.profileDir, 0555))
		defer os.Chmod(env.profileDir, 0755) // Restore for cleanup
		
		cfg := &config.Config{
			Path:        item,
			Editor:      "sed -i 's/content/CONTENT/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err == nil {
			t.Fatal("Expected error with read-only profile directory")
		}
	})
	
	t.Run("read-only store item", func(t *testing.T) {
		// Create item and make it read-only
		item := env.CreateStoreItem("readonly-item", "readonly content")
		itemDir := filepath.Dir(item)
		env.CreateProfileWithClosure(itemDir)
		
		// Make the item read-only
		must(t, os.Chmod(item, 0444))
		must(t, os.Chmod(itemDir, 0555))
		defer func() {
			os.Chmod(itemDir, 0755)
			os.Chmod(item, 0644)
		}()
		
		cfg := &config.Config{
			Path:        item,
			Editor:      "sed -i 's/readonly/READONLY/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		// Should handle read-only gracefully by creating new store item
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Read-only store item result: %v", err)
		}
	})
}

// TestProfileWithDifferentEditors tests various editor scenarios
func TestProfileWithDifferentEditors(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	env.SetupEnvironmentVariables()
	
	editors := []struct {
		name   string
		editor string
		setup  func(string) string // Returns the editor command
	}{
		{
			name: "sed in-place",
			editor: "sed -i 's/OLD/NEW/g'",
		},
		{
			name: "custom script editor",
			setup: func(tempDir string) string {
				scriptPath := filepath.Join(tempDir, "editor.sh")
				script := `#!/bin/sh
file="$1"
cat "$file" | sed 's/OLD/NEW/g' > "$file.tmp"
mv "$file.tmp" "$file"
`
				must(t, os.WriteFile(scriptPath, []byte(script), 0755))
				return scriptPath
			},
		},
		{
			name: "editor with arguments",
			editor: `sh -c "sed 's/OLD/NEW/g' '$1' > '$1.tmp' && mv '$1.tmp' '$1'" --`,
		},
	}
	
	for _, tt := range editors {
		t.Run(tt.name, func(t *testing.T) {
			item := env.CreateStoreItem(tt.name, "OLD content here")
			env.CreateProfileWithClosure(filepath.Dir(item))
			
			editor := tt.editor
			if tt.setup != nil {
				editor = tt.setup(env.tempDir)
			}
			
			cfg := &config.Config{
				Path:        item,
				Editor:      editor,
				SystemType:  "profile",
				ProfilePath: env.profile,
				DryRun:      false,
				Timeout:     30 * time.Second,
			}
			
			err := patch.Run(cfg)
			if err != nil {
				t.Errorf("Editor %s failed: %v", tt.name, err)
			}
		})
	}
}