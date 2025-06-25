package store

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		rootDir  string
		wantRoot string
		wantStore string
	}{
		{
			name:      "default paths",
			rootDir:   "",
			wantRoot:  "",
			wantStore: "/nix/store",
		},
		{
			name:      "custom root",
			rootDir:   "/custom/root",
			wantRoot:  "/custom/root",
			wantStore: "/custom/root/nix/store",
		},
		{
			name:      "relative path",
			rootDir:   "./foo",
			wantRoot:  "./foo",
			wantStore: "./foo/nix/store",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.rootDir)
			
			if s.RootDir != tt.wantRoot {
				t.Errorf("RootDir = %q, want %q", s.RootDir, tt.wantRoot)
			}
			if s.StoreDir != tt.wantStore {
				t.Errorf("StoreDir = %q, want %q", s.StoreDir, tt.wantStore)
			}
		})
	}
}

func TestExecNixArgs(t *testing.T) {
	// Test that --store flag is added correctly
	tests := []struct {
		name      string
		rootDir   string
		inputArgs []string
		wantStore bool
	}{
		{
			name:      "no store flag for default",
			rootDir:   "",
			inputArgs: []string{"why-depends", "foo", "bar"},
			wantStore: false,
		},
		{
			name:      "store flag for custom root",
			rootDir:   "./custom",
			inputArgs: []string{"why-depends", "foo", "bar"},
			wantStore: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.rootDir)
			
			// We can't easily test the actual command execution,
			// but we can verify the store setup is correct
			if tt.wantStore && s.RootDir == "" {
				t.Error("Expected RootDir to be set for custom store")
			}
			if !tt.wantStore && s.RootDir != "" {
				t.Error("Expected RootDir to be empty for default store")
			}
		})
	}
}