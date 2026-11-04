package main

import (
	"strings"
	"testing"
)

// escalateDecider answers every item with "escalate", which is what the LLM
// decider did in production for an unbound issue. It proves the binding gate is
// a pre-filter rather than a branch of the route verdict.
type escalateDecider struct{}

func (escalateDecider) Decide(items []QueueItem) ([]Decision, error) {
	out := make([]Decision, 0, len(items))
	for _, it := range items {
		out = append(out, Decision{
			Channel:    it.Channel,
			Thread:     it.Thread,
			MessageID:  it.MessageID,
			Decision:   DecisionEscalate,
			ReasonCode: "unclear_request",
		})
	}
	return out, nil
}

// gh-bug-binding AC-003: an issue that names no frozen AC is refused and kept
// queued even when the decider answers something other than "route".
func TestUnboundBugRefusedDespiteEscalate(t *testing.T) {
	h := newBugHarness(t, true, true)
	repo := t.TempDir()
	writeBugTruth(t, repo, "frozen", bugAC)
	h.register(bugRepoKey, repo)
	h.writeConfig(bugConfig("on"))
	const messageID = "m-unbound-escalate"
	h.enqueue(bugIssue(messageID, "bug"))
	h.router.Decider = escalateDecider{}

	code, out, stderr := h.cli()
	if code != 0 {
		t.Fatalf("route --once exit = %d, want 0 (refusal is not an error; stderr=%q)", code, stderr)
	}
	if !strings.Contains(out, "reason_code="+ReasonIssueUnbound) {
		t.Fatalf("route output = %q, want reason_code=%s", out, ReasonIssueUnbound)
	}
	if got := h.invocations(); len(got) != 0 {
		t.Fatalf("paseo was called %v, want zero calls", got)
	}
	if q := h.queue(); len(q) != 1 || q[0].MessageID != messageID {
		t.Fatalf("queue after refusal = %+v, want the item kept", q)
	}
	rec := h.traceFor(messageID)
	if rec.Decision != DecisionRefuse || rec.ReasonCode != ReasonIssueUnbound {
		t.Fatalf("trace = %+v, want refuse/%s even though the decider said %s",
			rec, ReasonIssueUnbound, DecisionEscalate)
	}
}
