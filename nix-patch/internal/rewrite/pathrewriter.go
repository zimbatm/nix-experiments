// Package rewrite - pathrewriter.go implements the actual path rewriting logic
package rewrite

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/archive"
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
	tempDir, err := os.MkdirTemp("", "nix-patch-rewrite-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

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
	if err := extractNARWritable(bytes.NewReader(narData), destDir); err != nil {
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

		newPath := fmt.Sprintf("/nix/store/%s-%s", hash, pathInfo.Name)
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

// extractNARWritable extracts a NAR archive to the specified directory with writable permissions
func extractNARWritable(narReader io.Reader, destDir string) error {
	return extractNARWithMode(narReader, destDir, true)
}

// extractNAR extracts a NAR archive to the specified directory preserving original permissions
func extractNAR(narReader io.Reader, destDir string) error {
	return extractNARWithMode(narReader, destDir, false)
}

// extractNARWithMode extracts a NAR archive with optional write permission adjustment
func extractNARWithMode(narReader io.Reader, destDir string, makeWritable bool) error {
	// Create NAR reader
	nr, err := nar.NewReader(narReader)
	if err != nil {
		return fmt.Errorf("failed to create NAR reader: %w", err)
	}
	defer nr.Close()

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
			// Create directory with writable permissions
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}

		case nar.TypeRegular:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Determine file mode
			mode := os.FileMode(0644)
			if hdr.Executable {
				mode = 0755
			}

			// If makeWritable is true, ensure owner write permission
			if makeWritable {
				mode |= 0200
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
