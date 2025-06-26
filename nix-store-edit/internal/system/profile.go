package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
)

// Profile represents a custom Nix profile
type Profile struct {
	ProfilePath string
	StoreRoot   string // Optional custom store root
}

// Type returns the system type
func (p *Profile) Type() Type {
	return TypeProfile
}

// GetClosurePath returns the path to the current profile closure
func (p *Profile) GetClosurePath() (string, error) {
	if p.ProfilePath == "" {
		return "", errors.New(errors.ErrCodeValidation, "Profile.GetClosurePath", "profile path not specified")
	}

	// Resolve the profile path
	closurePath, err := filepath.EvalSymlinks(p.ProfilePath)
	if err != nil {
		err = errors.Wrap(err, errors.ErrCodeSystem, "Profile.GetClosurePath")
		if e, ok := err.(*errors.Error); ok {
			e.Path = p.ProfilePath
		}
		return "", err
	}

	return closurePath, nil
}

// TestConfiguration tests a new closure path
func (p *Profile) TestConfiguration(closurePath string) error {
	// For custom profiles, we just verify the path exists
	if _, err := os.Stat(closurePath); err != nil {
		return errors.Wrap(err, errors.ErrCodeValidation, "Profile.TestConfiguration")
	}
	return nil
}

// GetDefaultCommand returns the default command for profiles (direct activation)
func (p *Profile) GetDefaultCommand(closurePath string) []string {
	args := []string{"nix-env"}
	
	// Add store flag if using custom root
	if p.StoreRoot != "" {
		args = append(args, "--store", p.StoreRoot)
		// Convert custom store path to standard path for nix-env
		closurePath = toStandardPath(closurePath)
	}
	
	// Add profile and path arguments
	args = append(args, "--profile", p.ProfilePath, "--set", closurePath)
	
	return args
}

// toStandardPath converts a custom store path to standard /nix/store format
func toStandardPath(path string) string {
	// Check if it's a custom store path (contains /nix/store but not at the beginning)
	if idx := strings.Index(path, "/nix/store/"); idx > 0 {
		// Extract everything after /nix/store/
		return path[idx:]
	}
	return path
}

// ApplyClosure applies a new profile closure
func (p *Profile) ApplyClosure(closurePath string, customCommand string) error {
	var cmd *exec.Cmd

	if customCommand != "" {
		// Parse custom command
		args := strings.Fields(customCommand)
		if len(args) == 0 {
			return errors.New(errors.ErrCodeValidation, "Profile.ApplyClosure", "empty activation command")
		}
		cmd = exec.Command(args[0], args[1:]...)
		// Replace {path} and {profile} placeholders
		// Convert path if using custom store
		pathToUse := closurePath
		if p.StoreRoot != "" {
			pathToUse = toStandardPath(closurePath)
		}
		for i, arg := range cmd.Args {
			arg = strings.ReplaceAll(arg, "{path}", pathToUse)
			cmd.Args[i] = strings.ReplaceAll(arg, "{profile}", p.ProfilePath)
		}
	} else {
		// Use default command
		defaultArgs := p.GetDefaultCommand(closurePath)
		cmd = exec.Command(defaultArgs[0], defaultArgs[1:]...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "Profile.ApplyClosure")
	}
	return nil
}

// IsAvailable checks if this system type is available
func (p *Profile) IsAvailable() bool {
	// Profile type is always available if a path is specified
	return p.ProfilePath != ""
}
