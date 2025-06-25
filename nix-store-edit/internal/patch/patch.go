// Package patch implements the nix-patch functionality
package patch

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/archive"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/constants"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/editor"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/nar"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/rewrite"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/system"
)



// Run executes the patch operation on a Nix store path
func Run(cfg *config.Config) error {
	// Create store instance
	s := store.New(cfg.StoreRoot)
	// Step 1: Validate and resolve target path
	targetPath, err := validateTargetPath(cfg.Path, s)
	if err != nil {
		return err
	}

	// Step 2: Detect or override system type
	sys, err := detectOrOverrideSystem(cfg)
	if err != nil {
		return err
	}

	// Step 3: Get system closure
	systemClosure, err := sys.GetClosurePath()
	if err != nil {
		return fmt.Errorf("failed to get system closure: %w", err)
	}

	// Step 4: Parse path components
	pathComponents, err := parseStorePath(targetPath)
	if err != nil {
		return err
	}

	// Step 5: Create workspace and edit
	workspace, hasChanges, err := createAndEditWorkspace(cfg, pathComponents, s)
	if err != nil {
		return err
	}
	if !hasChanges {
		log.Println("ignoring as no changes were detected")
		return nil
	}
	defer workspace.cleanup()

	// Step 6: Show diff of changes
	showDiff(workspace.compareOldPath, workspace.compareNewPath)

	// Step 7: Build dependency graph and verify path is in closure
	log.Println("Building dependency graph...")
	_, closureChain, affectedPaths, err := s.BuildDependencyChain(systemClosure, pathComponents.storePath)
	if err != nil {
		return err
	}
	log.Println("Dependency graph built successfully")

	log.Printf("store_path=%s", pathComponents.storePath)
	log.Printf("closure_chain=%s", strings.Join(closureChain, " -> "))

	// Create rewrite engine
	engine := rewrite.NewEngineWithStore(s)

	// Set dry-run mode
	engine.SetDryRun(cfg.DryRun)

	// Set progress callback
	engine.SetProgressCallback(func(current, total int, path string) {
		log.Printf("Rewriting progress: %d/%d - %s", current, total, path)
	})

	// Step 8: Import modified path to store
	modifiedStorePath, err := importModifiedPath(cfg, pathComponents, workspace.destPath, s)
	if err != nil {
		return err
	}

	// Rewrite the entire closure
	log.Println("Starting closure rewrite...")
	newSystemClosure, err := engine.RewriteClosure(systemClosure, pathComponents.storePath, modifiedStorePath, affectedPaths)
	if err != nil {
		return fmt.Errorf("failed to rewrite closure: %w", err)
	}

	log.Printf("New system closure: %s", newSystemClosure)

	// Step 10: Apply or preview changes
	if cfg.DryRun {
		showDryRunSummary(engine, sys, pathComponents.storePath, systemClosure, newSystemClosure, cfg)
	} else {
		if err := applySystemClosure(sys, newSystemClosure, cfg); err != nil {
			return err
		}
	}

	return nil
}

// validateTargetPath ensures the given path is in the nix store
func validateTargetPath(path string, s *store.Store) (string, error) {
	if !s.IsStorePath(path) {
		// Try to resolve symlink
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil || !s.IsStorePath(resolvedPath) {
			return "", fmt.Errorf("%s is not in the %s", path, s.StoreDir)
		}
		return resolvedPath, nil
	}
	return path, nil
}

// detectOrOverrideSystem detects the system type or uses the override
func detectOrOverrideSystem(cfg *config.Config) (system.System, error) {
	if cfg.SystemType != "" {
		// Use the system type override
		sys, err := system.GetSystemByType(cfg.SystemType, cfg.ProfilePath)
		if err != nil {
			return nil, fmt.Errorf("invalid system type: %w", err)
		}
		log.Printf("Using system type override: %s", sys.Type())
		return sys, nil
	}

	// Auto-detect system type
	sys, err := system.Detect()
	if err != nil {
		return nil, fmt.Errorf("failed to detect system type: %w", err)
	}
	if sys.Type() == system.TypeProfile {
		log.Printf("No specific system detected, using user profile")
	} else {
		log.Printf("Detected system type: %s", sys.Type())
	}
	return sys, nil
}


