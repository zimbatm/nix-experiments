// Package system provides abstractions for different Nix-based systems
package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Type represents the type of Nix-based system
type Type string

const (
	TypeNixOS         Type = "nixos"
	TypeNixDarwin     Type = "nix-darwin"
	TypeHomeManager   Type = "home-manager"
	TypeSystemManager Type = "system-manager"
	TypeProfile       Type = "profile"
	TypeUnknown       Type = "unknown"
)

// System represents a Nix-based system configuration
type System interface {
	// Type returns the system type
	Type() Type

	// GetClosurePath returns the path to the current system closure
	GetClosurePath() (string, error)

	// GetDefaultCommand returns the default command for this system (usually a safe test command)
	GetDefaultCommand(closurePath string) []string

	// ApplyClosure applies a new system closure
	// customCommand can be empty to use the default command
	ApplyClosure(closurePath string, customCommand string) error

	// IsAvailable checks if this system type is available on the current machine
	IsAvailable() bool
}

// Detect automatically detects the system type
func Detect() (System, error) {
	// Try each system type in order of specificity
	systems := []System{
		&NixOS{},
		&NixDarwin{},
		&SystemManager{},
		&HomeManager{},
	}

	for _, sys := range systems {
		if sys.IsAvailable() {
			return sys, nil
		}
	}

	// Fall back to user profile
	userProfile := getUserProfilePath()
	if fileExists(userProfile) {
		return &Profile{ProfilePath: userProfile}, nil
	}

	return nil, fmt.Errorf("no supported Nix system detected and no user profile found")
}

// GetSystemByType returns a system implementation for the given type
func GetSystemByType(systemType string, profilePath string) (System, error) {
	switch Type(systemType) {
	case TypeNixOS:
		return &NixOS{}, nil
	case TypeNixDarwin:
		return &NixDarwin{}, nil
	case TypeHomeManager:
		return &HomeManager{}, nil
	case TypeSystemManager:
		return &SystemManager{}, nil
	case TypeProfile:
		if profilePath == "" {
			// Default to user profile
			profilePath = getUserProfilePath()
			if profilePath == "" {
				return nil, fmt.Errorf("could not determine user profile path")
			}
		}
		return &Profile{ProfilePath: profilePath}, nil
	default:
		return nil, fmt.Errorf("unknown system type: %s", systemType)
	}
}

// detectOS returns the current operating system
func detectOS() string {
	return runtime.GOOS
}

// commandExists checks if a command exists in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// symlinkExists checks if a symlink exists and can be read
func symlinkExists(path string) bool {
	_, err := os.Readlink(path)
	return err == nil
}

// isNixOSFromLSBRelease checks if this is NixOS by reading /etc/lsb-release
func isNixOSFromLSBRelease() bool {
	data, err := os.ReadFile("/etc/lsb-release")
	if err != nil {
		return false
	}

	// Check if the file contains NixOS identifier
	content := string(data)
	return strings.Contains(content, "DISTRIB_ID=NixOS") ||
		strings.Contains(content, "DISTRIB_ID=\"NixOS\"")
}

// getUserProfilePath returns the path to the user's Nix profile
func getUserProfilePath() string {
	// First try the standard user profile symlink
	homeDir, err := os.UserHomeDir()
	if err == nil {
		profileLink := filepath.Join(homeDir, ".nix-profile")
		if fileExists(profileLink) || symlinkExists(profileLink) {
			return profileLink
		}
	}

	// Fall back to the per-user profile path
	user := os.Getenv("USER")
	if user != "" {
		return fmt.Sprintf("/nix/var/nix/profiles/per-user/%s/profile", user)
	}

	// Last resort - use the home directory symlink path even if it doesn't exist yet
	if homeDir != "" {
		return filepath.Join(homeDir, ".nix-profile")
	}

	return ""
}
