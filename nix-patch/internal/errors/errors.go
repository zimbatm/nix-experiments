// Package errors provides structured error handling
package errors

import (
	"fmt"
	"strings"
)

// ErrorCode represents a type of error
type ErrorCode string

const (
	// Configuration errors
	ErrCodeConfig ErrorCode = "CONFIG"

	// Store operation errors
	ErrCodeStore ErrorCode = "STORE"

	// Validation errors
	ErrCodeValidation ErrorCode = "VALIDATION"

	// Unknown errors
	ErrCodeUnknown ErrorCode = "UNKNOWN"
)

// Error represents a structured error
type Error struct {
	Code    ErrorCode
	Op      string
	Path    string
	Message string
	Err     error
}

// Error implements the error interface
func (e *Error) Error() string {
	var parts []string

	if e.Op != "" {
		parts = append(parts, e.Op)
	}

	if e.Path != "" {
		parts = append(parts, e.Path)
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}

	return strings.Join(parts, ": ")
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}

	return e.Code == t.Code
}

// New creates a new error
func New(code ErrorCode, op string, message string) *Error {
	return &Error{
		Code:    code,
		Op:      op,
		Message: message,
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, code ErrorCode, op string) *Error {
	if err == nil {
		return nil
	}

	// If it's already our error type, add context
	if e, ok := err.(*Error); ok {
		return &Error{
			Code: code,
			Op:   op,
			Path: e.Path,
			Err:  e,
		}
	}

	return &Error{
		Code: code,
		Op:   op,
		Err:  err,
	}
}

// WithPath adds a path to the error
func WithPath(err error, path string) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(*Error); ok {
		e.Path = path
		return e
	}

	return &Error{
		Code: ErrCodeUnknown,
		Path: path,
		Err:  err,
	}
}

// IsCode checks if an error has a specific code
func IsCode(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}

	e, ok := err.(*Error)
	if !ok {
		return false
	}

	return e.Code == code
}

// Common errors
var (
	// Store errors
	ErrInvalidStorePath = New(ErrCodeStore, "store", "invalid store path")
	ErrPathNotInStore   = New(ErrCodeStore, "store", "path is not in nix store")
	ErrStoreDump        = New(ErrCodeStore, "store.dump", "failed to dump store path")
	ErrStoreImport      = New(ErrCodeStore, "store.import", "failed to import to store")

	// Validation errors
	ErrCyclicDependency = New(ErrCodeValidation, "validate", "cyclic dependency detected")
)

// Format formats an error for user display
func Format(err error) string {
	if err == nil {
		return ""
	}

	e, ok := err.(*Error)
	if !ok {
		return err.Error()
	}

	// Format based on error code
	switch e.Code {
	case ErrCodeConfig:
		return fmt.Sprintf("Configuration error: %s", e.Message)
	case ErrCodeStore:
		return fmt.Sprintf("Store operation failed: %s", e.Error())
	case ErrCodeValidation:
		return fmt.Sprintf("Validation error: %s", e.Message)
	default:
		return e.Error()
	}
}
