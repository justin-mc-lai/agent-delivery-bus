package main

// Escalated inbox class (feature agent-supervisor, v0.1.5): TC-AS-011.
//
// AC-009 grew a third class: the router's own escalate verdicts, read out of
// .beacon/adbd-decisions.jsonl, are questions a human still has to answer. The
// test drives the real CLI over a real temp root, so the config key, the trace
// reader, the merge and the text rendering are all exercised for real. Nothing
// calls real paseo and nothing calls the network: the fake paseo from
// supervise_test.go answers `ls --json` and the decision trace is a file the
// test writes.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setEscalationWindow appends adbd_inbox_escalation_window to the harness config
// so the real config loader parses it.
func (h *supHarness) setEscalationWindow(window time.Duration) {
	h.t.Helper()
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		h.t.Fatalf("read config: %v", err)
	}
	data = append(data, []byte("adbd_inbox_escalation_window="+window.String()+"\n")...)
	if err := os.WriteFile(h.configPath, data, 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

// seedDecision appends one router verdict to .beacon/adbd-decisions.jsonl.
func (h *supHarness) seedDecision(thread, decision, reasonCode string, at time.Time) {
	h.t.Helper()
	path := filepath.Join(h.beacon, decisionsFile)
	rec := traceRecord{
		Channel:     "feishu",
		Thread:      thread,
		MessageID:   "m-" + thread,
		Decision:    decision,
		ReasonCode:  reasonCode,
		BatchSize:   1,
		Untrusted:   true,
		DecidedAt:   at.UTC().Format(time.RFC3339),
		SourceTrust: "untrusted",
	}
	data, err := json.Marshal(rec)
	if err != nil {
		h.t.Fatalf("marshal decision: %v", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		h.t.Fatalf("open decisions file: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		h.t.Fatalf("write decision: %v", err)
	}
}

// TestInboxShowsEscalations is TC-AS-011: a 24h escalation is a live question, an
// older one is history, and an inbox with none of the three classes is NOTHING.
func TestInboxShowsEscalations(t *testing.T) {
	h := newSupHarness(t)
	h.setEscalationWindow(24 * time.Hour)
	// One supervision row, so the merge and the shared ordering are real too.
	h.bind("skill:demo:attn", "ag_attn")
	h.fake.writeAgents(fakeAgent{
		AgentID: "ag_attn", Status: StatusIdle,
		Labels:         map[string]string{OwnershipLabel: "skill:demo:attn"},
		LastActivityAt: h.now.Add(-10 * time.Minute).Format(time.RFC3339),
	})
	h.report("skill:demo:attn", ReportAttention, h.now.Add(-10*time.Minute))

	// Inside the window: a router escalation from an hour ago.
	h.seedDecision("skill:demo:escalated", DecisionEscalate, ReasonDispatchAttemptsExhausted, h.now.Add(-time.Hour))
	// Outside the window: same decision, 30 hours old.
	h.seedDecision("skill:demo:stale", DecisionEscalate, ReasonDispatchAttemptsExhausted, h.now.Add(-30*time.Hour))
	// Inside the window but not an escalation: never an inbox row.
	h.seedDecision("skill:demo:routed", DecisionRoute, ReasonDispatchAttemptsExhausted, h.now.Add(-time.Hour))

	code, stdout, stderr := h.cli("inbox")
	if code != 0 {
		t.Fatalf("inbox exit = %d (stderr: %s)", code, stderr)
	}
	lines := nonEmptyLines(stdout)
	if len(lines) != 2 {
		t.Fatalf("inbox printed %d lines (%q), want the escalation plus waiting_input", len(lines), stdout)
	}
	// Longest wait first across classes: 1h escalation, then 10m waiting input.
	expectLines(t, lines[0],
		"class="+InboxClassEscalated,
		"item=skill:demo:escalated",
		"reason_code="+ReasonDispatchAttemptsExhausted,
		"waiting=1h",
	)
	expectLines(t, lines[1],
		"class="+SupStateWaitingInput,
		"item=skill:demo:attn",
		"waiting=10m0s",
		"agent=ag_attn",
	)
	// AC-009: the out-of-window escalation and the non-escalation verdict never
	// reach a human.
	for _, gone := range []string{"skill:demo:stale", "skill:demo:routed"} {
		if strings.Contains(stdout, gone) {
			t.Fatalf("inbox printed %q, which the window or the decision filter excludes\n%s", gone, stdout)
		}
	}

	// --outcome json keeps the existing structure and adds the escalated item to
	// the same list, marked with its class.
	code, stdout, stderr = h.cli("inbox", "--outcome", "json")
	if code != 0 {
		t.Fatalf("json inbox exit = %d (stderr: %s)", code, stderr)
	}
	var payload InboxResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("json inbox is not JSON: %v (%q)", err, stdout)
	}
	if payload.Count != 2 || len(payload.Entries) != 2 {
		t.Fatalf("json inbox = count=%d entries=%+v, want 2", payload.Count, payload.Entries)
	}
	if payload.Entries[0].Class != InboxClassEscalated {
		t.Fatalf("first entry class = %q (%+v), want %q",
			payload.Entries[0].Class, payload.Entries[0], InboxClassEscalated)
	}
	if payload.Entries[0].Item != "skill:demo:escalated" ||
		payload.Entries[0].ReasonCode != ReasonDispatchAttemptsExhausted ||
		payload.Entries[0].WaitingSeconds != 3600 {
		t.Fatalf("escalated entry = %+v, want skill:demo:escalated / %s / 3600s",
			payload.Entries[0], ReasonDispatchAttemptsExhausted)
	}
	if payload.Entries[1].Class != SupStateWaitingInput {
		t.Fatalf("second entry class = %q (%+v), want %q",
			payload.Entries[1].Class, payload.Entries[1], SupStateWaitingInput)
	}

	// A read-only command: one ls and nothing else (an escalated row is read from
	// a file, never fetched from paseo).
	h.fake.onlyLs()

	// AC-009: no waiting class, no escalated class, and no decisions file at all
	// is still NOTHING with exit 0 -- a missing file is not an error.
	empty := newSupHarness(t)
	empty.setEscalationWindow(24 * time.Hour)
	empty.fake.writeAgents(fakeAgent{
		AgentID: "ag_work", Status: StatusRunning,
		Labels: map[string]string{OwnershipLabel: "skill:demo:work"},
	})
	empty.bind("skill:demo:work", "ag_work")
	code, stdout, stderr = empty.cli("inbox")
	if code != 0 {
		t.Fatalf("empty inbox exit = %d (stderr: %s)", code, stderr)
	}
	if stdout != InboxNothing+"\n" {
		t.Fatalf("empty inbox printed %q, want %q", stdout, InboxNothing+"\n")
	}
	code, stdout, _ = empty.cli("inbox", "--outcome", "json")
	if code != 0 {
		t.Fatalf("empty json inbox exit = %d", code)
	}
	var emptyPayload InboxResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &emptyPayload); err != nil {
		t.Fatalf("empty json inbox is not JSON: %v (%q)", err, stdout)
	}
	if emptyPayload.Count != 0 || len(emptyPayload.Entries) != 0 {
		t.Fatalf("empty json inbox = %+v, want count=0", emptyPayload)
	}

	// An escalation that is the only question is still the whole inbox: the
	// window is what keeps the row, not the presence of another class.
	only := newSupHarness(t)
	only.setEscalationWindow(24 * time.Hour)
	only.fake.writeAgents(fakeAgent{
		AgentID: "ag_work", Status: StatusRunning,
		Labels: map[string]string{OwnershipLabel: "skill:demo:work"},
	})
	only.bind("skill:demo:work", "ag_work")
	only.seedDecision("skill:demo:only", DecisionEscalate, ReasonDispatchAttemptsExhausted, only.now.Add(-2*time.Hour))
	code, stdout, stderr = only.cli("inbox")
	if code != 0 {
		t.Fatalf("escalation-only inbox exit = %d (stderr: %s)", code, stderr)
	}
	if lines := nonEmptyLines(stdout); len(lines) != 1 {
		t.Fatalf("escalation-only inbox printed %d lines (%q), want exactly the escalation", len(lines), stdout)
	}
	expectLines(t, stdout,
		"class="+InboxClassEscalated,
		"item=skill:demo:only",
		"waiting=2h",
	)

	// The window is the configuration, not a constant: 1h hides the 2h-old row.
	only.setEscalationWindow(time.Hour)
	code, stdout, stderr = only.cli("inbox")
	if code != 0 {
		t.Fatalf("narrow-window inbox exit = %d (stderr: %s)", code, stderr)
	}
	if stdout != InboxNothing+"\n" {
		t.Fatalf("narrow window printed %q, want %q", stdout, InboxNothing+"\n")
	}
}
