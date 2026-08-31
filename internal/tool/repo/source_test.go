package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingSourceDirectoryGivesAnActionableError(t *testing.T) {
	// The first thing anyone hits with a wrong path. The raw OS error is
	// unreadable, especially on Windows, so this must say what to check.
	source := NewLocalSource(filepath.Join(t.TempDir(), "not-cloned-yet"), 400)

	_, err := source.Tree(context.Background())
	var unavailable *ErrSourceUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected ErrSourceUnavailable, got %v", err)
	}
	if unavailable.Reason != "no such directory" {
		t.Errorf("unexpected reason %q", unavailable.Reason)
	}
}

func TestSourcePointingAtAFileIsRejected(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := NewLocalSource(filePath, 400)

	_, err := source.Tree(context.Background())
	var unavailable *ErrSourceUnavailable
	if !errors.As(err, &unavailable) || unavailable.Reason != "not a directory" {
		t.Fatalf("pointing at a file should be rejected clearly, got %v", err)
	}
}
