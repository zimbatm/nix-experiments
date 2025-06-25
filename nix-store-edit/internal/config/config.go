// Package config provides centralized configuration and constants
package config

import (
	"time"
)

// Constants for the application
const (
	// Nix export format constants
	NixinMagic    = 0x4558494e // "NIXIN" magic number
	ExportVersion = 1

	// Default values
	DefaultEditor  = "vim"
	DefaultTimeout = 2 * time.Minute

	// Cache settings
	MaxCacheSize    = 100 * 1024 * 1024 // 100MB
)

// Config holds the application configuration
type Config struct {
	// Editor to use for editing files
	Editor string

	// Path to edit
	Path string

	// Timeout for operations
	Timeout time.Duration

	// DryRun mode - don't apply changes
	DryRun bool

	// Verbose logging
	Verbose bool

	// Force operation even if risky
	Force bool

	// System type override (e.g., "nixos", "nix-darwin", "home-manager", "system-manager", "profile")
	SystemType string

	// ProfilePath is the path to a custom profile (used when SystemType is "profile")
	ProfilePath string

	// ActivationCommand is a custom command to activate the configuration
	// If empty, the system's default activation command will be used
	ActivationCommand string

	// StoreRoot is the root directory for the Nix store (empty for default /nix)
	// When set, the actual store will be at StoreRoot/nix/store
	StoreRoot string
}

// NewConfig creates a new configuration with defaults
func NewConfig() *Config {
	return &Config{
		Editor:   DefaultEditor,
		Timeout:  DefaultTimeout,
		StoreRoot: "", // Default to system /nix
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Path == "" {
		return ErrMissingPath
	}
	if c.Editor == "" {
		return ErrMissingEditor
	}
	return nil
}

// Common errors
var (
	ErrMissingPath   = &ConfigError{Field: "path", Message: "path is required"}
	ErrMissingEditor = &ConfigError{Field: "editor", Message: "editor is required"}
)

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config error: " + e.Field + ": " + e.Message
}
