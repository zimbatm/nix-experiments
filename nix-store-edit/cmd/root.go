// Package cmd provides the command-line interface
package cmd

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"runtime/pprof"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/patch"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/system"
)

// Execute runs the main command
func Execute() error {
	cfg := config.NewConfig()

	// Use EDITOR environment variable if set
	if editor := os.Getenv("EDITOR"); editor != "" {
		cfg.Editor = editor
	}

	// Define flags
	var cpuprofile string
	var memprofile string
	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to file")
	flag.StringVar(&memprofile, "memprofile", "", "write memory profile to file")
	flag.StringVar(&cfg.Editor, "editor", cfg.Editor, "editor to use")
	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "operation timeout")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "preview changes without applying")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "enable verbose logging")
	flag.BoolVar(&cfg.Force, "force", false, "force operation even if risky")
	flag.StringVar(&cfg.SystemType, "system", "", "override detected system type (nixos, nix-darwin, home-manager, system-manager, profile)")
	flag.StringVar(&cfg.ProfilePath, "profile", "", "path to custom profile (defaults to user profile when using -system=profile)")
	flag.StringVar(&cfg.ActivationCommand, "activate", "", "custom activation command (e.g., 'nixos-rebuild switch')")
	flag.StringVar(&cfg.StoreRoot, "store", cfg.StoreRoot, "root directory for Nix store (e.g., ./foo for ./foo/nix/store)")

	flag.Usage = showUsage

	flag.Parse()

	// Enable CPU profiling if requested
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			return errors.New(errors.ErrCodeConfig, "create profile", err.Error())
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return errors.New(errors.ErrCodeConfig, "start profile", err.Error())
		}
		defer pprof.StopCPUProfile()
	}

	// Get positional arguments
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		return errors.New(errors.ErrCodeConfig, "parse", "exactly one path argument required")
	}

	cfg.Path = args[0]

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return errors.Wrap(err, errors.ErrCodeConfig, "validate")
	}

	// Check for macOS with custom store
	if cfg.StoreRoot != "" && runtime.GOOS == "darwin" {
		return errors.New(errors.ErrCodeConfig, "validate", "custom --store flag is not supported on macOS")
	}

	// Create store instance
	s := store.New(cfg.StoreRoot)

	// Check if user is trusted
	trusted, err := s.IsTrustedUser()
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
	err = patch.Run(cfg)

	// Write memory profile if requested
	if memprofile != "" {
		f, perr := os.Create(memprofile)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "Failed to create memory profile: %v\n", perr)
		} else {
			defer f.Close()
			runtime.GC() // get up-to-date statistics
			if perr := pprof.WriteHeapProfile(f); perr != nil {
				fmt.Fprintf(os.Stderr, "Failed to write memory profile: %v\n", perr)
			}
		}
	}

	return err
}

func showUsage() {
	// Detect current system
	detectedSystem := "unknown"
	defaultClosure := ""
	if sys, err := system.Detect(); err == nil {
		detectedSystem = string(sys.Type())
		// Get the closure path from the detected system
		if closurePath, err := sys.GetClosurePath(); err == nil {
			defaultClosure = closurePath
		}
	}

	fmt.Fprintf(os.Stderr, `nix-store-edit - Edit files in the Nix store

Usage:
  %s [options] <path>

Current System: %s
Default Closure: %s

Options:
`, os.Args[0], detectedSystem, defaultClosure)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Examples:
  # Edit a binary in the store
  %s /nix/store/...-vim-9.0/bin/vim

  # Preview changes without applying
  %s --dry-run /nix/store/...-config/etc/config.conf

  # Edit a specific profile
  %s --system=profile --profile=/nix/var/nix/profiles/system /nix/store/...-config/etc/config.conf

  # Use a custom store location
  %s --store ./mystore /nix/store/...-package/bin/app
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
