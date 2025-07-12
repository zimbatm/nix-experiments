package utils

import (
	"fmt"
	"strings"
)

// NormalizeRepository ensures repository is in the format "github.com/owner/repo"
func NormalizeRepository(repo string) string {
	// Remove any leading/trailing whitespace
	repo = strings.TrimSpace(repo)
	
	// If already has github.com prefix, return as is
	if strings.HasPrefix(repo, "github.com/") {
		return repo
	}
	
	// If it's in owner/repo format, add github.com prefix
	if strings.Count(repo, "/") == 1 && !strings.Contains(repo, "://") {
		return fmt.Sprintf("github.com/%s", repo)
	}
	
	// Otherwise return as is (might be invalid, but let caller handle)
	return repo
}

// ValidateRepository checks if a repository string is valid
func ValidateRepository(repo string) error {
	repo = strings.TrimSpace(repo)
	
	// Must contain github.com prefix
	if !strings.HasPrefix(repo, "github.com/") {
		return fmt.Errorf("repository must start with 'github.com/'")
	}
	
	// Remove prefix and check format
	parts := strings.Split(strings.TrimPrefix(repo, "github.com/"), "/")
	if len(parts) != 2 {
		return fmt.Errorf("repository must be in format 'github.com/owner/repo'")
	}
	
	owner, name := parts[0], parts[1]
	if owner == "" || name == "" {
		return fmt.Errorf("repository owner and name cannot be empty")
	}
	
	return nil
}