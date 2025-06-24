// Package cmd provides the command-line interface
package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/patch"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

// Execute runs the main command
func Execute() error {
	cfg := config.NewConfig()

	// Define flags
	flag.StringVar(&cfg.Editor, "editor", cfg.Editor, "editor to use")
	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "operation timeout")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "preview changes without applying")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "enable verbose logging")
	flag.BoolVar(&cfg.Force, "force", false, "force operation even if risky")
	flag.StringVar(&cfg.SystemType, "system", "", "override detected system type (nixos, nix-darwin, home-manager, system-manager, profile)")
	flag.StringVar(&cfg.ProfilePath, "profile", "", "path to custom profile (defaults to user profile when using -system=profile)")
	flag.StringVar(&cfg.ActivationCommand, "activate", "", "custom activation command (e.g., 'nixos-rebuild switch')")

	flag.Usage = func() {
		showUsage()
	}

	flag.Parse()

	// Get positional arguments
	args := flag.Args()
	if len(args) != 1 {
		showUsage()
		return errors.New(errors.ErrCodeConfig, "parse", "exactly one path argument required")
	}

	cfg.Path = args[0]

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return errors.Wrap(err, errors.ErrCodeConfig, "validate")
	}

	// Check if user is trusted
	trusted, err := store.IsTrustedUser()
	if err != nil {
		// If we can't determine trust status, default to dry-run
		fmt.Fprintf(os.Stderr, "Warning: Could not determine trusted user status: %s\n", err)
		fmt.Fprintf(os.Stderr, "Defaulting to dry-run mode.\n")
		cfg.DryRun = true
	} else if !trusted && !cfg.DryRun {
		fmt.Fprintf(os.Stderr, "Warning: You are not a trusted user.\n")
		fmt.Fprintf(os.Stderr, "Only trusted users can modify the Nix store.\n")
		fmt.Fprintf(os.Stderr, "Automatically enabling dry-run mode.\n")
		fmt.Fprintf(os.Stderr, "To become a trusted user, add yourself to nix.conf:\n")
		fmt.Fprintf(os.Stderr, "  trusted-users = %s\n\n", os.Getenv("USER"))
		cfg.DryRun = true
	}

	// Run the patch operation
	return patch.Run(cfg)
}

func showUsage() {
	fmt.Fprintf(os.Stderr, `nix-store-edit - Edit files in the Nix store

Usage:
  %s [options] <path>    Edit a path in the Nix store

Options:
  -editor string    Editor to use (default: $EDITOR or vim)
  -timeout duration Operation timeout (default: 2m)
  -dry-run         Preview changes without applying
  -verbose         Enable verbose logging
  -force           Force operation even if risky
  -system string   Override detected system type (nixos, nix-darwin, home-manager, system-manager, profile)
  -profile string  Path to custom profile (defaults to user profile when using -system=profile)
  -activate string Custom activation command (e.g., 'nixos-rebuild switch')
  -h, -help        Show this help

Environment:
  EDITOR        The editor to use (default: vim)

Description:
  Opens the path in a mutable buffer in your editor. When editing is finished,
  add the new content to the store, and rewrite your system closure recursively
  with it. By default, uses safe test commands where available (nixos-rebuild test, 
  darwin-rebuild check) or switch commands (home-manager switch).

Examples:
  # Edit a binary in the store
  %s /nix/store/...-vim-9.0/bin/vim

  # Preview changes without applying
  %s --dry-run /nix/store/...-config/etc/config.conf

  # Edit a specific profile
  %s --system=profile --profile=/nix/var/nix/profiles/system /nix/store/...-config/etc/config.conf

  # Use custom activation command (override default test to switch)
  %s --activate="nixos-rebuild switch" /nix/store/...-config/etc/config.conf

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