// pathComponents represents the components of a store path
type pathComponents struct {
	storePath string
	filePath  string
	drvName   string
}

// parseStorePath splits a path into its store and file components
func parseStorePath(targetPath string) (*pathComponents, error) {
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat target path: %w", err)
	}

	parts := strings.SplitN(targetPath, "/", 5)
	pc := &pathComponents{}
	
	if targetInfo.IsDir() || len(parts) > constants.StorePathComponents {
		// If it's a directory or has subdirectories, use the standard logic
		pc.storePath = strings.Join(parts[:constants.StorePathComponents], "/")
		if len(parts) > constants.StorePathComponents {
			pc.filePath = parts[constants.StorePathComponents]
		}
		// Extract derivation name
		nameWithHash := parts[3]
		nameParts := strings.Split(nameWithHash, "-")
		pc.drvName = strings.Join(nameParts[1:], "-")
	} else {
		// If it's a file in the store root, the whole path is the store path
		pc.storePath = targetPath
		pc.filePath = ""
		// Extract derivation name from the file
		nameWithHash := parts[3]
		nameParts := strings.Split(nameWithHash, "-")
		// Remove the hash prefix to get the actual name
		fullName := strings.Join(nameParts[1:], "-")
		// For files, we need to use a directory name for extraction
		pc.drvName = fullName + "-contents"
	}

	return pc, nil
}

// workspace represents the editing workspace
type workspace struct {
	workDir         string
	destPath        string
	compareOldPath  string
	compareNewPath  string
}

func (w *workspace) cleanup() {
	if w.workDir != "" {
		if err := os.RemoveAll(w.workDir); err != nil {
			log.Printf("Failed to clean up work directory: %v", err)
		}
	}
}

// createAndEditWorkspace creates a temporary workspace and opens it in the editor
func createAndEditWorkspace(cfg *config.Config, pc *pathComponents, s *store.Store) (*workspace, bool, error) {
	// Create workspace for editing
	workDir, err := os.MkdirTemp("", constants.TempDirPrefix)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create temp dir: %w", err)
	}

	w := &workspace{
		workDir:  workDir,
		destPath: filepath.Join(workDir, pc.drvName),
	}

	// Get NAR data from store
	narData, err := s.Dump(pc.storePath)
	if err != nil {
		w.cleanup()
		return nil, false, fmt.Errorf("failed to dump store path: %w", err)
	}

	// Extract NAR to destination with writable permissions
	if err := nar.Extract(narData, w.destPath, nar.ExtractOptions{MakeWritable: true}); err != nil {
		w.cleanup()
		return nil, false, fmt.Errorf("failed to extract store path: %w", err)
	}

	// Determine edit path
	editPath := w.destPath
	if pc.filePath != "" {
		editPath = filepath.Join(w.destPath, pc.filePath)
	}

	// Open in editor
	if err := editor.Open(cfg.Editor, editPath); err != nil {
		w.cleanup()
		return nil, false, err
	}

	// Determine comparison paths
	targetInfo, _ := os.Stat(pc.storePath)
	if !targetInfo.IsDir() && pc.filePath == "" {
		// For single files, compare the original file with the edited file
		w.compareOldPath = pc.storePath
		w.compareNewPath = editPath
	} else {
		// For directories, compare the whole directories
		w.compareOldPath = pc.storePath
		w.compareNewPath = w.destPath
	}
	
	// Check if there are any changes
	diffCmd := exec.Command("diff", "--recursive", w.compareOldPath, w.compareNewPath)
	if err := diffCmd.Run(); err == nil {
		w.cleanup()
		return nil, false, nil
	}

	return w, true, nil
}

