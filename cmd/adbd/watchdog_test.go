package main

// Watchdog tests (feature watchdog, v0.1.5): TC-WD-001 .. TC-WD-005 and
// TC-WD-008.
//
// Every test runs against a temp dir, an injected fake clock and no child process
// at all: the watchdog is judged on what it reads from three plain files, which is
// precisely the property AC-006 asks for (it must work with no adbd around).

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

type watchHarness struct {
	t      *testing.T
	root   string
	beacon string
	clock  *fakeClock
	watch  *Watchdog
	store  *Store
	config string
	paths  map[string]string
	base   time.Time
}

func newWatchHarness(t *testing.T) *watchHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h := &watchHarness{
		t:      t,
		root:   root,
		beacon: beacon,
		clock:  &fakeClock{t: start},
		store:  &Store{Path: filepath.Join(beacon, "adbd-state.json"), SlowPath: filepath.Join(beacon, "adbd-slowpath.jsonl")},
		config: filepath.Join(beacon, "config"),
		base:   start,
		paths: map[string]string{
			"state":       filepath.Join(beacon, "adbd-state.json"),
			"slow_path":   filepath.Join(beacon, "adbd-slowpath.jsonl"),
			"decisions":   filepath.Join(beacon, decisionsFile),
			"watch":       filepath.Join(beacon, watchdogFile),
			"heartbeat":   filepath.Join(beacon, heartbeatFile),
			"watch_state": filepath.Join(beacon, watchStateFile),
			"config":      filepath.Join(beacon, "config"),
		},
	}
	h.watch = NewWatchdog(h.store)
	h.watch.Now = h.clock.now
	// The CLI builds its own Watchdog, so the fake clock has to reach that seam
	// too; without it a "clean" scan would be judged against the wall clock.
	oldClock := watchdogClock
	watchdogClock = h.clock.now
	t.Cleanup(func() { watchdogClock = oldClock })
	return h
}

// load/saveState work directly on the state document: the watchdog tests care
// about file content, not about the fastpath lock.
func (h *watchHarness) loadState() *snapshot {
	h.t.Helper()
	snap, err := h.store.load()
	if err != nil {
		h.t.Fatalf("load state: %v", err)
	}
	return snap
}

