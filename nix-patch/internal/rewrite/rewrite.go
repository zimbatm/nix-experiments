// Package rewrite implements the store path rewriting engine
package rewrite

import (
	"fmt"
	"log"
	"sort"
	"sync"
	
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

// Engine manages the rewriting of store paths and their dependencies
type Engine struct {
	// rewrites tracks old path -> new path mappings
	rewrites map[string]string

	// visited tracks processed paths to avoid cycles
	visited map[string]bool

	// cache for store operations
	cache *StoreCache

	// mutex for concurrent access
	mu sync.RWMutex

	// rollbackStack for atomic operations
	rollbackStack []RollbackOp

	// progress callback
	onProgress func(current, total int, path string)

	// dryRun mode - don't actually modify store
	dryRun bool
	
	// storeDir is the path to the Nix store
	storeDir string
	
	// store is the Nix store instance
	store *store.Store
}

// NewEngine creates a new rewrite engine
func NewEngine() *Engine {
	return NewEngineWithStoreDir("/nix/store")
}

// NewEngineWithStoreDir creates a new rewrite engine with a custom store directory
func NewEngineWithStoreDir(storeDir string) *Engine {
	s := store.New(storeDir)
	return NewEngineWithStore(s)
}

// NewEngineWithStore creates a new rewrite engine with a store instance
func NewEngineWithStore(s *store.Store) *Engine {
	return &Engine{
		rewrites: make(map[string]string),
		visited:  make(map[string]bool),
		cache:    NewStoreCacheWithStore(s),
		storeDir: s.StoreDir,
		store:    s,
	}
}

// SetProgressCallback sets a callback for progress updates
func (e *Engine) SetProgressCallback(fn func(current, total int, path string)) {
	e.onProgress = fn
}

// SetDryRun enables or disables dry-run mode
func (e *Engine) SetDryRun(dryRun bool) {
	e.dryRun = dryRun
}

// RewriteStep represents a single rewrite operation
type RewriteStep struct {
	OldPath    string
	NewPath    string
	References []string
	Referrers  []string // paths that reference this one
}

// RollbackOp represents an operation that can be rolled back
type RollbackOp struct {
	Type string
	Path string
	Data interface{}
}

// RewriteClosure rewrites an entire closure with pre-computed affected paths
func (e *Engine) RewriteClosure(systemClosure, modifiedPath, newModifiedPath string, affectedPaths []string) (string, error) {
	log.Printf("Starting closure rewrite: %s -> %s", modifiedPath, newModifiedPath)
	log.Printf("Using pre-computed affected paths: %d paths", len(affectedPaths))

	// Initialize with the user's modification
	e.recordRewrite(modifiedPath, newModifiedPath)

	// Build dependency graph for sorting
	graph, err := e.buildReverseDependencyGraph(systemClosure)
	if err != nil {
		return "", fmt.Errorf("failed to build dependency graph for sorting: %w", err)
	}

	// Sort paths by dependency order (leaves first, roots last)
	sortedPaths, err := e.topologicalSort(affectedPaths, graph)
	if err != nil {
		return "", fmt.Errorf("failed to sort paths: %w", err)
	}

	// Rewrite each path in order
	total := len(sortedPaths)
	for i, path := range sortedPaths {
		if e.onProgress != nil {
			e.onProgress(i+1, total, path)
		}

		newPath, err := e.rewritePath(path)
		if err != nil {
			return "", e.rollback(fmt.Errorf("failed to rewrite %s: %w", path, err))
		}

		e.recordRewrite(path, newPath)
	}

	// Return the new system closure
	newClosure, ok := e.getRewrite(systemClosure)
	if !ok {
		return "", fmt.Errorf("system closure was not rewritten")
	}

	if e.dryRun {
		log.Printf("DRY-RUN: Would have created new system closure: %s", newClosure)
	}

	return newClosure, nil
}

// recordRewrite records a path rewrite
func (e *Engine) recordRewrite(oldPath, newPath string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rewrites[oldPath] = newPath
	e.rollbackStack = append(e.rollbackStack, RollbackOp{
		Type: "rewrite",
		Path: oldPath,
		Data: newPath,
	})
}

// getRewrite returns the new path for an old path
func (e *Engine) getRewrite(oldPath string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	newPath, ok := e.rewrites[oldPath]
	return newPath, ok
}

// GetPlannedRewrites returns all the paths that will be rewritten
func (e *Engine) GetPlannedRewrites() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	// Return a copy to avoid concurrent modification
	result := make(map[string]string)
	for old, new := range e.rewrites {
		result[old] = new
	}
	return result
}

// rollback undoes all operations
func (e *Engine) rollback(err error) error {
	log.Printf("Rolling back %d operations due to: %v", len(e.rollbackStack), err)

	// TODO: Implement actual rollback logic
	// For now, just clear the state
	e.rewrites = make(map[string]string)
	e.visited = make(map[string]bool)
	e.rollbackStack = nil

	return fmt.Errorf("rollback: %w", err)
}

// DependencyGraph represents the reverse dependency relationships
type DependencyGraph struct {
	// dependencies maps a path to all paths that depend on it
	dependencies map[string][]string

	// references maps a path to all paths it references
	references map[string][]string
}

// buildReverseDependencyGraph builds a graph of what depends on what
func (e *Engine) buildReverseDependencyGraph(root string) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		dependencies: make(map[string][]string),
		references:   make(map[string][]string),
	}

	// BFS to explore all paths
	queue := []string{root}
	visited := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		// Get references for current path
		refs, err := e.cache.GetReferences(current)
		if err != nil {
			// Some paths may not have references
			continue
		}

		graph.references[current] = refs

		// Record reverse dependencies
		for _, ref := range refs {
			graph.dependencies[ref] = append(graph.dependencies[ref], current)
			queue = append(queue, ref)
		}
	}

	return graph, nil
}


// topologicalSort sorts paths by dependency order
func (e *Engine) topologicalSort(paths []string, graph *DependencyGraph) ([]string, error) {
	// Create a subgraph with only affected paths
	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[p] = true
	}

	// Calculate in-degree for each path
	inDegree := make(map[string]int)
	for _, path := range paths {
		inDegree[path] = 0
	}

	// Count incoming edges
	for _, path := range paths {
		if refs, ok := graph.references[path]; ok {
			for _, ref := range refs {
				if pathSet[ref] {
					inDegree[ref]++
				}
			}
		}
	}

	// Kahn's algorithm for topological sort
	var sorted []string
	queue := make([]string, 0)

	// Find all nodes with no incoming edges
	for path, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, path)
		}
	}

	// Sort queue for deterministic results
	sort.Strings(queue)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		// Remove edges from current node
		if refs, ok := graph.references[current]; ok {
			for _, ref := range refs {
				if pathSet[ref] {
					inDegree[ref]--
					if inDegree[ref] == 0 {
						queue = append(queue, ref)
						sort.Strings(queue) // Keep deterministic
					}
				}
			}
		}
	}

	if len(sorted) != len(paths) {
		return nil, fmt.Errorf("dependency cycle detected")
	}

	return sorted, nil
}
