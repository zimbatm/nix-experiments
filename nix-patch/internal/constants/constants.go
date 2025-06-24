// Package constants defines common constants used throughout the application
package constants

const (
	// NixStore is the base path for the Nix store
	NixStore = "/nix/store"
	
	// NixStorePrefix includes the trailing slash for path operations
	NixStorePrefix = NixStore + "/"
	
	// TempDirPrefix is the prefix for temporary directories created by nix-patch
	TempDirPrefix = "nix-patch-*"
	
	// RewriteTempDirPrefix is the prefix for temporary directories during rewrite operations
	RewriteTempDirPrefix = "nix-patch-rewrite-*"
	
	// StorePathComponents is the number of path components in a store path (e.g., /nix/store/hash-name)
	StorePathComponents = 4
	
	// MaxCacheSize is the maximum size of the NAR cache in bytes (10MB)
	MaxCacheSize = 10 * 1024 * 1024
)