func (h *watchHarness) saveState(snap *snapshot) {
	h.t.Helper()
	if err := os.MkdirAll(h.beacon, 0o755); err != nil {
		h.t.Fatalf("mkdir beacon: %v", err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		h.t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(h.paths["state"], append(data, '\n'), 0o644); err != nil {
		h.t.Fatalf("write state: %v", err)
	}
}

// putBinding inserts one binding row as-is.
func (h *watchHarness) putBinding(b *Binding) {
	h.t.Helper()
	snap := h.loadState()
	snap.Bindings[bindingKey(b.Channel, b.Thread)] = b
	h.saveState(snap)
}

// bind inserts a healthy-looking bound thread last touched at updatedAt.
func (h *watchHarness) bind(channel, thread, agentID string, updatedAt time.Time) {
	h.t.Helper()
	h.putBinding(&Binding{
		Channel:   channel,
		Thread:    thread,
		AgentID:   agentID,
		State:     StateBound,
		TTL:       DefaultTTL,
		ExpiresAt: updatedAt.Add(DefaultTTL),
		UpdatedAt: updatedAt,
	})
}

func (h *watchHarness) bindingFor(channel, thread string) *Binding {
	h.t.Helper()
	return h.loadState().Bindings[bindingKey(channel, thread)]
}

// decision appends one router audit line, which is the AC-001 "progress" clock.
func (h *watchHarness) decision(channel, thread, messageID string, at time.Time) {
	h.t.Helper()
	rec := traceRecord{
		Channel:    channel,
		Thread:     thread,
		MessageID:  messageID,
		Decision:   DecisionRoute,
		ReasonCode: ReasonNewWork,
		BatchSize:  1,
		Untrusted:  true,
		DecidedAt:  at.Format(time.RFC3339),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		h.t.Fatalf("marshal decision: %v", err)
	}
	h.appendLine(h.paths["decisions"], string(data))
}

func (h *watchHarness) heartbeat(at time.Time) {
	h.t.Helper()
	if err := writeHeartbeat(h.paths["heartbeat"], at); err != nil {
		h.t.Fatalf("write heartbeat: %v", err)
	}
}

func (h *watchHarness) appendLine(path, line string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		h.t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		h.t.Fatalf("append %s: %v", path, err)
	}
}

// config writes .beacon/config with the given key=value lines.
func (h *watchHarness) configLines(lines ...string) {
	h.t.Helper()
	h.appendLine(h.paths["config"], strings.Join(lines, "\n"))
}

// watchArgs pins the CLI at this harness's temp files.
func (h *watchHarness) watchArgs(extra ...string) []string {
	args := []string{
		"watchdog", "--once",
		"--root", h.root,
		"--state", h.paths["state"],
		"--slow-path", h.paths["slow_path"],
		"--config", h.paths["config"],
		"--decisions", h.paths["decisions"],
		"--watch", h.paths["watch"],
		"--heartbeat", h.paths["heartbeat"],
		"--watch-state", h.paths["watch_state"],
	}
	return append(args, extra...)
}

func (h *watchHarness) cli(extra ...string) (int, string, string) {
	h.t.Helper()
	code, stdout, stderr := runCLI(h.watchArgs(extra...))
	return code, stdout, stderr
}

// scan runs the CLI and fails the test on a non-zero exit.
func (h *watchHarness) scan(extra ...string) (string, []watchAlert) {
	h.t.Helper()
	code, stdout, stderr := h.cli(extra...)
	if code != 0 {
		h.t.Fatalf("watchdog exited %d: %s", code, stderr)
	}
	return stdout, h.alerts()
}

func (h *watchHarness) alerts() []watchAlert {
	h.t.Helper()
	data, err := os.ReadFile(h.paths["watch"])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read watchdog trace: %v", err)
	}
	var out []watchAlert
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var alert watchAlert
		if err := json.Unmarshal([]byte(line), &alert); err != nil {
			h.t.Fatalf("watchdog trace line %q is not JSON: %v", line, err)
		}
		out = append(out, alert)
	}
	return out
}

