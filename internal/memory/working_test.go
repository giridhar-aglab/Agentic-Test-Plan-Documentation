package memory

import (
	"strings"
	"testing"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

func TestWorkingSetCompactsInsteadOfOverflowing(t *testing.T) {
	// The window filling must never be a failure. Old turns become a summary.
	workingSet := NewWorkingSet("you are an analyst", 1000)
	for turnNumber := 0; turnNumber < 40; turnNumber++ {
		workingSet.Append(llm.Message{
			Role: llm.RoleAssistant,
			Text: strings.Repeat("some reasoning about the code ", 20),
		})
	}
	if workingSet.CompactionCount() == 0 {
		t.Fatal("the working set should have compacted well before 40 long turns")
	}
	if workingSet.ApproxTokens() > 1000 {
		t.Fatalf("compaction must keep the context under the window, got %d tokens", workingSet.ApproxTokens())
	}
	// Dropped context must be represented, not silently lost.
	if !strings.Contains(workingSet.Messages()[0].Text, "Summary of earlier work") {
		t.Fatal("compacted history must be replaced by a summary, not discarded")
	}
}

func TestWorkingSetKeepsRecentTurns(t *testing.T) {
	workingSet := NewWorkingSet("system", 600)
	for turnNumber := 0; turnNumber < 30; turnNumber++ {
		workingSet.Append(llm.Message{Role: llm.RoleAssistant, Text: strings.Repeat("x", 200)})
	}
	workingSet.Append(llm.Message{Role: llm.RoleAssistant, Text: "the most recent thing"})

	messages := workingSet.Messages()
	if messages[len(messages)-1].Text != "the most recent thing" {
		t.Fatal("the current line of reasoning must survive compaction")
	}
}

func TestSpillStoreRoundTripsAndDigests(t *testing.T) {
	spillStore := NewSpillStore()
	largePayload := strings.Repeat("internal/some/path.go\n", 500)

	handle := spillStore.Put(largePayload)
	retrieved, found := spillStore.Get(handle)
	if !found || retrieved != largePayload {
		t.Fatal("a spilled payload must be retrievable by handle")
	}
	if _, found := spillStore.Get("spill-999-deadbeef"); found {
		t.Fatal("an unknown handle must not resolve")
	}
	if handles := spillStore.Handles(); len(handles) != 1 || handles[0] != handle {
		t.Fatalf("the store must be able to name what it holds, got %v", handles)
	}

	digest := Digest(largePayload, 100)
	if len(digest) > 400 {
		t.Fatalf("a digest must be far smaller than the payload, got %d bytes", len(digest))
	}
	// This assertion used to read "fetch the rest by handle", and it passed for
	// the whole time no tool could do that. Naming the tool is what makes the
	// instruction checkable — agent/loop_test asserts the same name is
	// registered.
	if !strings.Contains(digest, "context_fetch") {
		t.Fatal("a digest must name the tool that retrieves the rest")
	}
	if short := Digest("tiny", 100); short != "tiny" {
		t.Fatalf("a payload under the limit must pass through unchanged, got %q", short)
	}
}

func TestBlackboardRevisionTracksMutations(t *testing.T) {
	// The no-progress guard reads this counter, so it must move on every write
	// and only on a write.
	blackboard := NewBlackboard("r1")
	startingRevision := blackboard.Revision()

	blackboard.NoteGap("survey", "x.go", "unreadable")
	if blackboard.Revision() == startingRevision {
		t.Fatal("recording a gap must count as progress")
	}
	afterGap := blackboard.Revision()
	_ = blackboard.Gaps()
	if blackboard.Revision() != afterGap {
		t.Fatal("reading the blackboard must not count as progress")
	}
}
