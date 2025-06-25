// Package store provides utilities for working with the Nix store
package store

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/constants"
)

// ExtractHash extracts the hash part from a nix store path
func ExtractHash(path string) string {
	// Check if it's a store path first
	if !IsStorePath(path) {
		return ""
	}

	// The storepath package doesn't expose the hash directly
	// Fall back to manual parsing
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		return ""
	}
	hashAndName := parts[3]
	hashParts := strings.Split(hashAndName, "-")
	if len(hashParts) > 0 {
		return hashParts[0]
	}
	return ""
}

// IsStorePath checks if a path is in the nix store
func IsStorePath(path string) bool {
	return IsStorePathWithDir(path, constants.DefaultNixStore)
}

// IsStorePathWithDir checks if a path is in the specified nix store
func IsStorePathWithDir(path, storeDir string) bool {
	// For simple validation, just check the prefix
	// go-nix's Validate is too strict for our use case
	return strings.HasPrefix(path, storeDir+"/")
}

// GenerateHash generates a random nix32 hash
func GenerateHash() (string, error) {
	randomBytes := make([]byte, 20)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Use go-nix's nixbase32 encoding
	return nixbase32.EncodeToString(randomBytes), nil
}

// GenerateContentHash generates a content-based nix32 hash from NAR data
// This mimics how Nix generates store path hashes from content
func GenerateContentHash(narData []byte) string {
	// Generate SHA256 hash of the NAR content
	hasher := sha256.New()
	hasher.Write(narData)
	hashBytes := hasher.Sum(nil)
	
	// Convert to nixbase32 (truncate to 20 bytes as Nix does)
	return nixbase32.EncodeToString(hashBytes[:20])
}


// ParseStorePath parses a nix store path and returns its components
func ParseStorePath(path string) (*StorePathInfo, error) {
	// First check if it's a store path at all
	if !IsStorePath(path) {
		return nil, fmt.Errorf("not a store path: %s", path)
	}

	// Extract components manually since go-nix doesn't expose them directly
	// Handle paths with subdirectories by only looking at the store entry
	basePath := path
	if idx := strings.Index(path[len("/nix/store/"):], "/"); idx >= 0 {
		basePath = path[:len("/nix/store/")+idx]
	}

	parts := strings.Split(basePath, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid store path")
	}

	hashAndName := parts[3]
	hashParts := strings.Split(hashAndName, "-")
	if len(hashParts) < 2 {
		return nil, fmt.Errorf("invalid store path format")
	}

	return &StorePathInfo{
		Hash: hashParts[0],
		Name: strings.Join(hashParts[1:], "-"),
	}, nil
}

// StorePathInfo contains parsed store path components
type StorePathInfo struct {
	Hash string
	Name string
}
