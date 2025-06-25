package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HomeManager represents a home-manager configuration
type HomeManager struct{}

// Type returns the system type
func (h *HomeManager) Type() Type {
	return TypeHomeManager
}

// GetClosurePath returns the path to the current home-manager generation
func (h *HomeManager) GetClosurePath() (string, error) {
	// home-manager uses ~/.nix-profile or a specific profile path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Try common home-manager profile locations
	profilePaths := []string{
		filepath.Join(homeDir, ".nix-profile"),
		"/nix/var/nix/profiles/per-user/" + os.Getenv("USER") + "/home-manager",
	}

	for _, path := range profilePaths {
		if symlinkExists(path) {
			resolved, err := filepath.EvalSymlinks(path)
			if err == nil {
				return resolved, nil
			}
		}
	}

	return "", fmt.Errorf("failed to find home-manager profile")
}

// GetDefaultCommand returns the default command for home-manager (switch - no test mode available)
func (h *HomeManager) GetDefaultCommand(closurePath string) []string {
	return []string{"home-manager", "switch", "-I", fmt.Sprintf("home-manager-config=%s", closurePath)}
}

// ApplyClosure applies a new home-manager closure
func (h *HomeManager) ApplyClosure(closurePath string, customCommand string) error {
	var cmd *exec.Cmd

	if customCommand != "" {
		// Parse custom command
		args := strings.Fields(customCommand)
		if len(args) == 0 {
			return fmt.Errorf("empty activation command")
		}
		cmd = exec.Command(args[0], args[1:]...)
		// Replace {path} placeholder with actual closure path
		for i, arg := range cmd.Args {
			cmd.Args[i] = strings.ReplaceAll(arg, "{path}", closurePath)
		}
	} else {
		// Use default command
		defaultArgs := h.GetDefaultCommand(closurePath)
		cmd = exec.Command(defaultArgs[0], defaultArgs[1:]...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to activate home-manager configuration: %w", err)
	}
	return nil
}

// IsAvailable checks if home-manager is available
func (h *HomeManager) IsAvailable() bool {
	// Check for home-manager command and profile
	if !commandExists("home-manager") {
		return false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Check if any home-manager profile exists
	profilePaths := []string{
		filepath.Join(homeDir, ".nix-profile"),
		"/nix/var/nix/profiles/per-user/" + os.Getenv("USER") + "/home-manager",
	}

	for _, path := range profilePaths {
		if symlinkExists(path) {
			return true
		}
	}

	return false
}
