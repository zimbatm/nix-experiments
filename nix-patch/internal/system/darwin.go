package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NixDarwin represents a nix-darwin system
type NixDarwin struct{}

// Type returns the system type
func (d *NixDarwin) Type() Type {
	return TypeNixDarwin
}

// GetClosurePath returns the path to the current system closure
func (d *NixDarwin) GetClosurePath() (string, error) {
	// nix-darwin uses /run/current-system
	closurePath, err := filepath.EvalSymlinks("/run/current-system")
	if err != nil {
		return "", fmt.Errorf("failed to resolve nix-darwin system closure: %w", err)
	}
	return closurePath, nil
}

// GetDefaultCommand returns the default command for nix-darwin (safe check mode)
func (d *NixDarwin) GetDefaultCommand(closurePath string) []string {
	return []string{"darwin-rebuild", "check", "-I", fmt.Sprintf("darwin-config=%s", closurePath)}
}

// ApplyClosure applies a new system closure
func (d *NixDarwin) ApplyClosure(closurePath string, customCommand string) error {
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
		defaultArgs := d.GetDefaultCommand(closurePath)
		cmd = exec.Command(defaultArgs[0], defaultArgs[1:]...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to activate nix-darwin configuration: %w", err)
	}
	return nil
}

// IsAvailable checks if this is a nix-darwin system
func (d *NixDarwin) IsAvailable() bool {
	// Check for nix-darwin-specific indicators
	return detectOS() == "darwin" &&
		commandExists("darwin-rebuild") &&
		symlinkExists("/run/current-system")
}
