package whydepends

import (
	"testing"
)

func TestNewDependencyGraph(t *testing.T) {
	dg := NewDependencyGraph()
	if dg == nil {
		t.Fatal("NewDependencyGraph returned nil")
	}
	if dg.parents == nil {
		t.Fatal("parents map not initialized")
	}
}

func TestDependencyGraph_FindPathToRoot(t *testing.T) {
	dg := NewDependencyGraph()
	
	// Build a simple dependency chain: A -> B -> C
	dg.parents["C"] = "B"
	dg.parents["B"] = "A"
	
	path := dg.FindPathToRoot("C")
	
	if len(path) != 3 {
		t.Fatalf("expected path length 3, got %d", len(path))
	}
	
	expected := []string{"A", "B", "C"}
	for i, p := range path {
		if p != expected[i] {
			t.Errorf("path[%d] = %s, want %s", i, p, expected[i])
		}
	}
}

func TestDependencyGraph_FindPathToRoot_NoParent(t *testing.T) {
	dg := NewDependencyGraph()
	
	path := dg.FindPathToRoot("A")
	
	if len(path) != 1 {
		t.Fatalf("expected path length 1, got %d", len(path))
	}
	
	if path[0] != "A" {
		t.Errorf("path[0] = %s, want A", path[0])
	}
}

func TestStripAnsiCodes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello \x1b[31mWorld\x1b[0m",
			expected: "Hello World",
		},
		{
			input:    "\x1b[1;32mGreen\x1b[0m text",
			expected: "Green text",
		},
		{
			input:    "No color codes",
			expected: "No color codes",
		},
	}
	
	for _, tt := range tests {
		result := stripAnsiCodes(tt.input)
		if result != tt.expected {
			t.Errorf("stripAnsiCodes(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractStorePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "├───/nix/store/abc123-package",
			expected: "/nix/store/abc123-package",
		},
		{
			input:    "    /nix/store/def456-other some text",
			expected: "/nix/store/def456-other",
		},
		{
			input:    "no store path here",
			expected: "",
		},
		{
			input:    "/nix/store/ghi789-test\t(tab separated)",
			expected: "/nix/store/ghi789-test",
		},
	}
	
	for _, tt := range tests {
		result := extractStorePath(tt.input)
		if result != tt.expected {
			t.Errorf("extractStorePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}