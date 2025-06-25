package archive

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/nix-community/go-nix/pkg/wire"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

func TestWriteUint64(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
		want  []byte
	}{
		{
			name:  "zero",
			value: 0,
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:  "one",
			value: 1,
			want:  []byte{1, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:  "magic number",
			value: uint64(config.NixinMagic),
			want:  []byte{0x4e, 0x49, 0x58, 0x45, 0, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := wire.WriteUint64(&buf, tt.value)
			if err != nil {
				t.Fatalf("WriteUint64 failed: %v", err)
			}
			got := buf.Bytes()

			if !bytes.Equal(got, tt.want) {
				t.Errorf("WriteUint64() = %v, want %v", got, tt.want)
			}

			// Verify it's little-endian
			var readValue uint64
			err = binary.Read(bytes.NewReader(got), binary.LittleEndian, &readValue)
			if err != nil {
				t.Fatalf("Failed to read back value: %v", err)
			}
			if readValue != tt.value {
				t.Errorf("Read back value = %v, want %v", readValue, tt.value)
			}
		})
	}
}

func TestWriteString(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want int // expected total bytes written
	}{
		{
			name: "empty string",
			str:  "",
			want: 8, // just the length prefix
		},
		{
			name: "short string",
			str:  "hi",
			want: 16, // 8 (length) + 2 (string) + 6 (padding)
		},
		{
			name: "8 byte string",
			str:  "12345678",
			want: 16, // 8 (length) + 8 (string) + 0 (padding)
		},
		{
			name: "9 byte string",
			str:  "123456789",
			want: 24, // 8 (length) + 9 (string) + 7 (padding)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := wire.WriteString(&buf, tt.str)
			if err != nil {
				t.Fatalf("WriteString failed: %v", err)
			}
			got := buf.Bytes()

			if len(got) != tt.want {
				t.Errorf("WriteString() wrote %d bytes, want %d", len(got), tt.want)
			}

			// Verify length prefix
			var length uint64
			err = binary.Read(bytes.NewReader(got[:8]), binary.LittleEndian, &length)
			if err != nil {
				t.Fatalf("Failed to read length: %v", err)
			}
			if int(length) != len(tt.str) {
				t.Errorf("Length prefix = %d, want %d", length, len(tt.str))
			}

			// Verify string content
			if len(tt.str) > 0 {
				strBytes := got[8 : 8+len(tt.str)]
				if string(strBytes) != tt.str {
					t.Errorf("String content = %q, want %q", string(strBytes), tt.str)
				}
			}

			// Verify padding is zeros
			for i := 8 + len(tt.str); i < len(got); i++ {
				if got[i] != 0 {
					t.Errorf("Padding byte at position %d is %d, want 0", i, got[i])
				}
			}
		})
	}
}

// Test the structure of the export format
func TestExportFormatStructure(t *testing.T) {
	// This test validates the structure without requiring actual store operations
	// In a real test, we'd mock the store operations

	// Expected structure:
	// 1. Export version (int64)
	// 2. NAR data (variable length)
	// 3. Magic number (int64)
	// 4. Store path (string)
	// 5. Number of references (int64)
	// 6. References (strings)
	// 7. Deriver (string)
	// 8. Two zeros (int64 each)

	var buf bytes.Buffer

	// Write a minimal valid export format
	wire.WriteUint64(&buf, uint64(config.ExportVersion)) // 1
	buf.Write([]byte("fake-nar-data"))                   // 2 (normally would be actual NAR)
	wire.WriteUint64(&buf, uint64(config.NixinMagic))    // 3
	wire.WriteString(&buf, "/nix/store/abc-test")        // 4
	wire.WriteUint64(&buf, 0)                            // 5 (no references)
	wire.WriteString(&buf, "")                           // 7 (no deriver)
	wire.WriteUint64(&buf, 0)                            // 8
	wire.WriteUint64(&buf, 0)                            // 8

	data := buf.Bytes()
	if len(data) < 32 { // Minimum size check
		t.Errorf("Export format too small: %d bytes", len(data))
	}

	// Verify we can read back the version
	var version uint64
	err := binary.Read(bytes.NewReader(data[:8]), binary.LittleEndian, &version)
	if err != nil {
		t.Fatalf("Failed to read version: %v", err)
	}
	if version != uint64(config.ExportVersion) {
		t.Errorf("Version = %d, want %d", version, config.ExportVersion)
	}
}

// Note: Create and CreateWithRewrites require actual store operations
// These would be better tested as integration tests or with mocks
func TestArchiveOperations(t *testing.T) {
	t.Skip("Archive operations require Nix store access - implement as integration tests")
}

func TestContentBasedHashGeneration(t *testing.T) {
	t.Run("GenerateContentHash produces valid nixbase32", func(t *testing.T) {
		testData := []byte("test NAR content for hashing")
		hash := store.GenerateContentHash(testData)
		
		// Verify hash format (nixbase32 character set)
		validChars := "0123456789abcdfghijklmnpqrsvwxyz"
		for _, c := range hash {
			if !bytes.ContainsRune([]byte(validChars), c) {
				t.Errorf("Invalid character in nixbase32 hash: %c", c)
			}
		}
		
		// Verify hash length (20 bytes = 32 chars in base32)
		if len(hash) != 32 {
			t.Errorf("Expected hash length 32, got %d", len(hash))
		}
	})

	t.Run("hash is deterministic", func(t *testing.T) {
		data := []byte("deterministic test data")
		
		hash1 := store.GenerateContentHash(data)
		hash2 := store.GenerateContentHash(data)
		
		if hash1 != hash2 {
			t.Errorf("Hash not deterministic: %s vs %s", hash1, hash2)
		}
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		data1 := []byte("content 1")
		data2 := []byte("content 2")
		
		hash1 := store.GenerateContentHash(data1)
		hash2 := store.GenerateContentHash(data2)
		
		if hash1 == hash2 {
			t.Errorf("Different content produced same hash: %s", hash1)
		}
	})

	t.Run("store path parsing and reconstruction", func(t *testing.T) {
		// Test the path generation logic without actual store operations
		oldPath := "/nix/store/abc123def456ghi789jkl012mno345p-test-package-1.0"
		
		// Parse the old path to extract the name
		s := store.New("") // Default store
		sp, err := s.ParseStorePath(oldPath)
		if err != nil {
			t.Fatalf("Failed to parse store path: %v", err)
		}
		
		if sp.Name != "test-package-1.0" {
			t.Errorf("Expected name 'test-package-1.0', got '%s'", sp.Name)
		}
		
		// Simulate modified content
		narData := []byte("modified NAR content")
		newHash := store.GenerateContentHash(narData)
		
		// Build expected new path
		expectedPath := "/nix/store/" + newHash + "-" + sp.Name
		
		// Verify path format
		if !bytes.HasPrefix([]byte(expectedPath), []byte("/nix/store/")) {
			t.Errorf("Invalid store path prefix: %s", expectedPath)
		}
		
		if !bytes.Contains([]byte(expectedPath), []byte("-test-package-1.0")) {
			t.Errorf("Expected path to contain package name, got: %s", expectedPath)
		}
		
		// Verify the hash part is different from original
		if bytes.HasPrefix([]byte(expectedPath), []byte("/nix/store/abc123def456ghi789jkl012mno345p")) {
			t.Errorf("New path should have different hash than original")
		}
	})
}