func (h *watchHarness) queueLines() []slowPathRecord {
	h.t.Helper()
	data, err := os.ReadFile(h.paths["slow_path"])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read slow path: %v", err)
	}
	var out []slowPathRecord
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec slowPathRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			h.t.Fatalf("slow-path line %q is not JSON: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// runCLI keeps the CLI seam in one place; route_test.go has the same idea on its
// own harness, and this one is deliberately independent of it.
func runCLI(args []string) (int, string, string) {
	var stdout, stderr strings.Builder
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// fileStamp is what AC-003 asserts stays identical: content plus mtime.
type fileStamp struct {
	exists  bool
	content string
	modTime time.Time
	size    int64
}

func (h *watchHarness) stamp(path string) fileStamp {
	h.t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileStamp{}
		}
		h.t.Fatalf("stat %s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("read %s: %v", path, err)
	}
	return fileStamp{exists: true, content: string(data), modTime: info.ModTime(), size: info.Size()}
}

// ---------------------------------------------------------------------------
// TC-WD-001 (AC-001): progress, not liveness
// ---------------------------------------------------------------------------

func TestStalledByProgressNotLiveness(t *testing.T) {
	h := newWatchHarness(t)
	base := h.base
	h.bind("feishu", "t-quiet", "worker-quiet", base)
	h.bind("feishu", "t-working", "worker-busy", base)
	// A decision at base+5m is real progress: 20m later this thread is exactly at
	// the threshold and must NOT be flagged (the comparison is strictly ">).
	h.decision("feishu", "t-working", "m-busy", base.Add(5*time.Minute))
	// A thread that has never acted: creation time is derived from the lease, so
	// "从未有过动作的 thread 按绑定创建时间算" is honoured without an extra field.
	h.putBinding(&Binding{
		Channel:   "feishu",
		Thread:    "t-never",
		AgentID:   "worker-new",
		State:     StateBound,
		TTL:       DefaultTTL,
		ExpiresAt: base.Add(DefaultTTL),
	})
	h.heartbeat(base.Add(24 * time.Minute))
	h.clock.t = base.Add(25 * time.Minute)

	stdout, alerts := h.scan()

	// Nothing in this test starts, stops or inspects a process: the two threads
	// below are flagged purely because their recorded progress is old. A live
	// worker process with a silent file trail is still a stall.
	wantLines := []string{"stalled thread=t-never", "stalled thread=t-quiet"}
	gotLines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if strings.Join(gotLines, "|") != strings.Join(wantLines, "|") {
		t.Fatalf("stdout =\n%v\nwant\n%v", gotLines, wantLines)
	}
	if len(alerts) != 2 {
		t.Fatalf("alerts = %d, want 2: %+v", len(alerts), alerts)
	}
	for _, alert := range alerts {
		if alert.Kind != KindStalled || alert.ReasonCode != ReasonWatchdogStalled {
			t.Fatalf("alert = %+v, want kind=stalled reason_code=stalled", alert)
		}
		if alert.Thread == "t-working" {
			t.Fatalf("t-working had a fresh decision at the threshold and must stay quiet: %+v", alert)
		}
		if !strings.Contains(alert.Evidence, "last_activity=") || !strings.Contains(alert.Evidence, "threshold=20m0s") {
			t.Fatalf("evidence does not name the progress clock and the threshold: %q", alert.Evidence)
		}
	}

	// The threshold is configurable (truth.md 配置项): the same quiet thread is
	// healthy once the operator allows 30m of silence.
	os.Remove(h.paths["watch"])
	os.Remove(h.paths["watch_state"])
	h.configLines("adbd_stall_after=30m")
	stdout, alerts = h.scan()
	if stdout != "NOTHING\n" {
		t.Fatalf("stdout = %q, want NOTHING with adbd_stall_after=30m", stdout)
	}
	if len(alerts) != 0 {
		t.Fatalf("alerts = %+v, want none below the configured threshold", alerts)
	}
}

// ---------------------------------------------------------------------------
// TC-WD-002 (AC-002): the router is watched too
// ---------------------------------------------------------------------------

func TestRouterHeartbeatExpiry(t *testing.T) {
	h := newWatchHarness(t)
	base := h.base
	h.clock.t = base.Add(25 * time.Minute)
	// One completely healthy thread, so the only thing left to judge is the beat.
	h.bind("feishu", "t-alive", "main", base.Add(24*time.Minute))

	// (a) no heartbeat file at all == expired.
	stdout, alerts := h.scan()
	if stdout != "router_stalled\n" {
		t.Fatalf("stdout = %q, want router_stalled when the heartbeat is missing", stdout)
	}
	if len(alerts) != 1 || alerts[0].Kind != KindRouterStalled || alerts[0].ReasonCode != ReasonWatchdogRouterStalled {
		t.Fatalf("alerts = %+v, want one router_stalled line", alerts)
	}
	if alerts[0].Thread != "" {
		t.Fatalf("router_stalled thread = %q, want the empty thread", alerts[0].Thread)
	}
	if !strings.Contains(alerts[0].Evidence, "heartbeat=missing") {
		t.Fatalf("evidence = %q, want heartbeat=missing", alerts[0].Evidence)
	}

	// (b) a fresh beat is clean.
	h.heartbeat(base.Add(24 * time.Minute))
	if stdout, _ = h.scan(); stdout != "NOTHING\n" {
		t.Fatalf("stdout = %q, want NOTHING with a 1m-old heartbeat", stdout)
	}

	// (c) 11 minutes of silence is stale (default adbd_heartbeat_after=10m).
	h.heartbeat(base.Add(14 * time.Minute))
	if stdout, _ = h.scan(); !strings.Contains(stdout, "router_stalled") {
		t.Fatalf("stdout = %q, want router_stalled for an 11m-old heartbeat", stdout)
	}

	// (d) the same beat is fine once the operator widens the window.
	os.Remove(h.paths["watch"])
	os.Remove(h.paths["watch_state"])
	h.configLines("adbd_heartbeat_after=30m")
	if stdout, _ = h.scan(); stdout != "NOTHING\n" {
		t.Fatalf("stdout = %q, want NOTHING with adbd_heartbeat_after=30m", stdout)
	}

	// (e) round trip: `adbd route` itself stamps the beat the watchdog reads.
	os.Remove(h.paths["heartbeat"])
	router := NewRouter(h.store, RuleDecider{}, &stubDispatcher{})
	router.TracePath = h.paths["decisions"]
	router.Now = h.clock.now
	if _, err := router.DrainOnce(); err != nil {
		t.Fatalf("drain: %v", err)
	}
	beat, ok := readHeartbeat(h.paths["heartbeat"])
	if !ok {
		t.Fatalf("route did not write %s", h.paths["heartbeat"])
	}
	if !beat.Equal(h.clock.t.UTC()) {
		t.Fatalf("heartbeat at = %s, want %s", beat, h.clock.t.UTC())
	}
	if stdout, _ = h.scan(); stdout != "NOTHING\n" {
		t.Fatalf("stdout = %q, want NOTHING after a drain refreshed the heartbeat", stdout)
	}
}

// ---------------------------------------------------------------------------
// TC-WD-003 (AC-003): silence is the default
// ---------------------------------------------------------------------------

func TestSilentWhenClean(t *testing.T) {
	h := newWatchHarness(t)
	base := h.base
	h.clock.t = base.Add(time.Minute)
	h.bind("feishu", "t-clean", "main", base)
	h.decision("feishu", "t-clean", "m-1", base)
	h.heartbeat(base)

	before := map[string]fileStamp{
		"state":     h.stamp(h.paths["state"]),
		"decisions": h.stamp(h.paths["decisions"]),
	}
	code, stdout, stderr := h.cli()
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if stdout != "NOTHING\n" {
		t.Fatalf("stdout = %q, want exactly NOTHING", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want nothing", stderr)
	}
	for name, path := range map[string]string{"watchdog trace": h.paths["watch"], "scan ledger": h.paths["watch_state"]} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s must not exist after a clean scan (%v)", name, err)
		}
	}
	for name, want := range before {
		got := h.stamp(h.paths[name])
		if got.exists != want.exists || got.content != want.content {
			t.Fatalf("%s changed during a clean scan: before=%+v after=%+v", name, want, got)
		}
		if !got.modTime.Equal(want.modTime) || got.size != want.size {
			t.Fatalf("%s mtime/size changed during a clean scan: before=%v/%d after=%v/%d",
				name, want.modTime, want.size, got.modTime, got.size)
		}
	}

	// A stale alert from an earlier incident must also be left untouched.
	h.appendLine(h.paths["watch"], `{"kind":"stalled","thread":"t-old","reason_code":"stalled","evidence":"x","action":"observed","at":"`+base.Add(-time.Hour).Format(time.RFC3339)+`"}`)
	watchBefore := h.stamp(h.paths["watch"])
	stateBefore := h.stamp(h.paths["watch_state"])
	if stdout, _ = h.scan(); stdout != "NOTHING\n" {
		t.Fatalf("stdout = %q, want NOTHING", stdout)
	}
	if got := h.stamp(h.paths["watch"]); got.content != watchBefore.content || !got.modTime.Equal(watchBefore.modTime) {
		t.Fatalf("clean scan rewrote the watchdog trace")
	}
	if got := h.stamp(h.paths["watch_state"]); got.exists != stateBefore.exists {
		t.Fatalf("clean scan created the scan ledger")
	}
}