// showDiff displays the diff between old and new paths
func showDiff(oldPath, newPath string) {
	log.Println("Changes to be applied:")
	cmd := exec.Command("diff", "--recursive", "--unified", oldPath, newPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run() // Ignore error as diff returns non-zero when files differ
}


// importModifiedPath imports the modified path to the Nix store
func importModifiedPath(cfg *config.Config, pc *pathComponents, destPath string, s *store.Store) (string, error) {
	log.Println("Creating archive for modified path...")
	narData, expectedStorePath, err := archive.CreateWithStore(pc.storePath, destPath, s)
	if err != nil {
		return "", fmt.Errorf("failed to create archive: %w", err)
	}

	if cfg.DryRun {
		log.Printf("DRY-RUN: Would import modified path as: %s", expectedStorePath)
		return expectedStorePath, nil
	}

	log.Println("Importing modified path to store...")
	importedStorePath, err := s.Import(narData)
	if err != nil {
		return "", fmt.Errorf("failed to import modified path: %w", err)
	}
	
	// Verify the imported path matches what we expected
	if importedStorePath != expectedStorePath {
		log.Printf("Warning: imported path differs from expected: got %s, expected %s", importedStorePath, expectedStorePath)
	}
	
	log.Printf("Modified path imported as: %s", importedStorePath)
	return importedStorePath, nil
}

// showDryRunSummary displays a summary of what would be done in dry-run mode
func showDryRunSummary(engine *rewrite.Engine, sys system.System, storePath, systemClosure, newSystemClosure string, cfg *config.Config) {
	log.Println("\nDRY-RUN MODE: Preview of changes")
	log.Println("=================================")
	
	// Get all planned rewrites
	plannedRewrites := engine.GetPlannedRewrites()
	
	// Sort paths for consistent output
	var paths []string
	for oldPath := range plannedRewrites {
		paths = append(paths, oldPath)
	}
	sort.Strings(paths)
	
	// Show all paths that would be rewritten
	log.Printf("\nPaths that would be rewritten (%d total):", len(paths))
	for i, oldPath := range paths {
		newPath := plannedRewrites[oldPath]
		if i < 10 || oldPath == storePath || oldPath == systemClosure {
			log.Printf("  %s", oldPath)
			log.Printf("    -> %s", newPath)
		} else if i == 10 {
			log.Printf("  ... and %d more paths ...", len(paths)-10)
			break
		}
	}
	
	// Show the command that would be executed
	log.Println("\nCommand that would be executed:")
	if cfg.ActivationCommand != "" {
		log.Printf("  %s", cfg.ActivationCommand)
	} else {
		defaultCmd := sys.GetDefaultCommand(newSystemClosure)
		log.Printf("  %s", strings.Join(defaultCmd, " "))
	}
	
	log.Println("\nSystem information:")
	log.Printf("  System type: %s", sys.Type())
	log.Printf("  New closure: %s", newSystemClosure)
	
	log.Println("\nDRY-RUN: No changes were applied.")
}

// applySystemClosure applies the new system closure
func applySystemClosure(sys system.System, newSystemClosure string, cfg *config.Config) error {
	if cfg.ActivationCommand != "" {
		log.Printf("Applying new system closure with custom command: %s", cfg.ActivationCommand)
	} else {
		defaultCmd := sys.GetDefaultCommand(newSystemClosure)
		log.Printf("Applying new system closure with default command: %s", strings.Join(defaultCmd, " "))
	}
	if err := sys.ApplyClosure(newSystemClosure, cfg.ActivationCommand); err != nil {
		return fmt.Errorf("failed to apply new system closure: %w", err)
	}

	if cfg.ActivationCommand != "" {
		log.Println("Successfully applied changes!")
	} else {
		log.Println("Successfully applied changes! Use --activate with switch command to make permanent.")
	}
	return nil
}
