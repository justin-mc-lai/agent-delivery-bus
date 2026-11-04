package main

// Supervision tests (feature agent-supervisor, v0.1.5): TC-AS-001 .. TC-AS-010.
//
// Every test drives a REAL executable on disk -- a /bin/sh script acting as
// paseo -- so the production seam (argv building, JSON parsing, exit codes) is
// exercised for real. Nothing calls the real paseo and nothing touches the
// network: the script is a temp file written by the test, driven by env vars that
// point at temp state files.
//
// The fake speaks the six verbs this feature uses:
//
//	ls --json                     print the agents payload (the test authors it)
//	inspect <id>                  exit 0 when <id> is known, else 1
//	run ...                       append to "known", print {"agentId":...}
//	agent update <id> --label k=v append "<id>\t<k=v>" to the label log
//	permit allow <id> --all       exit 0 (recorded in the argv log)
//	send <id> <text>              exit 0 (recorded in the argv log)
//	archive <id>                  exit 0 (recorded in the argv log)
//
// Every invocation is appended to an argv log, which is what the assertions read.
// Labels are applied to the payload by the test (see applyLabels) exactly the way
// real paseo would after a successful `agent update`, which is what makes the
// idempotency assertion in TC-AS-003 meaningful.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// the fake paseo
// ---------------------------------------------------------------------------

type supFake struct {
	t          *testing.T
	bin        string
	dir        string
	logPath    string
	agentsPath string
	labelsPath string
	knownPath  string
}

func newSupFake(t *testing.T) *supFake {
	t.Helper()
	dir := t.TempDir()
	f := &supFake{
		t:          t,
		bin:        filepath.Join(dir, "fake-paseo.sh"),
		dir:        dir,
		logPath:    filepath.Join(dir, "argv.log"),
		agentsPath: filepath.Join(dir, "agents.json"),
		labelsPath: filepath.Join(dir, "labels.log"),
		knownPath:  filepath.Join(dir, "known-agents.txt"),
	}
	filterPath := filepath.Join(dir, "label-filter.py")
	if err := os.WriteFile(filterPath, []byte(`import json, sys
path, kv = sys.argv[1], sys.argv[2]
key, value = kv.split("=", 1)
doc = json.load(open(path))
agents = doc.get("agents", doc) if isinstance(doc, dict) else doc
keep = [a for a in agents if (a.get("labels") or {}).get(key) == value]
print(json.dumps({"agents": keep}))
`), 0o644); err != nil {
		t.Fatalf("write label filter: %v", err)
	}
	script := `#!/bin/sh
if [ -n "$FAKE_SUP_LOG" ]; then printf '%s\n' "$*" >> "$FAKE_SUP_LOG"; fi
case "$1" in
__ADBD_LIVENESS_ARMS__  ls)
    if [ "$2" = "--label" ]; then
      if [ -n "$FAKE_SUP_AGENTS" ] && [ -f "$FAKE_SUP_AGENTS" ]; then
        python3 "__ADBD_LABEL_FILTER__" "$FAKE_SUP_AGENTS" "$3"
      else
        printf '{"agents":[]}\n'
      fi
      exit 0
    fi
    if [ -n "$FAKE_SUP_AGENTS" ] && [ -f "$FAKE_SUP_AGENTS" ]; then
      cat "$FAKE_SUP_AGENTS"
    else
      printf '{"agents":[]}\n'
    fi
    exit 0 ;;
  inspect)
    if [ -n "$FAKE_SUP_KNOWN" ] && [ -f "$FAKE_SUP_KNOWN" ] && grep -qx -- "$2" "$FAKE_SUP_KNOWN"; then
      printf '{\n  "agentId": "%s",\n  "status": "idle"\n}\n' "$2"
      exit 0
    fi
    exit 1 ;;
  run)
    id="${FAKE_SUP_RUN_ID:-ag_new}"
    if [ -n "$FAKE_SUP_KNOWN" ]; then printf '%s\n' "$id" >> "$FAKE_SUP_KNOWN"; fi
    printf '{\n  "agentId": "%s",\n  "status": "running"\n}\n' "$id"
    exit 0 ;;
  agent)
    if [ "$2" = "update" ]; then
      shift 2
      id="$1"; shift
      while [ $# -gt 0 ]; do
        case "$1" in
          --label) printf '%s\t%s\n' "$id" "$2" >> "$FAKE_SUP_LABELS"; shift 2 ;;
          *) shift ;;
        esac
      done
    fi
    exit 0 ;;
esac
exit 0
`
	script = strings.ReplaceAll(script, "__ADBD_LABEL_FILTER__", filterPath)
	// dispatch-liveness AC-002: `send` records a timeline marker and `logs` answers
	// the probe, so a routed message is verifiable instead of unreadable.
	script = strings.ReplaceAll(script, "__ADBD_LIVENESS_ARMS__", livenessProbeCases(filepath.Join(dir, "timeline")))
	if err := os.WriteFile(f.bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake paseo: %v", err)
	}
	t.Setenv("FAKE_SUP_LOG", f.logPath)
	t.Setenv("FAKE_SUP_AGENTS", f.agentsPath)
	t.Setenv("FAKE_SUP_LABELS", f.labelsPath)
	t.Setenv("FAKE_SUP_KNOWN", f.knownPath)
	return f
}

