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

// FindPathToRoot finds the path from target to root
func (dg *DependencyGraph) FindPathToRoot(target string) []string {
	var pathChain []string

	current := target
	for current != "" {
		pathChain = append(pathChain, current)
		current = dg.parents[current]
	}

	// Reverse the chain
	for i, j := 0, len(pathChain)-1; i < j; i, j = i+1, j-1 {
		pathChain[i], pathChain[j] = pathChain[j], pathChain[i]
	}

	log.Println("Path from Root to target found:")
	for _, p := range pathChain {
		fmt.Fprintln(os.Stderr, p)
	}

	return pathChain
}

// Run executes the patch operation on a Nix store path
func Run(cfg *config.Config) error {
	targetPath := cfg.Path

	// Check that the given path is in the /nix/store
	if !store.IsStorePath(path) {
		// Try to resolve symlink
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil || !store.IsStorePath(resolvedPath) {
			return fmt.Errorf("%s is not in the /nix/store", path)
		}
		path = resolvedPath
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
	cmd := exec.Command("nix", "why-depends", systemClosure, path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("path is not part of system closure: %w", err)
	}

	// Split the path into store_path and file_path
	parts := strings.SplitN(path, "/", 5)
	storePath := strings.Join(parts[:4], "/")
	filePath := ""
	if len(parts) > 4 {
		filePath = parts[4]
	}

	// Extract derivation name
	nameWithHash := parts[3]
	nameParts := strings.Split(nameWithHash, "-")
	drvName := strings.Join(nameParts[1:], "-")

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
	cmd = exec.Command("diff", "--recursive", storePath, destPath)
	if err := cmd.Run(); err == nil {
		log.Println("ignoring as no changes were detected")
		return nil
	}

	// If dry-run mode, show diff and exit
	if cfg.DryRun {
		log.Println("DRY-RUN MODE: Showing changes that would be applied:")
		cmd = exec.Command("diff", "--recursive", "--unified", storePath, destPath)
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

	// Find path from store path to root
	pathChain := dg.FindPathToRoot(storePath)

	log.Printf("store_path=%s", storePath)
	log.Printf("path_chain=%s", strings.Join(pathChain, " "))

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
		log.Println("DRY-RUN: Would apply new closure")
		log.Printf("  System type: %s", sys.Type())
		log.Printf("  New closure: %s", newSystemClosure)
		log.Println("\nDRY-RUN: Summary of changes:")
		log.Printf("  - Modified path: %s -> %s", storePath, modifiedStorePath)
		log.Printf("  - New system closure: %s", newSystemClosure)
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

// extractNARFromStoreWritable extracts a NAR archive to the specified directory with writable permissions
func extractNARFromStoreWritable(narData []byte, destDir string) error {
	// Create NAR reader
	nr, err := nar.NewReader(bytes.NewReader(narData))
	if err != nil {
		return fmt.Errorf("failed to create NAR reader: %w", err)
	}
	defer nr.Close()

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Process each entry in the NAR
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

		// Remove leading slash for joining with destDir
		relPath := strings.TrimPrefix(hdr.Path, "/")
		destPath := filepath.Join(destDir, relPath)

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
	}

	return nil
}