// ---------------------------------------------------------------------------
// TC-WD-004 (AC-004): the trace shape and the two actions
// ---------------------------------------------------------------------------

func TestInterventionTraceShape(t *testing.T) {
	t.Run("observed", func(t *testing.T) {
		h := newWatchHarness(t)
		base := h.base
		h.bind("feishu", "t-stuck", "worker-1", base)
		h.heartbeat(base.Add(29 * time.Minute))
		h.clock.t = base.Add(30 * time.Minute)

		if _, alerts := h.scan(); len(alerts) != 1 {
			t.Fatalf("alerts = %+v, want exactly one", alerts)
		}
		line := h.lastTraceLine()
		keys := traceKeyOrder(t, line)
		want := []string{"kind", "thread", "reason_code", "evidence", "action", "at"}
		if strings.Join(keys, ",") != strings.Join(want, ",") {
			t.Fatalf("trace keys = %v, want %v (AC-004 froze this field set and its order)", keys, want)
		}
		alert := h.alerts()[0]
		if alert.Action != ActionObserved {
			t.Fatalf("action = %q, want observed without --apply", alert.Action)
		}
		if alert.Thread != "t-stuck" || alert.At != h.clock.t.Format(time.RFC3339) {
			t.Fatalf("alert = %+v, want thread=t-stuck and at=%s", alert, h.clock.t.Format(time.RFC3339))
		}
		if alert.Evidence == "" {
			t.Fatal("evidence must never be empty: an operator has to be able to check the call")
		}
		// Observation must not touch the world.
		if b := h.bindingFor("feishu", "t-stuck"); b.State != StateBound {
			t.Fatalf("binding state = %q, want it untouched without --apply", b.State)
		}
		if q := h.queueLines(); len(q) != 0 {
			t.Fatalf("queue = %+v, want no requeue without --apply", q)
		}
	})

	t.Run("requeue", func(t *testing.T) {
		h := newWatchHarness(t)
		base := h.base
		h.bind("feishu", "t-stuck", "worker-1", base)
		h.decision("feishu", "t-stuck", "m-42", base)
		h.heartbeat(base.Add(29 * time.Minute))
		h.clock.t = base.Add(30 * time.Minute)

		// Observe first, so the requeue lands inside the repeat window: an action
		// that changed the world must still be written down (AC-004).
		if _, alerts := h.scan(); len(alerts) != 1 {
			t.Fatalf("observation alerts = %+v, want one", alerts)
		}
		stdout, alerts := h.scan("--apply")
		if !strings.Contains(stdout, "stalled thread=t-stuck") {
			t.Fatalf("stdout = %q, want stalled thread=t-stuck", stdout)
		}
		if len(alerts) != 2 || alerts[1].Action != ActionRequeue {
			t.Fatalf("alerts = %+v, want a second line with action=requeue inside the window", alerts)
		}
		// The stall is real progress-wise, so the owner is released.
		binding := h.bindingFor("feishu", "t-stuck")
		if binding.State != StateReleased {
			t.Fatalf("binding state = %q, want released", binding.State)
		}
		if binding.AgentID != "" {
			t.Fatalf("binding agent_id = %q, want it cleared", binding.AgentID)
		}
		// The last message is back on the slow path for the front desk.
		queue := h.queueLines()
		if len(queue) != 1 {
			t.Fatalf("queue = %+v, want exactly one requeued message", queue)
		}
		if queue[0].ReasonCode != ReasonWatchdogRequeue || queue[0].Thread != "t-stuck" {
			t.Fatalf("requeued record = %+v, want reason_code=%s thread=t-stuck", queue[0], ReasonWatchdogRequeue)
		}
		if !strings.HasPrefix(queue[0].MessageID, "m-42") {
			t.Fatalf("requeued message_id = %q, want it derived from m-42", queue[0].MessageID)
		}
	})
}

