package store

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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
	return cmd.Output()
}

// QueryReferences returns the references of a store path
func (s *Store) QueryReferences(path string) ([]string, error) {
	output, err := s.execNixStore("--query", "--references", path)
	if err != nil {
		return nil, fmt.Errorf("failed to query references: %w", err)
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

// QueryReferencesBatch returns the references for multiple store paths in a single call
func (s *Store) QueryReferencesBatch(paths []string) (map[string][]string, error) {
	if len(paths) == 0 {
		return make(map[string][]string), nil
	}

	// Build command with all paths
	args := append([]string{"--query", "--references"}, paths...)
	output, err := s.execNixStore(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query references: %w", err)
	}

	// Parse output - nix-store outputs references grouped by path
	result := make(map[string][]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	currentPath := ""

	for _, line := range lines {
		if line == "" {
			continue
		}
		// Check if this line is a queried path (ends with colon)
		if strings.HasSuffix(line, ":") {
			currentPath = strings.TrimSuffix(line, ":")
			result[currentPath] = []string{}
		} else if currentPath != "" {
			// This is a reference for the current path
			result[currentPath] = append(result[currentPath], line)
		}
	}

	// Fill in empty results for paths with no references
	for _, path := range paths {
		if _, exists := result[path]; !exists {
			result[path] = []string{}
		}
	}

	return result, nil
}

// QueryRequisites returns all transitive references of a store path
func (s *Store) QueryRequisites(path string) ([]string, error) {
	output, err := s.execNixStore("--query", "--requisites", path)
	if err != nil {
		return nil, fmt.Errorf("failed to query requisites: %w", err)
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	// The output includes the path itself, so we might want to filter it
	allPaths := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, p := range allPaths {
		if p != path {
			result = append(result, p)
		}
	}

	return result, nil
}

// Dump creates a NAR dump of the given path
func (s *Store) Dump(path string) ([]byte, error) {
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
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to import to store: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
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
		return nil, fmt.Errorf("invalid store path format")
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
		return false, err
	}
	return info.Trusted == 1, nil
}

// WhyDepends runs nix why-depends to analyze dependencies
func (s *Store) WhyDepends(from, to string, all bool) ([]byte, error) {
	args := []string{"why-depends"}
	if all {
		args = append(args, "--all")
	}
	args = append(args, from, to)
	
	return s.execNix(args...)
}
