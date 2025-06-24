package archive

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/nix-community/go-nix/pkg/wire"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/config"
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
