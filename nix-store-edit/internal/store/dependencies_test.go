package store

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

func TestDependencyGraph_FindPathToRoot_SingleNode(t *testing.T) {
	dg := NewDependencyGraph()

	// Single node with no parent
	path := dg.FindPathToRoot("A")

	if len(path) != 1 {
		t.Fatalf("expected path length 1, got %d", len(path))
	}

	if path[0] != "A" {
		t.Errorf("path[0] = %s, want A", path[0])
	}
}

func TestParseWhyDependsOutput(t *testing.T) {
	// Example output from nix why-depends --all
	output := `/nix/store/abc123-system
├───/nix/store/def456-etc
│   ├───/nix/store/target-path
│   └───/nix/store/ghi789-bin
└───/nix/store/jkl012-lib
    └───/nix/store/target-path`

	systemClosure := "/nix/store/abc123-system"
	targetPath := "/nix/store/target-path"
	storeDir := "/nix/store"

	dg, closureChain, affectedPaths, err := parseWhyDependsOutput(output, systemClosure, targetPath, storeDir)
	if err != nil {
		t.Fatalf("parseWhyDependsOutput failed: %v", err)
	}

	// Check dependency graph
	if dg == nil {
		t.Fatal("dependency graph is nil")
	}

	// Check closure chain contains expected paths
	if len(closureChain) == 0 {
		t.Fatal("closure chain is empty")
	}

	// Check affected paths
	expectedAffected := map[string]bool{
		"/nix/store/abc123-system": true,
		"/nix/store/def456-etc":    true,
		"/nix/store/jkl012-lib":    true,
		"/nix/store/target-path":   true,
	}

	if len(affectedPaths) != len(expectedAffected) {
		t.Errorf("expected %d affected paths, got %d", len(expectedAffected), len(affectedPaths))
	}

	for _, path := range affectedPaths {
		if !expectedAffected[path] {
			t.Errorf("unexpected affected path: %s", path)
		}
	}
}

func TestStripAnsiCodes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "plain text",
			expected: "plain text",
		},
		{
			input:    "\x1b[31mred text\x1b[0m",
			expected: "red text",
		},
		{
			input:    "\x1b[1;32mbold green\x1b[0m normal",
			expected: "bold green normal",
		},
		{
			input:    "before \x1b[33;44myellow on blue\x1b[0m after",
			expected: "before yellow on blue after",
		},
	}

	for _, tt := range tests {
		result := stripAnsiCodes(tt.input)
		if result != tt.expected {
			t.Errorf("stripAnsiCodes(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractStorePathWithDir(t *testing.T) {
	tests := []struct {
		line     string
		storeDir string
		expected string
	}{
		{
			line:     "├───/nix/store/abc123-package",
			storeDir: "/nix/store",
			expected: "/nix/store/abc123-package",
		},
		{
			line:     "    /custom/store/def456-bin with extra text",
			storeDir: "/custom/store",
			expected: "/custom/store/def456-bin",
		},
		{
			line:     "no store path here",
			storeDir: "/nix/store",
			expected: "",
		},
		{
			line:     "/nix/store/path-with-tab\there",
			storeDir: "/nix/store",
			expected: "/nix/store/path-with-tab",
		},
	}

	for _, tt := range tests {
		result := extractStorePathWithDir(tt.line, tt.storeDir)
		if result != tt.expected {
			t.Errorf("extractStorePathWithDir(%q, %q) = %q, want %q", tt.line, tt.storeDir, result, tt.expected)
		}
	}
}
