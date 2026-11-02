package main

// Reconcile tests (feature worker-report, v0.1.5): TC-WR-001 .. TC-WR-003,
// TC-WR-005, TC-WR-006 and TC-WR-007.
//
// Every test runs against a temp root, an injected fake clock and a REAL fake
// paseo (a /bin/sh script the test writes to disk). That is deliberate: the seam
// under test is the production one -- argv building, exit codes, JSON parsing --
// so a stub interface would hide exactly the bugs AC-006 is about. Nothing here
// calls the real paseo and nothing touches the network.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// harness: temp root + fake clock + a fake `paseo inspect` on disk
// ---------------------------------------------------------------------------

// writeFakeStatusPaseo writes the script that answers `inspect <id> --json`.
//
// Behaviour knobs are environment variables, so one script covers every case:
//
//	FAKE_PASEO_STATUS_FILE   "<agent-id> <status>" lines; a missing id exits 1
//	FAKE_PASEO_ARGV_LOG      every invocation is appended (argv joined by spaces)
//	FAKE_PASEO_EXIT          non-zero forces that exit code with stderr noise
//	FAKE_PASEO_OUTPUT        "garbage" | "nostatus" -> an unparsable answer
func writeFakeStatusPaseo(t *testing.T, argvLog string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "fake-status-paseo.sh")
	body := "#!/bin/sh\n" +
		"if [ -n \"$FAKE_PASEO_ARGV_LOG\" ]; then printf '%s\\n' \"$*\" >> \"$FAKE_PASEO_ARGV_LOG\"; fi\n" +
		"case \"$1\" in\n" +
		"  inspect)\n" +
		"    if [ \"${FAKE_PASEO_EXIT:-0}\" != \"0\" ]; then\n" +
		"      printf '%s\\n' 'paseo: cannot read the agent store' >&2\n" +
		"      exit \"$FAKE_PASEO_EXIT\"\n" +
		"    fi\n" +
		"    case \"${FAKE_PASEO_OUTPUT:-}\" in\n" +
		"      garbage) printf '%s\\n' 'paseo: this is not json' ; exit 0 ;;\n" +
		"      nostatus) printf '%s\\n' '{\"AgentId\":\"ag_x\",\"Title\":\"no status here\"}' ; exit 0 ;;\n" +
		"    esac\n" +
		"    id=\"$2\"\n" +
		"    if [ -z \"$FAKE_PASEO_STATUS_FILE\" ] || [ ! -f \"$FAKE_PASEO_STATUS_FILE\" ]; then\n" +
		"      printf 'paseo: unknown agent %s\\n' \"$id\" >&2\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    status=\"$(grep -m1 -- \"^$id \" \"$FAKE_PASEO_STATUS_FILE\" | cut -d' ' -f2-)\"\n" +
		"    if [ -z \"$status\" ]; then\n" +
		"      printf '{\\n  \"error\": {\\n    \"code\": \"AGENT_NOT_FOUND\",\\n    \"message\": \"Agent not found: %s\"\\n  }\\n}\\n' \"$id\" >&2\n" +
		"      exit 1\n" +
		"    fi\n" +
		// Real paseo pretty-prints its JSON; the fake must too, so a
		// line-at-a-time parser regression would stay visible.
		"    printf '{\\n  \"AgentId\": \"%s\",\\n  \"Status\": \"%s\"\\n}\\n' \"$id\" \"$status\"\n" +
		"    exit 0 ;;\n" +
		"esac\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake paseo: %v", err)
	}
	return script
}

// reconcileHarness owns one temp ADB root, one fake clock and one fake paseo.
type reconcileHarness struct {
	t      *testing.T
	root   string
	beacon string
	clock  *fakeClock
	store  *Store
	bin    string
	// statusFile is where the fake paseo reads "<agent-id> <status>".
	statusFile string
	argvLog    string
	statuses   map[string]string
}

