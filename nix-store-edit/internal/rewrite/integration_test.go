package rewrite

import (
	"strings"
	"testing"
)

func TestRewriteEngine_RecordAndGetRewrite(t *testing.T) {
	engine := NewEngine()

	oldPath := "/nix/store/abc123-vim"
	newPath := "/nix/store/def456-vim"

	// Record a rewrite
	engine.recordRewrite(oldPath, newPath)

	// Get the rewrite
	got, ok := engine.getRewrite(oldPath)
	if !ok {
		t.Error("Expected to find rewrite, but didn't")
	}
	if got != newPath {
		t.Errorf("Expected rewrite to be %s, got %s", newPath, got)
	}

	// Check non-existent rewrite
	_, ok = engine.getRewrite("/nix/store/nonexistent")
	if ok {
		t.Error("Expected not to find rewrite for non-existent path")
	}
}

func TestRewriteEngine_Progress(t *testing.T) {
	engine := NewEngine()

	var progressCalls []string
	engine.SetProgressCallback(func(current, total int, path string) {
		progressCalls = append(progressCalls, path)
	})

	// Create a simple test scenario
	// This would normally interact with real store paths
	// For now, we just verify the callback mechanism works

	if engine.onProgress == nil {
		t.Error("Progress callback was not set")
	}

	// Simulate progress calls
	engine.onProgress(1, 3, "/nix/store/test1")
	engine.onProgress(2, 3, "/nix/store/test2")

	if len(progressCalls) != 2 {
		t.Errorf("Expected 2 progress calls, got %d", len(progressCalls))
	}
}

func TestRewriteEngine_RollbackStack(t *testing.T) {
	engine := NewEngine()

	// Record some operations
	engine.recordRewrite("/nix/store/old1", "/nix/store/new1")
	engine.recordRewrite("/nix/store/old2", "/nix/store/new2")

	// Check rollback stack
	if len(engine.rollbackStack) != 2 {
		t.Errorf("Expected 2 rollback operations, got %d", len(engine.rollbackStack))
	}

	// Verify rollback clears state
	err := engine.rollback(nil)
	if err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Error("Expected rollback error")
	}

	if len(engine.rewrites) != 0 {
		t.Error("Expected rewrites to be cleared after rollback")
	}
	if len(engine.rollbackStack) != 0 {
		t.Error("Expected rollback stack to be cleared")
	}
}
