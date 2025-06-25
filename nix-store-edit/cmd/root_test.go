package cmd

import (
	"flag"
	"os"
	"testing"
)

func TestExecute(t *testing.T) {
	// Save original command line args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args shows error",
			args:    []string{"nix-store-edit"},
			wantErr: true,
		},
		{
			name:    "help flag",
			args:    []string{"nix-store-edit", "-h"},
			wantErr: true, // flag.Parse exits with error on -h
		},
		{
			name:    "invalid path",
			args:    []string{"nix-store-edit", "/tmp/not-a-store-path"},
			wantErr: true,
		},
		{
			name:    "too many args",
			args:    []string{"nix-store-edit", "path1", "path2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags for each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

			// Set command line args
			os.Args = tt.args

			err := Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShowUsage(t *testing.T) {
	// Save original stderr
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Call showUsage
	showUsage()

	// Close writer and read output
	w.Close()
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Check that usage contains expected strings
	expectedStrings := []string{
		"nix-store-edit",
		"Usage:",
		"Options:",
		"Examples:",
	}

	for _, expected := range expectedStrings {
		if !contains(output, expected) {
			t.Errorf("Usage output missing %q", expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			contains(s[1:], substr)))
}
