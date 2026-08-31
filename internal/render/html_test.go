package render

import (
	"regexp"
	"strings"
	"testing"
)

func TestConvertedPlanCarriesNoUnrenderedMarkup(t *testing.T) {
	// Leftover asterisks and underscores are the tell that a construct was
	// missed. A reviewer seeing "*Steps:*" in a report loses confidence in
	// everything else on the page.
	markdown := strings.Join([]string{
		"# Test Plan — svc",
		"",
		"## Summary",
		"- **3 scenarios**. Priority mix: 2 P2",
		"- `TS-001` covers `RISK-a`",
		"",
		"| Risk | Level | Score |",
		"| --- | --- | --- |",
		"| `RISK-a` | **critical** | 42.0 |",
		"| `RISK-b` | low | 1.0 |",
		"",
		"*Steps:*",
		"",
		"1. Call Post",
		"2. Record the error",
		"",
		"_Deterministic scoring: branches ×1.0._",
		"",
		"> Low confidence (0.3) — worth a close read.",
		"",
		"See [the source](https://example.com/a.go#L10).",
	}, "\n")

	page := WrapHTML(markdown)
	body := page[strings.Index(page, "<main>"):]

	if strings.Contains(body, "*") {
		t.Errorf("an asterisk survived conversion:\n%s", body)
	}
	if regexp.MustCompile(`(^|[\s>])_[A-Za-z]`).MatchString(body) {
		t.Errorf("an underscore survived conversion:\n%s", body)
	}
}

func TestRiskLevelsBecomeBadgesEvenWhenEmphasised(t *testing.T) {
	// The Markdown renderer bolds risk levels. Matching only a bare word would
	// silently drop the badge on every row that matters most.
	page := WrapHTML("| Risk | Level |\n| --- | --- |\n| `R-1` | **critical** |\n| `R-2` | P0 |\n")
	for _, expected := range []string{
		`<span class="badge badge-critical">critical</span>`,
		`<span class="badge badge-p0">P0</span>`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("expected %s in the page", expected)
		}
	}
}

func TestLinksSurviveEscaping(t *testing.T) {
	// Escaping before extracting the URL mangles the href, and a source link
	// that does not resolve defeats the point of having one.
	page := WrapHTML("See [Post](https://example.com/a.go?x=1&y=2#L10).")
	if !strings.Contains(page, `href="https://example.com/a.go?x=1&amp;y=2#L10"`) {
		t.Fatalf("the link was not preserved:\n%s", page)
	}
}

func TestUserContentCannotInjectMarkup(t *testing.T) {
	// Responsibilities and titles come from a model, so the report renders
	// untrusted text. It must arrive as text.
	page := WrapHTML("## Findings\n\n- <script>alert(1)</script> and <img src=x onerror=y>\n")
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Fatal("model-supplied text must not become live markup")
	}
	if !strings.Contains(page, "&lt;script&gt;") {
		t.Fatal("the text should still be readable, escaped")
	}
}

func TestEveryHeadingIsAddressable(t *testing.T) {
	// The table of contents links to heading ids; a heading without one is
	// unreachable from the sidebar.
	page := WrapHTML("# Plan\n\n## Risk register\n\n### gateway/gateway\n")
	for _, expected := range []string{`id="plan"`, `id="risk-register"`, `id="gateway-gateway"`} {
		if !strings.Contains(page, expected) {
			t.Errorf("expected %s", expected)
		}
	}
}

func TestTablesAreDelimitedFromSurroundingProse(t *testing.T) {
	page := WrapHTML("Before.\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\nAfter.\n")
	if strings.Count(page, "<table>") != 1 || strings.Count(page, "</table>") != 1 {
		t.Fatalf("expected exactly one closed table:\n%s", page)
	}
	if !strings.Contains(page, "<p>Before.</p>") || !strings.Contains(page, "<p>After.</p>") {
		t.Fatalf("prose either side of a table must survive:\n%s", page)
	}
}

func TestThePageIsSelfContained(t *testing.T) {
	// The report gets mailed around and opened offline. A stylesheet or script
	// fetched from elsewhere would leave it unstyled, or leak where it was read.
	page := WrapHTML("# Plan\n\nBody.\n")
	for _, forbidden := range []string{"src=\"http", "href=\"http", "@import", "//cdn"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page reaches outside itself: %q", forbidden)
		}
	}
}
