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

func writeScope(reportBuilder *strings.Builder, repoMap model.RepoMap) {
	reportBuilder.WriteString("## Scope\n\n")

	selectedFiles := repoMap.SelectedFiles()
	fmt.Fprintf(reportBuilder, "%d files in the commit, %d selected for analysis.\n\n",
		len(repoMap.Files), len(selectedFiles))

	if len(selectedFiles) > 0 {
		reportBuilder.WriteString("| File | Language | Size |\n|---|---|---|\n")
		for _, sourceFile := range selectedFiles {
			fmt.Fprintf(reportBuilder, "| `%s` | %s | %d B |\n",
				sourceFile.Path, sourceFile.Language, sourceFile.SizeBytes)
		}
		reportBuilder.WriteString("\n")
	}

	// What was excluded, and why, matters as much as what was included: it is
	// the difference between a scoped plan and one with an unexplained hole.
	excludedByReason := map[string][]string{}
	for _, sourceFile := range repoMap.Files {
		if sourceFile.ExcludedReason == "" {
			continue
		}
		excludedByReason[sourceFile.ExcludedReason] = append(
			excludedByReason[sourceFile.ExcludedReason], sourceFile.Path)
	}
	if len(excludedByReason) > 0 {
		reportBuilder.WriteString("**Excluded:** ")
		reasons := make([]string, 0, len(excludedByReason))
		for reason := range excludedByReason {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		renderedReasons := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			renderedReasons = append(renderedReasons,
				fmt.Sprintf("%d %s", len(excludedByReason[reason]), reason))
		}
		fmt.Fprintf(reportBuilder, "%s.\n\n", strings.Join(renderedReasons, ", "))
	}

	if len(repoMap.ExistingTests) > 0 {
		fmt.Fprintf(reportBuilder, "**Existing tests:** %d found",
			len(repoMap.ExistingTests))
		testNames := make([]string, 0, len(repoMap.ExistingTests))
		for _, existingTest := range repoMap.ExistingTests {
			testNames = append(testNames, "`"+existingTest.TestName+"`")
		}
		fmt.Fprintf(reportBuilder, " — %s.\n\n", joinWithLimit(testNames, 10))
	} else {
		reportBuilder.WriteString("**Existing tests:** none found. Every scenario below is new work.\n\n")
	}

	if len(repoMap.Dependencies) > 0 {
		directCount := 0
		for _, dependency := range repoMap.Dependencies {
			if dependency.Direct {
				directCount++
			}
		}
		fmt.Fprintf(reportBuilder, "**Dependencies:** %d direct, %d total.\n\n",
			directCount, len(repoMap.Dependencies))
	}
}

// writeGaps is never omitted when there is something to report. A plan that
// quietly skipped three files is worse than one that names which three and why.
func writeGaps(reportBuilder *strings.Builder, blackboard *memory.Blackboard) {
	gaps := blackboard.Gaps()
	degradedTools := blackboard.DegradedTools()
	if len(gaps) == 0 && len(degradedTools) == 0 {
		return
	}

	reportBuilder.WriteString("## Gaps\n\n")
	for _, gap := range gaps {
		fmt.Fprintf(reportBuilder, "- **%s** — `%s`: %s\n", gap.Phase, gap.Subject, gap.Reason)
	}
	for _, toolName := range sortedKeys(degradedTools) {
		fmt.Fprintf(reportBuilder, "- **degraded tool** — `%s`: %s\n", toolName, degradedTools[toolName])
	}
	reportBuilder.WriteString("\n")
}

func shortSHA(commitSHA string) string {
	if commitSHA == "" {
		return "working tree"
	}
	if len(commitSHA) > 8 {
		return commitSHA[:8]
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

func joinWithLimit(values []string, maximumShown int) string {
	if len(values) <= maximumShown {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:maximumShown], ", ") +
		fmt.Sprintf(" and %d more", len(values)-maximumShown)
}
