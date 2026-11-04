package main

// Dispatch-liveness tests (feature dispatch-liveness, v0.1.5):
// TC-DL-001 .. TC-DL-007.
//
// Two layers, both real:
//
//   - the production seam (dispatch.go + liveness.go) driven directly, against a
//     /bin/sh fake paseo on disk that answers `run`, `send` and `logs`, so the
//     argv building, the probing and the counting are exercised for real;
//   - the `adbd route --once` command line, so the config keys (workspace mode,
//     check switch, timeout, retries) are proven to reach the dispatcher and the
//     trace, which is what an operator actually runs.
//
// Nothing here calls the real paseo and nothing touches the network.
//
// The fake's behaviour is steered by environment variables, so one script covers
// every scenario: a worker that starts, one that never starts, one that only
// starts on the second send, and one whose timeline cannot be read at all.

import (
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
// the fake paseo's liveness arms, shared with the older fixtures
// ---------------------------------------------------------------------------

// livenessProbeCases returns the `case` arms a fake paseo needs in order to look
// like a real one to the post-send check: `send` records one timeline marker for
// that agent, `logs` prints the markers recorded so far.
//
// The `send` arm deliberately does NOT exit, so the surrounding script still
// records the call the way its own assertions expect; `logs` exits early and is
// therefore invisible to every argv log and invocation counter.
//
// Why the older fixtures need this at all: a stub that cannot answer `logs` reads
// as dispatch_unverified, which is the correct production answer but would turn
// every unrelated routing test red for a reason that has nothing to do with it.
func livenessProbeCases(stateDir string) string {
	dir := shellQuote(stateDir)
	return "  logs)\n" +
		"    f=" + dir + "/timeline.$2\n" +
		"    if [ -f \"$f\" ]; then cat \"$f\"; fi\n" +
		"    exit 0 ;;\n" +
		"  send)\n" +
		"    mkdir -p " + dir + "\n" +
		"    printf '[Message received]\\n' >> " + dir + "/timeline.$2\n" +
		"    ;;\n"
}

// livenessProbeIfs is livenessProbeCases for the fixtures written as a flat
// script rather than a `case` block: the same two behaviours as two guarded
// lines that run before the script records the invocation.
func livenessProbeIfs(stateDir string) string {
	dir := shellQuote(stateDir)
	return "if [ \"$1\" = \"logs\" ]; then\n" +
		"  f=" + dir + "/timeline.$2\n" +
		"  if [ -f \"$f\" ]; then cat \"$f\"; fi\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"send\" ]; then\n" +
		"  mkdir -p " + dir + "\n" +
		"  printf '[Message received]\\n' >> " + dir + "/timeline.$2\n" +
		"fi\n"
}

// ---------------------------------------------------------------------------
// the liveness fake paseo
// ---------------------------------------------------------------------------

// Environment knobs of writeLivenessPaseo:
//
//	FAKE_LIVE_DIR             state/log directory (required)
//	FAKE_LIVE_RUN_ID          agent id `run` prints              (default ag_live)
//	FAKE_LIVE_PROGRESS_AFTER  the send number that starts making progress (default 1)
//	                          0 means NEVER -- a worker that stays silent
//	FAKE_LIVE_LOGS_EXIT       exit code of `logs`                (default 0)
const (
	liveRunIDEnv         = "FAKE_LIVE_RUN_ID"
	liveProgressAfterEnv = "FAKE_LIVE_PROGRESS_AFTER"
	liveLogsExitEnv      = "FAKE_LIVE_LOGS_EXIT"
	liveDirEnv           = "FAKE_LIVE_DIR"
)

// writeLivenessPaseo writes the fake paseo and points it at stateDir.
func writeLivenessPaseo(t *testing.T, stateDir string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "fake-live-paseo.sh")
	body := `#!/bin/sh
dir="$FAKE_LIVE_DIR"
mkdir -p "$dir"
printf '%s\n' "$*" >> "$dir/argv.log"
verb="$1"
case "$verb" in
  inspect) exit 0 ;;
  run)
    printf '{\n  "agentId": "%s",\n  "status": "running"\n}\n' "${FAKE_LIVE_RUN_ID:-ag_live}"
    exit 0 ;;
  send)
    id="$2"
    n=0
    if [ -f "$dir/sends.$id" ]; then n=$(cat "$dir/sends.$id"); fi
    n=$((n + 1))
    printf '%s\n' "$n" > "$dir/sends.$id"
    after="${FAKE_LIVE_PROGRESS_AFTER:-1}"
    if [ "$after" != "0" ] && [ "$n" -ge "$after" ]; then
      printf '[Message received]\n[Thought] working on it\n' >> "$dir/timeline.$id"
    fi
    exit 0 ;;
  logs)
    if [ "${FAKE_LIVE_LOGS_EXIT:-0}" != "0" ]; then
      printf 'paseo: cannot read the timeline\n' >&2
      exit "$FAKE_LIVE_LOGS_EXIT"
    fi
    id="$2"
    if [ -f "$dir/timeline.$id" ]; then cat "$dir/timeline.$id"; fi
    exit 0 ;;
esac
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake liveness paseo: %v", err)
	}
	t.Setenv(liveDirEnv, stateDir)
	t.Setenv(liveRunIDEnv, "ag_live")
	t.Setenv(liveProgressAfterEnv, "1")
	t.Setenv(liveLogsExitEnv, "0")
	return script
}

// liveCalls returns every recorded invocation of the fake, argv joined by spaces.
func liveCalls(t *testing.T, stateDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stateDir, "argv.log"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("read argv log: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// liveVerbs returns the recorded invocations of one verb.
func liveVerbs(t *testing.T, stateDir, verb string) []string {
	t.Helper()
	var out []string
	for _, line := range liveCalls(t, stateDir) {
		if line == verb || strings.HasPrefix(line, verb+" ") {
			out = append(out, line)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// the route CLI harness
// ---------------------------------------------------------------------------

// liveHarness is one temp ADB root: a fake paseo, a config the CLI reads, the
// queue and a trace, all driven through `adbd route --once`.
type liveHarness struct {
	t          *testing.T
	root       string
	bin        string
	stateDir   string
	configPath string
	statePath  string
	slowPath   string
	tracePath  string
}

// defaultLiveConfig is the config a liveness test starts from: a short timeout so
// a dead worker is detected in about a second, and no retries unless the test
// asks for them.
func defaultLiveConfig(bin, workdir string) []string {
	return []string{
		"adbd_paseo_bin=" + bin,
		"adbd_worker_workdir=" + workdir,
		"adbd_worker_dispatch_timeout_sec=1",
		"adbd_worker_dispatch_retries=0",
	}
}

// newLiveHarness builds the root; extra config lines override the defaults.
func newLiveHarness(t *testing.T, extra ...string) *liveHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatalf("mkdir beacon: %v", err)
	}
	h := &liveHarness{
		t:          t,
		root:       root,
		stateDir:   filepath.Join(root, "paseo-state"),
		configPath: filepath.Join(beacon, "config"),
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		tracePath:  filepath.Join(beacon, decisionsFile),
	}
	h.bin = writeLivenessPaseo(t, h.stateDir)
	workdir := filepath.Join(root, "work")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	lines := append(defaultLiveConfig(h.bin, workdir), extra...)
	if err := os.WriteFile(h.configPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return h
}

// route runs the real command line once and returns exit code, stdout, stderr.
func (h *liveHarness) route() (int, string, string) {
	h.t.Helper()
	var stdout, stderr strings.Builder
	code := run([]string{
		"route", "--once",
		"--root", h.root,
		"--state", h.statePath,
		"--slow-path", h.slowPath,
		"--config", h.configPath,
		"--trace", h.tracePath,
		"--outcome", "json",
	}, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// drain runs the CLI and fails the test if it did not exit 0, which is AC-004's
// "a dead worker never kills the router".
func (h *liveHarness) drain() DrainResult {
	h.t.Helper()
	code, stdout, stderr := h.route()
	if code != 0 {
		h.t.Fatalf("adbd route exit = %d, want 0 (stdout: %s stderr: %s)", code, stdout, stderr)
	}
	var res DrainResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		h.t.Fatalf("route output is not JSON: %v (output: %s)", err, stdout)
	}
	return res
}

func (h *liveHarness) enqueue(items ...QueueItem) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.slowPath), 0o755); err != nil {
		h.t.Fatalf("mkdir queue dir: %v", err)
	}
	if err := writeQueue(h.slowPath, items); err != nil {
		h.t.Fatalf("write queue: %v", err)
	}
}

func (h *liveHarness) queue() []QueueItem {
	h.t.Helper()
	items, err := readQueue(h.slowPath)
	if err != nil {
		h.t.Fatalf("read queue: %v", err)
	}
	return items
}

func (h *liveHarness) traces() []traceRecord {
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

func (h *liveHarness) binding(channel, thread string) *Binding {
	h.t.Helper()
	snap, err := (&Store{Path: h.statePath, SlowPath: h.slowPath}).load()
	if err != nil {
		h.t.Fatalf("load state: %v", err)
	}
	return snap.Bindings[bindingKey(channel, thread)]
}

// ---------------------------------------------------------------------------
// TC-DL-001 (AC-001): a new worker is created in a worktree by default
// ---------------------------------------------------------------------------

func TestWorkerUsesWorktreeWorkspace(t *testing.T) {
	root := t.TempDir()
	alphaDir := mustDir(t, filepath.Join(root, "vendor", "alpha"))
	writeProjectRegistry(t, root, []map[string]any{{"slug": "alpha", "repo": "vendor/alpha"}})
	registry := loadProjectRegistry("", root)
	fallback := mustDir(t, filepath.Join(root, "fallback"))

	runArgs := func(t *testing.T, workspace string) []string {
		t.Helper()
		f := newFakePaseo(t)
		t.Setenv("FAKE_PASEO_RUN_AGENT_ID", "ag_ws")
		d := workdirDispatcher(f, registry, fallback, filepath.Join(root, ".beacon", reportsFile))
		d.Workspace = workspace
		if _, err := d.EnsureWorker("skill:alpha:add-retry", ""); err != nil {
			t.Fatalf("EnsureWorker(workspace %q): %v", workspace, err)
		}
		runs := f.verbs("run")
		if len(runs) != 1 {
			t.Fatalf("paseo run calls = %v, want exactly one", runs)
		}
		return strings.Fields(runs[0])
	}

	// ① The default: worktree, not local. This is the whole of AC-001 -- the mode
	// that copies the repository is what aborted the first turn twice.
	args := runArgs(t, "")
	if got := argvValue(args, "--new-workspace"); got != DefaultWorkerWorkspace {
		t.Fatalf("default --new-workspace = %q, want %q (argv: %v)", got, DefaultWorkerWorkspace, args)
	}
	if DefaultWorkerWorkspace != WorkspaceWorktree {
		t.Fatalf("DefaultWorkerWorkspace = %q, want %q", DefaultWorkerWorkspace, WorkspaceWorktree)
	}
	// ... and the project mapping still decides --cwd, exactly as before.
	if got := argvValue(args, "--cwd"); got != alphaDir {
		t.Fatalf("default --cwd = %q, want the registered project %q", got, alphaDir)
	}

	// ② local stays available: some repositories are small enough that the copy is
	// cheaper than a worktree.
	args = runArgs(t, WorkspaceLocal)
	if got := argvValue(args, "--new-workspace"); got != WorkspaceLocal {
		t.Fatalf("--new-workspace = %q, want %q (argv: %v)", got, WorkspaceLocal, args)
	}
	if got := argvValue(args, "--cwd"); got != alphaDir {
		t.Fatalf("local --cwd = %q, want the registered project %q", got, alphaDir)
	}

	// ③ Both modes keep the rest of the frozen argv: title, provider, model, mode
	// and the ownership label.
	for _, want := range []string{"run", "--no-wait", "--json", "--title", "--label", "--mode", "--provider", "--model"} {
		if !containsArg(args, want) {
			t.Fatalf("run argv = %v, want it to still carry %q", args, want)
		}
	}
}

// TestWorkspaceConfig pins adbd_worker_workspace: worktree by default, local on
// request, and an unknown mode refused instead of silently defaulted.
func TestWorkspaceConfig(t *testing.T) {
	dir := t.TempDir()
	load := func(t *testing.T, body string) (Config, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		return LoadConfig(path)
	}

	// No config file at all: every dispatch-liveness default is the safe one.
	missing, err := LoadConfig(filepath.Join(dir, "absent"))
	if err != nil {
		t.Fatalf("LoadConfig(absent): %v", err)
	}
	if missing.WorkerWorkspace != WorkspaceWorktree {
		t.Fatalf("default adbd_worker_workspace = %q, want %q", missing.WorkerWorkspace, WorkspaceWorktree)
	}
	if !missing.DispatchCheck {
		t.Fatal("default adbd_worker_dispatch_check = off, want on")
	}
	if missing.DispatchTimeout != DefaultDispatchTimeout {
		t.Fatalf("default dispatch timeout = %v, want %v", missing.DispatchTimeout, DefaultDispatchTimeout)
	}
	if missing.DispatchRetries != DefaultDispatchRetries {
		t.Fatalf("default dispatch retries = %d, want %d", missing.DispatchRetries, DefaultDispatchRetries)
	}

	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, cfg Config)
	}{
		{"workspace_local", "adbd_worker_workspace=local\n", func(t *testing.T, cfg Config) {
			if cfg.WorkerWorkspace != WorkspaceLocal {
				t.Fatalf("workspace = %q, want local", cfg.WorkerWorkspace)
			}
		}},
		{"workspace_worktree", "adbd_worker_workspace=worktree\n", func(t *testing.T, cfg Config) {
			if cfg.WorkerWorkspace != WorkspaceWorktree {
				t.Fatalf("workspace = %q, want worktree", cfg.WorkerWorkspace)
			}
		}},
		{"check_off", "adbd_worker_dispatch_check=off\n", func(t *testing.T, cfg Config) {
			if cfg.DispatchCheck {
				t.Fatal("adbd_worker_dispatch_check=off -> on")
			}
		}},
		{"check_typo_keeps_it_on", "adbd_worker_dispatch_check=maybe\n", func(t *testing.T, cfg Config) {
			if !cfg.DispatchCheck {
				t.Fatal("a typo turned the check off; it must stay on")
			}
		}},
		{"timeout_and_retries", "adbd_worker_dispatch_timeout_sec=25\nadbd_worker_dispatch_retries=3\n", func(t *testing.T, cfg Config) {
			if cfg.DispatchTimeout != 25*time.Second {
				t.Fatalf("timeout = %v, want 25s", cfg.DispatchTimeout)
			}
			if cfg.DispatchRetries != 3 {
				t.Fatalf("retries = %d, want 3", cfg.DispatchRetries)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := load(t, tc.body)
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			tc.check(t, cfg)
		})
	}

	// An unusable value is a loud error: a mistyped workspace mode would reach a
	// `paseo run` that never creates anything, and a zero timeout would make every
	// dispatch look dead.
	for name, body := range map[string]string{
		"unknown_workspace": "adbd_worker_workspace=copy\n",
		"bad_timeout":       "adbd_worker_dispatch_timeout_sec=soon\n",
		"zero_timeout":      "adbd_worker_dispatch_timeout_sec=0\n",
		"bad_retries":       "adbd_worker_dispatch_retries=lots\n",
		"negative_retries":  "adbd_worker_dispatch_retries=-1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := load(t, body); err == nil {
				t.Fatalf("%s: LoadConfig accepted %q, want an error", name, strings.TrimSpace(body))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TC-DL-002 (AC-002): progress means alive, no progress before the deadline
// means dead
// ---------------------------------------------------------------------------

func TestDispatchDetectsLiveTurn(t *testing.T) {
	// ① The counting rule itself: only timeline entries count, and an empty answer
	// is not evidence of anything.
	counted, err := countTimelineMarkers([]byte(
		"[Message received] do it\n" +
			"[Thought] reading the repo\n" +
			"  [Tool] shell\n" +
			"just a banner line\n" +
			"\n" +
			"[Assistant] done\n"))
	if err != nil {
		t.Fatalf("countTimelineMarkers: %v", err)
	}
	if counted != 4 {
		t.Fatalf("countTimelineMarkers = %d, want 4 markers", counted)
	}
	// An empty timeline is a readable timeline: a brand-new agent has none yet.
	if counted, err := countTimelineMarkers([]byte("\n\n   \n")); err != nil || counted != 0 {
		t.Fatalf("empty timeline = %d, %v; want 0 markers and no error", counted, err)
	}
	// Output that is not text at all is unusable, not idle.
	if _, err := countTimelineMarkers([]byte("\xff\xfe[Thought]")); err == nil {
		t.Fatal("unparsable timeline output counted as a readable timeline")
	}

	// ② A real(ish) live turn: `send` makes the timeline grow, so the check passes.
	stateDir := t.TempDir()
	bin := writeLivenessPaseo(t, stateDir)
	live := PaseoDispatcher{Bin: bin, Check: true, Timeout: 2 * time.Second, Retries: 0}
	if err := live.Send("ag_live", "add a retry"); err != nil {
		t.Fatalf("Send(live worker) = %v, want nil", err)
	}
	if got := len(liveVerbs(t, stateDir, "send")); got != 1 {
		t.Fatalf("send calls = %d, want 1", got)
	}
	if got := len(liveVerbs(t, stateDir, "logs")); got == 0 {
		t.Fatal("no timeline probe was made, so nothing was verified")
	}

	// ③ A worker that stays silent: the timeline never grows, so the deadline wins.
	t.Setenv(liveProgressAfterEnv, "0")
	dead := PaseoDispatcher{Bin: bin, Check: true, Timeout: 120 * time.Millisecond, Retries: 0}
	start := time.Now()
	err = dead.Send("ag_silent", "add a retry")
	elapsed := time.Since(start)
	var deadErr *DispatchDeadError
	if !errors.As(err, &deadErr) {
		t.Fatalf("Send(silent worker) = %v, want *DispatchDeadError", err)
	}
	if deadErr.Attempts != 1 {
		t.Fatalf("dead attempts = %d, want 1", deadErr.Attempts)
	}
	// AC-005: the wait is bounded. The check may not hang on an opaque CLI.
	if elapsed > 3*time.Second {
		t.Fatalf("a silent worker took %v to be judged dead, want bounded by the timeout", elapsed)
	}
}

// ---------------------------------------------------------------------------
// TC-DL-003 (AC-003): a dead send is retried, within the configured budget
// ---------------------------------------------------------------------------

func TestDispatchRetriesDeadSend(t *testing.T) {
	stateDir := t.TempDir()
	bin := writeLivenessPaseo(t, stateDir)
	t.Setenv(liveProgressAfterEnv, "0") // never starts: every attempt looks dead

	for _, tc := range []struct {
		retries int
		sends   int
	}{
		{0, 1},
		{1, 2},
		{2, 3},
	} {
		t.Run("retries="+strconv.Itoa(tc.retries), func(t *testing.T) {
			before := len(liveVerbs(t, stateDir, "send"))
			d := PaseoDispatcher{Bin: bin, Check: true, Timeout: 60 * time.Millisecond, Retries: tc.retries}
			err := d.Send("ag_dead_"+strconv.Itoa(tc.retries), "ship it")
			var deadErr *DispatchDeadError
			if !errors.As(err, &deadErr) {
				t.Fatalf("Send = %v, want *DispatchDeadError", err)
			}
			if deadErr.Attempts != tc.sends {
				t.Fatalf("attempts = %d, want %d", deadErr.Attempts, tc.sends)
			}
			if got := len(liveVerbs(t, stateDir, "send")) - before; got != tc.sends {
				t.Fatalf("send calls = %d, want %d", got, tc.sends)
			}
		})
	}

	// A worker that only starts on a later send is exactly what the retry is for:
	// the second attempt makes progress and the dispatch succeeds.
	t.Setenv(liveProgressAfterEnv, "2")
	revived := PaseoDispatcher{Bin: bin, Check: true, Timeout: 80 * time.Millisecond, Retries: 1}
	before := len(liveVerbs(t, stateDir, "send"))
	if err := revived.Send("ag_revived", "ship it"); err != nil {
		t.Fatalf("Send(worker that started on the retry) = %v, want nil", err)
	}
	if got := len(liveVerbs(t, stateDir, "send")) - before; got != 2 {
		t.Fatalf("send calls = %d, want 2 (the retry is what saved it)", got)
	}
}

// ---------------------------------------------------------------------------
// TC-DL-004 (AC-004): a dead dispatch is traced, requeued, and never bound
// ---------------------------------------------------------------------------

func TestDispatchDeadRequeues(t *testing.T) {
	h := newLiveHarness(t)
	t.Setenv(liveProgressAfterEnv, "0") // the worker never starts
	h.enqueue(QueueItem{Channel: "skill", Thread: "skill:alpha:add-retry", MessageID: "m-dead", Text: "add a retry to the webhook"})

	res := h.drain()
	if res.Failed != 1 || res.Applied != 0 {
		t.Fatalf("drain = %+v, want failed=1 applied=0", res)
	}
	if res.ReasonCode != ReasonDispatchDead {
		t.Fatalf("drain reason_code = %q, want %q", res.ReasonCode, ReasonDispatchDead)
	}

	traces := h.traces()
	if len(traces) != 1 {
		t.Fatalf("trace lines = %d, want 1 (%+v)", len(traces), traces)
	}
	if rec := traces[0]; rec.Decision != DecisionDispatchDead || rec.ReasonCode != ReasonDispatchDead {
		t.Fatalf("trace = {decision=%s reason_code=%s}, want decision=%s reason_code=%s",
			rec.Decision, rec.ReasonCode, DecisionDispatchDead, ReasonDispatchDead)
	}

	// The item is back in the queue, with its text intact ...
	queued := h.queue()
	if len(queued) != 1 || queued[0].MessageID != "m-dead" || queued[0].Text != "add a retry to the webhook" {
		t.Fatalf("queue after a dead dispatch = %+v, want the one item back", queued)
	}
	// ... and no binding was written: a worker that never ran owns nothing.
	if b := h.binding("skill", "skill:alpha:add-retry"); b != nil {
		t.Fatalf("a dead dispatch wrote a binding: %+v", b)
	}

	// The requeue is real: the next drain tries the item again instead of reading
	// its own dispatch_dead line as "already applied" and dropping it.
	t.Setenv(liveProgressAfterEnv, "1") // now the worker starts
	res = h.drain()
	if res.Applied != 1 || res.Failed != 0 {
		t.Fatalf("second drain = %+v, want applied=1 failed=0", res)
	}
	if queued := h.queue(); len(queued) != 0 {
		t.Fatalf("queue after the retried dispatch = %+v, want empty", queued)
	}
	if b := h.binding("skill", "skill:alpha:add-retry"); b == nil || b.AgentID != "ag_live" {
		t.Fatalf("binding after the retried dispatch = %+v, want ag_live", b)
	}
}

// ---------------------------------------------------------------------------
// TC-DL-005 (AC-005): an unreadable probe is never success
// ---------------------------------------------------------------------------

func TestDispatchProbeFailureIsNotSuccess(t *testing.T) {
	// ① The seam: a `logs` that fails is neither alive nor dead.
	stateDir := t.TempDir()
	bin := writeLivenessPaseo(t, stateDir)
	t.Setenv(liveLogsExitEnv, "3")
	d := PaseoDispatcher{Bin: bin, Check: true, Timeout: 200 * time.Millisecond, Retries: 1}
	err := d.Send("ag_unreadable", "ship it")
	var unverified *DispatchUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("Send with a broken probe = %v, want *DispatchUnverifiedError", err)
	}
	var deadErr *DispatchDeadError
	if errors.As(err, &deadErr) {
		t.Fatal("a broken probe was reported as a dead worker")
	}
	// It must not retry blind: one send, then stop and say so.
	if got := len(liveVerbs(t, stateDir, "send")); got != 1 {
		t.Fatalf("send calls with a broken probe = %d, want 1 (no blind retry)", got)
	}

	// ② The router: the item goes back to the queue, the trace says
	// dispatch_unverified, and nothing is bound.
	h := newLiveHarness(t)
	t.Setenv(liveLogsExitEnv, "3")
	h.enqueue(QueueItem{Channel: "skill", Thread: "skill:beta:fix", MessageID: "m-unverified", Text: "fix the export"})
	res := h.drain()
	if res.Failed != 1 || res.Applied != 0 {
		t.Fatalf("drain = %+v, want failed=1 applied=0", res)
	}
	if res.ReasonCode != ReasonDispatchUnverified {
		t.Fatalf("drain reason_code = %q, want %q", res.ReasonCode, ReasonDispatchUnverified)
	}
	traces := h.traces()
	if len(traces) != 1 || traces[0].ReasonCode != ReasonDispatchUnverified {
		t.Fatalf("trace = %+v, want one row with reason_code=%s", traces, ReasonDispatchUnverified)
	}
	if queued := h.queue(); len(queued) != 1 || queued[0].MessageID != "m-unverified" {
		t.Fatalf("queue after an unverified dispatch = %+v, want the item back", queued)
	}
	if b := h.binding("skill", "skill:beta:fix"); b != nil {
		t.Fatalf("an unverified dispatch wrote a binding: %+v", b)
	}
}

// ---------------------------------------------------------------------------
// TC-DL-006 (AC-006): the check can be switched off, and the trace says so
// ---------------------------------------------------------------------------

func TestDispatchCheckCanBeDisabled(t *testing.T) {
	h := newLiveHarness(t, "adbd_worker_dispatch_check=off")
	// Even a worker that will never start must be dispatched unverified: with the
	// check off the router does not look.
	t.Setenv(liveProgressAfterEnv, "0")
	h.enqueue(QueueItem{Channel: "skill", Thread: "skill:gamma:docs", MessageID: "m-off", Text: "write the changelog"})

	res := h.drain()
	if res.Applied != 1 || res.Failed != 0 {
		t.Fatalf("drain = %+v, want applied=1 failed=0", res)
	}
	traces := h.traces()
	if len(traces) != 1 {
		t.Fatalf("trace lines = %d, want 1", len(traces))
	}
	rec := traces[0]
	if rec.Decision != DecisionRoute {
		t.Fatalf("decision = %q, want %q (switching the check off does not change the verdict)", rec.Decision, DecisionRoute)
	}
	if rec.ReasonCode != ReasonDispatchUnchecked {
		t.Fatalf("reason_code = %q, want %q", rec.ReasonCode, ReasonDispatchUnchecked)
	}
	if got := len(liveVerbs(t, h.stateDir, "logs")); got != 0 {
		t.Fatalf("logs probes = %d, want 0 when the check is off", got)
	}
	if got := len(liveVerbs(t, h.stateDir, "send")); got != 1 {
		t.Fatalf("send calls = %d, want exactly 1", got)
	}
	// Everything else is unchanged: the send happened, the worker is bound, the
	// item left the queue.
	if queued := h.queue(); len(queued) != 0 {
		t.Fatalf("queue = %+v, want empty", queued)
	}
	if b := h.binding("skill", "skill:gamma:docs"); b == nil || b.AgentID != "ag_live" {
		t.Fatalf("binding = %+v, want ag_live", b)
	}

	// With the check on (the default) the same dispatch is verified, and the row
	// keeps the decider's own reason.
	on := newLiveHarness(t)
	on.enqueue(QueueItem{Channel: "skill", Thread: "skill:gamma:docs", MessageID: "m-on", Text: "write the changelog"})
	if res := on.drain(); res.Applied != 1 {
		t.Fatalf("checked drain = %+v, want applied=1", res)
	}
	if traces := on.traces(); len(traces) != 1 || traces[0].ReasonCode != ReasonNewWork {
		t.Fatalf("checked trace = %+v, want reason_code=%s", traces, ReasonNewWork)
	}
	if got := len(liveVerbs(t, on.stateDir, "logs")); got == 0 {
		t.Fatal("the default (check on) made no probe at all")
	}
}

// ---------------------------------------------------------------------------
// TC-DL-007 (AC-007): the check only adds read-only paseo calls
// ---------------------------------------------------------------------------

// TestDispatchCheckOnlyAddsReadOnlyProbes pins the shape of AC-007: `paseo send`
// keeps its exact argv, and the only new paseo invocation is the read-only
// timeline probe. A check that changed the send would be a different feature.
func TestDispatchCheckOnlyAddsReadOnlyProbes(t *testing.T) {
	stateDir := t.TempDir()
	bin := writeLivenessPaseo(t, stateDir)

	unchecked := PaseoDispatcher{Bin: bin, Check: false}
	if err := unchecked.Send("ag_a", "add a retry"); err != nil {
		t.Fatalf("Send(unchecked) = %v", err)
	}
	checked := PaseoDispatcher{Bin: bin, Check: true, Timeout: 2 * time.Second}
	if err := checked.Send("ag_b", "add a retry"); err != nil {
		t.Fatalf("Send(checked) = %v", err)
	}

	sends := liveVerbs(t, stateDir, "send")
	if len(sends) != 2 {
		t.Fatalf("send calls = %v, want two", sends)
	}
	for i, send := range sends {
		if want := "send ag_" + string(rune('a'+i)) + " add a retry"; send != want {
			t.Fatalf("send[%d] argv = %q, want %q (the send itself must not change)", i, send, want)
		}
	}
	// The probe is the only addition, it is read-only, and it asks for a bounded
	// tail of the timeline.
	for _, probe := range liveVerbs(t, stateDir, "logs") {
		if !strings.Contains(probe, "--tail "+strconv.Itoa(DefaultDispatchTail)) {
			t.Fatalf("probe argv = %q, want a --tail bound", probe)
		}
		if strings.Contains(probe, "--no-wait") || strings.Contains(probe, "--json") {
			t.Fatalf("probe argv = %q, want a plain read-only logs call", probe)
		}
	}

	// And the closed check reports itself, which is what makes AC-006 traceable.
	if unchecked.DispatchChecked() {
		t.Fatal("an unchecked dispatcher reported itself as checking")
	}
	if !checked.DispatchChecked() {
		t.Fatal("a checked dispatcher reported itself as unchecked")
	}
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

// argvValue returns the value following flag in an argv slice.
func argvValue(argv []string, flag string) string {
	for i, arg := range argv {
		if arg == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}

// containsArg reports whether argv carries arg as a standalone element.
func containsArg(argv []string, arg string) bool {
	for _, candidate := range argv {
		if candidate == arg {
			return true
		}
	}
	return false
}
