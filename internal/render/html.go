package render

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/memory"
)

// PlanHTML renders the same plan as a self-contained HTML page.
//
// It converts the Markdown rather than re-deriving the report, which is the
// whole point: two renderers reading the blackboard independently would drift,
// and a reviewer comparing the Markdown and the HTML would find different
// documents. One source of truth, two presentations.
func PlanHTML(blackboard *memory.Blackboard, options Options) string {
	return WrapHTML(PlanMarkdown(blackboard, options))
}

// WrapHTML converts a plan's Markdown into a standalone page.
func WrapHTML(markdown string) string {
	title := firstHeading(markdown)
	if title == "" {
		title = "Test plan"
	}
	body := markdownToHTML(markdown)

	var page strings.Builder
	page.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	page.WriteString("<meta charset=\"utf-8\">\n")
	page.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&page, "<title>%s</title>\n", html.EscapeString(title))
	page.WriteString("<style>\n" + planStylesheet + "\n</style>\n</head>\n<body>\n")
	page.WriteString("<main>\n")
	page.WriteString(body)
	page.WriteString("\n</main>\n")
	page.WriteString("<script>\n" + planScript + "\n</script>\n")
	page.WriteString("</body>\n</html>\n")
	return page.String()
}

func firstHeading(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// ---------------------------------------------------------- conversion ---

var (
	headingPattern    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	orderedPattern    = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)
	tableRulePattern  = regexp.MustCompile(`^\|[\s:|-]+\|$`)
	boldPattern       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicPattern     = regexp.MustCompile(`(^|[\s(])[_*]([^_*\n]+)[_*]([\s.,;:)]|$)`)
	codeSpanPattern   = regexp.MustCompile("`([^`]+)`")
	linkPattern       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	priorityPattern   = regexp.MustCompile(`^(P[0-3]|critical|high|medium|low)$`)
	slugUnsafePattern = regexp.MustCompile(`[^a-z0-9]+`)
)

// markdownToHTML handles the subset the plan renderer emits. It is deliberately
// not a general Markdown implementation: a partial converter that is honest
// about its scope beats a dependency, and the input is produced by code in this
// repository rather than by a person.
func markdownToHTML(markdown string) string {
	var out strings.Builder
	lines := strings.Split(markdown, "\n")

	inParagraph, inList, inOrderedList, inQuote, inCode := false, false, false, false, false

	closeBlocks := func(except string) {
		if inParagraph && except != "p" {
			out.WriteString("</p>\n")
			inParagraph = false
		}
		if inList && except != "ul" {
			out.WriteString("</ul>\n")
			inList = false
		}
		if inOrderedList && except != "ol" {
			out.WriteString("</ol>\n")
			inOrderedList = false
		}
		if inQuote && except != "blockquote" {
			out.WriteString("</blockquote>\n")
			inQuote = false
		}
	}

	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		line := lines[lineIndex]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				out.WriteString("</code></pre>\n")
				inCode = false
			} else {
				closeBlocks("")
				out.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			out.WriteString(html.EscapeString(line) + "\n")
			continue
		}

		if trimmed == "" {
			closeBlocks("")
			continue
		}

		if matches := headingPattern.FindStringSubmatch(trimmed); matches != nil {
			closeBlocks("")
			level := len(matches[1])
			text := inlineToHTML(matches[2])
			fmt.Fprintf(&out, "<h%d id=\"%s\">%s</h%d>\n", level, slug(matches[2]), text, level)
			continue
		}

		if trimmed == "---" || trimmed == "***" {
			closeBlocks("")
			out.WriteString("<hr>\n")
			continue
		}

		// A table is consumed whole: its shape spans lines, so it cannot be
		// handled one line at a time like everything else here.
		if strings.HasPrefix(trimmed, "|") {
			closeBlocks("")
			consumed := writeTable(&out, lines[lineIndex:])
			lineIndex += consumed - 1
			continue
		}

		if strings.HasPrefix(trimmed, "> ") {
			if !inQuote {
				closeBlocks("blockquote")
				out.WriteString("<blockquote>\n")
				inQuote = true
			}
			fmt.Fprintf(&out, "<p>%s</p>\n", inlineToHTML(strings.TrimPrefix(trimmed, "> ")))
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			if !inList {
				closeBlocks("ul")
				out.WriteString("<ul>\n")
				inList = true
			}
			fmt.Fprintf(&out, "<li>%s</li>\n", inlineToHTML(strings.TrimPrefix(trimmed, "- ")))
			continue
		}

		if matches := orderedPattern.FindStringSubmatch(trimmed); matches != nil {
			if !inOrderedList {
				closeBlocks("ol")
				out.WriteString("<ol>\n")
				inOrderedList = true
			}
			fmt.Fprintf(&out, "<li>%s</li>\n", inlineToHTML(matches[2]))
			continue
		}

		if !inParagraph {
			closeBlocks("p")
			out.WriteString("<p>")
			inParagraph = true
		} else {
			out.WriteString(" ")
		}
		out.WriteString(inlineToHTML(trimmed))
	}

	if inCode {
		out.WriteString("</code></pre>\n")
	}
	closeBlocks("")
	return out.String()
}

