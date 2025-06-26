package nar

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
)

// ExtractOptions configures NAR extraction behavior
type ExtractOptions struct {
	// MakeWritable ensures all extracted files have writable permissions
	MakeWritable bool
	// PreserveMode attempts to preserve original file modes (when MakeWritable is false)
	PreserveMode bool
}

// Extract extracts a NAR archive to the specified destination path
func Extract(narData []byte, destPath string, opts ExtractOptions) error {
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
		return extractSingleFile(nr, hdr, destPath, opts)
	}

	// Otherwise, it's a directory NAR
	return extractDirectory(nr, hdr, destPath, opts)
}

// extractSingleFile extracts a single file NAR to destPath
func extractSingleFile(nr *nar.Reader, hdr *nar.Header, destPath string, opts ExtractOptions) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Determine file mode
	mode := getFileMode(hdr, opts)

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

// dirPermission tracks directories that need permission changes
type dirPermission struct {
	path string
	mode os.FileMode
}

// extractDirectory extracts a directory NAR to destPath
func extractDirectory(nr *nar.Reader, firstHdr *nar.Header, destPath string, opts ExtractOptions) error {
	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Track directories that need permission changes
	var dirPerms []dirPermission

	// Process the first entry if it's not the root
	if firstHdr.Path != "/" {
		if err := processEntry(firstHdr, nr, destPath, opts, &dirPerms); err != nil {
			return errors.Wrap(err, errors.ErrCodeNAR, "processFirstEntry")
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

		if err := processEntry(hdr, nr, destPath, opts, &dirPerms); err != nil {
			return errors.Wrap(err, errors.ErrCodeNAR, "processEntry")
		}
	}

	// Apply directory permissions after all files are extracted
	if opts.PreserveMode && !opts.MakeWritable {
		// Apply in reverse order to handle nested directories correctly
		for i := len(dirPerms) - 1; i >= 0; i-- {
			dp := dirPerms[i]
			if err := os.Chmod(dp.path, dp.mode); err != nil {
				return fmt.Errorf("failed to set directory permissions %s: %w", dp.path, err)
			}
		}
	}

	return nil
}

// processEntry processes a single NAR entry
func processEntry(hdr *nar.Header, nr *nar.Reader, basePath string, opts ExtractOptions, dirPerms *[]dirPermission) error {
	// Remove leading slash for joining with basePath
	relPath := strings.TrimPrefix(hdr.Path, "/")
	itemPath := filepath.Join(basePath, relPath)

	switch hdr.Type {
	case nar.TypeDirectory:
		// Create directory with write permissions first
		if err := os.MkdirAll(itemPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", itemPath, err)
		}

		// Track for permission changes later if needed
		if opts.PreserveMode && !opts.MakeWritable {
			*dirPerms = append(*dirPerms, dirPermission{
				path: itemPath,
				mode: hdr.FileInfo().Mode(),
			})
		}

	case nar.TypeRegular:
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(itemPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Determine file mode
		mode := getFileMode(hdr, opts)

		f, err := os.OpenFile(itemPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", itemPath, err)
		}

		// Copy content
		if _, err := io.Copy(f, nr); err != nil {
			f.Close()
			return fmt.Errorf("failed to write file %s: %w", itemPath, err)
		}
		f.Close()

	case nar.TypeSymlink:
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(itemPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Remove existing symlink if any
		os.Remove(itemPath)

		// Create symlink
		if err := os.Symlink(hdr.LinkTarget, itemPath); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", itemPath, hdr.LinkTarget, err)
		}

	default:
		return fmt.Errorf("unknown NAR entry type: %v", hdr.Type)
	}

	return nil
}

// getFileMode determines the file mode based on header and options
func getFileMode(hdr *nar.Header, opts ExtractOptions) os.FileMode {
	if opts.MakeWritable {
		if hdr.Executable {
			return 0755
		}
		return 0644
	}

	if opts.PreserveMode {
		return hdr.FileInfo().Mode()
	}

	// Default behavior
	if hdr.Executable {
		return 0555
	}
	return 0444
}
