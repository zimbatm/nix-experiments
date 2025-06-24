// Package rewrite - pathrewriter.go implements the actual path rewriting logic
package rewrite

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/archive"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/constants"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/nar"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

// rewritePath performs the actual rewriting of a single store path
func (e *Engine) rewritePath(path string) (string, error) {
	// Check if already rewritten
	if newPath, ok := e.getRewrite(path); ok {
		log.Printf("Path already rewritten: %s -> %s", path, newPath)
		return newPath, nil
	}

	log.Printf("Rewriting path: %s", path)

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", constants.RewriteTempDirPrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("Failed to clean up temp directory: %v", err)
		}
	}()

	// Extract the path contents
	extractPath := filepath.Join(tempDir, "contents")
	if err := e.extractPath(path, extractPath); err != nil {
		return "", fmt.Errorf("failed to extract path: %w", err)
	}

	// Apply reference rewrites to the extracted contents
	if err := e.applyRewrites(extractPath); err != nil {
		return "", fmt.Errorf("failed to apply rewrites: %w", err)
	}

	// Create new archive with updated references
	newPath, err := e.createNewStorePath(path, extractPath)
	if err != nil {
		return "", fmt.Errorf("failed to create new store path: %w", err)
	}

	log.Printf("Successfully rewrote: %s -> %s", path, newPath)
	return newPath, nil
}

// extractPath extracts a store path to a directory
func (e *Engine) extractPath(storePath, destDir string) error {
	// Get NAR data from store
	narData, err := e.cache.GetNARData(storePath)
	if err != nil {
		return fmt.Errorf("failed to get NAR data: %w", err)
	}

	// Extract NAR to destination with writable permissions
	if err := nar.Extract(narData, destDir, nar.ExtractOptions{MakeWritable: true}); err != nil {
		return fmt.Errorf("failed to extract NAR: %w", err)
	}

	return nil
}

// applyRewrites applies all reference rewrites to extracted contents
func (e *Engine) applyRewrites(path string) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip symlinks (handle their targets instead)
		if info.Mode()&os.ModeSymlink != 0 {
			return e.rewriteSymlink(filePath)
		}

		// Rewrite regular files
		return e.rewriteFile(filePath)
	})
}

// rewriteFile rewrites references in a single file
func (e *Engine) rewriteFile(filePath string) error {
	// Read file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Apply all rewrites
	modified := false
	e.mu.RLock()
	for oldPath, newPath := range e.rewrites {
		oldHash := store.ExtractHash(oldPath)
		newHash := store.ExtractHash(newPath)

		if oldHash != "" && newHash != "" && bytes.Contains(data, []byte(oldHash)) {
			data = bytes.ReplaceAll(data, []byte(oldHash), []byte(newHash))
			modified = true
			log.Printf("Replaced %s with %s in %s", oldHash, newHash, filePath)
		}
	}
	e.mu.RUnlock()

	// Write back if modified
	if modified {
		// Get file info for permissions
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to stat file: %w", err)
		}

		if err := os.WriteFile(filePath, data, info.Mode().Perm()); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	return nil
}

// rewriteSymlink rewrites references in a symbolic link
func (e *Engine) rewriteSymlink(linkPath string) error {
	// Read link target
	target, err := os.Readlink(linkPath)
	if err != nil {
		return fmt.Errorf("failed to read symlink: %w", err)
	}

	// Apply rewrites to target
	modified := false
	newTarget := target

	e.mu.RLock()
	for oldPath, newPath := range e.rewrites {
		if strings.Contains(target, oldPath) {
			newTarget = strings.ReplaceAll(newTarget, oldPath, newPath)
			modified = true
		} else {
			// Also try with just the hash
			oldHash := store.ExtractHash(oldPath)
			newHash := store.ExtractHash(newPath)
			if oldHash != "" && newHash != "" && strings.Contains(target, oldHash) {
				newTarget = strings.ReplaceAll(newTarget, oldHash, newHash)
				modified = true
			}
		}
	}
	e.mu.RUnlock()

	// Update symlink if modified
	if modified {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("failed to remove old symlink: %w", err)
		}
		if err := os.Symlink(newTarget, linkPath); err != nil {
			return fmt.Errorf("failed to create new symlink: %w", err)
		}
		log.Printf("Updated symlink %s: %s -> %s", linkPath, target, newTarget)
	}

	return nil
}

// createNewStorePath creates a new store path from modified contents
func (e *Engine) createNewStorePath(originalPath, contentsPath string) (string, error) {
	// Get the references from the original path
	refs, err := e.cache.GetReferences(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to get references: %w", err)
	}

	// Update references based on our rewrites
	updatedRefs := make([]string, 0, len(refs))
	for _, ref := range refs {
		if newRef, ok := e.getRewrite(ref); ok {
			updatedRefs = append(updatedRefs, newRef)
		} else {
			updatedRefs = append(updatedRefs, ref)
		}
	}

	// In dry-run mode, generate a fake path
	if e.dryRun {
		hash, err := store.GenerateHash()
		if err != nil {
			return "", fmt.Errorf("failed to generate hash: %w", err)
		}

		// Extract name from original path
		pathInfo, err := store.ParseStorePath(originalPath)
		if err != nil {
			return "", fmt.Errorf("failed to parse original path: %w", err)
		}

		newPath := fmt.Sprintf("%s/%s-%s", constants.NixStore, hash, pathInfo.Name)
		log.Printf("DRY-RUN: Would create new store path: %s", newPath)
		return newPath, nil
	}

	// Create archive with the rewrite map
	archiveData, err := archive.CreateWithRewrites(originalPath, contentsPath, e.rewrites)
	if err != nil {
		return "", fmt.Errorf("failed to create archive: %w", err)
	}

	// Import to store
	newPath, err := store.Import(archiveData)
	if err != nil {
		return "", fmt.Errorf("failed to import to store: %w", err)
	}

	return newPath, nil
}

// FileInfo represents information about a file being rewritten
type FileInfo struct {
	Path      string
	IsSymlink bool
	Target    string
	Size      int64
	Mode      os.FileMode
}

