package main

// Router-loop tests (feature router-loop, v0.1.5): TC-RL-001 .. TC-RL-008.
//
// Everything here runs against a temp dir, a stub Dispatcher and the real rule
// decider. No paseo binary and no process is ever started: the only production
// process seam (dispatch.go) is replaced by a recorder, so a failure here is
// always a logic failure, never an environment one.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// stubs
// ---------------------------------------------------------------------------

type dispatchCall struct {
	thread  string
	agentID string
	text    string
}

// stubDispatcher records what it was asked to send and can be told to fail.
type stubDispatcher struct {
	calls   []dispatchCall
	failFor func(call dispatchCall) error
}

func (d *stubDispatcher) Dispatch(thread, agentID, text string) error {
	call := dispatchCall{thread: thread, agentID: agentID, text: text}
	d.calls = append(d.calls, call)
	if d.failFor != nil {
		return d.failFor(call)
	}
	return nil
}

// countingDecider proves AC-001: one Decider call per drain, over the whole batch.
type countingDecider struct {
	inner   Decider
	calls   int
	batches [][]QueueItem
}

func (d *countingDecider) Decide(items []QueueItem) ([]Decision, error) {
	d.calls++
	batch := make([]QueueItem, len(items))
	copy(batch, items)
	d.batches = append(d.batches, batch)
	return d.inner.Decide(items)
}

// fixedDecider answers from a message_id -> decision table. An unknown message
// gets a verdict outside the four, which is how an illegal decider is simulated.
type fixedDecider struct {
	calls   int
	answers map[string]string
}

