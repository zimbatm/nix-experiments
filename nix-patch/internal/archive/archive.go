// Package archive provides functionality to create and manipulate Nix archives
package archive

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/nix-community/go-nix/pkg/wire"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

// CreateWithStore creates a new Nix archive export format using the given store
// Returns the archive data and the store path that will be created
func CreateWithStore(oldPath, newPath string, s *store.Store) ([]byte, string, error) {
	if newPath == "" {
		newPath = oldPath
	}

	// Get references
	refs, err := s.QueryReferences(oldPath)
	if err != nil {
		return nil, "", err
	}

	// Get deriver (commented out in original, keeping empty)
	deriver := ""

	var buf bytes.Buffer

	// Write the export format header
	// Number of NAR files to add
	if err := wire.WriteUint64(&buf, uint64(config.ExportVersion)); err != nil {
		return nil, "", err
	}

	// Create NAR of the path
	narBuf := &bytes.Buffer{}
	if err := nar.DumpPath(narBuf, newPath); err != nil {
		return nil, "", fmt.Errorf("failed to create NAR: %w", err)
	}
	narData := narBuf.Bytes()

	// Write the NAR data
	buf.Write(narData)

	// Magic number "NIXIN"
	if err := wire.WriteUint64(&buf, uint64(config.NixinMagic)); err != nil {
		return nil, "", err
	}

	// Determine the store path to write
	storePath := oldPath
	if oldPath != newPath {
		// Content has been modified, generate a new store path
		newHash := store.GenerateContentHash(narData)
		
		// Extract derivation name from old path
		sp, err := s.ParseStorePath(oldPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse store path: %w", err)
		}
		
		storePath = fmt.Sprintf("%s/%s-%s", s.StoreDir, newHash, sp.Name)
	}

	// Path
	if err := wire.WriteString(&buf, storePath); err != nil {
		return nil, "", err
	}

	// Number of references
	if err := wire.WriteUint64(&buf, uint64(len(refs))); err != nil {
		return nil, "", err
	}

	// References
	for _, ref := range refs {
		if err := wire.WriteString(&buf, ref); err != nil {
			return nil, "", err
		}
	}

	// Deriver
	if err := wire.WriteString(&buf, deriver); err != nil {
		return nil, "", err
	}

	// Two zeros at the end
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, "", err
	}
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), storePath, nil
}

// CreateWithRewritesAndStore creates a new Nix archive with path rewrites applied using the given store
// Returns the archive data and the store path that will be created
func CreateWithRewritesAndStore(oldPath, pathToAdd string, rewrites map[string]string, s *store.Store) ([]byte, string, error) {
	// Create NAR from the pathToAdd first to generate content-based hash
	narBuf := &bytes.Buffer{}
	if err := nar.DumpPath(narBuf, pathToAdd); err != nil {
		return nil, "", fmt.Errorf("failed to create NAR: %w", err)
	}
	narData := narBuf.Bytes()

	// Generate content-based hash from NAR data
	newHash := store.GenerateContentHash(narData)

	// Extract derivation name
	sp, err := s.ParseStorePath(oldPath)
	if err != nil {
		return nil, "", fmt.Errorf("invalid store path: %w", err)
	}

	newPath := fmt.Sprintf("%s/%s-%s", s.StoreDir, newHash, sp.Name)

	// Record the rewrite
	rewrites[oldPath] = newPath

	// Get references and apply rewrites
	refs, err := s.QueryReferences(oldPath)
	if err != nil {
		return nil, "", err
	}

	rewrittenRefs := make([]string, 0, len(refs))
	for _, ref := range refs {
		if rewritten, ok := rewrites[ref]; ok {
			rewrittenRefs = append(rewrittenRefs, rewritten)
		} else {
			rewrittenRefs = append(rewrittenRefs, ref)
		}
	}

	var buf bytes.Buffer

	// Write export format header
	if err := wire.WriteUint64(&buf, uint64(config.ExportVersion)); err != nil {
		return nil, "", err
	}

	// Apply hash replacements to the NAR data we already generated
	if len(rewrites) > 0 {
		narStr := string(narData)
		for oldRef, newRef := range rewrites {
			oldHash := s.ExtractHash(oldRef)
			newHash := s.ExtractHash(newRef)
			if oldHash != "" && newHash != "" {
				narStr = strings.ReplaceAll(narStr, oldHash, newHash)
			}
		}
		narData = []byte(narStr)
	}

	buf.Write(narData)

	// Magic number "NIXIN"
	if err := wire.WriteUint64(&buf, uint64(config.NixinMagic)); err != nil {
		return nil, "", err
	}

	// Path
	if err := wire.WriteString(&buf, newPath); err != nil {
		return nil, "", err
	}

	// Number of references
	if err := wire.WriteUint64(&buf, uint64(len(rewrittenRefs))); err != nil {
		return nil, "", err
	}

	// References
	for _, ref := range rewrittenRefs {
		if err := wire.WriteString(&buf, ref); err != nil {
			return nil, "", err
		}
	}

	// Deriver (empty)
	if err := wire.WriteString(&buf, ""); err != nil {
		return nil, "", err
	}

	// Two zeros at the end
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, "", err
	}
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), newPath, nil
}
