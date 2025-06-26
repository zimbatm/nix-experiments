package store

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
)

// Store represents a Nix store instance with its configuration
type Store struct {
	// RootDir is the root directory for the Nix store (empty for default /nix)
	RootDir string
	
	// StoreDir is the actual store directory (rootDir + /nix/store)
	StoreDir string
}

// StorePathInfo contains parsed store path components
type StorePathInfo struct {
	Hash string
	Name string
}

// New creates a new Store instance with the given root directory
// If rootDir is empty, it uses the default /nix paths
func New(rootDir string) *Store {
	if rootDir == "" {
		return &Store{
			RootDir:  "",
			StoreDir: "/nix/store",
		}
	}
	
	return &Store{
		RootDir:  rootDir,
		StoreDir: rootDir + "/nix/store",
	}
}

// execNix executes a nix command with proper error handling
func (s *Store) execNix(args ...string) ([]byte, error) {
	// Add --store flag if using custom root
	if s.RootDir != "" {
		args = append([]string{"--store", s.RootDir}, args...)
	}
	
	cmd := exec.Command("nix", args...)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("nix %s failed: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	
	return stdout.Bytes(), nil
}

// execNixStore executes a nix-store command
func (s *Store) execNixStore(args ...string) ([]byte, error) {
	// Add --store flag if using custom root
	if s.RootDir != "" {
		args = append([]string{"--store", s.RootDir}, args...)
	}
	
	cmd := exec.Command("nix-store", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nix-store %s failed: %w\nOutput: %s", strings.Join(args, " "), err, string(output))
	}
	return output, nil
}

// QueryReferences returns the references of a store path
func (s *Store) QueryReferences(path string) ([]string, error) {
	// Convert to standard path for nix-store command
	standardPath := s.toStandardPath(path)
	output, err := s.execNixStore("--query", "--references", standardPath)
	if err != nil {
		return nil, fmt.Errorf("failed to query references: %w", err)
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

// Dump creates a NAR dump of the given path
func (s *Store) Dump(path string) ([]byte, error) {
	// For dump, we need the actual filesystem path, not the standard path
	output, err := s.execNixStore("--dump", path)
	if err != nil {
		return nil, fmt.Errorf("failed to dump store path: %w", err)
	}
	return output, nil
}

// Import imports a NAR archive into the store
func (s *Store) Import(narData []byte) (string, error) {
	args := []string{"--import"}
	
	// Add --store flag if using custom root
	if s.RootDir != "" {
		args = append([]string{"--store", s.RootDir}, args...)
	}
	
	cmd := exec.Command("nix-store", args...)
	cmd.Stdin = bytes.NewReader(narData)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to import to store: %w\nstderr: %s", err, stderr.String())
	}
	
	// Get the standard path returned by nix-store
	standardPath := strings.TrimSpace(stdout.String())
	
	// Convert to custom store path if using custom root
	if s.RootDir != "" && strings.HasPrefix(standardPath, "/nix/store/") {
		relativePath := strings.TrimPrefix(standardPath, "/nix/store/")
		customPath := filepath.Join(s.StoreDir, relativePath)
		return customPath, nil
	}
	
	return standardPath, nil
}

// IsStorePath checks if a path is in this store
func (s *Store) IsStorePath(path string) bool {
	return strings.HasPrefix(path, s.StoreDir+"/")
}

// ExtractHash extracts the hash part from a nix store path
func (s *Store) ExtractHash(path string) string {
	// Check if it's a store path first
	if !s.IsStorePath(path) {
		return ""
	}

	// Extract the part after the store directory
	relPath := path[len(s.StoreDir)+1:]
	
	// Get the first component (hash-name)
	if idx := strings.Index(relPath, "/"); idx >= 0 {
		relPath = relPath[:idx]
	}
	
	// Extract hash from hash-name
	hashParts := strings.Split(relPath, "-")
	if len(hashParts) > 0 {
		return hashParts[0]
	}
	return ""
}

// ParseStorePath parses a nix store path and returns its components
func (s *Store) ParseStorePath(path string) (*StorePathInfo, error) {
	// First check if it's a store path at all
	if !s.IsStorePath(path) {
		return nil, fmt.Errorf("not a store path: %s", path)
	}

	// Extract the part after the store directory
	relPath := path[len(s.StoreDir)+1:]
	
	// Get the first component (hash-name)
	if idx := strings.Index(relPath, "/"); idx >= 0 {
		relPath = relPath[:idx]
	}
	
	// Extract hash and name
	hashParts := strings.Split(relPath, "-")
	if len(hashParts) < 2 {
		return nil, fmt.Errorf("invalid store path format: %s", path)
	}

	return &StorePathInfo{
		Hash: hashParts[0],
		Name: strings.Join(hashParts[1:], "-"),
	}, nil
}

// IsTrustedUser checks if the current user is a trusted Nix user
func (s *Store) IsTrustedUser() (bool, error) {
	info, err := s.GetStoreInfo()
	if err != nil {
		return false, errors.Wrap(err, errors.ErrCodeStore, "getTrustedUserStatus")
	}
	return info.Trusted == 1, nil
}

// WhyDepends runs nix why-depends to analyze dependencies
func (s *Store) WhyDepends(from, to string, all bool) ([]byte, error) {
	args := []string{"why-depends"}
	if all {
		args = append(args, "--all")
	}
	
	// Convert custom store paths to standard paths for nix commands
	fromPath := s.toStandardPath(from)
	toPath := s.toStandardPath(to)
	
	args = append(args, fromPath, toPath)
	
	return s.execNix(args...)
}

// toStandardPath converts a custom store path to standard /nix/store format
func (s *Store) toStandardPath(path string) string {
	if s.RootDir == "" {
		return path
	}
	
	// If it's already a standard path, return as-is
	if strings.HasPrefix(path, "/nix/store/") {
		return path
	}
	
	// If it's in our custom store, convert to standard path
	if strings.HasPrefix(path, s.StoreDir+"/") {
		// Extract the store item part (hash-name)
		relPath := strings.TrimPrefix(path, s.StoreDir+"/")
		return "/nix/store/" + relPath
	}
	
	return path
}


