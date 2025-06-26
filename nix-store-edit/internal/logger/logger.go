// Package logger provides formatted output utilities
package logger

import (
	"fmt"
	"io"
	"os"
)

// Logger provides formatted output with verbosity control
type Logger struct {
	verbose bool
	writer  io.Writer
	dryRun  bool
}

// New creates a new logger instance
func New(verbose bool, dryRun bool) *Logger {
	return &Logger{
		verbose: verbose,
		writer:  os.Stdout,
		dryRun:  dryRun,
	}
}

// Step logs a major step in the process
func (l *Logger) Step(message string) {
	prefix := "→"
	if l.dryRun {
		prefix = "→ [DRY RUN]"
	}
	fmt.Fprintf(l.writer, "\n%s %s\n", prefix, message)
}

// NumberedStep logs a numbered step in the process
func (l *Logger) NumberedStep(number int, message string) {
	prefix := fmt.Sprintf("→ %d.", number)
	if l.dryRun {
		prefix = fmt.Sprintf("→ [DRY RUN] %d.", number)
	}
	fmt.Fprintf(l.writer, "\n%s %s\n", prefix, message)
}

// Info logs informational messages
func (l *Logger) Info(message string) {
	fmt.Fprintf(l.writer, "  %s\n", message)
}

// Success logs success messages
func (l *Logger) Success(message string) {
	prefix := "✓"
	if l.dryRun {
		prefix = "✓ [DRY RUN]"
	}
	fmt.Fprintf(l.writer, "\n%s %s\n", prefix, message)
}

// PathMapping shows a path transformation
func (l *Logger) PathMapping(oldPath, newPath string) {
	fmt.Fprintf(l.writer, "\n%s -> %s\n", oldPath, newPath)
}

// Command shows a command that will be executed
func (l *Logger) Command(cmd string) {
	fmt.Fprintf(l.writer, "%s\n", cmd)
}

// Warning logs warning messages
func (l *Logger) Warning(message string) {
	fmt.Fprintf(os.Stderr, "⚠ %s\n", message)
}

// Error logs error messages
func (l *Logger) Error(message string) {
	fmt.Fprintf(os.Stderr, "✗ %s\n", message)
}

// Verbose logs messages only in verbose mode
func (l *Logger) Verbose(message string) {
	if l.verbose {
		fmt.Fprintf(l.writer, "  %s\n", message)
	}
}

// Progress shows a progress indicator
func (l *Logger) Progress(current, total int, item string) {
	// Show progress in the format [current/total] path
	fmt.Fprintf(l.writer, "[%d/%d] %s\n", current, total, item)
}

// ShowDiff shows a formatted diff output
func (l *Logger) ShowDiff(diff string) {
	// Always show the full diff as per FORMAT.md
	fmt.Fprintln(l.writer, diff)
}

// ListItems shows a list of items, condensed in non-verbose mode
func (l *Logger) ListItems(prefix string, items []string) {
	if l.verbose || len(items) <= 3 {
		fmt.Fprintf(l.writer, "%s\n", prefix)
		for _, item := range items {
			fmt.Fprintf(l.writer, "  • %s\n", item)
		}
	} else {
		// In non-verbose mode, show condensed list
		fmt.Fprintf(l.writer, "%s (%d items)\n", prefix, len(items))
		if len(items) > 0 {
			fmt.Fprintf(l.writer, "  • %s\n", items[0])
			if len(items) > 1 {
				fmt.Fprintf(l.writer, "  • ... and %d more\n", len(items)-1)
			}
		}
	}
}

// Header shows the welcome message and configuration
func (l *Logger) Header(systemType, editor, systemClosure, targetPath string) {
	fmt.Fprintln(l.writer, "-> Welcome to nix-store-edit!")
	if systemType != "" {
		fmt.Fprintf(l.writer, "Using system type override: %s\n", systemType)
	}
	fmt.Fprintf(l.writer, "Editor: %s\n", editor)
	if l.verbose {
		fmt.Fprintf(l.writer, "System closure: %s\n", systemClosure)
		fmt.Fprintf(l.writer, "Target store path: %s\n", targetPath)
	}
}

// SetWriter allows changing the output writer (useful for testing)
func (l *Logger) SetWriter(w io.Writer) {
	l.writer = w
}

// IsVerbose returns whether verbose mode is enabled
func (l *Logger) IsVerbose() bool {
	return l.verbose
}
