package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 3, 4, 9, 7, 0, 0, time.UTC) }
}

func TestTwoRepositoriesDoNotOverwriteEachOther(t *testing.T) {
	// The old default was a fixed "plan.md", so analysing a second repository
	// destroyed the first one's plan with no warning. That is data loss, not a
	// convenience.
	directory := t.TempDir()
	first, err := writeReports("# one\n", reportDestination{
		Directory: directory, RepositoryName: "gin-gonic/gin", Format: "md", nowFunc: fixedClock(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := writeReports("# two\n", reportDestination{
		Directory: directory, RepositoryName: "spf13/cobra", Format: "md", nowFunc: fixedClock(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first[0] == second[0] {
		t.Fatalf("both repositories wrote to %s", first[0])
	}
	for _, path := range []string{first[0], second[0]} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s should exist: %v", path, err)
		}
	}
	if !strings.Contains(filepath.Base(first[0]), "gin-gonic-gin") {
		t.Errorf("the filename should name its repository, got %s", filepath.Base(first[0]))
	}
}

func TestTwoRunsOfOneRepositoryAreDistinguishable(t *testing.T) {
	// Re-running after a fix and comparing the two plans is the main way this
	// report gets used. That needs both files to still exist.
	directory := t.TempDir()
	morning, _ := writeReports("# a\n", reportDestination{
		Directory: directory, RepositoryName: "gin", CommitSHA: "abc1234def5678", Format: "md",
		nowFunc: func() time.Time { return time.Date(2026, 3, 4, 9, 7, 0, 0, time.UTC) },
	})
	afternoon, _ := writeReports("# b\n", reportDestination{
		Directory: directory, RepositoryName: "gin", CommitSHA: "abc1234def5678", Format: "md",
		nowFunc: func() time.Time { return time.Date(2026, 3, 4, 14, 30, 0, 0, time.UTC) },
	})
	if morning[0] == afternoon[0] {
		t.Fatalf("two runs collided on %s", morning[0])
	}
	if !strings.Contains(morning[0], "abc1234") {
		t.Errorf("the commit should be identifiable in the name, got %s", morning[0])
	}
}

func TestBothFormatsAreWrittenByDefault(t *testing.T) {
	directory := t.TempDir()
	written, err := writeReports("# plan\n\n| A |\n| --- |\n| 1 |\n", reportDestination{
		Directory: directory, RepositoryName: "svc", nowFunc: fixedClock(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("expected markdown and html, got %v", written)
	}
	htmlBytes, err := os.ReadFile(written[1])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(htmlBytes), "<table>") {
		t.Error("the html should be converted, not a copy of the markdown")
	}
}

func TestAnExplicitPathPicksItsOwnFormat(t *testing.T) {
	directory := t.TempDir()
	for extension, expectedMarker := range map[string]string{
		".md": "# plan", ".html": "<!doctype html>",
	} {
		target := filepath.Join(directory, "report"+extension)
		written, err := writeReports("# plan\n", reportDestination{ExplicitPath: target})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(written) != 1 || written[0] != target {
			t.Fatalf("expected exactly %s, got %v", target, written)
		}
		content, _ := os.ReadFile(target)
		if !strings.Contains(string(content), expectedMarker) {
			t.Errorf("%s should contain %q", target, expectedMarker)
		}
	}
}

func TestStdoutRemainsReachable(t *testing.T) {
	// A run piped into something else must not start writing files.
	written, err := writeReports("# plan\n", reportDestination{Format: "stdout"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(written) != 0 {
		t.Fatalf("stdout should write no files, got %v", written)
	}
}

func TestAnUnknownFormatIsRefusedByName(t *testing.T) {
	_, err := writeReports("# plan\n", reportDestination{Format: "pdf"})
	if err == nil || !strings.Contains(err.Error(), "pdf") {
		t.Fatalf("expected the bad value named in the error, got %v", err)
	}
}

func TestAWindowsHostileRepositoryNameIsFlattened(t *testing.T) {
	// "owner/repo" is the natural display name and an invalid filename.
	if got := sanitiseForFilename("gin-gonic/gin"); got != "gin-gonic-gin" {
		t.Errorf("got %q", got)
	}
	if got := sanitiseForFilename("a:b*c?d"); strings.ContainsAny(got, `:*?/\`) {
		t.Errorf("reserved characters survived: %q", got)
	}
}

func TestAnExplicitPathStillMeansThatExactPath(t *testing.T) {
	// -out has to keep meaning what it says: scripts depend on it, and
	// second-guessing an explicit instruction is worse than overwriting.
	// The warning is what makes the consequence visible.
	directory := t.TempDir()
	target := filepath.Join(directory, "plan.md")
	for range 2 {
		written, err := writeReports("# plan\n", reportDestination{ExplicitPath: target})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(written) != 1 || written[0] != target {
			t.Fatalf("expected exactly %s, got %v", target, written)
		}
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatalf("-out must not invent extra files, got %d", len(entries))
	}
}

func TestTheBannerDescribesEveryTierWhenTheyDiffer(t *testing.T) {
	// The banner reported only the balanced tier, so setting a stronger model
	// for the Author and Critic changed the run but not the line describing it.
	// A tool that misreports its own configuration is worse than one that says
	// nothing: the reader stops trusting the rest of the output.
	captured := captureStderr(t, func() {
		describeBackend("openai-compatible", "https://api.openai.com/v1", map[llm.Tier]string{
			llm.TierFast: "gpt-4o-mini", llm.TierBalanced: "gpt-4o-mini", llm.TierStrong: "gpt-4o",
		})
	})
	for _, expected := range []string{"gpt-4o-mini", "gpt-4o", "author + critic"} {
		if !strings.Contains(captured, expected) {
			t.Errorf("the banner should mention %q, got:\n%s", expected, captured)
		}
	}
}

func TestTheBannerStaysOneLineWhenOneModelServesEverything(t *testing.T) {
	captured := captureStderr(t, func() {
		describeBackend("openai-compatible", "http://localhost:11434/v1", map[llm.Tier]string{
			llm.TierFast: "qwen", llm.TierBalanced: "qwen", llm.TierStrong: "qwen",
		})
	})
	if strings.Count(strings.TrimSpace(captured), "\n") != 0 {
		t.Errorf("one model should print one line, got:\n%s", captured)
	}
}

func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	run()
	_ = writer.Close()
	os.Stderr = original

	captured, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(captured)
}
