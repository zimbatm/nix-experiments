// Package patch implements the nix-patch functionality
package patch

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/archive"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/rewrite"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/system"
)

// DependencyGraph represents the dependency relationships in a closure
type DependencyGraph struct {
	parents map[string]string // child -> parent mapping of store paths
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		parents: make(map[string]string),
	}
}

// Build builds a dependency graph from root
func (dg *DependencyGraph) Build(root string) error {
	visited := make(map[string]bool)
	queue := []string{root}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		log.Printf("path=%s", path)

		if visited[path] {
			continue
		}
		visited[path] = true

		// Get references
		refs, err := store.QueryReferences(path)
		if err != nil {
			// Path might not have references
			continue
		}

		for _, ref := range refs {
			dg.parents[ref] = path
			queue = append(queue, ref)
		}
	}

	return nil
}

// FindPathToRoot finds the dependency chain from target to root
func (dg *DependencyGraph) FindPathToRoot(target string) []string {
	var closureChain []string

	current := target
	for current != "" {
		closureChain = append(closureChain, current)
		current = dg.parents[current]
	}

	// Reverse the chain
	for i, j := 0, len(closureChain)-1; i < j; i, j = i+1, j-1 {
		closureChain[i], closureChain[j] = closureChain[j], closureChain[i]
	}

	log.Println("Dependency chain from root to target:")
	for _, p := range closureChain {
		fmt.Fprintln(os.Stderr, p)
	}

	return closureChain
}

