// Package store provides utilities for working with the Nix store
package store

import (
	"crypto/rand"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nix-community/go-nix/pkg/nixbase32"
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
	// For simple validation, just check the prefix
	// go-nix's Validate is too strict for our use case
	return strings.HasPrefix(path, "/nix/store/")
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

// QueryReferences returns the references of a store path
func QueryReferences(path string) ([]string, error) {
	cmd := exec.Command("nix-store", "--query", "--references", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query references: %w", err)
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

// Dump creates a NAR dump of the given path
func Dump(path string) ([]byte, error) {
	cmd := exec.Command("nix-store", "--dump", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to dump store path: %w", err)
	}
	return output, nil
}

// Import imports a NAR archive into the store
func Import(narData []byte) (string, error) {
	cmd := exec.Command("nix-store", "--import")
	cmd.Stdin = strings.NewReader(string(narData))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to import to store: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
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
