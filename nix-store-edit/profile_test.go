package main

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/archive"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/rewrite"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

func TestStep2Performance(t *testing.T) {
	// Enable CPU profiling
	cpuFile, err := os.Create("step2_cpu.prof")
	if err != nil {
		t.Fatal(err)
	}
	defer cpuFile.Close()
	
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		t.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	// Create a test store
	s := store.New("")
	
	// Find a real store path to test with
	storePaths, err := filepath.Glob("/nix/store/*-coreutils-*/bin/ls")
	if err != nil || len(storePaths) == 0 {
		t.Skip("No suitable test store path found")
	}
	testPath := storePaths[0]
	
	// Extract store path
	storeItem := filepath.Dir(filepath.Dir(testPath))
	
	t.Logf("Testing with store path: %s", storeItem)
	
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	
	destPath := filepath.Join(tmpDir, "test-contents")
	
	// Copy the file for testing
	if err := os.WriteFile(destPath, []byte("modified content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Measure Step 2: Import modified path
	start := time.Now()
	narData, expectedPath, err := archive.CreateWithStore(storeItem, destPath, s)
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)
	
	t.Logf("Step 2 (archive.Create) took: %v", duration)
	t.Logf("NAR size: %d bytes", len(narData))
	t.Logf("Expected path: %s", expectedPath)
	
	// Write memory profile
	memFile, err := os.Create("step2_mem.prof")
	if err != nil {
		t.Fatal(err)
	}
	defer memFile.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		t.Fatal(err)
	}
}

func TestStep3Performance(t *testing.T) {
	// Enable CPU profiling
	cpuFile, err := os.Create("step3_cpu.prof")
	if err != nil {
		t.Fatal(err)
	}
	defer cpuFile.Close()
	
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		t.Fatal(err)
	}
	defer pprof.StopCPUProfile()
	
	// Create a test store
	s := store.New("")
	
	// Find system closure (profile)
	profilePath := "/nix/var/nix/profiles/per-user/" + os.Getenv("USER") + "/profile"
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skip("No user profile found")
	}
	
	// Find a store path in the closure
	storePaths, err := filepath.Glob("/nix/store/*-coreutils-*/bin/ls")
	if err != nil || len(storePaths) == 0 {
		t.Skip("No suitable test store path found")
	}
	oldPath := filepath.Dir(filepath.Dir(storePaths[0]))
	
	// Create a fake new path (just for testing rewrite logic)
	newPath := "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-test"
	
	t.Logf("Testing rewrite from %s to %s", oldPath, newPath)
	t.Logf("System closure: %s", profilePath)
	
	// Build dependency chain
	_, _, affectedPaths, err := s.BuildDependencyChain(profilePath, oldPath)
	if err != nil {
		t.Logf("Warning: BuildDependencyChain failed: %v", err)
		affectedPaths = []string{} // Continue with empty affected paths
	}
	
	t.Logf("Affected paths: %d", len(affectedPaths))
	
	// Create rewrite engine
	engine := rewrite.NewEngineWithStore(s)
	engine.SetDryRun(true)
	
	var progressCount int
	engine.SetProgressCallback(func(current, total int, path string) {
		progressCount++
		if progressCount % 100 == 0 {
			t.Logf("Progress: %d/%d - %s", current, total, path)
		}
	})
	
	// Measure Step 3: Rewrite closure
	start := time.Now()
	newClosure, err := engine.RewriteClosure(profilePath, oldPath, newPath, affectedPaths)
	duration := time.Since(start)
	
	if err != nil {
		t.Logf("Warning: RewriteClosure failed: %v", err)
	}
	
	t.Logf("Step 3 (RewriteClosure) took: %v", duration)
	t.Logf("New closure: %s", newClosure)
	t.Logf("Total rewrites: %d", len(engine.GetPlannedRewrites()))
	
	// Write memory profile
	memFile, err := os.Create("step3_mem.prof")
	if err != nil {
		t.Fatal(err)
	}
	defer memFile.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		t.Fatal(err)
	}
}