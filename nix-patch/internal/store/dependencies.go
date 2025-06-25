// Package store - dependencies.go provides dependency analysis functionality
package store

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// DependencyGraph represents the dependency relationships in a closure
type DependencyGraph struct {
	parents map[string]string // child -> parent mapping of store paths
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		parents: make(map[string]string),
	}
}

// FindPathToRoot finds the dependency chain from target to root
func (dg *DependencyGraph) FindPathToRoot(target string) []string {
	var closureChain []string

	current := target
	for current != "" {
		closureChain = append(closureChain, current)
		current = dg.parents[current]
	}

	// Reverse the chain
	for i, j := 0, len(closureChain)-1; i < j; i, j = i+1, j-1 {
		closureChain[i], closureChain[j] = closureChain[j], closureChain[i]
	}

	log.Println("Dependency chain from root to target:")
	for _, p := range closureChain {
		fmt.Fprintln(os.Stderr, p)
	}

	return closureChain
}

// BuildDependencyChain builds the dependency graph and finds the closure chain
func (s *Store) BuildDependencyChain(systemClosure, storePath string) (*DependencyGraph, []string, []string, error) {
	// Use nix why-depends --all to get the complete dependency information
	output, err := s.WhyDepends(systemClosure, storePath, true)
	if err != nil {
		// Check if the error is because the path is not in the closure
		if strings.Contains(err.Error(), "does not depend on") {
			return nil, nil, nil, fmt.Errorf("path %s is not part of system closure %s", storePath, systemClosure)
		}
		return nil, nil, nil, fmt.Errorf("failed to run nix why-depends: %w", err)
	}

	// Parse the output to build dependency graph
	dg, closureChain, affectedPaths, err := parseWhyDependsOutput(string(output), systemClosure, storePath, s.StoreDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse why-depends output: %w", err)
	}

	return dg, closureChain, affectedPaths, nil
}

// parseWhyDependsOutput parses the output of 'nix why-depends --all' to extract dependency information
func parseWhyDependsOutput(output, systemClosure, targetPath, storeDir string) (*DependencyGraph, []string, []string, error) {
	dg := NewDependencyGraph()
	
	// Track all paths that depend on the target
	dependentPaths := make(map[string]bool)
	
	// Split output into lines
	lines := strings.Split(output, "\n")
	
	// Parse the tree structure
	// The output format is like:
	// /nix/store/abc-system
	// ├───/nix/store/def-etc
	// │   └───/nix/store/target-path
	// └───/nix/store/ghi-other
	//     └───/nix/store/target-path
	
	pathStack := []string{}
	indentStack := []int{-1}
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		// Remove ANSI color codes
		line = stripAnsiCodes(line)
		
		// Calculate indentation level
		indent := 0
		for _, ch := range line {
			if ch == ' ' || ch == '│' || ch == '├' || ch == '└' || ch == '─' {
				indent++
			} else {
				break
			}
		}
		
		// Extract the store path from the line
		storePath := extractStorePathWithDir(line, storeDir)
		if storePath == "" {
			continue
		}
		
		// Adjust stack based on indentation
		for len(indentStack) > 1 && indent <= indentStack[len(indentStack)-1] {
			pathStack = pathStack[:len(pathStack)-1]
			indentStack = indentStack[:len(indentStack)-1]
		}
		
		// Add current path to stack
		pathStack = append(pathStack, storePath)
		indentStack = append(indentStack, indent)
		
		// If this is the target path, record all paths in the stack as dependent
		if storePath == targetPath {
			for _, p := range pathStack {
				dependentPaths[p] = true
			}
		}
		
		// Build parent-child relationships
		if len(pathStack) > 1 {
			parent := pathStack[len(pathStack)-2]
			child := storePath
			dg.parents[child] = parent
		}
	}
	
	// Build closure chain from target to system closure
	closureChain := dg.FindPathToRoot(targetPath)
	
	// Convert dependent paths map to slice
	affectedPaths := make([]string, 0, len(dependentPaths))
	for path := range dependentPaths {
		affectedPaths = append(affectedPaths, path)
	}
	
	// Log all paths that need to be rewritten
	log.Printf("Found %d paths that depend on %s", len(affectedPaths), targetPath)
	log.Println("Affected paths:")
	for _, p := range affectedPaths {
		log.Printf("  %s", p)
	}
	
	return dg, closureChain, affectedPaths, nil
}

// stripAnsiCodes removes ANSI color codes from a string
func stripAnsiCodes(s string) string {
	// Simple regex to remove ANSI escape sequences
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// extractStorePathWithDir extracts a nix store path from a line with custom store dir
func extractStorePathWithDir(line, storeDir string) string {
	// Find store dir in the line
	storePrefix := storeDir + "/"
	idx := strings.Index(line, storePrefix)
	if idx == -1 {
		return ""
	}
	
	// Extract the path starting from store dir
	path := line[idx:]
	
	// Find the end of the path (space or end of line)
	endIdx := strings.IndexAny(path, " \t")
	if endIdx != -1 {
		path = path[:endIdx]
	}
	
	return path
}