func (d *fixedDecider) Decide(items []QueueItem) ([]Decision, error) {
	d.calls++
	out := make([]Decision, 0, len(items))
	for _, item := range items {
		token := d.answers[item.MessageID]
		if token == "" {
			token = "summon_a_helper"
		}
		out = append(out, Decision{
			Channel:    item.Channel,
			Thread:     item.Thread,
			MessageID:  item.MessageID,
			Decision:   token,
			ReasonCode: "made_up_rule",
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

type routeHarness struct {
	t          *testing.T
	beacon     string
	statePath  string
	slowPath   string
	tracePath  string
	configPath string
	clock      *fakeClock
	dispatcher *stubDispatcher
	router     *Router
}

func newRouteHarness(t *testing.T, decider Decider) *routeHarness {
	t.Helper()
	beacon := filepath.Join(t.TempDir(), ".beacon")
	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h := &routeHarness{
		t:          t,
		beacon:     beacon,
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		tracePath:  filepath.Join(beacon, decisionsFile),
		configPath: filepath.Join(beacon, "config"),
		clock:      &fakeClock{t: start},
		dispatcher: &stubDispatcher{},
	}
	h.router = NewRouter(
		&Store{Path: h.statePath, SlowPath: h.slowPath},
		decider,
		h.dispatcher,
	)
	h.router.TracePath = h.tracePath
	h.router.Now = h.clock.now
	return h
}

// withDecider swaps the CLI's decider factory for the duration of a test.
func withDecider(t *testing.T, decider Decider) {
	t.Helper()
	old := deciderFor
	deciderFor = func() Decider { return decider }
	t.Cleanup(func() { deciderFor = old })
}

// routeArgs is the CLI surface, pinned at this harness's temp files.
func (h *routeHarness) routeArgs(extra ...string) []string {
	args := []string{
		"route", "--once",
		"--state", h.statePath,
		"--slow-path", h.slowPath,
		"--config", h.configPath,
		"--trace", h.tracePath,
	}
	return append(args, extra...)
}

func (h *routeHarness) cli(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func (h *routeHarness) drain() DrainResult {
	h.t.Helper()
	res, err := h.router.DrainOnce()
	if err != nil {
		h.t.Fatalf("DrainOnce: %v", err)
	}
	return res
}

func (h *routeHarness) enqueue(items ...QueueItem) {
	h.t.Helper()
	h.appendQueue(items...)
}

// appendQueue adds lines without rewriting what is already queued; the fastpath
// appends, so the router must read an append-only file in order.
func (h *routeHarness) appendQueue(items ...QueueItem) {
	h.t.Helper()
	if err := os.MkdirAll(h.beacon, 0o755); err != nil {
		h.t.Fatalf("mkdir beacon: %v", err)
	}
	f, err := os.OpenFile(h.slowPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		h.t.Fatalf("open slow path: %v", err)
	}
	defer f.Close()
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			h.t.Fatalf("marshal queue item: %v", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			h.t.Fatalf("append queue item: %v", err)
		}
	}
}

// restoreQueue pretends a drain was killed before it rewrote the queue.
func (h *routeHarness) restoreQueue(items ...QueueItem) {
	h.t.Helper()
	if err := writeQueue(h.slowPath, items); err != nil {
		h.t.Fatalf("restore queue: %v", err)
	}
}

func (h *routeHarness) queue() []QueueItem {
	h.t.Helper()
	items, err := readQueue(h.slowPath)
	if err != nil {
		h.t.Fatalf("read queue: %v", err)
	}
	return items
}

func (h *routeHarness) trace() []traceRecord {
	h.t.Helper()
	data, err := os.ReadFile(h.tracePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read trace: %v", err)
	}
	var out []traceRecord
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec traceRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			h.t.Fatalf("trace line %q is not JSON: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func (h *routeHarness) traceFor(messageID string) traceRecord {
	h.t.Helper()
	for _, rec := range h.trace() {
		if rec.MessageID == messageID {
			return rec
		}
	}
	h.t.Fatalf("no trace line for message %q (trace=%+v)", messageID, h.trace())
	return traceRecord{}
}

func (h *routeHarness) binding(channel, thread string) *Binding {
	h.t.Helper()
	snap, err := h.router.Store.load()
	if err != nil {
		h.t.Fatalf("load state: %v", err)
	}
	return snap.Bindings[bindingKey(channel, thread)]
}

func assertNoError(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-001 (AC-001): one drain, one decider call, one batch
// ---------------------------------------------------------------------------

func TestBatchDrainsQueueOnce(t *testing.T) {
	decider := &countingDecider{inner: RuleDecider{}}
	h := newRouteHarness(t, decider)

	// Three questions: they all land on reply, so the batch is exercised without
	// any dispatch.
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "t1", MessageID: "m1", Text: "what is the status?"},
		QueueItem{Channel: "feishu", Thread: "t2", MessageID: "m2", Text: "how long will it take?"},
		QueueItem{Channel: "feishu", Thread: "t3", MessageID: "m3", Text: "who owns it？"},
	)

	res := h.drain()
	if decider.calls != 1 {
		t.Fatalf("decider called %d times for one drain, want exactly 1", decider.calls)
	}
	if len(decider.batches) != 1 || len(decider.batches[0]) != 3 {
		t.Fatalf("decider batches = %d (first has %d items), want one batch of 3",
			len(decider.batches), len(decider.batches[0]))
	}
	if res.Batch != 3 || res.Decided != 3 {
		t.Fatalf("drain = %+v, want batch=3 decided=3", res)
	}
	if res.Applied != 3 || res.Failed != 0 || res.Queued != 0 {
		t.Fatalf("drain = %+v, want applied=3 failed=0 queued=0", res)
	}
	if len(h.dispatcher.calls) != 0 {
		t.Fatalf("replies must not dispatch, dispatcher saw %#v", h.dispatcher.calls)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("queue after a full drain = %+v, want empty", q)
	}
	if got := len(h.trace()); got != 3 {
		t.Fatalf("trace lines = %d, want 3", got)
	}

	// AC-001: an empty backlog is batch=0 AND zero decider calls.
	res = h.drain()
	if decider.calls != 1 {
		t.Fatalf("decider called for an empty queue: calls=%d", decider.calls)
	}
	if res.Batch != 0 || res.Decided != 0 {
		t.Fatalf("empty drain = %+v, want batch=0 decided=0", res)
	}

	// The CLI prints the frozen summary line.
	withDecider(t, RuleDecider{})
	cli := newRouteHarness(t, RuleDecider{})
	cli.enqueue(
		QueueItem{Channel: "feishu", Thread: "c1", MessageID: "cm1", Text: "is it done?"},
		QueueItem{Channel: "feishu", Thread: "c2", MessageID: "cm2", Text: "when?"},
		QueueItem{Channel: "feishu", Thread: "c3", MessageID: "cm3", Text: "why?"},
	)
	code, out, stderr := cli.cli(cli.routeArgs()...)
	if code != 0 {
		t.Fatalf("route --once exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(out, "batch=3 decided=3") {
		t.Fatalf("route output = %q, want batch=3 decided=3", out)
	}

	// Empty queue through the CLI: batch=0, exit 0.
	code, out, stderr = cli.cli(cli.routeArgs()...)
	if code != 0 {
		t.Fatalf("second route --once exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(out, "batch=0") {
		t.Fatalf("empty drain output = %q, want batch=0", out)
	}

	// --limit caps the batch and leaves the rest queued.
	cli.enqueue(
		QueueItem{Channel: "feishu", Thread: "l1", MessageID: "lm1", Text: "one?"},
		QueueItem{Channel: "feishu", Thread: "l2", MessageID: "lm2", Text: "two?"},
	)
	code, out, stderr = cli.cli(cli.routeArgs("--limit", "1")...)
	if code != 0 {
		t.Fatalf("route --limit exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(out, "batch=1 decided=1") {
		t.Fatalf("route --limit output = %q, want batch=1 decided=1", out)
	}
	if q := cli.queue(); len(q) != 1 || q[0].MessageID != "lm2" {
		t.Fatalf("queue after --limit 1 = %+v, want only lm2", q)
	}

	// JSON outcome carries the same counts.
	code, out, stderr = cli.cli(cli.routeArgs("--outcome", "json")...)
	if code != 0 {
		t.Fatalf("route --outcome json exit = %d, stderr = %q", code, stderr)
	}
	var parsed DrainResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("route json %q: %v", out, err)
	}
	if parsed.Batch != 1 || parsed.Decided != 1 {
		t.Fatalf("route json = %+v, want batch=1 decided=1", parsed)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-002 (AC-002): only four decisions exist, and a bad one costs the item
// ---------------------------------------------------------------------------

func TestFourWayDecisionOnly(t *testing.T) {
	decider := &fixedDecider{answers: map[string]string{
		"m-route":    DecisionRoute,
		"m-reply":    DecisionReply,
		"m-ignore":   DecisionIgnore,
		"m-escalate": DecisionEscalate,
		// m-bad has no answer -> the decider returns an illegal token.
	}}
	h := newRouteHarness(t, decider)
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "t-route", MessageID: "m-route", Text: "build the export endpoint"},
		QueueItem{Channel: "feishu", Thread: "t-reply", MessageID: "m-reply", Text: "shipping soon"},
		QueueItem{Channel: "feishu", Thread: "t-ignore", MessageID: "m-ignore", Text: "thanks"},
		QueueItem{Channel: "feishu", Thread: "t-esc", MessageID: "m-escalate", Text: "the customer is angry"},
		QueueItem{Channel: "feishu", Thread: "t-bad", MessageID: "m-bad", Text: "make it faster"},
	)

	res, err := h.router.DrainOnce()
	if err != nil {
		// AC-002: a rejected decision must not fail the drain.
		t.Fatalf("an invalid decision returned an error: %v", err)
	}
	if res.Applied != 4 || res.Failed != 1 {
		t.Fatalf("drain = %+v, want applied=4 failed=1", res)
	}
	if decider.calls != 1 {
		t.Fatalf("decider calls = %d, want 1", decider.calls)
	}

	for _, want := range []string{DecisionRoute, DecisionReply, DecisionIgnore, DecisionEscalate, DecisionInvalid} {
		found := false
		for _, rec := range h.trace() {
			if rec.Decision == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("no trace line with decision=%q (trace=%+v)", want, h.trace())
		}
	}
	bad := h.traceFor("m-bad")
	if bad.Decision != DecisionInvalid || bad.ReasonCode != ReasonDecisionInvalid {
		t.Fatalf("invalid decision traced as %+v, want decision/reason %q", bad, ReasonDecisionInvalid)
	}
	// The failed item is terminal: it does not re-enter the backlog.
	for _, item := range h.queue() {
		if item.MessageID == "m-bad" {
			t.Fatalf("invalid decision item stayed queued: %+v", item)
		}
	}
	// Only the route verdict reached the dispatcher.
	if len(h.dispatcher.calls) != 1 {
		t.Fatalf("dispatcher calls = %d, want 1 (%#v)", len(h.dispatcher.calls), h.dispatcher.calls)
	}

	// The process exit code stays 0 even when the decider goes off-script.
	withDecider(t, &fixedDecider{answers: map[string]string{}})
	cli := newRouteHarness(t, RuleDecider{})
	cli.enqueue(QueueItem{Channel: "feishu", Thread: "t-x", MessageID: "m-x", Text: "anything"})
	code, out, stderr := cli.cli(cli.routeArgs()...)
	if code != 0 {
		t.Fatalf("route --once with an invalid decision exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	if !strings.Contains(out, "failed=1") {
		t.Fatalf("route output = %q, want failed=1", out)
	}
	if rec := cli.traceFor("m-x"); rec.Decision != DecisionInvalid {
		t.Fatalf("cli trace = %+v, want decision_invalid", rec)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-003 (AC-003): the router has no execution tools, provably
// ---------------------------------------------------------------------------

func TestRouterToolsetIsEmpty(t *testing.T) {
	// 1) runtime enumeration
	var stdout, stderr bytes.Buffer
	if code := run([]string{"route", "--list-tools"}, &stdout, &stderr); code != 0 {
		t.Fatalf("route --list-tools exit = %d, stderr = %q", code, stderr)
	}
	if got := strings.TrimSpace(stdout.String()); got != "[]" {
		t.Fatalf("route --list-tools output = %q, want []", got)
	}
	var tools []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &tools); err != nil {
		t.Fatalf("--list-tools output is not a JSON array: %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("router toolset = %v, want empty", tools)
	}
	if len(routerToolset) != 0 {
		t.Fatalf("routerToolset = %v, want empty", routerToolset)
	}
	// --list-tools answers without --once.
	if code := run([]string{"route", "--list-tools"}, &stdout, &stderr); code != 0 {
		t.Fatalf("route --list-tools without --once exit = %d, want 0", code)
	}

	// 2) static scan: the router package files must not reach for the process
	// package at all. The token is assembled here so this test file cannot be
	// mistaken for an import of it.
	processPkg := "os" + "/exec"
	matches, err := filepath.Glob("route*.go")
	if err != nil {
		t.Fatalf("glob route sources: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no route source files found; the scan would prove nothing")
	}
	scanned := 0
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		scanned++
		if strings.Contains(string(src), processPkg) {
			t.Fatalf("%s references %q; the router must own no execution tool", path, processPkg)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no route sources")
	}

	// 3) and the one seam that may spawn a process is a separate file behind the
	// injected Dispatcher interface.
	src, err := os.ReadFile("dispatch.go")
	if err != nil {
		t.Fatalf("read dispatch.go: %v", err)
	}
	if !strings.Contains(string(src), processPkg) {
		t.Fatalf("dispatch.go does not carry the %q seam; the split is meaningless", processPkg)
	}
	if !strings.Contains(string(src), "Dispatcher") {
		t.Fatal("dispatch.go does not implement the Dispatcher seam")
	}
}

// ---------------------------------------------------------------------------
// TC-RL-004 (AC-004): an audit line per decision, exactly once per message
// ---------------------------------------------------------------------------

func TestDecisionTraceIsIdempotent(t *testing.T) {
	h := newRouteHarness(t, RuleDecider{})
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "t-a", MessageID: "m-a", Text: "add a health endpoint"},
		QueueItem{Channel: "feishu", Thread: "t-b", MessageID: "m-b", Text: "please deploy the fix"},
	)
	res := h.drain()
	if res.Batch != 2 || res.Decided != 2 {
		t.Fatalf("drain = %+v, want batch=2 decided=2", res)
	}

	lines := h.trace()
	if len(lines) != 2 {
		t.Fatalf("trace lines = %d, want 2 (%+v)", len(lines), lines)
	}
	wantKeys := []string{
		"batch_size", "channel", "decided_at", "decision",
		"message_id", "reason_code", "thread", "untrusted",
	}
	raw, err := os.ReadFile(h.tracePath)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("trace line %d is not JSON: %v", i+1, err)
		}
		gotKeys := make([]string, 0, len(fields))
		for key := range fields {
			gotKeys = append(gotKeys, key)
		}
		sort.Strings(gotKeys)
		if strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
			t.Fatalf("trace line %d keys = %v, want exactly %v", i+1, gotKeys, wantKeys)
		}
	}

	for _, rec := range lines {
		if !rec.Untrusted {
			t.Fatalf("trace line %+v does not carry untrusted=true", rec)
		}
		if rec.BatchSize != 2 {
			t.Fatalf("trace line %+v batch_size = %d, want 2", rec, rec.BatchSize)
		}
		if rec.Channel == "" || rec.Thread == "" || rec.MessageID == "" || rec.ReasonCode == "" {
			t.Fatalf("trace line %+v is missing a field", rec)
		}
		if _, err := time.Parse(time.RFC3339, rec.DecidedAt); err != nil {
			t.Fatalf("trace line %+v decided_at %q is not RFC3339: %v", rec, rec.DecidedAt, err)
		}
	}
	if rec := h.traceFor("m-b"); rec.Decision != DecisionEscalate || rec.ReasonCode != ReasonUntrustedHighRisk {
		t.Fatalf("high-risk trace = %+v, want escalate/untrusted_high_risk", rec)
	}

	// A crash between "trace written" and "queue rewritten" replays both items.
	h.restoreQueue(
		QueueItem{Channel: "feishu", Thread: "t-a", MessageID: "m-a", Text: "add a health endpoint"},
		QueueItem{Channel: "feishu", Thread: "t-b", MessageID: "m-b", Text: "please deploy the fix"},
	)
	callsBefore := len(h.dispatcher.calls)
	res = h.drain()
	if res.Skipped != 2 || res.Decided != 0 {
		t.Fatalf("replayed drain = %+v, want skipped=2 decided=0", res)
	}
	if got := len(h.trace()); got != 2 {
		t.Fatalf("re-drain added trace lines: %d, want 2", got)
	}
	if got := len(h.dispatcher.calls); got != callsBefore {
		t.Fatalf("re-drain dispatched again: %d calls, want %d", got, callsBefore)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("replayed items were not consumed: %+v", q)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-005 (AC-005): three failed dispatches escalate, they never loop
// ---------------------------------------------------------------------------

func TestDispatchRetryBackoffEscalates(t *testing.T) {
	h := newRouteHarness(t, RuleDecider{})
	h.dispatcher.failFor = func(call dispatchCall) error {
		return errors.New("worker refused the message")
	}
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t-flaky", MessageID: "m-flaky", Text: "fix the flaky test"})

	for attempt := 1; attempt <= 2; attempt++ {
		res := h.drain()
		if res.Failed != 1 || res.Applied != 0 {
			t.Fatalf("attempt %d drain = %+v, want failed=1 applied=0", attempt, res)
		}
		q := h.queue()
		if len(q) != 1 {
			t.Fatalf("attempt %d left %d queued items, want 1 (%+v)", attempt, len(q), q)
		}
		if q[0].Attempts != attempt {
			t.Fatalf("attempt %d queued attempts = %d, want %d", attempt, q[0].Attempts, attempt)
		}
		if got := len(h.trace()); got != 0 {
			t.Fatalf("attempt %d wrote %d trace lines; a backed-off dispatch is not applied", attempt, got)
		}
	}

	// Third consecutive failure: escalate instead of retrying a fourth time.
	res := h.drain()
	if res.Applied != 1 || res.Queued != 0 {
		t.Fatalf("third drain = %+v, want applied=1 queued=0", res)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("escalated item stayed queued: %+v", q)
	}
	rec := h.traceFor("m-flaky")
	if rec.Decision != DecisionEscalate || rec.ReasonCode != ReasonDispatchAttemptsExhausted {
		t.Fatalf("trace = %+v, want escalate/dispatch_attempts_exhausted", rec)
	}
	if got := len(h.trace()); got != 1 {
		t.Fatalf("trace lines = %d, want exactly 1 (a retried message traces once)", got)
	}
	if got := len(h.dispatcher.calls); got != 3 {
		t.Fatalf("dispatcher calls = %d, want 3", got)
	}

	// A fourth drain does nothing: the item is gone for good.
	res = h.drain()
	if res.Batch != 0 {
		t.Fatalf("post-escalation drain = %+v, want batch=0", res)
	}
	if got := len(h.dispatcher.calls); got != 3 {
		t.Fatalf("post-escalation dispatcher calls = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-006 (AC-006): restart relpays the queue without double dispatch
// ---------------------------------------------------------------------------

func TestRestartReplayNoDoubleDispatch(t *testing.T) {
	h := newRouteHarness(t, RuleDecider{})
	h.dispatcher.failFor = func(call dispatchCall) error {
		if call.thread == "t-broken" {
			return errors.New("worker gone")
		}
		return nil
	}
	original := []QueueItem{
		{Channel: "feishu", Thread: "t-ok", MessageID: "m-ok", Text: "write the migration"},
		{Channel: "feishu", Thread: "t-broken", MessageID: "m-broken", Text: "rotate the keys"},
	}
	h.enqueue(original...)

	res := h.drain()
	if res.Applied != 1 || res.Failed != 1 {
		t.Fatalf("first drain = %+v, want applied=1 failed=1", res)
	}
	q := h.queue()
	if len(q) != 1 || q[0].MessageID != "m-broken" || q[0].Attempts != 1 {
		t.Fatalf("queue after a failed dispatch = %+v, want m-broken with attempts=1", q)
	}
	if got := len(h.trace()); got != 1 {
		t.Fatalf("trace lines = %d, want 1", got)
	}

	// Crash: the process died before it rewrote the queue, so the file still has
	// both items (m-ok carries an audit line already).
	h.restoreQueue(original...)
	h.dispatcher.failFor = nil
	res = h.drain()
	if res.Skipped != 1 {
		t.Fatalf("restart drain = %+v, want skipped=1 (already applied)", res)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("restart drain left %+v queued", q)
	}
	if got := len(h.trace()); got != 2 {
		t.Fatalf("trace lines after restart = %d, want 2", got)
	}

	// m-ok (applied) was dispatched exactly once even though the queue replay
	// offered it again; m-broken is the retried one: one refused attempt plus one
	// success.
	seen := map[string]int{}
	for _, call := range h.dispatcher.calls {
		seen[call.text]++
	}
	if seen[guardedText("write the migration")] != 1 {
		t.Fatalf("applied message dispatched %d times, want 1 (%#v)",
			seen[guardedText("write the migration")], h.dispatcher.calls)
	}
	if seen[guardedText("rotate the keys")] != 2 {
		t.Fatalf("retried message dispatched %d times, want 2 (%#v)",
			seen[guardedText("rotate the keys")], h.dispatcher.calls)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-007 (AC-007): untrusted guard, high-risk verbs, reply word limit
// ---------------------------------------------------------------------------

func TestUntrustedGuardAndLimits(t *testing.T) {
	h := newRouteHarness(t, RuleDecider{})
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "t1", MessageID: "risk-delete", Text: "please Delete the production database"},
		QueueItem{Channel: "feishu", Thread: "t2", MessageID: "risk-push", Text: "Push it to main now"},
		QueueItem{Channel: "feishu", Thread: "t3", MessageID: "risk-deploy", Text: "DEPLOY everything"},
		QueueItem{Channel: "feishu", Thread: "t4", MessageID: "risk-release", Text: "just release it"},
		QueueItem{Channel: "feishu", Thread: "t5", MessageID: "work", Text: "add a retry to the webhook handler"},
		QueueItem{Channel: "feishu", Thread: "t6", MessageID: "question", Text: "why is the export slow?"},
		// 99 runes + "?" = 100 runes: right at the ceiling, still a reply.
		QueueItem{Channel: "feishu", Thread: "t7", MessageID: "reply-ok", Text: strings.Repeat("a", 99) + "?"},
		// 100 runes + "?" = 101 runes: one over, rejected.
		QueueItem{Channel: "feishu", Thread: "t8", MessageID: "reply-long", Text: strings.Repeat("a", 100) + "?"},
	)

	res := h.drain()
	if res.Batch != 8 {
		t.Fatalf("drain = %+v, want batch=8", res)
	}

	// High-risk verbs: escalated, never dispatched.
	for _, id := range []string{"risk-delete", "risk-push", "risk-deploy", "risk-release"} {
		rec := h.traceFor(id)
		if rec.Decision != DecisionEscalate || rec.ReasonCode != ReasonUntrustedHighRisk {
			t.Fatalf("%s traced as %+v, want escalate/untrusted_high_risk", id, rec)
		}
	}

	// Every inbound text is untrusted, in the trace too.
	for _, rec := range h.trace() {
		if !rec.Untrusted {
			t.Fatalf("trace line %+v does not carry untrusted=true", rec)
		}
	}

	// Exactly one dispatch happened: the plain work item, behind the guard line.
	if len(h.dispatcher.calls) != 1 {
		t.Fatalf("dispatcher calls = %d, want 1 (%#v)", len(h.dispatcher.calls), h.dispatcher.calls)
	}
	call := h.dispatcher.calls[0]
	if !strings.HasPrefix(call.text, UntrustedGuardLine+"\n") {
		t.Fatalf("dispatched text = %q, want it to start with %q", call.text, UntrustedGuardLine+"\n")
	}
	if call.text != guardedText("add a retry to the webhook handler") {
		t.Fatalf("dispatched text = %q, want the guard line plus the original text", call.text)
	}
	if rec := h.traceFor("work"); rec.Decision != DecisionRoute || rec.ReasonCode != ReasonNewWork {
		t.Fatalf("work trace = %+v, want route/new_work", rec)
	}

	// Questions reply, and the reply ceiling is 100 runes.
	if rec := h.traceFor("question"); rec.Decision != DecisionReply || rec.ReasonCode != ReasonQuestion {
		t.Fatalf("question trace = %+v, want reply/question", rec)
	}
	if rec := h.traceFor("reply-ok"); rec.Decision != DecisionReply || rec.ReasonCode != ReasonQuestion {
		t.Fatalf("100-rune reply traced as %+v, want reply/question", rec)
	}
	long := h.traceFor("reply-long")
	if long.Decision != DecisionReply || long.ReasonCode != ReasonReplyTooLong {
		t.Fatalf("101-rune reply traced as %+v, want reply/reply_too_long", long)
	}
	if res.Failed != 1 {
		t.Fatalf("drain = %+v, want failed=1 (the over-long reply)", res)
	}
	for _, item := range h.queue() {
		if item.MessageID == "reply-long" {
			t.Fatalf("rejected reply stayed queued: %+v", item)
		}
	}
}

// ---------------------------------------------------------------------------
// TC-RL-008 (AC-006): queued -> claimed -> decided -> applied | failed -> escalated
// ---------------------------------------------------------------------------

func TestQueueStateWalks(t *testing.T) {
	h := newRouteHarness(t, RuleDecider{})

	// queued -> claimed -> decided -> applied, and the route binds the thread so
	// the next message on it is a bound_thread route to the same worker.
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t-walk", MessageID: "m-1", Text: "ship the loader"})
	drain := h.drain()
	if drain.Applied != 1 || drain.Queued != 0 {
		t.Fatalf("first walk drain = %+v, want applied=1 queued=0", drain)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("queue after applied = %+v, want empty", q)
	}
	if rec := h.traceFor("m-1"); rec.Decision != DecisionRoute || rec.ReasonCode != ReasonNewWork {
		t.Fatalf("m-1 trace = %+v, want route/new_work", rec)
	}
	if got := len(h.dispatcher.calls); got != 1 {
		t.Fatalf("dispatcher calls = %d, want 1", got)
	}
	if got := h.dispatcher.calls[0].agentID; got != DefaultRouterAgentID {
		t.Fatalf("dispatch agent = %q, want %q", got, DefaultRouterAgentID)
	}
	binding := h.binding("feishu", "t-walk")
	if binding == nil || binding.State != StateBound || binding.AgentID != DefaultRouterAgentID {
		t.Fatalf("binding after a route = %+v, want bound/%s", binding, DefaultRouterAgentID)
	}

	h.enqueue(QueueItem{Channel: "feishu", Thread: "t-walk", MessageID: "m-2", Text: "also add pagination"})
	drain = h.drain()
	if drain.Applied != 1 {
		t.Fatalf("second walk drain = %+v, want applied=1", drain)
	}
	if rec := h.traceFor("m-2"); rec.Decision != DecisionRoute || rec.ReasonCode != ReasonBoundThread {
		t.Fatalf("m-2 trace = %+v, want route/bound_thread", rec)
	}
	if got := len(h.dispatcher.calls); got != 2 {
		t.Fatalf("dispatcher calls = %d, want 2", got)
	}
	if got := h.dispatcher.calls[1].agentID; got != DefaultRouterAgentID {
		t.Fatalf("bound route dispatched to %q, want the bound worker %q", got, DefaultRouterAgentID)
	}

	// queued -> claimed -> decided -> failed (backed off, still queued).
	h.dispatcher.failFor = func(call dispatchCall) error { return errors.New("nope") }
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t-doomed", MessageID: "m-doomed", Text: "resurrect the old job"})
	drain = h.drain()
	if drain.Failed != 1 || drain.Applied != 0 || drain.Queued != 1 {
		t.Fatalf("failed walk drain = %+v, want failed=1 applied=0 queued=1", drain)
	}
	if q := h.queue(); len(q) != 1 || q[0].Attempts != 1 {
		t.Fatalf("queue after failure = %+v, want the item with attempts=1", q)
	}

	// failed -> escalated after the retry ceiling.
	h.drain()
	drain = h.drain()
	if drain.Applied != 1 || drain.Queued != 0 {
		t.Fatalf("escalation drain = %+v, want applied=1 queued=0", drain)
	}
	if rec := h.traceFor("m-doomed"); rec.Decision != DecisionEscalate || rec.ReasonCode != ReasonDispatchAttemptsExhausted {
		t.Fatalf("m-doomed trace = %+v, want escalate/dispatch_attempts_exhausted", rec)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("queue after escalation = %+v, want empty", q)
	}
}
