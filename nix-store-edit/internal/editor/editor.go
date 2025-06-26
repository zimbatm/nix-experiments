// Package editor provides utilities for opening files in external editors
package editor

import (
	"fmt"
	"os"
	"os/exec"
)

// Open opens the specified path in the configured editor
// The editorCmd can contain arguments, e.g., "vim -n" or "sed -i 's/foo/bar/g'"
// For complex commands, it uses the shell to handle proper parsing
func Open(editorCmd, path string) error {
	if editorCmd == "" {
		return fmt.Errorf("empty editor command")
	}

	// Use sh -c to handle complex commands with quotes and arguments
	cmd := exec.Command("sh", "-c", editorCmd+" "+path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	return nil
}
