// Package patch implements the nix-patch functionality
package patch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/archive"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/editor"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/logger"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/nar"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/rewrite"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/system"
)

// Run executes the patch operation on a Nix store path
func Run(cfg *config.Config) error {
	// Create logger
	log := logger.New(cfg.Verbose, cfg.DryRun)

	// Create store instance
	s := store.New(cfg.StoreRoot)
	// Step 1: Validate and resolve target path
	targetPath, err := validateTargetPath(cfg.Path, s)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeValidation, "validateTargetPath")
	}

	// Step 2: Detect or override system type
	sys, err := detectOrOverrideSystem(cfg, log)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "detectOrOverrideSystem")
	}

	// Step 3: Get system closure
	systemClosure, err := sys.GetClosurePath()
	if err != nil {
		return fmt.Errorf("failed to get system closure: %w", err)
	}

	// Show welcome header
	var systemTypeStr string
	if cfg.SystemType != "" {
		systemTypeStr = cfg.SystemType
	}
	log.Header(systemTypeStr, cfg.Editor, systemClosure, targetPath)

	// Step 4: Parse path components
	pathComponents, err := parseStorePath(targetPath, s)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeValidation, "parseStorePath")
	}

	// Step 5: Create workspace and edit
	log.NumberedStep(1, "Opening editor")
	workspace, hasChanges, err := createAndEditWorkspace(cfg, pathComponents, s)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeEditor, "createAndEditWorkspace")
	}
	if !hasChanges {
		log.Info("No changes detected")
		return nil
	}
	defer workspace.cleanup()

	// Step 6: Show diff of changes
	log.Step("Reviewing changes")
	showDiff(workspace.compareOldPath, workspace.compareNewPath, log)

	// Step 7: Build dependency graph and verify path is in closure
	_, _, affectedPaths, err := s.BuildDependencyChain(systemClosure, pathComponents.storePath)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeStore, "buildDependencyChain")
	}

	// Create rewrite engine
	engine := rewrite.NewEngineWithStore(s)

	// Set dry-run mode
	engine.SetDryRun(cfg.DryRun)

	// Set progress callback
	engine.SetProgressCallback(func(current, total int, path string) {
		log.Progress(current, total, path)
	})

	// Step 8: Import modified path to store
	log.NumberedStep(2, "Importing modified package")
	modifiedStorePath, err := importModifiedPath(cfg, pathComponents, workspace.destPath, s, log)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeStore, "importModifiedPath")
	}

	// Show the path mapping
	log.PathMapping(pathComponents.storePath, modifiedStorePath)

	// Rewrite the entire closure
	log.NumberedStep(3, "Updating system references")
	newSystemClosure, err := engine.RewriteClosure(systemClosure, pathComponents.storePath, modifiedStorePath, affectedPaths)
	if err != nil {
		return fmt.Errorf("failed to rewrite closure: %w", err)
	}

	// Step 10: Apply or preview changes
	if cfg.DryRun {
		showDryRunSummary(engine, sys, pathComponents.storePath, systemClosure, newSystemClosure, cfg, log)
	} else {
		if err := applySystemClosure(sys, newSystemClosure, cfg, log); err != nil {
			return errors.Wrap(err, errors.ErrCodeSystem, "applySystemClosure")
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
func detectOrOverrideSystem(cfg *config.Config, log *logger.Logger) (system.System, error) {
	if cfg.SystemType != "" {
		// Use the system type override
		sys, err := system.GetSystemByType(cfg.SystemType, cfg.ProfilePath, cfg.StoreRoot)
		if err != nil {
			return nil, fmt.Errorf("invalid system type: %w", err)
		}
		// System type override is shown in the header
		return sys, nil
	}

	// Auto-detect system type
	sys, err := system.Detect()
	if err != nil {
		return nil, fmt.Errorf("failed to detect system type: %w", err)
	}
	// System type detection is handled internally
	return sys, nil
}

// pathComponents represents the components of a store path
type pathComponents struct {
	storePath string
	filePath  string
	drvName   string
}

// parseStorePath splits a path into its store and file components
func parseStorePath(targetPath string, s *store.Store) (*pathComponents, error) {
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat target path: %w", err)
	}

	pc := &pathComponents{}

	// Find the store path boundary
	if !s.IsStorePath(targetPath) {
		return nil, fmt.Errorf("not a store path: %s", targetPath)
	}

	// Extract the store item path (everything up to and including hash-name)
	storeDir := s.StoreDir
	if !strings.HasPrefix(targetPath, storeDir+"/") {
		return nil, fmt.Errorf("path %s is not in store directory %s", targetPath, storeDir)
	}

	// Get the relative path from store directory
	relPath := targetPath[len(storeDir)+1:]

	// Find the first component (hash-name)
	firstSlash := strings.Index(relPath, "/")
	var storeItem string
	if firstSlash == -1 {
		// It's a direct store item
		storeItem = relPath
		pc.filePath = ""
	} else {
		// It has subpaths
		storeItem = relPath[:firstSlash]
		pc.filePath = relPath[firstSlash+1:]
	}

	pc.storePath = filepath.Join(storeDir, storeItem)

	// Extract derivation name from store item
	nameParts := strings.Split(storeItem, "-")
	if len(nameParts) > 1 {
		pc.drvName = strings.Join(nameParts[1:], "-")
	} else {
		pc.drvName = storeItem
	}

	// For single files, adjust the name
	if !targetInfo.IsDir() && pc.filePath == "" {
		pc.drvName = pc.drvName + "-contents"
	}

	return pc, nil
}

// workspace represents the editing workspace
type workspace struct {
	workDir        string
	destPath       string
	compareOldPath string
	compareNewPath string
}

func (w *workspace) cleanup() {
	if w.workDir != "" {
		_ = os.RemoveAll(w.workDir)
	}
}

// createAndEditWorkspace creates a temporary workspace and opens it in the editor
func createAndEditWorkspace(cfg *config.Config, pc *pathComponents, s *store.Store) (*workspace, bool, error) {
	// Create workspace for editing
	workDir, err := os.MkdirTemp("", "nix-patch-*")
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
func showDiff(oldPath, newPath string, log *logger.Logger) {
	cmd := exec.Command("diff", "--recursive", "--unified", oldPath, newPath)
	diffOutput, _ := cmd.Output() // Ignore error as diff returns non-zero when files differ
	if len(diffOutput) > 0 {
		log.ShowDiff(string(diffOutput))
	}
}

// importModifiedPath imports the modified path to the Nix store
func importModifiedPath(cfg *config.Config, pc *pathComponents, destPath string, s *store.Store, log *logger.Logger) (string, error) {
	narData, expectedStorePath, err := archive.CreateWithStore(pc.storePath, destPath, s)
	if err != nil {
		return "", fmt.Errorf("failed to create archive: %w", err)
	}

	if cfg.DryRun {
		return expectedStorePath, nil
	}

	importedStorePath, err := s.Import(narData)
	if err != nil {
		return "", fmt.Errorf("failed to import modified path: %w", err)
	}

	// Verify the imported path matches what we expected
	if importedStorePath != expectedStorePath {
		log.Warning(fmt.Sprintf("Imported path differs from expected: got %s, expected %s", importedStorePath, expectedStorePath))
	}

	return importedStorePath, nil
}

// showDryRunSummary displays a summary of what would be done in dry-run mode
func showDryRunSummary(engine *rewrite.Engine, sys system.System, storePath, systemClosure, newSystemClosure string, cfg *config.Config, log *logger.Logger) {
	log.Step("Preview of changes")

	// Get all planned rewrites
	plannedRewrites := engine.GetPlannedRewrites()

	// Sort paths for consistent output
	var paths []string
	for oldPath := range plannedRewrites {
		paths = append(paths, oldPath)
	}
	sort.Strings(paths)

	// Show all paths that would be rewritten
	var rewriteItems []string
	for i, oldPath := range paths {
		if log.IsVerbose() || i < 3 || oldPath == storePath || oldPath == systemClosure {
			newPath := plannedRewrites[oldPath]
			shortOld := filepath.Base(oldPath)
			shortNew := filepath.Base(newPath)
			rewriteItems = append(rewriteItems, fmt.Sprintf("%s → %s", shortOld, shortNew))
		}
	}
	log.ListItems("Paths to rewrite:", rewriteItems)

	// Show the command that would be executed
	log.Info("Command to execute:")
	if cfg.ActivationCommand != "" {
		log.Info(fmt.Sprintf("  %s", cfg.ActivationCommand))
	} else {
		defaultCmd := sys.GetDefaultCommand(newSystemClosure)
		log.Info(fmt.Sprintf("  %s", strings.Join(defaultCmd, " ")))
	}

	log.Info("No changes applied (dry-run mode)")
}

// applySystemClosure applies the new system closure
func applySystemClosure(sys system.System, newSystemClosure string, cfg *config.Config, log *logger.Logger) error {
	log.NumberedStep(4, "Applying changes")

	var cmdStr string
	if cfg.ActivationCommand != "" {
		cmdStr = cfg.ActivationCommand
	} else {
		defaultCmd := sys.GetDefaultCommand(newSystemClosure)
		cmdStr = strings.Join(defaultCmd, " ")
	}

	log.Command(cmdStr)

	if err := sys.ApplyClosure(newSystemClosure, cfg.ActivationCommand); err != nil {
		return fmt.Errorf("failed to apply new system closure: %w", err)
	}

	log.Success("Changes applied!")
	return nil
}
