package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "full error",
			err: &Error{
				Code:    ErrCodeStore,
				Op:      "test.op",
				Path:    "/test/path",
				Message: "test message",
				Err:     errors.New("underlying error"),
			},
			want: "test.op: /test/path: test message: underlying error",
		},
		{
			name: "no underlying error",
			err: &Error{
				Code:    ErrCodeConfig,
				Op:      "config.validate",
				Message: "invalid config",
			},
			want: "config.validate: invalid config",
		},
		{
			name: "only message",
			err: &Error{
				Code:    ErrCodeValidation,
				Message: "validation failed",
			},
			want: "validation failed",
		},
		{
			name: "empty error",
			err:  &Error{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := &Error{
		Code: ErrCodeStore,
		Err:  underlying,
	}

	if err.Unwrap() != underlying {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), underlying)
	}

	// Test nil underlying error
	err2 := &Error{Code: ErrCodeStore}
	if err2.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", err2.Unwrap())
	}
}

func TestError_Is(t *testing.T) {
	err1 := &Error{Code: ErrCodeStore}
	err2 := &Error{Code: ErrCodeStore}
	err3 := &Error{Code: ErrCodeConfig}

	if !err1.Is(err2) {
		t.Error("Expected errors with same code to match")
	}

	if err1.Is(err3) {
		t.Error("Expected errors with different codes not to match")
	}

	// Test with non-Error type
	if err1.Is(errors.New("other error")) {
		t.Error("Expected Error not to match non-Error type")
	}
}

func TestNew(t *testing.T) {
	err := New(ErrCodeStore, "store.import", "import failed")

	if err.Code != ErrCodeStore {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeStore)
	}
	if err.Op != "store.import" {
		t.Errorf("Op = %v, want store.import", err.Op)
	}
	if err.Message != "import failed" {
		t.Errorf("Message = %v, want import failed", err.Message)
	}
	if err.Err != nil {
		t.Errorf("Err = %v, want nil", err.Err)
	}
}

func TestWrap(t *testing.T) {
	// Test wrapping nil
	if Wrap(nil, ErrCodeStore, "op") != nil {
		t.Error("Wrap(nil) should return nil")
	}

	// Test wrapping regular error
	underlying := errors.New("underlying error")
	wrapped := Wrap(underlying, ErrCodeStore, "store.op")

	if wrapped.Code != ErrCodeStore {
		t.Errorf("Code = %v, want %v", wrapped.Code, ErrCodeStore)
	}
	if wrapped.Op != "store.op" {
		t.Errorf("Op = %v, want store.op", wrapped.Op)
	}
	if wrapped.Err != underlying {
		t.Errorf("Err = %v, want %v", wrapped.Err, underlying)
	}

	// Test wrapping Error type
	err := &Error{
		Code:    ErrCodeValidation,
		Path:    "/test/path",
		Message: "original error",
	}
	wrapped2 := Wrap(err, ErrCodeStore, "new.op")

	if wrapped2.Code != ErrCodeStore {
		t.Errorf("Code = %v, want %v", wrapped2.Code, ErrCodeStore)
	}
	if wrapped2.Op != "new.op" {
		t.Errorf("Op = %v, want new.op", wrapped2.Op)
	}
	if wrapped2.Path != "/test/path" {
		t.Errorf("Path = %v, want /test/path", wrapped2.Path)
	}
}

func TestWithPath(t *testing.T) {
	// Test with nil
	if WithPath(nil, "/path") != nil {
		t.Error("WithPath(nil) should return nil")
	}

	// Test with Error type
	err := &Error{
		Code: ErrCodeStore,
		Op:   "test",
	}
	result := WithPath(err, "/test/path")
	if e, ok := result.(*Error); !ok || e.Path != "/test/path" {
		t.Errorf("WithPath failed to set path: %v", result)
	}

	// Test with regular error
	regularErr := errors.New("regular error")
	result2 := WithPath(regularErr, "/test/path2")
	if e, ok := result2.(*Error); !ok || e.Path != "/test/path2" || e.Code != ErrCodeUnknown {
		t.Errorf("WithPath failed with regular error: %v", result2)
	}
}

func TestIsCode(t *testing.T) {
	// Test with nil
	if IsCode(nil, ErrCodeStore) {
		t.Error("IsCode(nil) should return false")
	}

	// Test with matching code
	err := &Error{Code: ErrCodeStore}
	if !IsCode(err, ErrCodeStore) {
		t.Error("IsCode should return true for matching code")
	}

	// Test with non-matching code
	if IsCode(err, ErrCodeConfig) {
		t.Error("IsCode should return false for non-matching code")
	}

	// Test with non-Error type
	if IsCode(errors.New("other"), ErrCodeStore) {
		t.Error("IsCode should return false for non-Error type")
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
		{
			name: "config error",
			err:  &Error{Code: ErrCodeConfig, Message: "bad config"},
			want: "Configuration error: bad config",
		},
		{
			name: "store error",
			err:  &Error{Code: ErrCodeStore, Op: "import", Message: "failed"},
			want: "Store operation failed: import: failed",
		},
		{
			name: "validation error",
			err:  &Error{Code: ErrCodeValidation, Message: "invalid input"},
			want: "Validation error: invalid input",
		},
		{
			name: "regular error",
			err:  errors.New("regular error"),
			want: "regular error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.err); got != tt.want {
				t.Errorf("Format() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommonErrors(t *testing.T) {
	// Test that common errors are properly initialized
	if ErrInvalidStorePath.Code != ErrCodeStore {
		t.Error("ErrInvalidStorePath has wrong code")
	}
	if !strings.Contains(ErrInvalidStorePath.Message, "invalid") {
		t.Error("ErrInvalidStorePath has wrong message")
	}

	if ErrCyclicDependency.Code != ErrCodeValidation {
		t.Error("ErrCyclicDependency has wrong code")
	}
}