func (h *watchHarness) lastTraceLine() string {
	h.t.Helper()
	data, err := os.ReadFile(h.paths["watch"])
	if err != nil {
		h.t.Fatalf("read watchdog trace: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) == 0 {
		h.t.Fatal("watchdog trace is empty")
	}
	return lines[len(lines)-1]
}

// traceKeyOrder reads the JSON object keys in document order, which is what
// AC-004 actually froze.
func traceKeyOrder(t *testing.T, line string) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(line))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("decode trace line: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		t.Fatalf("trace line does not start an object: %q", line)
	}
	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			t.Fatalf("decode trace key: %v", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			t.Fatalf("trace key %v is not a string", keyTok)
		}
		keys = append(keys, key)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatalf("decode trace value for %q: %v", key, err)
		}
	}
	return keys
}

// ---------------------------------------------------------------------------
// TC-WD-005 (AC-005): repeat stall escalates, then goes quiet
// ---------------------------------------------------------------------------

func TestRepeatStallEscalation(t *testing.T) {
	h := newWatchHarness(t)
	base := h.base
	h.bind("feishu", "t-stuck", "worker-1", base)

	// Three consecutive scans, ten minutes apart, all stalled.
	h.clock.t = base.Add(30 * time.Minute)
	h.heartbeat(h.clock.t.Add(-time.Minute))
	if _, alerts := h.scan(); len(alerts) != 1 || alerts[0].Kind != KindStalled {
		t.Fatalf("scan 1 alerts = %+v, want one stalled", alerts)
	}
	h.clock.advance(10 * time.Minute)
	h.heartbeat(h.clock.t.Add(-time.Minute))
	if _, alerts := h.scan(); len(alerts) != 1 {
		t.Fatalf("scan 2 wrote a duplicate stalled alert inside the window: %+v", alerts)
	}
	h.clock.advance(10 * time.Minute)
	h.heartbeat(h.clock.t.Add(-time.Minute))
	var third []watchAlert
	if _, third = h.scan(); len(third) != 2 {
		t.Fatalf("scan 3 alerts = %+v, want the escalation line appended", third)
	}
	repeat := third[1]
	if repeat.Kind != KindRepeatStall || repeat.ReasonCode != ReasonWatchdogRepeatStall {
		t.Fatalf("scan 3 alert = %+v, want kind=repeat_stall reason_code=repeat_stall", repeat)
	}

	// Still stuck, but the window is not over: no fourth line.
	h.clock.advance(10 * time.Minute)
	h.heartbeat(h.clock.t.Add(-time.Minute))
	if _, alerts := h.scan(); len(alerts) != 2 {
		t.Fatalf("scan 4 alerts = %+v, want the 6h window to silence it", alerts)
	}

	// Past the window the silence ends, and the thread is a fresh episode again.
	h.clock.advance(7 * time.Hour)
	h.heartbeat(h.clock.t.Add(-time.Minute))
	stdout, alerts := h.scan()
	if len(alerts) != 3 {
		t.Fatalf("post-window alerts = %+v, want one new line", alerts)
	}
	if alerts[2].Kind != KindStalled {
		t.Fatalf("post-window kind = %q, want stalled (a new episode, not a repeat)", alerts[2].Kind)
	}
	if !strings.Contains(stdout, "stalled thread=t-stuck") {
		t.Fatalf("stdout = %q, want the stall still reported", stdout)
	}

	// --apply on a repeat stall escalates instead of requeueing a second time.
	// A requeue releases the binding, so a repeat only happens when the front desk
	// re-dispatched the message and the fresh binding stalled again -- which is
	// exactly the case a human has to look at.
	t.Run("apply_escalates", func(t *testing.T) {
		h2 := newWatchHarness(t)
		var last watchAlert
		for i := 0; i < 3; i++ {
			h2.clock.t = h2.base.Add(time.Duration(30+10*i) * time.Minute)
			h2.bind("feishu", "t-stuck", "worker-1", h2.clock.t.Add(-30*time.Minute))
			h2.heartbeat(h2.clock.t.Add(-time.Minute))
			_, alerts := h2.scan("--apply")
			last = alerts[len(alerts)-1]
		}
		if last.Kind != KindRepeatStall || last.Action != ActionEscalate {
			t.Fatalf("repeat with --apply = %+v, want kind=repeat_stall action=escalate", last)
		}
		if b := h2.bindingFor("feishu", "t-stuck"); b.State != StateBound {
			t.Fatalf("escalation must not silently unbind (state=%q)", b.State)
		}
	})
}

