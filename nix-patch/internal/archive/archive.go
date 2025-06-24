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

// Create creates a new Nix archive export format with the given path
// This format is what nix-store --export produces
func Create(oldPath, newPath string) ([]byte, error) {
	if newPath == "" {
		newPath = oldPath
	}

	// Get references
	refs, err := store.QueryReferences(oldPath)
	if err != nil {
		return nil, err
	}

	// Get deriver (commented out in original, keeping empty)
	deriver := ""

	var buf bytes.Buffer

	// Write the export format header
	// Number of NAR files to add
	if err := wire.WriteUint64(&buf, uint64(config.ExportVersion)); err != nil {
		return nil, err
	}

	// Create NAR of the path
	narBuf := &bytes.Buffer{}
	if err := nar.DumpPath(narBuf, newPath); err != nil {
		return nil, fmt.Errorf("failed to create NAR: %w", err)
	}

	// Write the NAR data
	buf.Write(narBuf.Bytes())

	// Magic number "NIXIN"
	if err := wire.WriteUint64(&buf, uint64(config.NixinMagic)); err != nil {
		return nil, err
	}

	// Path
	if err := wire.WriteString(&buf, oldPath); err != nil {
		return nil, err
	}

	// Number of references
	if err := wire.WriteUint64(&buf, uint64(len(refs))); err != nil {
		return nil, err
	}

	// References
	for _, ref := range refs {
		if err := wire.WriteString(&buf, ref); err != nil {
			return nil, err
		}
	}

	// Deriver
	if err := wire.WriteString(&buf, deriver); err != nil {
		return nil, err
	}

	// Two zeros at the end
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, err
	}
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// CreateWithRewrites creates a new Nix archive with path rewrites applied
func CreateWithRewrites(oldPath, pathToAdd string, rewrites map[string]string) ([]byte, error) {
	// Generate new hash and path
	newHash, err := store.GenerateHash()
	if err != nil {
		return nil, err
	}

	// Extract derivation name
	sp, err := store.ParseStorePath(oldPath)
	if err != nil {
		return nil, fmt.Errorf("invalid store path: %w", err)
	}

	newPath := fmt.Sprintf("/nix/store/%s-%s", newHash, sp.Name)

	// Record the rewrite
	rewrites[oldPath] = newPath

	// Get references and apply rewrites
	refs, err := store.QueryReferences(oldPath)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	// For rewriting, we need to create a temporary directory with rewrites applied
	// This is a limitation - we can't easily rewrite inside a NAR stream
	// So we'll use the original approach of dumping and rewriting
	narData, err := store.Dump(pathToAdd)
	if err != nil {
		return nil, err
	}

	// Apply hash replacements
	if len(rewrites) > 0 {
		narStr := string(narData)
		for oldRef, newRef := range rewrites {
			oldHash := store.ExtractHash(oldRef)
			newHash := store.ExtractHash(newRef)
			if oldHash != "" && newHash != "" {
				narStr = strings.ReplaceAll(narStr, oldHash, newHash)
			}
		}
		narData = []byte(narStr)
	}

	buf.Write(narData)

	// Magic number "NIXIN"
	if err := wire.WriteUint64(&buf, uint64(config.NixinMagic)); err != nil {
		return nil, err
	}

	// Path
	if err := wire.WriteString(&buf, newPath); err != nil {
		return nil, err
	}

	// Number of references
	if err := wire.WriteUint64(&buf, uint64(len(rewrittenRefs))); err != nil {
		return nil, err
	}

	// References
	for _, ref := range rewrittenRefs {
		if err := wire.WriteString(&buf, ref); err != nil {
			return nil, err
		}
	}

	// Deriver (empty)
	if err := wire.WriteString(&buf, ""); err != nil {
		return nil, err
	}

	// Two zeros at the end
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, err
	}
	if err := wire.WriteUint64(&buf, 0); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
