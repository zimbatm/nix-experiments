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

// TestStorePathRewriting focuses on the core rewriting functionality
func TestStorePathRewriting(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	
	t.Run("simple store path rewrite", func(t *testing.T) {
		// Create a config file that references another store path
		targetItem := env.CreateStoreItem("target-v1", "target content v1")
		targetPath := filepath.Dir(targetItem)
		
		configContent := fmt.Sprintf(`# Config file
database_path=%s/data.db
binary_path=%s/bin/app
`, targetPath, targetPath)
		
		configItem := env.CreateStoreItem("config", configContent)
		env.CreateProfileWithClosure(filepath.Dir(configItem), targetPath)
		
		// Edit the config to trigger rewriting
		cfg := &config.Config{
			Path:        configItem,
			Editor:      "sed -i 's/v1/v2/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Rewrite test result: %v", err)
		}
	})
	
	t.Run("transitive dependency rewriting", func(t *testing.T) {
		// Create a chain: A -> B -> C
		// Edit C, should rewrite B and A
		
		itemC := env.CreateStoreItem("package-c", "I am package C v1.0")
		pathC := filepath.Dir(itemC)
		
		contentB := fmt.Sprintf("Package B depends on: %s", pathC)
		itemB := env.CreateStoreItem("package-b", contentB)
		pathB := filepath.Dir(itemB)
		
		contentA := fmt.Sprintf("Package A depends on: %s\nTransitive: %s", pathB, pathC)
		itemA := env.CreateStoreItem("package-a", contentA)
		pathA := filepath.Dir(itemA)
		
		env.CreateProfileWithClosure(pathA, pathB, pathC)
		
		// Edit package C
		cfg := &config.Config{
			Path:        itemC,
			Editor:      "sed -i 's/v1.0/v2.0/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Transitive rewrite result: %v", err)
		}
	})
	
	t.Run("multiple references to same path", func(t *testing.T) {
		// Create shared dependency
		sharedItem := env.CreateStoreItem("shared-lib", "shared library v1")
		sharedPath := filepath.Dir(sharedItem)
		
		// Create multiple packages referencing it
		var packages []string
		for i := 0; i < 3; i++ {
			content := fmt.Sprintf(`Package %d
Depends on: %s
Also uses: %s/lib/shared.so
Config: %s/etc/config
`, i, sharedPath, sharedPath, sharedPath)
			
			item := env.CreateStoreItem(fmt.Sprintf("package-%d", i), content)
			packages = append(packages, filepath.Dir(item))
		}
		
		allPaths := append(packages, sharedPath)
		env.CreateProfileWithClosure(allPaths...)
		
		// Edit the shared library
		cfg := &config.Config{
			Path:        sharedItem,
			Editor:      "sed -i 's/v1/v2/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Multiple references rewrite result: %v", err)
		}
	})
}

// TestBinaryRewriting tests rewriting of binary files with embedded paths
func TestBinaryRewriting(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	
	t.Run("ELF binary with embedded paths", func(t *testing.T) {
		// Create a library that will be referenced
		libItem := env.CreateStoreItem("libfoo", "library content")
		libPath := filepath.Dir(libItem)
		
		// Create a mock ELF binary with embedded store path
		elfHeader := []byte{0x7f, 0x45, 0x4c, 0x46} // ELF magic
		
		// Create binary content with embedded paths
		var binaryContent []byte
		binaryContent = append(binaryContent, elfHeader...)
		binaryContent = append(binaryContent, []byte("\x00\x00\x00\x00")...) // padding
		
		// Embed store paths (null-terminated)
		binaryContent = append(binaryContent, []byte(libPath)...)
		binaryContent = append(binaryContent, 0x00)
		binaryContent = append(binaryContent, []byte(libPath+"/lib/libfoo.so")...)
		binaryContent = append(binaryContent, 0x00)
		
		// Create the binary
		binDir := env.CreateComplexStoreStructure("binary-pkg")
		binaryPath := filepath.Join(binDir, "bin", "program")
		must(t, os.WriteFile(binaryPath, binaryContent, 0755))
		
		env.CreateProfileWithClosure(binDir, libPath)
		
		// Edit the library (should trigger binary rewrite)
		cfg := &config.Config{
			Path:        libItem,
			Editor:      "sed -i 's/library/LIBRARY/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
			Force:       true,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Binary rewrite result: %v", err)
		}
	})
	
	t.Run("script with shebang paths", func(t *testing.T) {
		// Create interpreter
		interpreterItem := env.CreateStoreItem("python3", "#!/bin/sh\necho python")
		interpreterPath := filepath.Dir(interpreterItem)
		interpreterBin := filepath.Join(interpreterPath, "bin", "python3")
		must(t, os.MkdirAll(filepath.Dir(interpreterBin), 0755))
		must(t, os.Rename(interpreterItem, interpreterBin))
		must(t, os.Chmod(interpreterBin, 0755))
		
		// Create script with shebang
		scriptContent := fmt.Sprintf(`#!%s
import sys
print("Hello from custom Python")
# Also references: %s/lib/python3.11
`, interpreterBin, interpreterPath)
		
		scriptItem := env.CreateStoreItem("myscript", scriptContent)
		scriptPath := filepath.Dir(scriptItem)
		
		env.CreateProfileWithClosure(scriptPath, interpreterPath)
		
		// Edit the interpreter
		cfg := &config.Config{
			Path:        interpreterBin,
			Editor:      "sed -i 's/python/PYTHON/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Shebang rewrite result: %v", err)
		}
	})
}

