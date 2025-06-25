package store

import (
	"strings"
	"testing"
)

func TestExtractHash(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "valid store path",
			path: "/nix/store/abc123def456-vim-9.0",
			want: "abc123def456",
		},
		{
			name: "store path with multiple dashes",
			path: "/nix/store/xyz789-my-cool-package-1.0",
			want: "xyz789",
		},
		{
			name: "invalid path - too short",
			path: "/nix/store",
			want: "",
		},
		{
			name: "invalid path - not store path",
			path: "/usr/bin/vim",
			want: "",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
	}

	s := New("") // Default store
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.ExtractHash(tt.path); got != tt.want {
				t.Errorf("ExtractHash() = %v, want %v", got, tt.want)
			}
		})
	}
	
	// Test with custom store root
	t.Run("custom store root", func(t *testing.T) {
		customStore := New("/custom/root")
		
		hash := customStore.ExtractHash("/custom/root/nix/store/abc123-package")
		if hash != "abc123" {
			t.Errorf("Expected hash 'abc123', got '%s'", hash)
		}
		
		// Should not extract from default store path
		hash = customStore.ExtractHash("/nix/store/abc123-package")
		if hash != "" {
			t.Errorf("Expected empty hash for wrong store, got '%s'", hash)
		}
	})
}

func TestIsStorePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "valid store path",
			path: "/nix/store/abc123-package",
			want: true,
		},
		{
			name: "store path with subdirectory",
			path: "/nix/store/abc123-package/bin/program",
			want: true,
		},
		{
			name: "not a store path",
			path: "/usr/bin/program",
			want: false,
		},
		{
			name: "partial store path",
			path: "/nix/stor/abc123-package",
			want: false,
		},
		{
			name: "empty path",
			path: "",
			want: false,
		},
	}

	s := New("") // Default store at /nix/store
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.IsStorePath(tt.path); got != tt.want {
				t.Errorf("IsStorePath() = %v, want %v", got, tt.want)
			}
		})
	}
	
	// Test with custom store root
	t.Run("custom store root", func(t *testing.T) {
		customStore := New("/custom/root")
		
		if !customStore.IsStorePath("/custom/root/nix/store/abc-pkg") {
			t.Error("Expected custom store path to be valid")
		}
		
		if customStore.IsStorePath("/nix/store/abc-pkg") {
			t.Error("Expected default store path to be invalid for custom store")
		}
	})
}

func TestGenerateHash(t *testing.T) {
	// Test that GenerateHash returns a valid nix32 hash
	hash1, err := GenerateHash()
	if err != nil {
		t.Fatalf("GenerateHash() error = %v", err)
	}

	// Nix32 hashes should be 32 characters
	if len(hash1) != 32 {
		t.Errorf("GenerateHash() returned hash of length %d, want 32", len(hash1))
	}

	// Should only contain valid nix32 characters
	validChars := "0123456789abcdfghijklmnpqrsvwxyz"
	for _, c := range hash1 {
		if !strings.Contains(validChars, string(c)) {
			t.Errorf("GenerateHash() returned invalid character: %c", c)
		}
	}

	// Two calls should return different hashes
	hash2, err := GenerateHash()
	if err != nil {
		t.Fatalf("GenerateHash() error = %v", err)
	}
	if hash1 == hash2 {
		t.Error("GenerateHash() returned same hash twice")
	}
}

func TestParseStorePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    *StorePathInfo
		wantErr bool
	}{
		{
			name: "valid store path",
			path: "/nix/store/abc123-vim-9.0",
			want: &StorePathInfo{
				Hash: "abc123",
				Name: "vim-9.0",
			},
			wantErr: false,
		},
		{
			name: "path with multiple dashes",
			path: "/nix/store/xyz789-my-cool-package-1.0",
			want: &StorePathInfo{
				Hash: "xyz789",
				Name: "my-cool-package-1.0",
			},
			wantErr: false,
		},
		{
			name: "path with subdirectory",
			path: "/nix/store/abc123-package/bin/program",
			want: &StorePathInfo{
				Hash: "abc123",
				Name: "package",
			},
			wantErr: false,
		},
		{
			name:    "invalid path",
			path:    "/usr/bin/vim",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty path",
			path:    "",
			want:    nil,
			wantErr: true,
		},
	}

	s := New("") // Default store
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.ParseStorePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStorePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Hash != tt.want.Hash {
					t.Errorf("ParseStorePath() Hash = %v, want %v", got.Hash, tt.want.Hash)
				}
				if got.Name != tt.want.Name {
					t.Errorf("ParseStorePath() Name = %v, want %v", got.Name, tt.want.Name)
				}
			}
		})
	}
	
	// Test with custom store root
	t.Run("custom store root", func(t *testing.T) {
		customStore := New("/custom/root")
		
		info, err := customStore.ParseStorePath("/custom/root/nix/store/xyz789-test-pkg-1.0")
		if err != nil {
			t.Fatalf("ParseStorePath() error = %v", err)
		}
		
		if info.Hash != "xyz789" || info.Name != "test-pkg-1.0" {
			t.Errorf("ParseStorePath() = %+v, want Hash='xyz789', Name='test-pkg-1.0'", info)
		}
		
		// Should fail for wrong store
		_, err = customStore.ParseStorePath("/nix/store/abc123-package")
		if err == nil {
			t.Error("Expected error for wrong store path")
		}
	})
}

// Note: QueryReferences, Import, and Dump require actual Nix store interaction
// These would be better tested as integration tests
func TestStoreOperations(t *testing.T) {
	t.Skip("Store operations require Nix store access - implement as integration tests")
}
