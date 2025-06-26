package config

import (
	"testing"
	"time"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	
	// Test default values
	if cfg.Editor != DefaultEditor {
		t.Errorf("Expected default editor %s, got %s", DefaultEditor, cfg.Editor)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultTimeout, cfg.Timeout)
	}
	if cfg.StoreRoot != "" {
		t.Errorf("Expected empty StoreRoot, got %s", cfg.StoreRoot)
	}
	if cfg.DryRun != false {
		t.Errorf("Expected DryRun to be false, got %v", cfg.DryRun)
	}
	if cfg.Verbose != false {
		t.Errorf("Expected Verbose to be false, got %v", cfg.Verbose)
	}
	if cfg.Force != false {
		t.Errorf("Expected Force to be false, got %v", cfg.Force)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errType error
	}{
		{
			name: "valid config",
			config: &Config{
				Editor: "vim",
				Path:   "/nix/store/abc-test",
			},
			wantErr: false,
		},
		{
			name: "missing path",
			config: &Config{
				Editor: "vim",
				Path:   "",
			},
			wantErr: true,
			errType: ErrMissingPath,
		},
		{
			name: "missing editor",
			config: &Config{
				Editor: "",
				Path:   "/nix/store/abc-test",
			},
			wantErr: true,
			errType: ErrMissingEditor,
		},
		{
			name: "missing both",
			config: &Config{
				Editor: "",
				Path:   "",
			},
			wantErr: true,
			errType: ErrMissingPath, // Path is checked first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errType != nil && err != tt.errType {
				t.Errorf("Validate() error = %v, want %v", err, tt.errType)
			}
		})
	}
}

func TestConfigError_Error(t *testing.T) {
	err := &ConfigError{
		Field:   "test_field",
		Message: "test message",
	}
	expected := "config error: test_field: test message"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestConstants(t *testing.T) {
	// Verify constants have expected values
	if NixinMagic != 0x4558494e {
		t.Errorf("NixinMagic has unexpected value: %x", NixinMagic)
	}
	if ExportVersion != 1 {
		t.Errorf("ExportVersion has unexpected value: %d", ExportVersion)
	}
	if DefaultTimeout != 2*time.Minute {
		t.Errorf("DefaultTimeout has unexpected value: %v", DefaultTimeout)
	}
}