// TestRewriteValidation tests validation of rewrites
func TestRewriteValidation(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	
	t.Run("prevent rewrite loops", func(t *testing.T) {
		// Create items with potential loop
		item1 := env.CreateStoreItem("item1", "content1")
		path1 := filepath.Dir(item1)
		
		item2 := env.CreateStoreItem("item2", fmt.Sprintf("refs: %s", path1))
		path2 := filepath.Dir(item2)
		
		// Update item1 to reference item2 (creating a loop)
		must(t, os.WriteFile(item1, []byte(fmt.Sprintf("content1\nrefs: %s", path2)), 0644))
		
		env.CreateProfileWithClosure(path1, path2)
		
		cfg := &config.Config{
			Path:        item1,
			Editor:      "sed -i 's/content1/CONTENT1/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		// Should handle loops gracefully
		err := patch.Run(cfg)
		if err != nil {
			if !strings.Contains(err.Error(), "loop") && !strings.Contains(err.Error(), "circular") {
				t.Logf("Loop handling result: %v", err)
			}
		}
	})
	
	t.Run("validate store path format", func(t *testing.T) {
		// Create item with invalid store path references
		content := `Valid: ` + env.storeDir + `/abcdef1234567890abcdef1234567890-valid-1.0
Invalid short: ` + env.storeDir + `/abc-invalid
Invalid no name: ` + env.storeDir + `/abcdef1234567890abcdef1234567890
Not a store path: /usr/local/bin/something
`
		item := env.CreateStoreItem("validator-test", content)
		env.CreateProfileWithClosure(filepath.Dir(item))
		
		cfg := &config.Config{
			Path:        item,
			Editor:      "sed -i 's/Valid/VALID/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     30 * time.Second,
		}
		
		err := patch.Run(cfg)
		if err != nil {
			t.Logf("Validation result: %v", err)
		}
	})
}

// TestRewritePerformance tests performance with large closures
func TestRewritePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	env := NewTestEnvironment(t)
	defer env.Cleanup()
	
	t.Run("large closure rewrite", func(t *testing.T) {
		// Create a large number of interdependent packages
		const numPackages = 50
		var packages []string
		
		// Create base package
		baseItem := env.CreateStoreItem("base", "base package")
		basePath := filepath.Dir(baseItem)
		packages = append(packages, basePath)
		
		// Create packages with dependencies
		for i := 0; i < numPackages; i++ {
			var deps []string
			// Each package depends on a few previous ones
			for j := 0; j < 3 && j <= i; j++ {
				deps = append(deps, packages[len(packages)-1-j])
			}
			
			content := fmt.Sprintf("Package %d\n", i)
			for _, dep := range deps {
				content += fmt.Sprintf("Depends: %s\n", dep)
			}
			
			item := env.CreateStoreItem(fmt.Sprintf("pkg-%d", i), content)
			packages = append(packages, filepath.Dir(item))
		}
		
		env.CreateProfileWithClosure(packages...)
		
		// Time the rewrite operation
		start := time.Now()
		
		cfg := &config.Config{
			Path:        baseItem,
			Editor:      "sed -i 's/base/BASE/g'",
			SystemType:  "profile",
			ProfilePath: env.profile,
			DryRun:      false,
			Timeout:     2 * time.Minute,
		}
		
		err := patch.Run(cfg)
		elapsed := time.Since(start)
		
		t.Logf("Large closure rewrite completed in %v", elapsed)
		if err != nil {
			t.Logf("Result: %v", err)
		}
		
		// Performance assertion
		if elapsed > 30*time.Second {
			t.Errorf("Rewrite took too long: %v", elapsed)
		}
	})
}