func newReconcileHarness(t *testing.T) *reconcileHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h := &reconcileHarness{
		t:          t,
		root:       root,
		beacon:     beacon,
		clock:      &fakeClock{t: start},
		store:      &Store{Path: filepath.Join(beacon, "adbd-state.json"), SlowPath: filepath.Join(beacon, "adbd-slowpath.jsonl")},
		argvLog:    filepath.Join(root, "paseo-argv.log"),
		statusFile: filepath.Join(root, "agent-status.txt"),
		statuses:   map[string]string{},
	}
	h.bin = writeFakeStatusPaseo(t, h.argvLog)
	t.Setenv("FAKE_PASEO_ARGV_LOG", h.argvLog)
	t.Setenv("FAKE_PASEO_STATUS_FILE", h.statusFile)
	// The CLI builds its own Reconciler, so the fake clock has to reach that seam
	// too; without it finished_at would be the wall clock.
	oldClock := reconcileClock
	reconcileClock = h.clock.now
	t.Cleanup(func() { reconcileClock = oldClock })
	return h
}

// status makes the fake paseo answer "<status>" for these agent ids.
func (h *reconcileHarness) status(status string, ids ...string) {
	h.t.Helper()
	for _, id := range ids {
		h.statuses[id] = status
	}
	var buf strings.Builder
	for id, value := range h.statuses {
		buf.WriteString(id + " " + value + "\n")
	}
	if err := os.WriteFile(h.statusFile, []byte(buf.String()), 0o644); err != nil {
		h.t.Fatalf("write agent status: %v", err)
	}
}

func (h *reconcileHarness) loadState() *snapshot {
	h.t.Helper()
	snap, err := h.store.load()
	if err != nil {
		h.t.Fatalf("load state: %v", err)
	}
	return snap
}

