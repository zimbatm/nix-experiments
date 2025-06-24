package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SystemManager represents a system-manager configuration
type SystemManager struct{}

// Type returns the system type
func (s *SystemManager) Type() Type {
	return TypeSystemManager
}

// GetClosurePath returns the path to the current system-manager closure
func (s *SystemManager) GetClosurePath() (string, error) {
	// system-manager typically uses /nix/var/nix/profiles/system
	systemPath := "/nix/var/nix/profiles/system"
	if !symlinkExists(systemPath) {
		// Try user-specific profile
		userProfile := fmt.Sprintf("/nix/var/nix/profiles/per-user/%s/system-manager", os.Getenv("USER"))
		if symlinkExists(userProfile) {
			systemPath = userProfile
		} else {
			return "", fmt.Errorf("system-manager profile not found")
		}
	}

	resolved, err := filepath.EvalSymlinks(systemPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve system-manager closure: %w", err)
	}
	return resolved, nil
}

// GetDefaultCommand returns the default command for system-manager (safe test mode)
func (s *SystemManager) GetDefaultCommand(closurePath string) []string {
	return []string{"system-manager", "test", "--config", closurePath}
}

// ApplyClosure applies a new system-manager closure
func (s *SystemManager) ApplyClosure(closurePath string, customCommand string) error {
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
		defaultArgs := s.GetDefaultCommand(closurePath)
		cmd = exec.Command(defaultArgs[0], defaultArgs[1:]...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to activate system-manager configuration: %w", err)
	}
	return nil
}

// IsAvailable checks if system-manager is available
func (s *SystemManager) IsAvailable() bool {
	// Check for system-manager command and profile
	if !commandExists("system-manager") {
		return false
	}

	// Check for system-manager profiles
	profiles := []string{
		"/nix/var/nix/profiles/system",
		fmt.Sprintf("/nix/var/nix/profiles/per-user/%s/system-manager", os.Getenv("USER")),
	}

	for _, profile := range profiles {
		if symlinkExists(profile) {
			return true
		}
	}

	return false
}