// ---------------------------------------------------------------------------
// TC-WD-008 (AC-001): the state machine walks
// ---------------------------------------------------------------------------

func TestWatchStateWalks(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		h := newWatchHarness(t)
		phases := []string{}
		h.watch.OnPhase = func(phase string) { phases = append(phases, phase) }
		if h.watch.Phase != PhaseIdle {
			t.Fatalf("phase = %q, want idle before the first scan", h.watch.Phase)
		}
		h.heartbeat(h.base)
		res, err := h.watch.Scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if want := []string{PhaseScanning, PhaseClean}; !equalStrings(phases, want) {
			t.Fatalf("phases = %v, want %v", phases, want)
		}
		if !res.Clean || res.Phase != PhaseClean {
			t.Fatalf("result = %+v, want a clean scan", res)
		}
	})

	t.Run("acted", func(t *testing.T) {
		h := newWatchHarness(t)
		h.bind("feishu", "t-stuck", "worker-1", h.base)
		h.heartbeat(h.base.Add(29 * time.Minute))
		h.clock.t = h.base.Add(30 * time.Minute)
		h.watch.Apply = true
		phases := []string{}
		h.watch.OnPhase = func(phase string) { phases = append(phases, phase) }
		res, err := h.watch.Scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if want := []string{PhaseScanning, PhaseActed}; !equalStrings(phases, want) {
			t.Fatalf("phases = %v, want %v", phases, want)
		}
		if res.Phase != PhaseActed || res.Applied != 1 {
			t.Fatalf("result = %+v, want one applied action", res)
		}
	})

	t.Run("alerted", func(t *testing.T) {
		h := newWatchHarness(t)
		h.bind("feishu", "t-stuck", "worker-1", h.base)
		h.heartbeat(h.base.Add(29 * time.Minute))
		h.clock.t = h.base.Add(30 * time.Minute)
		res, err := h.watch.Scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if res.Phase != PhaseAlerted || res.Clean || len(res.Alerts) != 1 {
			t.Fatalf("result = %+v, want alerted with one observation", res)
		}
		if res.Applied != 0 {
			t.Fatalf("applied = %d, want 0 without Apply", res.Applied)
		}
	})
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// supporting checks: config parsing and deterministic output order
// ---------------------------------------------------------------------------

