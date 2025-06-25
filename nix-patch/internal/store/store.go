// Package store provides utilities for working with the Nix store
package store

import (
	"crypto/rand"
	"crypto/sha256"

	"github.com/nix-community/go-nix/pkg/nixbase32"
)




// GenerateContentHash generates a content-based nix32 hash from NAR data
// This mimics how Nix generates store path hashes from content
func GenerateContentHash(narData []byte) string {
	// Generate SHA256 hash of the NAR content
	hasher := sha256.New()
	hasher.Write(narData)
	hashBytes := hasher.Sum(nil)
	
	// Convert to nixbase32 (truncate to 20 bytes as Nix does)
	return nixbase32.EncodeToString(hashBytes[:20])
}

// GenerateHash generates a random nix32 hash for testing/temporary use
func GenerateHash() (string, error) {
	// Generate 20 random bytes (160 bits)
	randomBytes := make([]byte, 20)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	
	// Convert to nixbase32
	return nixbase32.EncodeToString(randomBytes), nil
}


