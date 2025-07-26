package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
)

// Config represents the complete chronixpkgs configuration
type Config struct {
	// Repository to monitor
	Repository string `toml:"repository"`

	// Storage configuration
	Storage StorageConfig `toml:"storage"`

	// Server configuration
	Server ServerConfig `toml:"server"`

	// Fetcher configuration
	Fetcher FetcherConfig `toml:"fetcher"`

	// S3 export configuration
	S3 S3Config `toml:"s3"`

	// Observability configuration
	Observability ObservabilityConfig `toml:"observability"`
	
	// LoadedFrom tracks which config file was loaded (if any)
	LoadedFrom string `toml:"-"`
}

// StorageConfig contains database and storage settings
type StorageConfig struct {
	// Data directory path
	DataDir string `toml:"data_dir"`

	// Retention period in days (0 = keep forever)
	RetentionDays int `toml:"retention_days"`

	// Vacuum interval (e.g., "24h")
	VacuumInterval string `toml:"vacuum_interval"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	// Listen address (e.g., ":8080")
	Listen string `toml:"listen"`

	// Enable rate limiting
	EnableRateLimit bool `toml:"enable_rate_limit"`

	// Rate limit requests per minute
	RateLimitPerMinute int `toml:"rate_limit_per_minute"`

	// Rate limit burst size
	RateLimitBurst int `toml:"rate_limit_burst"`

	// Default API limit
	DefaultLimit int `toml:"default_limit"`

	// Maximum API limit
	MaxLimit int `toml:"max_limit"`

	// SSE polling interval (e.g., "10s")
	SSEInterval string `toml:"sse_interval"`
}

// FetcherConfig contains event fetcher settings
type FetcherConfig struct {
	// GitHub API token (can also be set via GITHUB_TOKEN env var)
	GitHubToken string `toml:"github_token"`

	// Polling interval (e.g., "1m")
	PollInterval string `toml:"poll_interval"`

	// Run once and exit
	Once bool `toml:"once"`
}

// S3Config contains S3 export settings
type S3Config struct {
	// Enable S3 export
	Enabled bool `toml:"enabled"`

	// S3 endpoint URL
	Endpoint string `toml:"endpoint"`

	// S3 bucket name
	Bucket string `toml:"bucket"`

	// S3 access key (can also be set via AWS_ACCESS_KEY_ID env var)
	AccessKey string `toml:"access_key"`

	// S3 secret key (can also be set via AWS_SECRET_ACCESS_KEY env var)
	SecretKey string `toml:"secret_key"`

	// Export interval (e.g., "1h")
	ExportInterval string `toml:"export_interval"`
}

// ObservabilityConfig contains monitoring settings
type ObservabilityConfig struct {
	// OpenTelemetry endpoint
	OTelEndpoint string `toml:"otel_endpoint"`

	// Service name for telemetry
	ServiceName string `toml:"service_name"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Repository: "NixOS/nixpkgs",
		Storage: StorageConfig{
			DataDir:        "./data",
			RetentionDays:  0, // Keep forever
			VacuumInterval: "24h",
		},
		Server: ServerConfig{
			Listen:             ":8080",
			EnableRateLimit:    false,
			RateLimitPerMinute: 60,
			RateLimitBurst:     10,
			DefaultLimit:       100,
			MaxLimit:           1000,
			SSEInterval:        "10s",
		},
		Fetcher: FetcherConfig{
			PollInterval: "1m",
			Once:         false,
		},
		S3: S3Config{
			Enabled:        false,
			ExportInterval: "1h",
		},
		Observability: ObservabilityConfig{
			ServiceName: "chronixpkgs",
		},
	}
}

// LoadConfig loads configuration from a TOML file
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	// Parse TOML file
	if _, err := toml.DecodeFile(path, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables
	config.applyEnvOverrides()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Set the loaded from path
	config.LoadedFrom = path

	return config, nil
}

