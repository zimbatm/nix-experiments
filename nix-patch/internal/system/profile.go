package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Profile represents a custom Nix profile
type Profile struct {
	ProfilePath string
}

// Type returns the system type
func (p *Profile) Type() Type {
	return TypeProfile
}

// GetClosurePath returns the path to the current profile closure
func (p *Profile) GetClosurePath() (string, error) {
	if p.ProfilePath == "" {
		return "", fmt.Errorf("profile path not specified")
	}

	// Resolve the profile path
	closurePath, err := filepath.EvalSymlinks(p.ProfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve profile path %s: %w", p.ProfilePath, err)
	}

	return closurePath, nil
}

// TestConfiguration tests a new closure path
func (p *Profile) TestConfiguration(closurePath string) error {
	// For custom profiles, we just verify the path exists
	if _, err := os.Stat(closurePath); err != nil {
		return fmt.Errorf("closure path does not exist: %w", err)
	}
	return nil
}

// GetDefaultCommand returns the default command for profiles (direct activation)
func (p *Profile) GetDefaultCommand(closurePath string) []string {
	return []string{"nix-env", "--profile", p.ProfilePath, "--set", closurePath}
}

// ApplyClosure applies a new profile closure
func (p *Profile) ApplyClosure(closurePath string, customCommand string) error {
	var cmd *exec.Cmd

	if customCommand != "" {
		// Parse custom command
		args := strings.Fields(customCommand)
		if len(args) == 0 {
			return fmt.Errorf("empty activation command")
		}
		cmd = exec.Command(args[0], args[1:]...)
		// Replace {path} and {profile} placeholders
		for i, arg := range cmd.Args {
			arg = strings.ReplaceAll(arg, "{path}", closurePath)
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
		return fmt.Errorf("failed to activate profile: %w", err)
	}
	return nil
}

// IsAvailable checks if this system type is available
func (p *Profile) IsAvailable() bool {
	// Profile type is always available if a path is specified
	return p.ProfilePath != ""
}
