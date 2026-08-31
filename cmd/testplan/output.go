package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/giri-ms19/testplan-agent/internal/render"
)

// reportDestination describes where a run's report should land.
//
// The default is a name derived from the repository and the run, not a fixed
// "plan.md": analysing a second repository silently overwriting the first one's
// plan is a data-loss bug, not a convenience.
type reportDestination struct {
	// ExplicitPath is -out. When set it wins, and the extension picks the
	// format unless -format overrides it.
	ExplicitPath string
	// Format is md, html, both, or empty to infer.
	Format string
	// Directory receives generated names when ExplicitPath is empty.
	Directory string
	// RepositoryName and CommitSHA make the generated name identify its run.
	RepositoryName string
	CommitSHA      string

	// nowFunc exists so the generated name is testable.
	nowFunc func() time.Time
}

var unsafeNameCharacters = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// writeReports writes every requested format and returns the paths written.
// An empty result means the caller should print to stdout instead.
func writeReports(reportMarkdown string, destination reportDestination) ([]string, error) {
	formats, err := destination.formats()
	if err != nil {
		return nil, err
	}
	if len(formats) == 0 {
		return nil, nil
	}

	writtenPaths := make([]string, 0, len(formats))
	for _, format := range formats {
		targetPath := destination.pathFor(format)
		if directory := filepath.Dir(targetPath); directory != "." && directory != "" {
			if err := os.MkdirAll(directory, 0o755); err != nil {
				return nil, fmt.Errorf("create %s: %w", directory, err)
			}
		}

		content := reportMarkdown
		if format == "html" {
			content = render.WrapHTML(reportMarkdown)
		}
		// -out means "this exact path", which is the right behaviour and also a
		// quiet way to lose the previous repository's plan. Say so once rather
		// than letting the overwrite be silent.
		overwriting := false
		if destination.ExplicitPath != "" {
			if _, statErr := os.Stat(targetPath); statErr == nil {
				overwriting = true
			}
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write report: %w", err)
		}
		if overwriting {
			fmt.Fprintf(os.Stderr,
				"note: -out overwrote the existing %s. Omit -out to get "+
					"reports/<repo>_<commit>_<time>.md and .html instead, so runs accumulate.\n",
				targetPath)
		}
		writtenPaths = append(writtenPaths, targetPath)
	}
	return writtenPaths, nil
}

// formats resolves which formats to write. Stdout stays reachable — a run piped
// into something else must not start writing files — but only by asking for it.
func (destination reportDestination) formats() ([]string, error) {
	requested := strings.ToLower(strings.TrimSpace(destination.Format))
	switch requested {
	case "md", "markdown":
		return []string{"md"}, nil
	case "html":
		return []string{"html"}, nil
	case "both":
		return []string{"md", "html"}, nil
	case "-", "stdout":
		return nil, nil
	case "":
	default:
		return nil, fmt.Errorf("unknown -format %q; use md, html, both, or stdout", destination.Format)
	}

	if destination.ExplicitPath != "" {
		if strings.EqualFold(filepath.Ext(destination.ExplicitPath), ".html") {
			return []string{"html"}, nil
		}
		return []string{"md"}, nil
	}
	// Both, by default: the Markdown is what gets diffed and committed, the
	// HTML is what gets read and shown to someone.
	return []string{"md", "html"}, nil
}

func (destination reportDestination) pathFor(format string) string {
	if destination.ExplicitPath != "" {
		if len(destination.formatsIgnoringError()) == 1 {
			return destination.ExplicitPath
		}
		// -out with -format both: keep the stem, vary the extension, so the two
		// files are obviously the same report.
		extension := filepath.Ext(destination.ExplicitPath)
		stem := strings.TrimSuffix(destination.ExplicitPath, extension)
		return stem + "." + format
	}
	return filepath.Join(destination.Directory, destination.generatedName()+"."+format)
}

func (destination reportDestination) formatsIgnoringError() []string {
	formats, _ := destination.formats()
	return formats
}

// generatedName identifies the repository and the run, so successive runs
// accumulate rather than overwrite and a reviewer can tell two plans apart
// without opening them.
func (destination reportDestination) generatedName() string {
	nameParts := []string{sanitiseForFilename(destination.RepositoryName)}
	if nameParts[0] == "" {
		nameParts[0] = "testplan"
	}
	if shortSHA := shortCommitSHA(destination.CommitSHA); shortSHA != "" {
		nameParts = append(nameParts, shortSHA)
	}
	nameParts = append(nameParts, destination.now().Format("2006-01-02-1504"))
	return strings.Join(nameParts, "_")
}

func (destination reportDestination) now() time.Time {
	if destination.nowFunc != nil {
		return destination.nowFunc()
	}
	return time.Now()
}

// sanitiseForFilename flattens a repository name into something every operating
// system accepts. "gin-gonic/gin" becomes "gin-gonic-gin".
func sanitiseForFilename(name string) string {
	flattened := strings.ReplaceAll(strings.TrimSpace(name), "/", "-")
	flattened = unsafeNameCharacters.ReplaceAllString(flattened, "-")
	return strings.Trim(flattened, "-.")
}

func shortCommitSHA(commitSHA string) string {
	trimmed := strings.TrimSpace(commitSHA)
	if len(trimmed) < 7 {
		return ""
	}
	return trimmed[:7]
}
