package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
)

// NixOS represents a NixOS system
type NixOS struct{}

// Type returns the system type
func (n *NixOS) Type() Type {
	return TypeNixOS
}

// GetClosurePath returns the path to the current system closure
func (n *NixOS) GetClosurePath() (string, error) {
	// NixOS uses /run/current-system
	closurePath, err := filepath.EvalSymlinks("/run/current-system")
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeSystem, "NixOS.GetClosurePath")
	}
	return closurePath, nil
}

// GetDefaultCommand returns the default command for NixOS (safe test mode)
func (n *NixOS) GetDefaultCommand(closurePath string) []string {
	return []string{"nixos-rebuild", "test", "--use-remote-sudo"}
}

// ApplyClosure applies a new system closure
func (n *NixOS) ApplyClosure(closurePath string, customCommand string) error {
	var cmd *exec.Cmd

	if customCommand != "" {
		// Parse custom command
		args := strings.Fields(customCommand)
		if len(args) == 0 {
			return errors.New(errors.ErrCodeValidation, "NixOS.ApplyClosure", "empty activation command")
		}
		cmd = exec.Command(args[0], args[1:]...)
		// Replace {path} placeholder with actual closure path
		for i, arg := range cmd.Args {
			cmd.Args[i] = strings.ReplaceAll(arg, "{path}", closurePath)
		}
	} else {
		// Use default command
		defaultArgs := n.GetDefaultCommand(closurePath)
		cmd = exec.Command(defaultArgs[0], defaultArgs[1:]...)
		cmd.Env = append(os.Environ(), fmt.Sprintf("NIX_PATH=nixos-config=%s", closurePath))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "NixOS.ApplyClosure")
	}
	return nil
}

// IsAvailable checks if this is a NixOS system
func (n *NixOS) IsAvailable() bool {
	// Check for NixOS by reading /etc/lsb-release only
	return detectOS() == "linux" && isNixOSFromLSBRelease()
}
