package rewrite

import (
	"testing"
)

// TestDependencyGraph has been removed since findAffectedPaths is now handled by store package

func TestTopologicalSort(t *testing.T) {
	// For rewriting: if A depends on B, we need to rewrite B first
	// Graph: a -> b -> c (a depends on b, b depends on c)
	graph := &DependencyGraph{
		references: map[string][]string{
			"/nix/store/a": {"/nix/store/b", "/nix/store/c"},
			"/nix/store/b": {"/nix/store/c"},
			"/nix/store/c": {},
		},
	}

	engine := NewEngine()
	paths := []string{"/nix/store/a", "/nix/store/b", "/nix/store/c"}

	sorted, err := engine.topologicalSort(paths, graph)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// For rewriting, we need to process dependencies before dependents
	// In our case: c has no dependencies (leaf), a depends on b and c (root)
	// So order should be: c, b, a (leaves first, roots last)
	t.Logf("Topological sort order: %v", sorted)

	// Verify the sort maintains dependency order
	order := make(map[string]int)
	for i, p := range sorted {
		order[p] = i
	}

	// c should be processed first (it's a leaf)
	if sorted[0] != "/nix/store/c" {
		t.Errorf("Expected c to be first in sort order, got %s", sorted[0])
	}

	// a should be processed last (it's the root)
	if sorted[2] != "/nix/store/a" {
		t.Errorf("Expected a to be last in sort order, got %s", sorted[2])
	}

	// Dependencies should be processed before dependents
	if order["/nix/store/c"] >= order["/nix/store/b"] {
		t.Errorf("c should be processed before b")
	}
	if order["/nix/store/b"] >= order["/nix/store/a"] {
		t.Errorf("b should be processed before a")
	}
}

func TestCycleDetection(t *testing.T) {
	// Create a graph with a cycle
	graph := &DependencyGraph{
		references: map[string][]string{
			"/nix/store/a": {"/nix/store/b"},
			"/nix/store/b": {"/nix/store/c"},
			"/nix/store/c": {"/nix/store/a"}, // Cycle!
		},
	}

	engine := NewEngine()
	paths := []string{"/nix/store/a", "/nix/store/b", "/nix/store/c"}

	_, err := engine.topologicalSort(paths, graph)
	if err == nil {
		t.Error("Expected error for cyclic dependency, got nil")
	}
}

func TestSimpleDependency(t *testing.T) {
	// Test the exact scenario from the issue: profile -> claude-code
	graph := &DependencyGraph{
		references: map[string][]string{
			"/nix/store/profile":     {"/nix/store/claude-code"},
			"/nix/store/claude-code": {},
		},
	}

	engine := NewEngine()
	paths := []string{"/nix/store/profile", "/nix/store/claude-code"}

	sorted, err := engine.topologicalSort(paths, graph)
	if err != nil {
		t.Fatalf("Unexpected error for simple dependency: %v", err)
	}

	t.Logf("Sorted order: %v", sorted)

	// claude-code should be processed first (it's the dependency)
	if sorted[0] != "/nix/store/claude-code" {
		t.Errorf("Expected claude-code to be first, got %s", sorted[0])
	}

	// profile should be processed second
	if sorted[1] != "/nix/store/profile" {
		t.Errorf("Expected profile to be second, got %s", sorted[1])
	}
}
