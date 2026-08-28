// Package render turns blackboard state into the human-readable report. It is
// entirely deterministic and holds no provider dependency, so the same
// blackboard always renders the same document and two runs can be diffed.
package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/model"
)

// SurveyMarkdown renders the survey portion of the report. The full plan
// renderer builds on this once the later phases land.
func SurveyMarkdown(blackboard *memory.Blackboard) string {
	var reportBuilder strings.Builder
	repoMap := blackboard.RepoMap()

	if repoMap == nil {
		reportBuilder.WriteString("# Test Plan\n\nThe survey phase did not complete; there is nothing to report.\n")
		writeGaps(&reportBuilder, blackboard)
		return reportBuilder.String()
	}

	fmt.Fprintf(&reportBuilder, "# Test Plan — %s\n\n", repoMap.RepositoryName)

	// Verdict summary first: what a reviewer needs to decide where to spend
	// attention comes before the material they are deciding about.
	reportBuilder.WriteString("## Summary\n\n")
	selectedFiles := repoMap.SelectedFiles()
	fmt.Fprintf(&reportBuilder, "- Commit: `%s`\n", shortSHA(repoMap.CommitSHA))
	fmt.Fprintf(&reportBuilder, "- Languages: %s\n", joinLanguages(repoMap.Languages))
	fmt.Fprintf(&reportBuilder, "- Files: %d total, %d selected for analysis\n", len(repoMap.Files), len(selectedFiles))
	fmt.Fprintf(&reportBuilder, "- Existing tests found: %d\n", len(repoMap.ExistingTests))
	if repoMap.TestFrameworkName != "" {
		fmt.Fprintf(&reportBuilder, "- Test framework: %s\n", repoMap.TestFrameworkName)
	}
	if repoMap.BuildSystemNotes != "" {
		fmt.Fprintf(&reportBuilder, "- Build: %s\n", repoMap.BuildSystemNotes)
	}
	reportBuilder.WriteString("\n")

	writeScope(&reportBuilder, *repoMap)
	writeDependencies(&reportBuilder, *repoMap)
	writeExistingCoverage(&reportBuilder, *repoMap)
	writeGaps(&reportBuilder, blackboard)
	return reportBuilder.String()
}

func writeScope(reportBuilder *strings.Builder, repoMap model.RepoMap) {
	reportBuilder.WriteString("## Scope\n\n")
	reportBuilder.WriteString("### Selected for analysis\n\n")
	selectedFiles := repoMap.SelectedFiles()
	if len(selectedFiles) == 0 {
		reportBuilder.WriteString("_No analysable source files were found._\n\n")
	} else {
		reportBuilder.WriteString("| File | Language | Size |\n|---|---|---|\n")
		for _, sourceFile := range selectedFiles {
			fmt.Fprintf(reportBuilder, "| `%s` | %s | %d B |\n",
				sourceFile.Path, sourceFile.Language, sourceFile.SizeBytes)
		}
		reportBuilder.WriteString("\n")
	}

	excludedByReason := map[string][]string{}
	for _, sourceFile := range repoMap.Files {
		if sourceFile.ExcludedReason == "" {
			continue
		}
		excludedByReason[sourceFile.ExcludedReason] = append(excludedByReason[sourceFile.ExcludedReason], sourceFile.Path)
	}
	if len(excludedByReason) > 0 {
		reportBuilder.WriteString("### Excluded\n\n")
		reasons := make([]string, 0, len(excludedByReason))
		for reason := range excludedByReason {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			fmt.Fprintf(reportBuilder, "- **%s** (%d): %s\n",
				reason, len(excludedByReason[reason]), joinCode(excludedByReason[reason], 6))
		}
		reportBuilder.WriteString("\n")
	}

	if len(repoMap.Entrypoints) > 0 {
		reportBuilder.WriteString("### Entrypoints\n\n")
		for _, entrypoint := range repoMap.Entrypoints {
			fmt.Fprintf(reportBuilder, "- `%s` at `%s:%d`\n", entrypoint.Symbol, entrypoint.Path, entrypoint.StartLine)
		}
		reportBuilder.WriteString("\n")
	}
}

func writeDependencies(reportBuilder *strings.Builder, repoMap model.RepoMap) {
	if len(repoMap.Dependencies) == 0 {
		return
	}
	reportBuilder.WriteString("## External dependencies\n\n")
	reportBuilder.WriteString("Each of these is a boundary a test must either cross or stub.\n\n")
	reportBuilder.WriteString("| Package | Version | Direct |\n|---|---|---|\n")
	for _, dependency := range repoMap.Dependencies {
		directLabel := "indirect"
		if dependency.Direct {
			directLabel = "direct"
		}
		fmt.Fprintf(reportBuilder, "| `%s` | %s | %s |\n", dependency.Name, dependency.Version, directLabel)
	}
	reportBuilder.WriteString("\n")
}

func writeExistingCoverage(reportBuilder *strings.Builder, repoMap model.RepoMap) {
	reportBuilder.WriteString("## Existing coverage\n\n")
	if len(repoMap.ExistingTests) == 0 {
		reportBuilder.WriteString("No existing tests were found. Every scenario below is new work.\n\n")
		return
	}
	reportBuilder.WriteString("| Test | Declared in | Appears to cover |\n|---|---|---|\n")
	for _, existingTest := range repoMap.ExistingTests {
		fmt.Fprintf(reportBuilder, "| `%s` | `%s` | `%s` |\n",
			existingTest.TestName, existingTest.Path, existingTest.CoversRef.Symbol)
	}
	reportBuilder.WriteString("\n")
}

func writeGaps(reportBuilder *strings.Builder, blackboard *memory.Blackboard) {
	gaps := blackboard.Gaps()
	degradedTools := blackboard.DegradedTools()
	if len(gaps) == 0 && len(degradedTools) == 0 {
		return
	}
	// Gaps are never omitted. A plan that quietly skipped three files is worse
	// than one that says which three and why.
	reportBuilder.WriteString("## Gaps\n\n")
	for _, gap := range gaps {
		fmt.Fprintf(reportBuilder, "- **%s** — `%s`: %s\n", gap.Phase, gap.Subject, gap.Reason)
	}
	if len(degradedTools) > 0 {
		toolNames := make([]string, 0, len(degradedTools))
		for toolName := range degradedTools {
			toolNames = append(toolNames, toolName)
		}
		sort.Strings(toolNames)
		for _, toolName := range toolNames {
			fmt.Fprintf(reportBuilder, "- **degraded tool** — `%s`: %s\n", toolName, degradedTools[toolName])
		}
	}
	reportBuilder.WriteString("\n")
}

func shortSHA(commitSHA string) string {
	if len(commitSHA) > 8 {
		return commitSHA[:8]
	}
	if commitSHA == "" {
		return "working tree"
	}
	return commitSHA
}

func joinLanguages(languages []model.Language) string {
	if len(languages) == 0 {
		return "none detected"
	}
	rendered := make([]string, 0, len(languages))
	for _, language := range languages {
		rendered = append(rendered, string(language))
	}
	return strings.Join(rendered, ", ")
}

func joinCode(values []string, maximumShown int) string {
	shown := values
	suffix := ""
	if len(values) > maximumShown {
		shown = values[:maximumShown]
		suffix = fmt.Sprintf(" … and %d more", len(values)-maximumShown)
	}
	quoted := make([]string, 0, len(shown))
	for _, value := range shown {
		quoted = append(quoted, "`"+value+"`")
	}
	return strings.Join(quoted, ", ") + suffix
}