func TestWatchdogConfigAndOrder(t *testing.T) {
	h := newWatchHarness(t)
	base := h.base
	// Bind out of alphabetical order on purpose: output must be sorted anyway.
	for _, thread := range []string{"t-zeta", "t-alpha", "t-mid"} {
		h.bind("feishu", thread, "worker", base)
	}
	h.heartbeat(base.Add(29 * time.Minute))
	h.clock.t = base.Add(30 * time.Minute)
	stdout, _ := h.scan()
	want := []string{"stalled thread=t-alpha", "stalled thread=t-mid", "stalled thread=t-zeta"}
	if got := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n"); !equalStrings(got, want) {
		t.Fatalf("stdout = %v, want sorted %v", got, want)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if !sort.StringsAreSorted(lines) {
		t.Fatalf("stdout is not sorted: %v", lines)
	}

	// A bad duration is a loud error, not a silent fallback.
	bad := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(bad, []byte("adbd_stall_after=twenty\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfig(bad); err == nil {
		t.Fatal("LoadConfig accepted adbd_stall_after=twenty")
	}
	good := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(good, []byte("adbd_stall_after=20m\nadbd_heartbeat_after=10m\nadbd_repeat_window=6h\nadbd_interval=600\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfig(good); err == nil {
		t.Fatal("LoadConfig accepted a bare 600 for adbd_interval; the plist takes a duration")
	}
	good2 := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(good2, []byte("adbd_stall_after=20m\nadbd_interval=10m\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfig(good2)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.StallAfter != 20*time.Minute || cfg.Interval != 10*time.Minute {
		t.Fatalf("cfg = %+v, want 20m/10m", cfg)
	}
	if cfg.HeartbeatAfter != DefaultHeartbeatAfter || cfg.RepeatWindow != DefaultRepeatWindow {
		t.Fatalf("absent keys must keep the frozen defaults: %+v", cfg)
	}
	if strconv.FormatInt(int64(cfg.Interval/time.Second), 10) != "600" {
		t.Fatalf("default interval renders as %s, want 600", cfg.Interval)
	}
}