// writeTable renders a Markdown table and reports how many lines it consumed.
func writeTable(out *strings.Builder, lines []string) int {
	rows := [][]string{}
	consumed := 0
	headerSeen := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		consumed++
		if tableRulePattern.MatchString(trimmed) {
			headerSeen = true
			continue
		}
		rows = append(rows, splitTableRow(trimmed))
	}
	if len(rows) == 0 {
		return consumed
	}

	out.WriteString("<div class=\"table-wrap\">\n<table>\n")
	startIndex := 0
	if headerSeen {
		out.WriteString("<thead><tr>")
		for _, cell := range rows[0] {
			fmt.Fprintf(out, "<th>%s</th>", inlineToHTML(cell))
		}
		out.WriteString("</tr></thead>\n")
		startIndex = 1
	}
	out.WriteString("<tbody>\n")
	for _, row := range rows[startIndex:] {
		out.WriteString("<tr>")
		for _, cell := range row {
			fmt.Fprintf(out, "<td>%s</td>", cellToHTML(cell))
		}
		out.WriteString("</tr>\n")
	}
	out.WriteString("</tbody>\n</table>\n</div>\n")
	return consumed
}

func splitTableRow(line string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	cells := strings.Split(trimmed, "|")
	for index := range cells {
		cells[index] = strings.TrimSpace(cells[index])
	}
	return cells
}

// cellToHTML badges a priority or risk level so the eye can find P0 rows
// without reading them. Anything else is ordinary inline Markdown.
func cellToHTML(cell string) string {
	// The renderer bolds risk levels, so the badge test has to see through the
	// emphasis markers rather than only matching a bare word.
	unemphasised := strings.TrimSpace(strings.Trim(cell, "*_"))
	if priorityPattern.MatchString(unemphasised) {
		return fmt.Sprintf("<span class=\"badge badge-%s\">%s</span>",
			strings.ToLower(unemphasised), html.EscapeString(unemphasised))
	}
	return inlineToHTML(cell)
}

func inlineToHTML(text string) string {
	// Links are extracted before escaping so the URL survives, then reinserted
	// as placeholders — escaping first would mangle the href.
	placeholders := []string{}
	withPlaceholders := linkPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := linkPattern.FindStringSubmatch(match)
		placeholders = append(placeholders, fmt.Sprintf(`<a href="%s">%s</a>`,
			html.EscapeString(parts[2]), html.EscapeString(parts[1])))
		return fmt.Sprintf("\x00%d\x00", len(placeholders)-1)
	})

	escaped := html.EscapeString(withPlaceholders)
	escaped = codeSpanPattern.ReplaceAllString(escaped, "<code>$1</code>")
	escaped = boldPattern.ReplaceAllString(escaped, "<strong>$1</strong>")
	// Italics run after bold so "**x**" is not mistaken for two emphasis marks.
	escaped = italicPattern.ReplaceAllString(escaped, "$1<em>$2</em>$3")

	for index, replacement := range placeholders {
		escaped = strings.ReplaceAll(escaped, fmt.Sprintf("\x00%d\x00", index), replacement)
	}
	return escaped
}

func slug(text string) string {
	lowered := strings.ToLower(strings.TrimSpace(text))
	return strings.Trim(slugUnsafePattern.ReplaceAllString(lowered, "-"), "-")
}