func (h *reconcileHarness) saveState(snap *snapshot) {
	h.t.Helper()
	if err := os.MkdirAll(h.beacon, 0o755); err != nil {
		h.t.Fatalf("mkdir beacon: %v", err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		h.t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(h.store.Path, append(data, '\n'), 0o644); err != nil {
		h.t.Fatalf("write state: %v", err)
	}
}

func (h *reconcileHarness) putBinding(binding *Binding) {
	h.t.Helper()
	snap := h.loadState()
	snap.Bindings[bindingKey(binding.Channel, binding.Thread)] = binding
	h.saveState(snap)
}

// bind inserts one leased binding as the router would have left it.
func (h *reconcileHarness) bind(thread, agentID, lastSeen string) *Binding {
	h.t.Helper()
	now := h.clock.now()
	binding := &Binding{
		Channel:        DefaultFeedChannel,
		Thread:         thread,
		AgentID:        agentID,
		State:          StateBound,
		TTL:            DefaultTTL,
		ExpiresAt:      now.Add(DefaultTTL),
		UpdatedAt:      now,
		LastSeenStatus: lastSeen,
	}
	h.putBinding(binding)
	return binding
}

func (h *reconcileHarness) bindingFor(thread string) *Binding {
	h.t.Helper()
	return h.loadState().Bindings[bindingKey(DefaultFeedChannel, thread)]
}

// reconcile runs one pass through the in-process seam.
func (h *reconcileHarness) reconcile() (ReconcileResult, error) {
	h.t.Helper()
	return NewReconciler(h.store, PaseoStatusRunner{Bin: h.bin}).Reconcile()
}

// reconcileOK runs one pass and fails the test on error.
func (h *reconcileHarness) reconcileOK() ReconcileResult {
	h.t.Helper()
	res, err := h.reconcile()
	if err != nil {
		h.t.Fatalf("reconcile: %v", err)
	}
	return res
}

// args pins the CLI at this harness's temp files.
func (h *reconcileHarness) args(extra ...string) []string {
	args := []string{
		"reconcile", "--once",
		"--root", h.root,
		"--state", h.store.Path,
		"--slow-path", h.store.SlowPath,
		"--config", filepath.Join(h.beacon, "config"),
		"--runner-bin", h.bin,
	}
	return append(args, extra...)
}

func (h *reconcileHarness) cli(extra ...string) (int, string, string) {
	h.t.Helper()
	return runCLI(h.args(extra...))
}

func (h *reconcileHarness) reportsPath() string {
	return filepath.Join(h.beacon, reportsFile)
}

func (h *reconcileHarness) execStatePath() string {
	return filepath.Join(h.beacon, execStateFile)
}

// reportLines returns the raw receipt lines; a missing ledger is no receipts.
func (h *reconcileHarness) reportLines() []string {
	h.t.Helper()
	data, err := os.ReadFile(h.reportsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read reports: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func (h *reconcileHarness) reports() []workerReport {
	h.t.Helper()
	var out []workerReport
	for _, line := range h.reportLines() {
		var report workerReport
		if err := json.Unmarshal([]byte(line), &report); err != nil {
			h.t.Fatalf("receipt line %q is not JSON: %v", line, err)
		}
		out = append(out, report)
	}
	return out
}

// execDoc reads the exec_state document; a missing file is an empty one.
func (h *reconcileHarness) execDoc() execStateDoc {
	h.t.Helper()
	doc := execStateDoc{}
	data, err := os.ReadFile(h.execStatePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc
		}
		h.t.Fatalf("read exec state: %v", err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		h.t.Fatalf("exec state is not JSON: %v", err)
	}
	return doc
}

// snapshotOf captures one file's bytes, or "" when it does not exist.
func (h *reconcileHarness) snapshotOf(path string) string {
	h.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ""
		}
		h.t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// paseoCalls returns the recorded argv lines the fake paseo saw.
func (h *reconcileHarness) paseoCalls() []string {
	h.t.Helper()
	data, err := os.ReadFile(h.argvLog)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read paseo argv log: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// TC-WR-001 (AC-001): one pass counts what it looked at
// ---------------------------------------------------------------------------

func TestReconcileCountsBindings(t *testing.T) {
	h := newReconcileHarness(t)

	// AC-001: no bindings -> checked=0 and nothing else happens.
	code, stdout, stderr := h.cli()
	if code != 0 {
		t.Fatalf("reconcile with no bindings exited %d: %s", code, stderr)
	}
	if stdout != "checked=0 done=0 running=0\n" {
		t.Fatalf("empty root printed %q", stdout)
	}
	if _, err := os.Stat(h.reportsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a pass with nothing to check wrote a receipt ledger: %v", err)
	}
	if _, err := os.Stat(h.execStatePath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a pass with nothing to check wrote an exec state file: %v", err)
	}
	if calls := h.paseoCalls(); len(calls) != 0 {
		t.Fatalf("no bindings, but paseo was asked: %v", calls)
	}

	// One agent still working, one that just finished.
	h.bind("skill:alpha:one", "ag_alpha", "")
	h.bind("skill:alpha:two", "ag_beta", StatusRunning)
	h.status(StatusRunning, "ag_alpha")
	h.status(StatusIdle, "ag_beta")

	res := h.reconcileOK()
	if res.Checked != 2 || res.Running != 1 || res.Done != 1 {
		t.Fatalf("counts = checked:%d running:%d done:%d, want 2/1/1",
			res.Checked, res.Running, res.Done)
	}
	if got := h.paseoCalls(); len(got) != 2 {
		t.Fatalf("expected one inspect per agent, got %v", got)
	}
	for _, call := range h.paseoCalls() {
		if !strings.HasPrefix(call, "inspect ") || !strings.HasSuffix(call, " --json") {
			t.Fatalf("status read is not `paseo inspect <id> --json`: %q", call)
		}
	}

	code, stdout, stderr = h.cli()
	if code != 0 {
		t.Fatalf("reconcile exited %d: %s", code, stderr)
	}
	// AC-005 in the same breath: the second pass sees one already-done thread and
	// one still-running one, so nothing new completes.
	if !strings.HasPrefix(stdout, "checked=1 done=0 running=1") {
		t.Fatalf("second pass printed %q", stdout)
	}
}

// ---------------------------------------------------------------------------
// TC-WR-002 (AC-002): running -> idle writes exactly one receipt
// ---------------------------------------------------------------------------

func TestRunningToIdleRecordsReport(t *testing.T) {
	h := newReconcileHarness(t)
	dispatched := h.clock.now()
	binding := h.bind("skill:alpha:one", "ag_alpha", "")

	// AC-001: a first `running` sighting is remembered, but it is not a receipt.
	h.status(StatusRunning, "ag_alpha")
	res := h.reconcileOK()
	if res.Running != 1 || res.Done != 0 {
		t.Fatalf("running pass = running:%d done:%d, want 1/0", res.Running, res.Done)
	}
	afterRunning := h.bindingFor("skill:alpha:one")
	if afterRunning.State != StateBound {
		t.Fatalf("a running worker left the binding in %q, want %q", afterRunning.State, StateBound)
	}
	if afterRunning.LastSeenStatus != StatusRunning {
		t.Fatalf("LastSeenStatus = %q, want %q", afterRunning.LastSeenStatus, StatusRunning)
	}
	if !afterRunning.UpdatedAt.Equal(binding.UpdatedAt) {
		t.Fatalf("a running sighting renewed the activity clock (%s -> %s): "+
			"the watchdog must still be able to call a busy-but-silent worker stalled",
			binding.UpdatedAt, afterRunning.UpdatedAt)
	}
	if lines := h.reportLines(); len(lines) != 0 {
		t.Fatalf("a running worker produced a receipt: %v", lines)
	}
	if _, err := os.Stat(h.execStatePath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a running worker wrote an exec state file: %v", err)
	}

	// AC-002: idle after running = done, one receipt, exactly five fields.
	h.clock.advance(2 * time.Minute)
	finished := h.clock.now()
	h.status(StatusIdle, "ag_alpha")
	res = h.reconcileOK()
	if res.Done != 1 || res.Running != 0 || res.Checked != 1 {
		t.Fatalf("idle pass = checked:%d running:%d done:%d, want 1/0/1",
			res.Checked, res.Running, res.Done)
	}
	if got := h.bindingFor("skill:alpha:one").State; got != StateDone {
		t.Fatalf("binding state = %q after running->idle, want %q", got, StateDone)
	}

	lines := h.reportLines()
	if len(lines) != 1 {
		t.Fatalf("expected exactly one receipt, got %v", lines)
	}
	// AC-002 froze the field set; the order is checked too, so a future field
	// cannot be smuggled in silently.
	order := []string{`"thread"`, `"agent_id"`, `"dispatched_at"`, `"finished_at"`, `"status"`}
	last := -1
	for _, key := range order {
		at := strings.Index(lines[0], key)
		if at < 0 {
			t.Fatalf("receipt is missing %s: %s", key, lines[0])
		}
		if at < last {
			t.Fatalf("receipt fields are out of order: %s", lines[0])
		}
		last = at
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &fields); err != nil {
		t.Fatalf("receipt is not JSON: %v", err)
	}
	if len(fields) != len(order) {
		t.Fatalf("receipt has %d fields, want %d: %s", len(fields), len(order), lines[0])
	}
	report := h.reports()[0]
	if report.Thread != "skill:alpha:one" || report.AgentID != "ag_alpha" {
		t.Fatalf("receipt names the wrong thread/agent: %+v", report)
	}
	if report.DispatchedAt != dispatched.Format(time.RFC3339) {
		t.Fatalf("dispatched_at = %q, want %q", report.DispatchedAt, dispatched.Format(time.RFC3339))
	}
	if report.FinishedAt != finished.Format(time.RFC3339) {
		t.Fatalf("finished_at = %q, want %q", report.FinishedAt, finished.Format(time.RFC3339))
	}
	if report.Status != StateDone {
		t.Fatalf("receipt status = %q, want %q", report.Status, StateDone)
	}

	// AC-005: a third pass changes nothing.
	h.clock.advance(2 * time.Minute)
	res = h.reconcileOK()
	if res.Done != 0 || res.Checked != 0 {
		t.Fatalf("third pass re-counted a finished thread: checked:%d done:%d",
			res.Checked, res.Done)
	}
	if got := len(h.reportLines()); got != 1 {
		t.Fatalf("a finished thread was re-receipted (%d lines)", got)
	}

	// AC-002 "同一 agent 只写一次": the same worker finishing a second thread
	// completes that binding too, but does not earn a second receipt.
	h.bind("skill:alpha:two", "ag_alpha", StatusRunning)
	h.status(StatusIdle, "ag_alpha")
	res = h.reconcileOK()
	if res.Done != 1 {
		t.Fatalf("the second thread on the same agent did not complete (done=%d)", res.Done)
	}
	if got := h.bindingFor("skill:alpha:two").State; got != StateDone {
		t.Fatalf("second binding state = %q, want %q", got, StateDone)
	}
	if got := h.reportLines(); len(got) != 1 {
		t.Fatalf("the same agent earned %d receipts, want 1", len(got))
	}
}

// ---------------------------------------------------------------------------
// TC-WR-003 (AC-003): req_id mapping + atomic exec_state
// ---------------------------------------------------------------------------

func TestExecStateFileMapping(t *testing.T) {
	h := newReconcileHarness(t)
	h.bind("skill:alpha:one", "ag_alpha", StatusRunning)
	h.bind("gh:sample/repo#12", "ag_gh", StatusRunning)
	h.status(StatusIdle, "ag_alpha", "ag_gh")

	finished := h.clock.now()
	res := h.reconcileOK()
	if res.Done != 2 {
		t.Fatalf("done = %d, want 2", res.Done)
	}

	doc := h.execDoc()
	if len(doc) != 1 {
		t.Fatalf("exec state has %d entries, want 1 (only skill threads map): %+v", len(doc), doc)
	}
	entry, ok := doc["req:alpha:one"]
	if !ok {
		t.Fatalf("exec state is missing req:alpha:one: %+v", doc)
	}
	if entry.ExecState != StateDone || entry.Worker != "ag_alpha" || entry.Thread != "skill:alpha:one" {
		t.Fatalf("exec entry = %+v", entry)
	}
	if entry.UpdatedAt != finished.Format(time.RFC3339) {
		t.Fatalf("exec updated_at = %q, want %q", entry.UpdatedAt, finished.Format(time.RFC3339))
	}

	// The github thread is still finished work: it earns a receipt, it just has
	// no requirement to write an exec_state for.
	if got := len(h.reportLines()); got != 2 {
		t.Fatalf("expected a receipt for both agents, got %d", got)
	}

	// AC-003 "文件原子写入": no temp file survives, and the file is a single
	// parsable JSON object with a trailing newline.
	entries, err := os.ReadDir(h.beacon)
	if err != nil {
		t.Fatalf("read beacon dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".adbd-write-") {
			t.Fatalf("atomic write left a temp file behind: %s", entry.Name())
		}
	}
	raw := h.snapshotOf(h.execStatePath())
	if !strings.HasSuffix(raw, "\n") {
		t.Fatalf("exec state does not end with a newline: %q", raw)
	}
	var parsed map[string]execStateEntry
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("exec state is not one JSON object: %v", err)
	}

	// AC-003: every non-skill thread shape is refused, not guessed at.
	for _, thread := range []string{
		"skill:alpha",
		"skill:alpha:one:extra",
		"gh:sample/repo#12",
		"skill:Alpha:one",
	} {
		if got := reqIDForThread(thread); got != "" {
			t.Fatalf("thread %q mapped to %q, want no mapping", thread, got)
		}
	}
	if got := reqIDForThread("skill:alpha:one"); got != "req:alpha:one" {
		t.Fatalf("reqIDForThread = %q, want req:alpha:one", got)
	}
}

// ---------------------------------------------------------------------------
// TC-WR-005 (AC-005): reconcile is idempotent
// ---------------------------------------------------------------------------

func TestReconcileIdempotent(t *testing.T) {
	h := newReconcileHarness(t)
	h.bind("skill:alpha:one", "ag_alpha", StatusRunning)
	h.status(StatusIdle, "ag_alpha")

	if res := h.reconcileOK(); res.Done != 1 {
		t.Fatalf("first pass done = %d, want 1", res.Done)
	}
	firstReports := h.snapshotOf(h.reportsPath())
	firstExec := h.snapshotOf(h.execStatePath())
	if firstReports == "" || firstExec == "" {
		t.Fatalf("first pass wrote no ledgers: reports=%q exec=%q", firstReports, firstExec)
	}

	for attempt := 2; attempt <= 4; attempt++ {
		h.clock.advance(time.Minute)
		// The fake paseo keeps saying idle: the interesting part is that a
		// finished thread is no longer even asked about.
		h.status(StatusIdle, "ag_alpha")
		res := h.reconcileOK()
		if res.Done != 0 {
			t.Fatalf("pass %d re-counted a finished thread (done=%d)", attempt, res.Done)
		}
		if res.Checked != 0 {
			t.Fatalf("pass %d checked a finished thread (checked=%d)", attempt, res.Checked)
		}
		if got := h.snapshotOf(h.reportsPath()); got != firstReports {
			t.Fatalf("pass %d rewrote the receipt ledger:\n%s\n->\n%s", attempt, firstReports, got)
		}
		if got := h.snapshotOf(h.execStatePath()); got != firstExec {
			t.Fatalf("pass %d rewrote the exec state:\n%s\n->\n%s", attempt, firstExec, got)
		}
	}
}

// ---------------------------------------------------------------------------
// TC-WR-006 (AC-006): an unreadable paseo mutates nothing
// ---------------------------------------------------------------------------

func TestPaseoUnavailableNoMutation(t *testing.T) {
	cases := []struct {
		name string
		// setup returns the binary to hand the reconciler.
		setup func(h *reconcileHarness) string
	}{
		{
			name: "missing binary",
			setup: func(h *reconcileHarness) string {
				return filepath.Join(h.root, "no-such-paseo")
			},
		},
		{
			name: "non-zero exit",
			setup: func(h *reconcileHarness) string {
				h.t.Setenv("FAKE_PASEO_EXIT", "3")
				return h.bin
			},
		},
		{
			name: "unparsable output",
			setup: func(h *reconcileHarness) string {
				h.t.Setenv("FAKE_PASEO_OUTPUT", "garbage")
				return h.bin
			},
		},
		{
			name: "no status field",
			setup: func(h *reconcileHarness) string {
				h.t.Setenv("FAKE_PASEO_OUTPUT", "nostatus")
				return h.bin
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newReconcileHarness(t)
			// Two leased threads: one that would complete, one still running. If
			// the pass mutated anything it would flip the first one to done.
			h.bind("skill:alpha:one", "ag_alpha", StatusRunning)
			h.bind("skill:alpha:two", "ag_beta", "")
			h.status(StatusIdle, "ag_alpha")
			h.status(StatusRunning, "ag_beta")

			// Pre-existing ledgers, so "no mutation" is provable byte for byte.
			if err := os.MkdirAll(h.beacon, 0o755); err != nil {
				t.Fatalf("mkdir beacon: %v", err)
			}
			if err := os.WriteFile(h.reportsPath(), []byte(`{"thread":"skill:alpha:old","agent_id":"ag_old","dispatched_at":"2026-02-06T08:00:00Z","finished_at":"2026-02-06T08:30:00Z","status":"done"}`+"\n"), 0o644); err != nil {
				t.Fatalf("seed reports: %v", err)
			}
			if err := os.WriteFile(h.execStatePath(), []byte("{\n  \"req:alpha:old\": {\n    \"exec_state\": \"done\",\n    \"worker\": \"ag_old\",\n    \"thread\": \"skill:alpha:old\",\n    \"updated_at\": \"2026-02-06T08:30:00Z\"\n  }\n}\n"), 0o644); err != nil {
				t.Fatalf("seed exec state: %v", err)
			}
			stateBefore := h.snapshotOf(h.store.Path)
			reportsBefore := h.snapshotOf(h.reportsPath())
			execBefore := h.snapshotOf(h.execStatePath())

			bin := tc.setup(h)
			code, stdout, stderr := runCLI([]string{
				"reconcile", "--once",
				"--root", h.root,
				"--state", h.store.Path,
				"--slow-path", h.store.SlowPath,
				"--config", filepath.Join(h.beacon, "config"),
				"--runner-bin", bin,
			})
			if code != 2 {
				t.Fatalf("exit code = %d, want 2 (stdout %q, stderr %q)", code, stdout, stderr)
			}
			if !strings.Contains(stdout, "reason_code="+ReasonPaseoUnavailable) {
				t.Fatalf("stdout %q does not report %s", stdout, ReasonPaseoUnavailable)
			}

			if got := h.snapshotOf(h.store.Path); got != stateBefore {
				t.Fatalf("state document changed on a failed pass:\n%s\n->\n%s", stateBefore, got)
			}
			if got := h.snapshotOf(h.reportsPath()); got != reportsBefore {
				t.Fatalf("receipt ledger changed on a failed pass:\n%s\n->\n%s", reportsBefore, got)
			}
			if got := h.snapshotOf(h.execStatePath()); got != execBefore {
				t.Fatalf("exec state changed on a failed pass:\n%s\n->\n%s", execBefore, got)
			}
			if got := h.bindingFor("skill:alpha:one").State; got != StateBound {
				t.Fatalf("binding state = %q after a failed pass, want %q", got, StateBound)
			}
			if got := h.bindingFor("skill:alpha:one").LastSeenStatus; got != StatusRunning {
				t.Fatalf("LastSeenStatus = %q after a failed pass, want %q", got, StatusRunning)
			}

			// AC-006 is also provable in-process: the typed error is what the CLI
			// maps, and it must wrap the underlying cause.
			_, err := NewReconciler(h.store, PaseoStatusRunner{Bin: bin}).Reconcile()
			var unavailable *PaseoUnavailableError
			if !errors.As(err, &unavailable) {
				t.Fatalf("in-process error = %v, want a PaseoUnavailableError", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// done is terminal on the intake side too
// ---------------------------------------------------------------------------

func TestDoneBindingTakesSlowPath(t *testing.T) {
	// A finished thread holds no worker any more. A follow-up message must go
	// back to the front desk, and the runner must not be called at all -- the
	// dangerous shape here is "send first, fail the transition after".
	h := newHarness(t)
	h.mustBind(DefaultFeedChannel, "skill:alpha:one", "ag_alpha", time.Hour)
	snap, err := h.core.Store.load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	// Simulate the reconcile pass having observed running -> idle.
	snap.Bindings[bindingKey(DefaultFeedChannel, "skill:alpha:one")].State = StateDone
	if err := h.core.Store.save(snap); err != nil {
		t.Fatalf("save state: %v", err)
	}

	res := h.mustIntake(DefaultFeedChannel, "skill:alpha:one", "msg-1", "next round of work")
	if res.Route != RouteSlow || res.ReasonCode != ReasonNoBinding {
		t.Fatalf("intake on a done thread = %s/%s, want %s/%s",
			res.Route, res.ReasonCode, RouteSlow, ReasonNoBinding)
	}
	if res.State != StateDone {
		t.Fatalf("reported state = %q, want %q", res.State, StateDone)
	}
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("a done thread still reached the runner: %v", calls)
	}
	if recs := h.slowPathRecords(); len(recs) != 1 || recs[0].ReasonCode != ReasonNoBinding {
		t.Fatalf("slow path = %+v, want one no_binding record", recs)
	}
	if got := h.status(DefaultFeedChannel, "skill:alpha:one").State; got != StateDone {
		t.Fatalf("a done binding was moved to %q by a slow-path intake", got)
	}
}

// ---------------------------------------------------------------------------
// TC-WR-007 (AC-007): a finished thread is not a stall
// ---------------------------------------------------------------------------

func TestDoneBindingNotStalled(t *testing.T) {
	h := newWatchHarness(t)
	// The thread was dispatched 25 minutes ago and finished: the watchdog's
	// threshold (20m) has long passed, but the work is done, not stuck.
	stale := h.base.Add(-25 * time.Minute)
	h.putBinding(&Binding{
		Channel:   DefaultFeedChannel,
		Thread:    "skill:alpha:one",
		AgentID:   "ag_alpha",
		State:     StateDone,
		TTL:       DefaultTTL,
		ExpiresAt: h.base.Add(time.Hour),
		UpdatedAt: stale,
	})
	h.heartbeat(h.base)

	stdout, alerts := h.scan()
	if len(alerts) != 0 {
		t.Fatalf("a done binding raised %d alerts: %+v", len(alerts), alerts)
	}
	if strings.Contains(stdout, KindStalled) {
		t.Fatalf("a done binding was reported stalled: %q", stdout)
	}
	if !strings.Contains(stdout, "NOTHING") {
		t.Fatalf("clean scan printed %q", stdout)
	}

	// Control: the same binding in a live state is a genuine stall, so the test
	// above proves the done skip rather than a broken scan.
	h.putBinding(&Binding{
		Channel:   DefaultFeedChannel,
		Thread:    "skill:alpha:one",
		AgentID:   "ag_alpha",
		State:     StateBound,
		TTL:       DefaultTTL,
		ExpiresAt: h.base.Add(time.Hour),
		UpdatedAt: stale,
	})
	stdout, alerts = h.scan()
	if len(alerts) == 0 {
		t.Fatalf("the control binding produced no alert: %q", stdout)
	}
	if alerts[0].Kind != KindStalled || alerts[0].Thread != "skill:alpha:one" {
		t.Fatalf("control alert = %+v, want kind=%s thread=skill:alpha:one", alerts[0], KindStalled)
	}
}

// AC-001 amendment: a worker paseo no longer knows is a per-binding condition.
// One stale binding must be released, and the pass must keep going.
func TestReconcileReleasesMissingAgentAndContinues(t *testing.T) {
	h := newReconcileHarness(t)
	h.bind("t-gone", "ag_gone", "")
	h.bind("t-alive", "ag_alive", "")
	h.status(StatusRunning, "ag_alive")

	res := h.reconcileOK()
	if res.Released != 1 {
		t.Fatalf("released = %d, want 1", res.Released)
	}
	if res.Running != 1 {
		t.Fatalf("running = %d, want 1 (the live binding must still be observed)", res.Running)
	}
	if b := h.bindingFor("t-gone"); b == nil || b.State != StateReleased {
		t.Fatalf("gone binding = %+v, want released", b)
	}
	if b := h.bindingFor("t-alive"); b == nil || b.State != StateBound {
		t.Fatalf("live binding = %+v, want still bound", b)
	}
}

// AC-002 amendment: a worker that starts and stops between two passes is never
// caught in "running". Past the grace window an idle agent counts as finished.
func TestReconcileIdleAfterGraceIsDone(t *testing.T) {
	h := newReconcileHarness(t)
	h.bind("t-fast", "ag_fast", "")
	h.status(StatusIdle, "ag_fast")

	// Inside the grace window: not yet a completion.
	if res := h.reconcileOK(); res.Done != 0 {
		t.Fatalf("done inside grace = %d, want 0", res.Done)
	}
	h.clock.advance(ReconcileGrace + time.Second)
	res := h.reconcileOK()
	if res.Done != 1 {
		t.Fatalf("done after grace = %d, want 1", res.Done)
	}
	if b := h.bindingFor("t-fast"); b == nil || b.State != StateDone {
		t.Fatalf("binding = %+v, want done", b)
	}
}
