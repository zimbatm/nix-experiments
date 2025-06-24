package rewrite

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nix-community/go-nix/pkg/nar"
)

func TestExtractNAR(t *testing.T) {
	// Create a temporary directory with test files
	srcDir := t.TempDir()

	// Create test file structure
	testFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testDir := filepath.Join(srcDir, "subdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	execFile := filepath.Join(testDir, "exec.sh")
	if err := os.WriteFile(execFile, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create executable: %v", err)
	}

	linkTarget := filepath.Join(testDir, "link")
	if err := os.Symlink("../test.txt", linkTarget); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create NAR
	var narBuf bytes.Buffer
	if err := nar.DumpPath(&narBuf, srcDir); err != nil {
		t.Fatalf("Failed to create NAR: %v", err)
	}

	// Extract NAR to new location
	destDir := t.TempDir()
	if err := extractNAR(&narBuf, destDir); err != nil {
		t.Fatalf("extractNAR failed: %v", err)
	}

	// Verify extracted files
	// Check regular file
	extractedFile := filepath.Join(destDir, "test.txt")
	data, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("File content = %q, want %q", string(data), "hello world")
	}

	// Check directory
	extractedDir := filepath.Join(destDir, "subdir")
	info, err := os.Stat(extractedDir)
	if err != nil {
		t.Errorf("Failed to stat extracted dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory, got file")
	}

	// Check executable
	extractedExec := filepath.Join(extractedDir, "exec.sh")
	info, err = os.Stat(extractedExec)
	if err != nil {
		t.Errorf("Failed to stat extracted executable: %v", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Error("Executable permission not preserved")
	}

	// Check symlink
	extractedLink := filepath.Join(extractedDir, "link")
	target, err := os.Readlink(extractedLink)
	if err != nil {
		t.Errorf("Failed to read symlink: %v", err)
	}
	if target != "../test.txt" {
		t.Errorf("Symlink target = %q, want %q", target, "../test.txt")
	}
}

func TestExtractNAR_EmptyArchive(t *testing.T) {
	// Create empty directory
	srcDir := t.TempDir()

	// Create NAR
	var narBuf bytes.Buffer
	if err := nar.DumpPath(&narBuf, srcDir); err != nil {
		t.Fatalf("Failed to create NAR: %v", err)
	}

	// Extract NAR
	destDir := t.TempDir()
	if err := extractNAR(&narBuf, destDir); err != nil {
		t.Fatalf("extractNAR failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		t.Error("Destination directory does not exist")
	}
}

func TestExtractNARWritable(t *testing.T) {
	// Create a directory with read-only files
	srcDir := t.TempDir()

	// Create a read-only file
	readOnlyFile := filepath.Join(srcDir, "readonly.txt")
	if err := os.WriteFile(readOnlyFile, []byte("read only content"), 0444); err != nil {
		t.Fatalf("Failed to create read-only file: %v", err)
	}

	// Create NAR
	var narBuf bytes.Buffer
	if err := nar.DumpPath(&narBuf, srcDir); err != nil {
		t.Fatalf("Failed to create NAR: %v", err)
	}

	// Extract NAR with writable mode
	destDir := t.TempDir()
	if err := extractNARWritable(&narBuf, destDir); err != nil {
		t.Fatalf("extractNARWritable failed: %v", err)
	}

	// Verify file is writable
	extractedFile := filepath.Join(destDir, "readonly.txt")
	info, err := os.Stat(extractedFile)
	if err != nil {
		t.Fatalf("Failed to stat extracted file: %v", err)
	}

	if info.Mode().Perm()&0200 == 0 {
		t.Error("File is not writable after extractNARWritable")
	}

	// Test that we can write to the file
	if err := os.WriteFile(extractedFile, []byte("modified"), 0644); err != nil {
		t.Errorf("Failed to write to file after extractNARWritable: %v", err)
	}
}