// LoadConfigOrDefault loads config from file if it exists, otherwise returns defaults
func LoadConfigOrDefault(path string) (*Config, error) {
	// Load .env file if it exists
	envFiles := []string{
		".env",
		".env.local",
		filepath.Join(os.Getenv("HOME"), ".config", "chronixpkgs", ".env"),
	}
	
	for _, envFile := range envFiles {
		if _, err := os.Stat(envFile); err == nil {
			_ = godotenv.Load(envFile) // Ignore errors, just load what we can
			break
		}
	}
	
	if path == "" {
		// Try default locations
		configPaths := []string{
			"chronixpkgs.toml",
			"config.toml",
			filepath.Join(os.Getenv("HOME"), ".config", "chronixpkgs", "config.toml"),
			"/etc/chronixpkgs/config.toml",
		}

		for _, p := range configPaths {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return LoadConfig(path)
		}
	}

	// No config file found, use defaults with env overrides
	config := DefaultConfig()
	config.applyEnvOverrides()
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// No config file was loaded
	config.LoadedFrom = ""

	return config, nil
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	// Repository
	if repo := os.Getenv("CHRONIXPKGS_REPO"); repo != "" {
		c.Repository = repo
	}

	// GitHub token
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		c.Fetcher.GitHubToken = token
	}

	// S3 configuration
	if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
		c.S3.Endpoint = endpoint
	}
	if bucket := os.Getenv("S3_BUCKET"); bucket != "" {
		c.S3.Bucket = bucket
	}
	if key := os.Getenv("AWS_ACCESS_KEY_ID"); key != "" {
		c.S3.AccessKey = key
	}
	if secret := os.Getenv("AWS_SECRET_ACCESS_KEY"); secret != "" {
		c.S3.SecretKey = secret
	}

	// Data directory
	if dir := os.Getenv("CHRONIXPKGS_DATA_DIR"); dir != "" {
		c.Storage.DataDir = dir
	}

	// Server listen address
	if addr := os.Getenv("CHRONIXPKGS_LISTEN"); addr != "" {
		c.Server.Listen = addr
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Repository == "" {
		return fmt.Errorf("repository must be specified")
	}

	if c.Storage.DataDir == "" {
		return fmt.Errorf("data directory must be specified")
	}

	// Validate durations
	durations := map[string]string{
		"vacuum_interval": c.Storage.VacuumInterval,
		"poll_interval":   c.Fetcher.PollInterval,
		"sse_interval":    c.Server.SSEInterval,
		"export_interval": c.S3.ExportInterval,
	}

	for name, value := range durations {
		if value != "" {
			if _, err := time.ParseDuration(value); err != nil {
				return fmt.Errorf("invalid %s: %w", name, err)
			}
		}
	}

	// Validate S3 config if enabled
	if c.S3.Enabled {
		if c.S3.Endpoint == "" {
			return fmt.Errorf("S3 endpoint must be specified when S3 export is enabled")
		}
		if c.S3.Bucket == "" {
			return fmt.Errorf("S3 bucket must be specified when S3 export is enabled")
		}
	}

	return nil
}

// GetVacuumInterval returns the vacuum interval as a Duration
func (c *StorageConfig) GetVacuumInterval() time.Duration {
	d, _ := time.ParseDuration(c.VacuumInterval)
	return d
}

// GetPollInterval returns the poll interval as a Duration
func (c *FetcherConfig) GetPollInterval() time.Duration {
	d, _ := time.ParseDuration(c.PollInterval)
	return d
}

// GetSSEInterval returns the SSE interval as a Duration
func (c *ServerConfig) GetSSEInterval() time.Duration {
	d, _ := time.ParseDuration(c.SSEInterval)
	return d
}

// GetExportInterval returns the export interval as a Duration
func (c *S3Config) GetExportInterval() time.Duration {
	d, _ := time.ParseDuration(c.ExportInterval)
	return d
}

// WriteExampleConfig writes an example configuration file
func WriteExampleConfig(path string) error {
	example := `# Chronixpkgs Configuration File

# Repository to monitor (required)
repository = "NixOS/nixpkgs"

[storage]
# Data directory path
data_dir = "./data"

# Retention period in days (0 = keep forever)
retention_days = 0

# How often to run vacuum/cleanup
vacuum_interval = "24h"

[server]
# Listen address for HTTP server
listen = ":8080"

# Rate limiting
enable_rate_limit = false
rate_limit_per_minute = 60
rate_limit_burst = 10

# API limits
default_limit = 100
max_limit = 1000

# Server-sent events polling interval
sse_interval = "10s"

[fetcher]
# GitHub API token (can also use GITHUB_TOKEN env var)
# github_token = "ghp_..."

# How often to poll GitHub API
poll_interval = "1m"

# Run once and exit
once = false

[s3]
# Enable S3 export
enabled = false

# S3 configuration (required if enabled)
# endpoint = "https://s3.amazonaws.com"
# bucket = "chronixpkgs-events"
# access_key = ""  # Can also use AWS_ACCESS_KEY_ID env var
# secret_key = ""  # Can also use AWS_SECRET_ACCESS_KEY env var

# Export interval
export_interval = "1h"

[observability]
# OpenTelemetry endpoint (optional)
# otel_endpoint = "localhost:4317"

# Service name for telemetry
service_name = "chronixpkgs"
`

	return os.WriteFile(path, []byte(example), 0644)
}

// WriteExampleEnv writes an example .env file
func WriteExampleEnv(path string) error {
	example := `# Chronixpkgs Secrets
# Copy this file to .env and fill in your actual secrets
# DO NOT COMMIT .env TO VERSION CONTROL!

# GitHub API token (required for fetcher)
# Get one from: https://github.com/settings/tokens
# Required scopes: public_repo
GITHUB_TOKEN=ghp_your_token_here

# S3 credentials (optional - only if using S3 export)
# AWS_ACCESS_KEY_ID=your_access_key_here
# AWS_SECRET_ACCESS_KEY=your_secret_key_here
`

	return os.WriteFile(path, []byte(example), 0644)
}