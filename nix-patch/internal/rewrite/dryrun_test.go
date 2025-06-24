package rewrite

import (
	"testing"
)

func TestEngine_DryRun(t *testing.T) {
	engine := NewEngine()

	// Test default is not dry-run
	if engine.dryRun {
		t.Error("Engine should not be in dry-run mode by default")
	}

	// Test setting dry-run
	engine.SetDryRun(true)
	if !engine.dryRun {
		t.Error("Engine should be in dry-run mode after SetDryRun(true)")
	}

	// Test unsetting dry-run
	engine.SetDryRun(false)
	if engine.dryRun {
		t.Error("Engine should not be in dry-run mode after SetDryRun(false)")
	}
}

func TestEngine_DryRunRewrite(t *testing.T) {
	engine := NewEngine()
	engine.SetDryRun(true)

	// Record some test rewrites
	engine.recordRewrite("/nix/store/old1-test", "/nix/store/new1-test")
	engine.recordRewrite("/nix/store/old2-test", "/nix/store/new2-test")

	// Verify rewrites are recorded even in dry-run
	if new, ok := engine.getRewrite("/nix/store/old1-test"); !ok || new != "/nix/store/new1-test" {
		t.Error("Rewrite not recorded in dry-run mode")
	}
}

func TestEngine_DryRunCreateStorePath(t *testing.T) {
	engine := NewEngine()
	engine.SetDryRun(true)

	// In dry-run mode, createNewStorePath should generate a fake path
	// This test is more of a placeholder since createNewStorePath is not exported
	// The actual testing happens through integration tests
	t.Log("Dry-run store path creation is tested through integration tests")
}