// fakeAgent is one row of the payload the fake prints for `ls --json`.
type fakeAgent struct {
	AgentID            string            `json:"agentId"`
	Name               string            `json:"name,omitempty"`
	Status             string            `json:"status"`
	Labels             map[string]string `json:"labels"`
	PendingPermissions []fakePermission  `json:"pendingPermissions,omitempty"`
	LastActivityAt     string            `json:"lastActivityAt,omitempty"`
}

// fakePermission is one pending permission request.
type fakePermission struct {
	Tool string `json:"tool"`
	At   string `json:"at,omitempty"`
}

// writeAgents replaces the payload `paseo ls --json` prints.
func (f *supFake) writeAgents(rows ...fakeAgent) {
	f.t.Helper()
	for i := range rows {
		if rows[i].Labels == nil {
			rows[i].Labels = map[string]string{}
		}
	}
	doc := struct {
		Agents []fakeAgent `json:"agents"`
	}{Agents: rows}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		f.t.Fatalf("marshal agents payload: %v", err)
	}
	if err := os.WriteFile(f.agentsPath, append(data, '\n'), 0o644); err != nil {
		f.t.Fatalf("write agents payload: %v", err)
	}
}

// readAgents reads the current payload back.
func (f *supFake) readAgents() []fakeAgent {
	f.t.Helper()
	data, err := os.ReadFile(f.agentsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		f.t.Fatalf("read agents payload: %v", err)
	}
	var doc struct {
		Agents []fakeAgent `json:"agents"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		f.t.Fatalf("agents payload is not JSON: %v", err)
	}
	return doc.Agents
}

// applyLabels is what real paseo does after `agent update --label`: the labels
// land on the agents. The test calls it explicitly so the next pass sees the
// labels the previous pass wrote, which is the whole point of AC-003.
func (f *supFake) applyLabels() {
	f.t.Helper()
	data, err := os.ReadFile(f.labelsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		f.t.Fatalf("read label log: %v", err)
	}
	agents := f.readAgents()
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		id, kv, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		for i := range agents {
			if agents[i].AgentID != id {
				continue
			}
			if agents[i].Labels == nil {
				agents[i].Labels = map[string]string{}
			}
			agents[i].Labels[key] = value
		}
	}
	f.writeAgents(agents...)
}

// calls returns every recorded invocation, argv joined by spaces.
func (f *supFake) calls() []string {
	f.t.Helper()
	data, err := os.ReadFile(f.logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		f.t.Fatalf("read argv log: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// callsWithPrefix returns the invocations that start with prefix.
func (f *supFake) callsWithPrefix(prefix string) []string {
	f.t.Helper()
	var out []string
	for _, line := range f.calls() {
		if line == prefix || strings.HasPrefix(line, prefix+" ") {
			out = append(out, line)
		}
	}
	return out
}

// markKnown records the agent ids the fake will answer `inspect` for.
func (f *supFake) markKnown(ids ...string) {
	f.t.Helper()
	var buf strings.Builder
	for _, id := range ids {
		buf.WriteString(id)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(f.knownPath, []byte(buf.String()), 0o644); err != nil {
		f.t.Fatalf("write known agents: %v", err)
	}
}

// onlyLs fails when the pass called anything other than `ls`: the dry-run
// guarantee of AC-002 and AC-004.
func (f *supFake) onlyLs() {
	f.t.Helper()
	for _, line := range f.calls() {
		if !strings.HasPrefix(line, "ls") {
			f.t.Fatalf("a read-only pass made a call: %q (all calls: %v)", line, f.calls())
		}
	}
}

// ---------------------------------------------------------------------------
// the ADB root
// ---------------------------------------------------------------------------

// supHarness is one ADB root: bindings, reports, config and a fake paseo behind
// it. The clock is fixed so grace and waiting durations are exact.
type supHarness struct {
	t          *testing.T
	root       string
	beacon     string
	fake       *supFake
	now        time.Time
	grace      time.Duration
	maxActive  int
	statePath  string
	slowPath   string
	configPath string
}

// supClock is the instant every supervision test starts from.
var supClock = time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC)

func newSupHarness(t *testing.T) *supHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatalf("mkdir beacon: %v", err)
	}
	h := &supHarness{
		t:          t,
		root:       root,
		beacon:     beacon,
		now:        supClock,
		grace:      5 * time.Minute,
		maxActive:  DefaultMaxActive,
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		configPath: filepath.Join(beacon, "config"),
	}
	h.fake = newSupFake(t)
	// The CLI clock is a package seam (like reconcileClock); pin it at the
	// harness instant so grace windows and waiting durations are exact.
	previousClock := superviseClock
	superviseClock = func() time.Time { return h.now }
	t.Cleanup(func() { superviseClock = previousClock })
	h.writeConfig()
	return h
}

// writeConfig rewrites .beacon/config so the CLI tests drive the real config
// loader (including adbd_grace_after and adbd_max_active).
func (h *supHarness) writeConfig() {
	h.t.Helper()
	body := strings.Join([]string{
		"adbd_allow_all=true",
		"adbd_paseo_bin=" + h.fake.bin,
		"adbd_grace_after=" + h.grace.String(),
		"adbd_max_active=" + strconv.Itoa(h.maxActive),
		"",
	}, "\n")
	if err := os.WriteFile(h.configPath, []byte(body), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

// store is the state document handle.
func (h *supHarness) store() *Store {
	return &Store{Path: h.statePath, SlowPath: h.slowPath}
}

// bind records one live binding: thread -> agent.
func (h *supHarness) bind(thread, agentID string) {
	h.t.Helper()
	err := h.store().Update(func(snap *snapshot) error {
		snap.Bindings[bindingKey("feishu", thread)] = &Binding{
			Channel:   "feishu",
			Thread:    thread,
			AgentID:   agentID,
			State:     StateBound,
			TTL:       DefaultTTL,
			ExpiresAt: h.now.Add(DefaultTTL),
			UpdatedAt: h.now,
		}
		return nil
	})
	if err != nil {
		h.t.Fatalf("bind %s -> %s: %v", thread, agentID, err)
	}
}

// report publishes one work item report.
func (h *supHarness) report(item, status string, at time.Time) {
	h.t.Helper()
	dir := filepath.Join(h.beacon, reportsDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatalf("mkdir reports: %v", err)
	}
	data, err := json.Marshal(WorkItemReport{Status: status, At: at.UTC().Format(time.RFC3339)})
	if err != nil {
		h.t.Fatalf("marshal report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, reportFileName(item)), append(data, '\n'), 0o644); err != nil {
		h.t.Fatalf("write report: %v", err)
	}
}

// supervisor builds one pass at this harness's clock and files.
func (h *supHarness) supervisor(apply bool) *Supervisor {
	return &Supervisor{
		Runner:     PaseoCliRunner{Bin: h.fake.bin},
		Store:      h.store(),
		GraceAfter: h.grace,
		Apply:      apply,
		Now:        func() time.Time { return h.now },
	}
}

// pass runs one supervision pass.
func (h *supHarness) pass(apply bool) SuperviseResult {
	h.t.Helper()
	res, err := h.supervisor(apply).Pass()
	if err != nil {
		h.t.Fatalf("supervise pass (apply=%v): %v", apply, err)
	}
	return res
}

// cli runs one adbd command with --root pinned at this harness, so the flags, the
// config loader and the text rendering are all exercised for real.
func (h *supHarness) cli(args ...string) (int, string, string) {
	h.t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append(args, "--root", h.root), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// audit reads the supervision audit ledger.
func (h *supHarness) audit() []superviseAudit {
	h.t.Helper()
	lines, err := readSuperviseAudit(filepath.Join(h.beacon, superviseAuditFile))
	if err != nil {
		h.t.Fatalf("read audit: %v", err)
	}
	return lines
}

// rowFor finds one item's row.
func rowFor(t *testing.T, res SuperviseResult, item string) SuperviseRow {
	t.Helper()
	for _, row := range res.Rows {
		if row.Item == item {
			return row
		}
	}
	t.Fatalf("no row for item %q (rows: %+v)", item, res.Rows)
	return SuperviseRow{}
}

// expectLines requires every needle to appear somewhere in the output.
func expectLines(t *testing.T, out string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(out, needle) {
			t.Fatalf("output missing %q\n--- output ---\n%s", needle, out)
		}
	}
}

// itemAgent is one agent the ownership tests share: the thread it serves, the
// agent id, and the labels paseo already carries for it.
type itemAgent struct {
	item   string
	agent  string
	labels map[string]string
	name   string
}

// seedOwnership creates a root where two threads are bound and three agents
// exist: one already labelled, one matchable only by name, and one stranger.
func seedOwnership(t *testing.T) (*supHarness, []itemAgent) {
	t.Helper()
	h := newSupHarness(t)
	rows := []itemAgent{
		{item: "skill:demo:a", agent: "ag_label", labels: map[string]string{OwnershipLabel: "skill:demo:a"}},
		{item: "skill:demo:b", agent: "ag_name", name: "skill:demo:b"},
		{item: "", agent: "ag_scratch", name: "scratch-work"},
	}
	for _, row := range rows {
		if row.item != "" {
			h.bind(row.item, row.agent)
		}
	}
	agents := make([]fakeAgent, 0, len(rows))
	for _, row := range rows {
		agents = append(agents, fakeAgent{
			AgentID:        row.agent,
			Name:           row.name,
			Status:         StatusIdle,
			Labels:         row.labels,
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		})
	}
	h.fake.writeAgents(agents...)
	return h, rows
}

// ---------------------------------------------------------------------------
// TC-AS-001: a new worker is born claimed
// ---------------------------------------------------------------------------

func TestWorkerGetsOwnershipLabel(t *testing.T) {
	f := newSupFake(t)
	dispatcher := PaseoDispatcher{Bin: f.bin, FallbackWorkdir: t.TempDir()}

	// A newly created worker carries the ownership label in its own argv: the
	// agent is claimed at birth, not adopted afterwards (AC-001).
	id, err := dispatcher.EnsureWorker("skill:demo:loan", "")
	if err != nil {
		t.Fatalf("EnsureWorker: %v", err)
	}
	if id != "ag_new" {
		t.Fatalf("created worker id = %q, want ag_new", id)
	}
	runs := f.callsWithPrefix("run")
	if len(runs) != 1 {
		t.Fatalf("run calls = %v, want exactly one", runs)
	}
	if !strings.Contains(runs[0], "--label "+OwnershipLabel+"=skill:demo:loan") {
		t.Fatalf("run argv = %q, want it to carry --label %s=skill:demo:loan",
			runs[0], OwnershipLabel)
	}

	// A worker that already exists is reused, not recreated: no second run.
	reused, err := dispatcher.EnsureWorker("skill:demo:loan", id)
	if err != nil {
		t.Fatalf("EnsureWorker reuse: %v", err)
	}
	if reused != id {
		t.Fatalf("reused worker = %q, want %q", reused, id)
	}
	if got := f.callsWithPrefix("run"); len(got) != 1 {
		t.Fatalf("reusing a live worker created another: %v", got)
	}

	// The other half of AC-001: an existing agent is retagged with the paired
	// `paseo agent update <id> --label adbd.item=<thread>` call.
	runner := PaseoCliRunner{Bin: f.bin}
	if err := runner.TagLabel("ag_old", OwnershipLabel, "skill:demo:loan"); err != nil {
		t.Fatalf("TagLabel: %v", err)
	}
	updates := f.callsWithPrefix("agent")
	if len(updates) != 1 {
		t.Fatalf("agent update calls = %v, want exactly one", updates)
	}
	want := "agent update ag_old --label " + OwnershipLabel + "=skill:demo:loan"
	if updates[0] != want {
		t.Fatalf("update call = %q, want %q", updates[0], want)
	}
}

// ---------------------------------------------------------------------------
// TC-AS-002: a dry run classifies and writes nothing
// ---------------------------------------------------------------------------

func TestAdoptDryRunClassifies(t *testing.T) {
	h, _ := seedOwnership(t)

	code, stdout, stderr := h.cli("adopt")
	if code != 0 {
		t.Fatalf("adopt exit = %d (stderr: %s)", code, stderr)
	}
	expectLines(t, stdout,
		"mode=dry_run owned=2 foreign=1",
		"agent=ag_label ownership=owned",
		"label="+OwnershipLabel+"=skill:demo:a",
		"agent=ag_name ownership=owned",
		"item=skill:demo:b",
		"agent=ag_scratch ownership=foreign",
		"label="+ForeignLabel+"=1",
	)
	// Nothing was written and nothing but the read was called (AC-002).
	h.fake.onlyLs()
	if data, err := os.ReadFile(h.fake.labelsPath); err == nil && strings.TrimSpace(string(data)) != "" {
		t.Fatalf("dry run wrote labels: %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(h.beacon, superviseAuditFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run wrote an audit ledger: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TC-AS-003: applying is idempotent
// ---------------------------------------------------------------------------

func TestAdoptApplyIdempotent(t *testing.T) {
	h, _ := seedOwnership(t)

	code, stdout, stderr := h.cli("adopt", "--apply")
	if code != 0 {
		t.Fatalf("adopt --apply exit = %d (stderr: %s)", code, stderr)
	}
	// ag_label already carried the label; only the other two are written.
	expectLines(t, stdout,
		"mode=apply owned=2 foreign=1 tagged=2 skipped=1",
		"agent=ag_label ownership=owned",
	)
	updates := h.fake.callsWithPrefix("agent")
	if len(updates) != 2 {
		t.Fatalf("first apply made %d update calls (%v), want 2", len(updates), updates)
	}
	var sawItem, sawForeign bool
	for _, line := range updates {
		if line == "agent update ag_name --label "+OwnershipLabel+"=skill:demo:b" {
			sawItem = true
		}
		if line == "agent update ag_scratch --label "+ForeignLabel+"=1" {
			sawForeign = true
		}
	}
	if !sawItem || !sawForeign {
		t.Fatalf("update calls = %v, want the item label on ag_name and the foreign label on ag_scratch", updates)
	}

	// paseo applies the labels; the second pass must find nothing left to do.
	h.fake.applyLabels()
	code, stdout, stderr = h.cli("adopt", "--apply")
	if code != 0 {
		t.Fatalf("second adopt exit = %d (stderr: %s)", code, stderr)
	}
	expectLines(t, stdout,
		"mode=apply owned=2 foreign=1 tagged=0 skipped=3",
	)
	for _, want := range []struct{ agent, ownership, label string }{
		{"ag_label", AdoptOwnershipOwned, OwnershipLabel + "=skill:demo:a"},
		{"ag_name", AdoptOwnershipOwned, OwnershipLabel + "=skill:demo:b"},
		{"ag_scratch", AdoptOwnershipForeign, ForeignLabel + "=1"},
	} {
		line := lineFor(stdout, "agent="+want.agent+" ")
		expectLines(t, line, "ownership="+want.ownership, "label="+want.label, "action=skip")
	}
	if got := h.fake.callsWithPrefix("agent"); len(got) != 2 {
		t.Fatalf("the second apply issued %d update calls (%v), want the 2 from the first run only",
			len(got)-2, got)
	}
	// The classification itself never moved.
	if !strings.Contains(stdout, "owned=2 foreign=1") {
		t.Fatalf("second run classified differently:\n%s", stdout)
	}
}

// ---------------------------------------------------------------------------
// TC-AS-004: seven states from the four facts
// ---------------------------------------------------------------------------

func TestSuperviseClassifiesByFourFacts(t *testing.T) {
	h := newSupHarness(t)
	type want struct {
		item    string
		agent   string
		state   string
		because string
	}
	cases := []want{
		{item: "skill:demo:perm", agent: "ag_perm", state: SupStateWaitingPermission, because: "pending_permission"},
		{item: "skill:demo:work", agent: "ag_work", state: SupStateWorking, because: "status=running"},
		{item: "skill:demo:done", agent: "ag_done", state: SupStateDone, because: "report=ready"},
		{item: "skill:demo:attn", agent: "ag_attn", state: SupStateWaitingInput, because: "report=attention"},
		{item: "skill:demo:grace", agent: "ag_grace", state: SupStateGrace, because: "idle_within_grace"},
		{item: "skill:demo:orphan", agent: "ag_orphan", state: SupStateOrphan, because: "idle_beyond_grace"},
		{item: "skill:demo:err", agent: "ag_err", state: SupStateFailed, because: "status=error"},
		{item: "skill:demo:odd", agent: "ag_odd", state: SupStateUnknown, because: "status=warming-up"},
	}
	for _, c := range cases {
		h.bind(c.item, c.agent)
	}
	label := func(item string) map[string]string {
		return map[string]string{OwnershipLabel: item}
	}
	h.fake.writeAgents(
		fakeAgent{
			AgentID: "ag_perm", Status: StatusRunning, Labels: label("skill:demo:perm"),
			PendingPermissions: []fakePermission{{Tool: "bash", At: h.now.Add(-2 * time.Minute).Format(time.RFC3339)}},
			LastActivityAt:     h.now.Add(-2 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_work", Status: StatusRunning, Labels: label("skill:demo:work"),
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_done", Status: StatusIdle, Labels: label("skill:demo:done"),
			LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_attn", Status: StatusIdle, Labels: label("skill:demo:attn"),
			LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_grace", Status: StatusIdle, Labels: label("skill:demo:grace"),
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_orphan", Status: StatusIdle, Labels: label("skill:demo:orphan"),
			LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_err", Status: "error", Labels: label("skill:demo:err"),
			LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_odd", Status: "warming-up", Labels: label("skill:demo:odd"),
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		},
	)
	h.report("skill:demo:done", ReportReady, h.now.Add(-6*time.Minute))
	h.report("skill:demo:attn", ReportAttention, h.now.Add(-10*time.Minute))

	// The dry run is the CLI, so the printed contract is what gets checked.
	code, stdout, stderr := h.cli("supervise")
	if code != 0 {
		t.Fatalf("supervise exit = %d (stderr: %s)", code, stderr)
	}
	res := h.pass(false)
	if len(res.Rows) != len(cases) {
		t.Fatalf("rows = %d, want %d (rows: %+v)", len(res.Rows), len(cases), res.Rows)
	}
	for _, c := range cases {
		wantLine := "item=" + c.item + " state=" + c.state + " because=" + c.because
		expectLines(t, stdout, wantLine)
		row := rowFor(t, res, c.item)
		if row.State != c.state || row.Because != c.because {
			t.Fatalf("%s = state=%s because=%s, want state=%s because=%s",
				c.item, row.State, row.Because, c.state, c.because)
		}
		if row.AgentID != c.agent {
			t.Fatalf("%s agent = %q, want %q", c.item, row.AgentID, c.agent)
		}
	}
	// AC-004: an unrecognised status keeps the raw value in the fact, and a dry
	// run performs nothing at all.
	if row := rowFor(t, res, "skill:demo:odd"); !strings.Contains(row.Because, "warming-up") {
		t.Fatalf("unknown row because = %q, want the raw status", row.Because)
	}
	if strings.Contains(stdout, "performed=") {
		t.Fatalf("a dry run reported a performed action:\n%s", stdout)
	}
	for _, row := range res.Rows {
		if row.State != SupStateWorking && row.State != SupStateGrace && row.State != SupStateUnknown && len(row.Actions) != 0 {
			t.Fatalf("a dry run executed %v on %s", row.Actions, row.Item)
		}
	}
	h.fake.onlyLs()
}

// ---------------------------------------------------------------------------
// TC-AS-005: the permission policy allows cheap tools and escalates the rest
// ---------------------------------------------------------------------------

func TestPermissionPolicyAllowsAndEscalates(t *testing.T) {
	h := newSupHarness(t)
	h.bind("skill:demo:safe", "ag_safe")
	h.bind("skill:demo:risk", "ag_risk")
	h.fake.writeAgents(
		fakeAgent{
			AgentID: "ag_safe", Status: StatusRunning,
			Labels: map[string]string{OwnershipLabel: "skill:demo:safe"},
			PendingPermissions: []fakePermission{
				{Tool: "read_file", At: h.now.Add(-3 * time.Minute).Format(time.RFC3339)},
				{Tool: "bash", At: h.now.Add(-3 * time.Minute).Format(time.RFC3339)},
			},
			LastActivityAt: h.now.Add(-3 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_risk", Status: StatusRunning,
			Labels:             map[string]string{OwnershipLabel: "skill:demo:risk"},
			PendingPermissions: []fakePermission{{Tool: "git push", At: h.now.Add(-time.Minute).Format(time.RFC3339)}},
			LastActivityAt:     h.now.Add(-time.Minute).Format(time.RFC3339),
		},
	)

	code, stdout, stderr := h.cli("supervise", "--apply")
	if code != 0 {
		t.Fatalf("supervise --apply exit = %d (stderr: %s)", code, stderr)
	}
	// A cheap tool set is released automatically; a high-risk verb is not.
	permits := h.fake.callsWithPrefix("permit")
	if len(permits) != 1 || permits[0] != "permit allow ag_safe --all" {
		t.Fatalf("permit calls = %v, want exactly [permit allow ag_safe --all]", permits)
	}
	expectLines(t, stdout,
		"item=skill:demo:safe state=waiting_permission",
		"action=allow",
		"reason=permission_safe",
		"item=skill:demo:risk state=waiting_permission",
		"action=escalate",
		"reason=permission_high_risk",
	)

	// AC-005: one audit line per action, with exactly the five frozen fields.
	audits := h.audit()
	if len(audits) != 2 {
		t.Fatalf("audit lines = %d (%+v), want 2", len(audits), audits)
	}
	byAction := map[string]superviseAudit{}
	for _, line := range audits {
		byAction[line.Action] = line
		if line.Item == "" || line.AgentID == "" || line.Reason == "" || line.At == "" {
			t.Fatalf("audit line has an empty field: %+v", line)
		}
		if _, err := time.Parse(time.RFC3339, line.At); err != nil {
			t.Fatalf("audit at = %q is not RFC3339: %v", line.At, err)
		}
	}
	if allow := byAction[SupActionAllow]; allow.AgentID != "ag_safe" || allow.Reason != "permission_safe" {
		t.Fatalf("allow audit = %+v, want ag_safe/permission_safe", allow)
	}
	if escalate := byAction[SupActionEscalate]; escalate.AgentID != "ag_risk" || escalate.Reason != "permission_high_risk" {
		t.Fatalf("escalate audit = %+v, want ag_risk/permission_high_risk", escalate)
	}
	// The ledger shape itself: five keys, in the frozen order.
	raw, err := os.ReadFile(filepath.Join(h.beacon, superviseAuditFile))
	if err != nil {
		t.Fatalf("read audit ledger: %v", err)
	}
	first := strings.SplitN(strings.TrimSpace(string(raw)), "\n", 2)[0]
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(first), &fields); err != nil {
		t.Fatalf("audit line is not JSON: %v", err)
	}
	if len(fields) != 5 {
		t.Fatalf("audit line has %d fields (%v), want exactly 5: %s", len(fields), fields, first)
	}
	for _, key := range []string{"item", "agent_id", "action", "reason", "at"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("audit line is missing %q: %s", key, first)
		}
	}
	if !strings.HasPrefix(first, `{"item":`) {
		t.Fatalf("audit line does not start with item: %s", first)
	}
}

// ---------------------------------------------------------------------------
// TC-AS-006: an orphan is probed exactly once, then failed
// ---------------------------------------------------------------------------

func TestOrphanProbesOnceThenFails(t *testing.T) {
	h := newSupHarness(t)
	h.bind("skill:demo:orphan", "ag_orphan")
	h.fake.writeAgents(fakeAgent{
		AgentID: "ag_orphan", Status: StatusIdle,
		Labels:         map[string]string{OwnershipLabel: "skill:demo:orphan"},
		LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
	})

	// First pass: one probe, and the probe time is remembered.
	res := h.pass(true)
	row := rowFor(t, res, "skill:demo:orphan")
	if row.State != SupStateOrphan || row.Action != SupActionProbe {
		t.Fatalf("first pass = state=%s action=%s, want orphan/probe", row.State, row.Action)
	}
	sends := h.fake.callsWithPrefix("send")
	if len(sends) != 1 || sends[0] != "send ag_orphan "+probeMessage {
		t.Fatalf("probe calls = %v, want exactly [send ag_orphan %s]", sends, probeMessage)
	}
	doc, err := loadSuperviseDoc(filepath.Join(h.beacon, superviseStateFile))
	if err != nil {
		t.Fatalf("load supervise state: %v", err)
	}
	if doc.Items["skill:demo:orphan"].ProbedAt == "" {
		t.Fatalf("probed_at was not recorded: %+v", doc.Items)
	}
	// AC-006: at most one probe per work item per pass.
	if got := len(h.fake.callsWithPrefix("send")); got != 1 {
		t.Fatalf("first pass sent %d probes, want 1", got)
	}

	// Second pass: still orphaned and already probed -> failed, no second probe.
	res = h.pass(true)
	row = rowFor(t, res, "skill:demo:orphan")
	if row.State != SupStateFailed || row.Action != SupActionFail {
		t.Fatalf("second pass = state=%s action=%s, want failed/fail", row.State, row.Action)
	}
	if got := len(h.fake.callsWithPrefix("send")); got != 1 {
		t.Fatalf("second pass sent another probe (%d total), want exactly 1", got)
	}
	// A failed item is reaped, never left behind (AC-007).
	if got := h.fake.callsWithPrefix("archive"); len(got) != 1 || got[0] != "archive ag_orphan" {
		t.Fatalf("archive calls = %v, want exactly [archive ag_orphan]", got)
	}

	// Third pass: archived means zero acts, forever.
	before := len(h.fake.calls())
	res = h.pass(true)
	row = rowFor(t, res, "skill:demo:orphan")
	if row.State != SupStateArchived || row.Action != SupActionNone {
		t.Fatalf("third pass = state=%s action=%s, want archived/none", row.State, row.Action)
	}
	if after := h.fake.calls(); len(after) != before+1 || !strings.HasPrefix(after[len(after)-1], "ls") {
		t.Fatalf("an archived item produced actions: %v", after[before:])
	}

	// The audit tells the whole story: probe, then fail, then archive.
	var actions []string
	for _, line := range h.audit() {
		actions = append(actions, line.Action)
	}
	want := []string{SupActionProbe, SupActionFail, SupActionArchive}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
}

// ---------------------------------------------------------------------------
// TC-AS-007: done and failed items are archived, never deleted
// ---------------------------------------------------------------------------

func TestTerminalItemsArchived(t *testing.T) {
	h := newSupHarness(t)
	h.bind("skill:demo:done", "ag_done")
	h.bind("skill:demo:err", "ag_err")
	h.bind("skill:demo:work", "ag_work")
	h.fake.writeAgents(
		fakeAgent{
			AgentID: "ag_done", Status: StatusIdle,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:done"},
			LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_err", Status: "error",
			Labels:         map[string]string{OwnershipLabel: "skill:demo:err"},
			LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_work", Status: StatusRunning,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:work"},
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		},
	)
	h.report("skill:demo:done", ReportReady, h.now.Add(-10*time.Minute))

	res := h.pass(true)
	if row := rowFor(t, res, "skill:demo:done"); row.State != SupStateDone || row.Action != SupActionArchive {
		t.Fatalf("done row = state=%s action=%s, want done/archive", row.State, row.Action)
	}
	if row := rowFor(t, res, "skill:demo:err"); row.State != SupStateFailed || row.Action != SupActionArchive {
		t.Fatalf("error row = state=%s action=%s, want failed/archive", row.State, row.Action)
	}
	if row := rowFor(t, res, "skill:demo:work"); row.Action != SupActionNone {
		t.Fatalf("working row action = %s, want none", row.Action)
	}
	archives := h.fake.callsWithPrefix("archive")
	if len(archives) != 2 {
		t.Fatalf("archive calls = %v, want exactly 2", archives)
	}
	if got := h.fake.callsWithPrefix("delete"); len(got) != 0 {
		t.Fatalf("a delete was called: %v", got)
	}
	for _, line := range h.fake.calls() {
		if strings.Contains(line, "delete") {
			t.Fatalf("a call mentioned delete: %q", line)
		}
	}

	// A second pass acts on the terminal items zero times (AC-007).
	before := len(h.fake.calls())
	res = h.pass(true)
	for _, item := range []string{"skill:demo:done", "skill:demo:err"} {
		if row := rowFor(t, res, item); row.State != SupStateArchived || row.Action != SupActionNone {
			t.Fatalf("%s on the second pass = state=%s action=%s, want archived/none",
				item, row.State, row.Action)
		}
	}
	if after := h.fake.calls(); len(after) != before+1 {
		t.Fatalf("second pass made calls: %v", after[before:])
	}
}

// ---------------------------------------------------------------------------
// TC-AS-008: the active ceiling stops new workers, not queued work
// ---------------------------------------------------------------------------

func TestMaxActiveBlocksNewWorker(t *testing.T) {
	h := newSupHarness(t)
	h.maxActive = 2
	h.writeConfig()
	// Two owned agents are working, which is exactly the ceiling.
	h.bind("skill:demo:a", "ag_a")
	h.bind("skill:demo:b", "ag_b")
	h.fake.writeAgents(
		fakeAgent{
			AgentID: "ag_a", Status: StatusRunning,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:a"},
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_b", Status: StatusRunning,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:b"},
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		},
	)
	// A third thread needs a brand new worker.
	if err := os.WriteFile(h.slowPath,
		[]byte(`{"channel":"feishu","thread":"skill:demo:c","message_id":"m-c","text":"add pagination"}`+"\n"),
		0o644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	code, stdout, stderr := h.cli("route", "--once")
	if code != 0 {
		t.Fatalf("route exit = %d (stderr: %s)", code, stderr)
	}
	expectLines(t, stdout, "applied=0", "reason_code="+ReasonMaxActiveReached)
	if got := h.fake.callsWithPrefix("run"); len(got) != 0 {
		t.Fatalf("a worker was created at the ceiling: %v", got)
	}
	if got := h.fake.callsWithPrefix("send"); len(got) != 0 {
		t.Fatalf("work was dispatched at the ceiling: %v", got)
	}
	// AC-008: the queue item keeps its place.
	items, err := readQueue(h.slowPath)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(items) != 1 || items[0].Thread != "skill:demo:c" {
		t.Fatalf("queue = %+v, want the blocked item kept", items)
	}
	if items[0].Attempts != 0 {
		t.Fatalf("a refused creation burned an attempt: %+v", items[0])
	}

	// With room under the ceiling the same item goes through, and the worker it
	// creates carries the ownership label (AC-001 + AC-008 together).
	h.maxActive = 8
	h.writeConfig()
	code, stdout, stderr = h.cli("route", "--once")
	if code != 0 {
		t.Fatalf("second route exit = %d (stderr: %s)", code, stderr)
	}
	expectLines(t, stdout, "applied=1")
	runs := h.fake.callsWithPrefix("run")
	if len(runs) != 1 {
		t.Fatalf("run calls = %v, want exactly one once there is room", runs)
	}
	if !strings.Contains(runs[0], "--label "+OwnershipLabel+"=skill:demo:c") {
		t.Fatalf("created worker argv = %q, want the ownership label for skill:demo:c", runs[0])
	}
}

// ---------------------------------------------------------------------------
// TC-AS-009: the inbox only shows what needs a human
// ---------------------------------------------------------------------------

func TestInboxOnlyNeedsHuman(t *testing.T) {
	h := newSupHarness(t)
	for _, row := range []struct{ item, agent string }{
		{"skill:demo:perm", "ag_perm"},
		{"skill:demo:attn", "ag_attn"},
		{"skill:demo:work", "ag_work"},
		{"skill:demo:done", "ag_done"},
	} {
		h.bind(row.item, row.agent)
	}
	h.fake.writeAgents(
		fakeAgent{
			AgentID: "ag_perm", Status: StatusRunning,
			Labels:             map[string]string{OwnershipLabel: "skill:demo:perm"},
			PendingPermissions: []fakePermission{{Tool: "bash", At: h.now.Add(-2 * time.Minute).Format(time.RFC3339)}},
			LastActivityAt:     h.now.Add(-2 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_attn", Status: StatusIdle,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:attn"},
			LastActivityAt: h.now.Add(-10 * time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_work", Status: StatusRunning,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:work"},
			LastActivityAt: h.now.Add(-time.Minute).Format(time.RFC3339),
		},
		fakeAgent{
			AgentID: "ag_done", Status: StatusIdle,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:done"},
			LastActivityAt: h.now.Add(-30 * time.Minute).Format(time.RFC3339),
		},
	)
	h.report("skill:demo:attn", ReportAttention, h.now.Add(-10*time.Minute))
	h.report("skill:demo:done", ReportReady, h.now.Add(-20*time.Minute))

	code, stdout, stderr := h.cli("inbox")
	if code != 0 {
		t.Fatalf("inbox exit = %d (stderr: %s)", code, stderr)
	}
	lines := nonEmptyLines(stdout)
	if len(lines) != 2 {
		t.Fatalf("inbox printed %d lines (%q), want exactly the 2 human-facing ones", len(lines), stdout)
	}
	// Longest wait first: 10m of waiting input, then 2m of pending permission.
	expectLines(t, lines[0],
		"item=skill:demo:attn", "state="+SupStateWaitingInput, "waiting=10m0s", "agent=ag_attn")
	expectLines(t, lines[1],
		"item=skill:demo:perm", "state="+SupStateWaitingPermission, "waiting=2m0s", "agent=ag_perm")
	// A read-only command: one ls and nothing else.
	h.fake.onlyLs()

	// AC-009: an empty inbox is exactly NOTHING, exit 0.
	empty := newSupHarness(t)
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
	var payload InboxResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("empty json inbox is not JSON: %v (%q)", err, stdout)
	}
	if payload.Count != 0 || len(payload.Entries) != 0 {
		t.Fatalf("empty json inbox = %+v, want count=0", payload)
	}
}

// nonEmptyLines splits output into its non-blank lines.
func nonEmptyLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// lineFor returns the first output line containing needle.
func lineFor(out, needle string) string {
	for _, line := range nonEmptyLines(out) {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// TC-AS-010: working -> grace -> orphan -> failed
// ---------------------------------------------------------------------------

func TestSupervisorStateWalk(t *testing.T) {
	h := newSupHarness(t)
	h.bind("skill:demo:walk", "ag_walk")
	agent := func(status string, idle time.Duration) fakeAgent {
		return fakeAgent{
			AgentID: "ag_walk", Status: status,
			Labels:         map[string]string{OwnershipLabel: "skill:demo:walk"},
			LastActivityAt: h.now.Add(-idle).Format(time.RFC3339),
		}
	}

	var states []string
	step := func(apply bool) SuperviseRow {
		t.Helper()
		res := h.pass(apply)
		row := rowFor(t, res, "skill:demo:walk")
		states = append(states, row.State)
		return row
	}

	// 1. working: the agent is starting a turn.
	h.fake.writeAgents(agent(StatusRunning, time.Minute))
	if row := step(false); row.Action != SupActionNone {
		t.Fatalf("working row action = %s, want none", row.Action)
	}

	// 2. grace: idle, no report, inside the window.
	h.fake.writeAgents(agent(StatusIdle, time.Minute))
	if row := step(false); row.Because != "idle_within_grace" {
		t.Fatalf("grace row because = %q, want idle_within_grace", row.Because)
	}
	if got := h.fake.callsWithPrefix("send"); len(got) != 0 {
		t.Fatalf("a grace item was probed: %v", got)
	}

	// 3. orphan: the same idle agent, now past the window -> one probe.
	h.now = supClock.Add(10 * time.Minute)
	h.fake.writeAgents(agent(StatusIdle, 11*time.Minute))
	if row := step(true); row.State != SupStateOrphan || row.Action != SupActionProbe {
		t.Fatalf("orphan row = state=%s action=%s, want orphan/probe", row.State, row.Action)
	}

	// 4. failed: the probe went unanswered -> archived, never probed twice.
	h.now = supClock.Add(20 * time.Minute)
	h.fake.writeAgents(agent(StatusIdle, 21*time.Minute))
	if row := step(true); row.State != SupStateFailed || row.Action != SupActionFail {
		t.Fatalf("failed row = state=%s action=%s, want failed/fail", row.State, row.Action)
	}

	want := []string{SupStateWorking, SupStateGrace, SupStateOrphan, SupStateFailed}
	if strings.Join(states, " -> ") != strings.Join(want, " -> ") {
		t.Fatalf("state walk = %v, want %v", states, want)
	}
	if got := h.fake.callsWithPrefix("send"); len(got) != 1 {
		t.Fatalf("probe count = %d (%v), want exactly 1", len(got), got)
	}
	if got := h.fake.callsWithPrefix("archive"); len(got) != 1 {
		t.Fatalf("archive count = %d (%v), want exactly 1", len(got), got)
	}
}
