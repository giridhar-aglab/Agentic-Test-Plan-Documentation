package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/model"
	"github.com/giri-ms19/testplan-agent/internal/tool"
)

// TreeTool lists the repository. One fetch per run; everything downstream reads
// the cached result.
type TreeTool struct {
	Source Source
}

func (treeTool *TreeTool) Name() string { return "repo_tree" }

func (treeTool *TreeTool) Description() string {
	return "List every file in the repository under analysis, with its git blob SHA and size. " +
		"Call this once before reading any file; it is the only way to learn valid paths."
}

func (treeTool *TreeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
}

func (treeTool *TreeTool) Idempotent() bool { return true }

func (treeTool *TreeTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	entries, err := treeTool.Source.Tree(ctx)
	if err != nil {
		var tooManyFiles *ErrTooManyFiles
		if errors.As(err, &tooManyFiles) {
			// Above the ceiling is fatal and says so plainly, rather than
			// truncating and producing a plan that looks complete.
			return tool.Result{}, tool.NewFailure(tool.FailureFatal, treeTool.Name(), tooManyFiles.Error(), err)
		}
		return tool.Result{}, tool.NewFailure(tool.FailureFatal, treeTool.Name(),
			"the repository tree could not be read; the run cannot continue", err)
	}

	var renderedTree strings.Builder
	fmt.Fprintf(&renderedTree, "%d files\n", len(entries))
	for _, entry := range entries {
		fmt.Fprintf(&renderedTree, "%s\t%d bytes\t%s\n", entry.Path, entry.SizeBytes, entry.BlobSHA[:8])
	}
	return tool.Text(renderedTree.String(), sourceProvenance(treeTool.Source)), nil
}

// ReadFileTool returns a bounded slice of one file.
type ReadFileTool struct {
	Source Source
	// MaxLines caps a single read so one call cannot swamp the context. Larger
	// reads are still possible, just paginated.
	MaxLines int
	// MaxBytes caps the rendered result. This exists because a line cap is not
	// a size cap: 1500 lines of dense Go with line-number prefixes is several
	// times 1500 lines of sparse Go, and a result that overshoots the loop's
	// spill threshold gets digested and then re-fetched in pieces, which costs
	// far more than sending it once. A byte cap makes "this read never spills"
	// an exact property rather than an estimate. Zero means no byte cap.
	MaxBytes int
	// PathGuidance tells the model where valid paths come from. It must name
	// something the model can actually do: an agent whose registry has no
	// repo_tree spends its whole budget trying to call it, which is how a
	// helpful sentence becomes an infinite loop.
	PathGuidance string
}

const defaultPathGuidance = "Paths must be repository-relative, exactly as given to you."

func (readTool *ReadFileTool) pathGuidance() string {
	if strings.TrimSpace(readTool.PathGuidance) == "" {
		return defaultPathGuidance
	}
	return readTool.PathGuidance
}

func (readTool *ReadFileTool) Name() string { return "repo_read_file" }

func (readTool *ReadFileTool) Description() string {
	return "Read a file from the repository. Supply startLine and endLine to read part of a large file. " +
		readTool.pathGuidance()
}

func (readTool *ReadFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "properties":{
    "path":{"type":"string","description":"Repository-relative path, exactly as given to you"},
    "startLine":{"type":"integer","minimum":1},
    "endLine":{"type":"integer","minimum":1}
  },
  "required":["path"],
  "additionalProperties":false
}`)
}

func (readTool *ReadFileTool) Idempotent() bool { return true }

type readFileArguments struct {
	Path      string `json:"path"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

func (readTool *ReadFileTool) Invoke(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	var parsedArguments readFileArguments
	if err := json.Unmarshal(arguments, &parsedArguments); err != nil {
		return tool.Result{}, tool.Correctable(readTool.Name(),
			"arguments must be a JSON object with a required string field \"path\"")
	}
	if strings.TrimSpace(parsedArguments.Path) == "" {
		return tool.Result{}, tool.Correctable(readTool.Name(),
			"\"path\" is required. "+readTool.pathGuidance())
	}

	fileContent, err := readTool.Source.ReadFile(ctx, parsedArguments.Path)
	if err != nil {
		var notFound *ErrNotFound
		if errors.As(err, &notFound) {
			// The model's mistake, phrased so the model can fix it: name the
			// problem and the tool that resolves it.
			return tool.Result{}, tool.Correctable(readTool.Name(), fmt.Sprintf(
				"no such path %q in this repository. %s",
				parsedArguments.Path, readTool.pathGuidance()))
		}
		return tool.Result{}, tool.NewFailure(tool.FailureDegradable, readTool.Name(),
			fmt.Sprintf("could not read %q; continue without it", parsedArguments.Path), err)
	}

	if strings.TrimSpace(fileContent) == "" {
		// Rendering "lines 1-1 of 1" for an empty payload tells the model the
		// read succeeded and there is nothing there, so it tries another range,
		// and another. Saying the file could not be read ends that.
		return tool.Result{}, tool.NewFailure(tool.FailureDegradable, readTool.Name(),
			fmt.Sprintf("%q came back empty and cannot be analysed; "+
				"do not retry this read", parsedArguments.Path),
			fmt.Errorf("empty content for %s", parsedArguments.Path))
	}

	lines := strings.Split(fileContent, "\n")
	startLine, endLine := readTool.resolveRange(parsedArguments, len(lines))

	var renderedSlice strings.Builder
	fmt.Fprintf(&renderedSlice, "%s (lines %d-%d of %d)\n", parsedArguments.Path, startLine, endLine, len(lines))
	lastRenderedLine := startLine - 1
	for lineNumber := startLine; lineNumber <= endLine; lineNumber++ {
		if readTool.MaxBytes > 0 && renderedSlice.Len() >= readTool.MaxBytes {
			break
		}
		fmt.Fprintf(&renderedSlice, "%d\t%s\n", lineNumber, lines[lineNumber-1])
		lastRenderedLine = lineNumber
	}
	endLine = lastRenderedLine
	if endLine < len(lines) {
		// The old wording ("read them with startLine=N") was an instruction the
		// model followed, one page at a time, until its iteration budget was
		// gone — and each page had different arguments, so the repeat guard
		// never saw a repeat. Offering the next page is fine; implying the work
		// is unfinished without it is not.
		fmt.Fprintf(&renderedSlice,
			"… %d further lines were not shown. This is usually enough to analyse the file; "+
				"only read on (startLine=%d) if what you need is definitely not above.\n",
			len(lines)-endLine, endLine+1)
	}
	return tool.Text(renderedSlice.String(), sourceProvenance(readTool.Source)), nil
}

func (readTool *ReadFileTool) resolveRange(parsedArguments readFileArguments, lineCount int) (int, int) {
	maxLines := readTool.MaxLines
	if maxLines <= 0 {
		maxLines = 400
	}
	startLine := parsedArguments.StartLine
	if startLine < 1 {
		startLine = 1
	}
	if startLine > lineCount {
		startLine = lineCount
	}
	endLine := parsedArguments.EndLine
	if endLine < startLine || endLine > lineCount {
		endLine = lineCount
	}
	if endLine-startLine+1 > maxLines {
		endLine = startLine + maxLines - 1
	}
	if lineCount == 0 {
		return 0, 0
	}
	return startLine, endLine
}

// sourceProvenance records where the bytes came from, so a run that fell back
// from the GitHub server to a local clone says so in the final report.
func sourceProvenance(source Source) model.Provenance {
	if strings.HasPrefix(source.Name(), "github:") {
		return tool.MCP("github")
	}
	return tool.Internal()
}
