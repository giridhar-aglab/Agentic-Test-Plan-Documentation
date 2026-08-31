package agent

import "testing"

func TestMostCommonReasonGroupsIdenticalCauses(t *testing.T) {
	// Every component usually fails for the same reason — an unreachable
	// endpoint, a bad key. The message must name that cause rather than an
	// arbitrary one, and the per-component prefix must not split it into
	// several apparently distinct failures.
	reasons := []string{
		`agent analyst:a.go: completion: openai-compatible: request failed: dial tcp: connection refused`,
		`agent analyst:b.go: completion: openai-compatible: request failed: dial tcp: connection refused`,
		`agent analyst:c.go: completion: openai-compatible: request failed: dial tcp: connection refused`,
	}
	got := mostCommonReason(reasons)
	if got != `openai-compatible: request failed: dial tcp: connection refused` {
		t.Fatalf("the shared cause should survive without its per-component prefix, got %q", got)
	}
}

func TestMostCommonReasonPicksTheMajority(t *testing.T) {
	reasons := []string{
		"completion: rate limited",
		"completion: rate limited",
		"completion: something else entirely",
	}
	if got := mostCommonReason(reasons); got != "rate limited" {
		t.Fatalf("expected the majority cause, got %q", got)
	}
}

func TestMostCommonReasonIsEmptyWhenNothingFailed(t *testing.T) {
	if got := mostCommonReason(nil); got != "" {
		t.Fatalf("expected no reason, got %q", got)
	}
}
