package store

import (
	"strings"
	"testing"
)

func TestGenerateContentHash(t *testing.T) {
	t.Run("produces valid nixbase32 hash", func(t *testing.T) {
		data := []byte("test content")
		hash := GenerateContentHash(data)
		
		// Check length
		if len(hash) != 32 {
			t.Errorf("Expected hash length 32, got %d", len(hash))
		}
		
		// Check valid nixbase32 characters
		validChars := "0123456789abcdfghijklmnpqrsvwxyz"
		for _, c := range hash {
			if !strings.ContainsRune(validChars, c) {
				t.Errorf("Invalid nixbase32 character: %c", c)
			}
		}
	})
	
	t.Run("is deterministic", func(t *testing.T) {
		data := []byte("deterministic content")
		hash1 := GenerateContentHash(data)
		hash2 := GenerateContentHash(data)
		
		if hash1 != hash2 {
			t.Errorf("Hash not deterministic: %s != %s", hash1, hash2)
		}
	})
	
	t.Run("different content produces different hashes", func(t *testing.T) {
		data1 := []byte("content 1")
		data2 := []byte("content 2")
		
		hash1 := GenerateContentHash(data1)
		hash2 := GenerateContentHash(data2)
		
		if hash1 == hash2 {
			t.Errorf("Different content produced same hash")
		}
	})
}