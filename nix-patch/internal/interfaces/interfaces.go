// Package interfaces defines the core interfaces for nix-store-edit
package interfaces

import (
	"os"
	"path/filepath"
	"time"
)

// Store represents operations on the Nix store
type Store interface {
	// QueryReferences returns the references of a store path
	QueryReferences(path string) ([]string, error)

	// QueryReferrers returns paths that reference the given path
	QueryReferrers(path string) ([]string, error)

	// Import imports a NAR archive and returns the store path
	Import(data []byte) (string, error)

	// Dump exports a store path as NAR data
	Dump(path string) ([]byte, error)

	// IsValidPath checks if a path is a valid store path
	IsValidPath(path string) bool

	// ParsePath parses a store path into components
	ParsePath(path string) (*StorePathInfo, error)
}

// StorePathInfo contains parsed store path information
type StorePathInfo struct {
	Hash string
	Name string
	Path string
}

// Cache represents a caching layer
type Cache interface {
	// Get retrieves a value from cache
	Get(key string) (any, bool)

	// Set stores a value in cache
	Set(key string, value any, ttl time.Duration)

	// Delete removes a value from cache
	Delete(key string)

	// Clear removes all values from cache
	Clear()
}

// RewriteEngine handles path rewriting operations
type RewriteEngine interface {
	// RewriteClosure rewrites an entire closure
	RewriteClosure(systemClosure, modifiedPath, newModifiedPath string) (string, error)

	// SetProgressReporter sets the progress reporter
	SetProgressReporter(reporter ProgressReporter)
}

// ProgressReporter reports progress of operations
type ProgressReporter interface {
	// OnStart is called when an operation starts
	OnStart(total int)

	// OnProgress is called during progress
	OnProgress(current, total int, message string)

	// OnComplete is called when operation completes
	OnComplete()

	// OnError is called when an error occurs
	OnError(err error)
}

// FileSystem represents file system operations
type FileSystem interface {
	// ReadFile reads a file
	ReadFile(path string) ([]byte, error)

	// WriteFile writes a file
	WriteFile(path string, data []byte, perm os.FileMode) error

	// Stat returns file info
	Stat(path string) (os.FileInfo, error)

	// Walk walks a directory tree
	Walk(root string, walkFn filepath.WalkFunc) error

	// MkdirTemp creates a temporary directory
	MkdirTemp(dir, pattern string) (string, error)

	// RemoveAll removes a path and any children
	RemoveAll(path string) error

	// Readlink reads a symbolic link
	Readlink(path string) (string, error)

	// Symlink creates a symbolic link
	Symlink(oldname, newname string) error
}

// Commander executes external commands
type Commander interface {
	// Execute runs a command and returns output
	Execute(name string, args ...string) ([]byte, error)

	// ExecuteWithInput runs a command with input
	ExecuteWithInput(input []byte, name string, args ...string) ([]byte, error)

	// ExecuteInteractive runs a command interactively
	ExecuteInteractive(name string, args ...string) error
}

// Logger provides logging capabilities
type Logger interface {
	// Debug logs debug messages
	Debug(msg string, fields ...Field)

	// Info logs info messages
	Info(msg string, fields ...Field)

	// Warn logs warning messages
	Warn(msg string, fields ...Field)

	// Error logs error messages
	Error(msg string, fields ...Field)

	// WithFields returns a logger with additional fields
	WithFields(fields ...Field) Logger
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value any
}

// Archive handles NAR archive operations
type Archive interface {
	// Create creates an archive from a path
	Create(storePath, sourcePath string) ([]byte, error)

	// CreateWithRewrites creates an archive with path rewrites
	CreateWithRewrites(storePath, sourcePath string, rewrites map[string]string) ([]byte, error)

	// Extract extracts an archive to a path
	Extract(data []byte, destPath string) error
}