// Run executes the patch operation on a Nix store path
func Run(cfg *config.Config) error {
	targetPath := cfg.Path

	// Check that the given path is in the /nix/store
	if !store.IsStorePath(targetPath) {
		// Try to resolve symlink
		resolvedPath, err := filepath.EvalSymlinks(targetPath)
		if err != nil || !store.IsStorePath(resolvedPath) {
			return fmt.Errorf("%s is not in the /nix/store", targetPath)
		}
		targetPath = resolvedPath
	}

	// Detect or use override for system type
	var sys system.System
	var err error
	if cfg.SystemType != "" {
		// Use the system type override
		sys, err = system.GetSystemByType(cfg.SystemType, cfg.ProfilePath)
		if err != nil {
			return fmt.Errorf("invalid system type: %w", err)
		}
		log.Printf("Using system type override: %s", sys.Type())
	} else {
		// Auto-detect system type
		sys, err = system.Detect()
		if err != nil {
			return fmt.Errorf("failed to detect system type: %w", err)
		}
		if sys.Type() == system.TypeProfile {
			log.Printf("No specific system detected, using user profile")
		} else {
			log.Printf("Detected system type: %s", sys.Type())
		}
	}

	// Get system closure
	systemClosure, err := sys.GetClosurePath()
	if err != nil {
		return fmt.Errorf("failed to get system closure: %w", err)
	}

	// Check that the given path is part of the system closure
	cmd := exec.Command("nix", "why-depends", systemClosure, targetPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("path is not part of system closure: %w", err)
	}

	// Check if targetPath is a file or directory
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("failed to stat target path: %w", err)
	}

	// Split the path into store_path and file_path
	parts := strings.SplitN(targetPath, "/", 5)
	var storePath, filePath string
	var drvName string
	
	if targetInfo.IsDir() || len(parts) > 4 {
		// If it's a directory or has subdirectories, use the standard logic
		storePath = strings.Join(parts[:4], "/")
		if len(parts) > 4 {
			filePath = parts[4]
		}
		// Extract derivation name
		nameWithHash := parts[3]
		nameParts := strings.Split(nameWithHash, "-")
		drvName = strings.Join(nameParts[1:], "-")
	} else {
		// If it's a file in the store root, the whole path is the store path
		storePath = targetPath
		filePath = ""
		// Extract derivation name from the file
		nameWithHash := parts[3]
		nameParts := strings.Split(nameWithHash, "-")
		// Remove the hash prefix to get the actual name
		fullName := strings.Join(nameParts[1:], "-")
		// For files, we need to use a directory name for extraction
		drvName = fullName + "-contents"
	}

	// Create workspace for editing
	workDir, err := os.MkdirTemp("", "nix-patch-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Extract store path to work directory using NAR
	destPath := filepath.Join(workDir, drvName)

	// Get NAR data from store
	narData, err := store.Dump(storePath)
	if err != nil {
		return fmt.Errorf("failed to dump store path: %w", err)
	}

	// Extract NAR to destination with writable permissions
	if err := extractNARFromStoreWritable(narData, destPath); err != nil {
		return fmt.Errorf("failed to extract store path: %w", err)
	}

	// Determine edit path
	editPath := destPath
	if filePath != "" {
		editPath = filepath.Join(destPath, filePath)
	}

	// Open in editor
	cmd = exec.Command(cfg.Editor, editPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Compare the work dir with the old one
	// We need to compare the right paths based on what was edited
	var compareOldPath, compareNewPath string
	if !targetInfo.IsDir() && filePath == "" {
		// For single files, compare the original file with the edited file
		compareOldPath = storePath
		compareNewPath = editPath
	} else {
		// For directories, compare the whole directories
		compareOldPath = storePath
		compareNewPath = destPath
	}
	
	cmd = exec.Command("diff", "--recursive", compareOldPath, compareNewPath)
	if err := cmd.Run(); err == nil {
		log.Println("ignoring as no changes were detected")
		return nil
	}

	// If dry-run mode, show diff and exit
	if cfg.DryRun {
		log.Println("DRY-RUN MODE: Showing changes that would be applied:")
		cmd = exec.Command("diff", "--recursive", "--unified", compareOldPath, compareNewPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // Ignore error as diff returns non-zero when files differ

		log.Println("\nDRY-RUN MODE: No changes were applied to the system.")
		return nil
	}

	// Build dependency graph
	dg := NewDependencyGraph()
	if err := dg.Build(systemClosure); err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Find dependency chain from store path to root
	closureChain := dg.FindPathToRoot(storePath)

	log.Printf("store_path=%s", storePath)
	log.Printf("closure_chain=%s", strings.Join(closureChain, " "))

	// Create rewrite engine
	engine := rewrite.NewEngine()

	// Set dry-run mode
	engine.SetDryRun(cfg.DryRun)

	// Set progress callback
	engine.SetProgressCallback(func(current, total int, path string) {
		log.Printf("Rewriting progress: %d/%d - %s", current, total, path)
	})

	// First, we need to import the modified path to the store
	var modifiedStorePath string
	if cfg.DryRun {
		// In dry-run mode, generate a fake path
		hash, err := store.GenerateHash()
		if err != nil {
			return fmt.Errorf("failed to generate hash: %w", err)
		}
		modifiedStorePath = fmt.Sprintf("/nix/store/%s-%s", hash, drvName)
		log.Printf("DRY-RUN: Would import modified path as: %s", modifiedStorePath)
	} else {
		log.Println("Importing modified path to store...")
		narData, err := archive.Create(storePath, destPath)
		if err != nil {
			return fmt.Errorf("failed to create archive: %w", err)
		}

		modifiedStorePath, err = store.Import(narData)
		if err != nil {
			return fmt.Errorf("failed to import modified path: %w", err)
		}
		log.Printf("Modified path imported as: %s", modifiedStorePath)
	}

	// Rewrite the entire closure
	log.Println("Starting closure rewrite...")
	newSystemClosure, err := engine.RewriteClosure(systemClosure, storePath, modifiedStorePath)
	if err != nil {
		return fmt.Errorf("failed to rewrite closure: %w", err)
	}

	log.Printf("New system closure: %s", newSystemClosure)

	if cfg.DryRun {
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
	} else {
		// Apply the new system closure
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
	}

	return nil
}

// extractNARFromStoreWritable extracts a NAR archive to the specified path with writable permissions
func extractNARFromStoreWritable(narData []byte, destPath string) error {
	// Create NAR reader
	nr, err := nar.NewReader(bytes.NewReader(narData))
	if err != nil {
		return fmt.Errorf("failed to create NAR reader: %w", err)
	}
	defer nr.Close()

	// Check if this is a single file or directory NAR by peeking at the first entry
	hdr, err := nr.Next()
	if err != nil {
		return fmt.Errorf("failed to read first NAR header: %w", err)
	}

	// If the first entry is "/" and it's a regular file, this is a single file NAR
	if hdr.Path == "/" && hdr.Type == nar.TypeRegular {
		// Extract single file directly to destPath
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Create file with writable permissions
		mode := os.FileMode(0644)
		if hdr.Executable {
			mode = 0755
		}

		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", destPath, err)
		}
		defer f.Close()

		// Copy content
		if _, err := io.Copy(f, nr); err != nil {
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}

		return nil
	}

	// Otherwise, it's a directory NAR
	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Process the first entry if it's not the root
	if hdr.Path != "/" {
		// Remove leading slash for joining with destPath
		relPath := strings.TrimPrefix(hdr.Path, "/")
		itemPath := filepath.Join(destPath, relPath)

		if err := processNAREntry(hdr, nr, itemPath); err != nil {
			return err
		}
	}

	// Process remaining entries
	for {
		hdr, err := nr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read NAR header: %w", err)
		}

		// Skip the root entry "/"
		if hdr.Path == "/" {
			continue
		}

		// Remove leading slash for joining with destPath
		relPath := strings.TrimPrefix(hdr.Path, "/")
		itemPath := filepath.Join(destPath, relPath)

		if err := processNAREntry(hdr, nr, itemPath); err != nil {
			return err
		}
	}

	return nil
}

// processNAREntry processes a single NAR entry
func processNAREntry(hdr *nar.Header, nr *nar.Reader, destPath string) error {
	switch hdr.Type {
	case nar.TypeDirectory:
		// Create directory
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", destPath, err)
		}

	case nar.TypeRegular:
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Create file with writable permissions
		mode := os.FileMode(0644)
		if hdr.Executable {
			mode = 0755
		}

		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", destPath, err)
		}

		// Copy content
		if _, err := io.Copy(f, nr); err != nil {
			f.Close()
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}
		f.Close()

	case nar.TypeSymlink:
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Remove existing symlink if any
		os.Remove(destPath)

		// Create symlink
		if err := os.Symlink(hdr.LinkTarget, destPath); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", destPath, hdr.LinkTarget, err)
		}

	default:
		return fmt.Errorf("unknown NAR entry type: %v", hdr.Type)
	}

	return nil
}
