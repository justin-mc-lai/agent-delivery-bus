package main

// Worker-resolution tests (feature router-loop, v0.1.5): TC-RL-010 for AC-009.
//
// The point of these tests is that the router never sends work to a name nobody
// verified. They run against a REAL executable on disk (a /bin/sh script acting
// as paseo), so the production seam in dispatch.go -- argv building, exit codes,
// JSON parsing -- is exercised for real. Nothing calls the real paseo and nothing
// touches the network: the script is a temp file written by the test.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// a fake paseo, on disk
// ---------------------------------------------------------------------------

// fakePaseo is a temp executable speaking the three paseo verbs the router uses:
//
//	inspect <id>            exit 0 when <id> is in the "known" file, else 1
//	run --no-wait --json... append the created id to "known", print {"agentId":...}
//	send <id> <text>        exit 0 (recorded in the argv log)
//
// Every invocation is appended to an argv log, which is what the assertions read.
type fakePaseo struct {
	t *testing.T
	// bin is the executable path handed to PaseoDispatcher.Bin.
	bin string
	// logPath receives one line per invocation, argv joined by spaces.
	logPath string
	// knownPath lists the agent ids that exist. `run` appends to it, so a worker
	// created in one drain is inspectable in the next.
	knownPath string
}

// newFakePaseo writes the script and points the behaviour env vars at its files.
func newFakePaseo(t *testing.T) *fakePaseo {
	t.Helper()
	dir := t.TempDir()
	f := &fakePaseo{
		t:         t,
		bin:       filepath.Join(dir, "fake-paseo.sh"),
		logPath:   filepath.Join(dir, "argv.log"),
		knownPath: filepath.Join(dir, "known-agents.txt"),
	}
	script := "#!/bin/sh\n" +
		"if [ -n \"$FAKE_PASEO_LOG\" ]; then printf '%s\\n' \"$*\" >> \"$FAKE_PASEO_LOG\"; fi\n" +
		"case \"$1\" in\n" +
		"  inspect)\n" +
		"    if [ -n \"$FAKE_PASEO_KNOWN\" ] && [ -f \"$FAKE_PASEO_KNOWN\" ] && grep -qx -- \"$2\" \"$FAKE_PASEO_KNOWN\"; then\n" +
		"      exit 0\n" +
		"    fi\n" +
		"    exit 1 ;;\n" +
		"  run)\n" +
		"    if [ \"${FAKE_PASEO_RUN_EXIT:-0}\" != \"0\" ]; then\n" +
		"      printf '%s\\n' 'paseo: cannot start a worker' >&2\n" +
		"      exit \"$FAKE_PASEO_RUN_EXIT\"\n" +
		"    fi\n" +
		"    id=\"${FAKE_PASEO_RUN_AGENT_ID:-ag_new}\"\n" +
		"    if [ -n \"$FAKE_PASEO_KNOWN\" ]; then printf '%s\\n' \"$id\" >> \"$FAKE_PASEO_KNOWN\"; fi\n" +
		// Real paseo pretty-prints its JSON; the fake must too, or the parser
		// regression (line-at-a-time parsing) would stay invisible.
		"    printf '{\\n  \"agentId\": \"%s\",\\n  \"status\": \"running\"\\n}\\n' \"$id\"\n" +
		"    exit 0 ;;\n" +
		"  send) exit 0 ;;\n" +
		"esac\n" +
		"exit 0\n"
	if err := os.WriteFile(f.bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake paseo: %v", err)
	}
	t.Setenv("FAKE_PASEO_LOG", f.logPath)
	t.Setenv("FAKE_PASEO_KNOWN", f.knownPath)
	return f
}

// exists records that these agent ids are alive.
func (f *fakePaseo) exists(ids ...string) {
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

// lines returns every recorded invocation, in order, argv joined by spaces.
func (f *fakePaseo) lines() []string {
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

// verbs returns the recorded invocations of one paseo verb.
func (f *fakePaseo) verbs(verb string) []string {
	f.t.Helper()
	var out []string
	for _, line := range f.lines() {
		if line == verb || strings.HasPrefix(line, verb+" ") {
			out = append(out, line)
		}
	}
	return out
}

// dispatcher is the production dispatcher pointed at the fake binary.
func (f *fakePaseo) dispatcher(workerID, workdir string) PaseoDispatcher {
	return PaseoDispatcher{
		Bin:      f.bin,
		WorkerID: workerID,
		Provider: DefaultWorkerProvider,
		Model:    DefaultWorkerModel,
		Workdir:  workdir,
	}
}

// newWorkerHarness builds a router whose dispatcher is the fake paseo.
func newWorkerHarness(t *testing.T, f *fakePaseo, workerID string) *routeHarness {
	t.Helper()
	h := newRouteHarness(t, RuleDecider{})
	// The router's configured default and the dispatcher's fallback are the same
	// setting in production (adbd_worker_agent_id), so the tests set both.
	h.router.AgentID = workerID
	h.router.Dispatcher = f.dispatcher(workerID, t.TempDir())
	return h
}

// seedBinding binds a thread through the public API, so the router sees a live
// binding with that worker.
func seedBinding(t *testing.T, h *routeHarness, channel, thread, agentID string) {
	t.Helper()
	core := NewCore(&Store{Path: h.statePath, SlowPath: h.slowPath}, nil)
	core.Now = h.clock.now
	if _, err := core.Bind(channel, thread, agentID, DefaultTTL); err != nil {
		t.Fatalf("bind %s/%s: %v", channel, thread, err)
	}
}

// assertNoSendTo asserts that no recorded `send` targets this id.
func assertNoSendTo(t *testing.T, f *fakePaseo, agentID string) {
	t.Helper()
	for _, line := range f.verbs("send") {
		if line == "send "+agentID || strings.HasPrefix(line, "send "+agentID+" ") {
			t.Fatalf("work was sent to %q, which no one verified: %q", agentID, line)
		}
	}
}

// ---------------------------------------------------------------------------
// TC-RL-010 (AC-009): resolve a real worker before dispatching
// ---------------------------------------------------------------------------

func TestWorkerResolution(t *testing.T) {
	// ① A live binding is reused and nothing new is created.
	t.Run("reuse_bound_worker", func(t *testing.T) {
		f := newFakePaseo(t)
		f.exists("ag_bound")
		h := newWorkerHarness(t, f, "")
		seedBinding(t, h, "feishu", "t-bound", "ag_bound")

		h.enqueue(QueueItem{Channel: "feishu", Thread: "t-bound", MessageID: "m-bound", Text: "add a retry to the webhook"})
		res := h.drain()
		if res.Applied != 1 || res.Failed != 0 {
			t.Fatalf("drain = %+v, want applied=1 failed=0", res)
		}
		if got := f.verbs("inspect"); len(got) != 1 || got[0] != "inspect ag_bound" {
			t.Fatalf("inspect calls = %v, want exactly [inspect ag_bound]", got)
		}
		if got := f.verbs("run"); len(got) != 0 {
			t.Fatalf("a live binding created a new worker: %v", got)
		}
		sends := f.verbs("send")
		if len(sends) != 1 || !strings.HasPrefix(sends[0], "send ag_bound ") {
			t.Fatalf("sends = %v, want one send to ag_bound", sends)
		}
		if !strings.Contains(sends[0], UntrustedGuardLine) {
			t.Fatalf("send = %q, want the untrusted guard line", sends[0])
		}
		binding := h.binding("feishu", "t-bound")
		if binding == nil || binding.AgentID != "ag_bound" {
			t.Fatalf("binding = %+v, want it to keep ag_bound", binding)
		}
	})

	// ② With no binding, the configured adbd_worker_agent_id is used once paseo
	// confirms it.
	t.Run("use_configured_worker", func(t *testing.T) {
		f := newFakePaseo(t)
		f.exists("ag_config")
		h := newWorkerHarness(t, f, "ag_config")

		h.enqueue(QueueItem{Channel: "feishu", Thread: "t-cfg", MessageID: "m-cfg", Text: "write the migration"})
		res := h.drain()
		if res.Applied != 1 {
			t.Fatalf("drain = %+v, want applied=1", res)
		}
		if got := f.verbs("run"); len(got) != 0 {
			t.Fatalf("a verified configured worker was ignored and a new one created: %v", got)
		}
		sends := f.verbs("send")
		if len(sends) != 1 || !strings.HasPrefix(sends[0], "send ag_config ") {
			t.Fatalf("sends = %v, want one send to ag_config", sends)
		}
		binding := h.binding("feishu", "t-cfg")
		if binding == nil || binding.AgentID != "ag_config" {
			t.Fatalf("binding = %+v, want the resolved id persisted as ag_config", binding)
		}
	})

	// ③ Neither works: create one with `paseo run --no-wait --json`, parse the
	// agentId, persist it, and reuse it on the next message.
	t.Run("create_new_and_persist", func(t *testing.T) {
		f := newFakePaseo(t)
		f.exists() // nothing exists yet: the stale configured id is not usable
		workdir := t.TempDir()
		h := newRouteHarness(t, RuleDecider{})
		h.router.AgentID = "ag_stale"
		h.router.Dispatcher = f.dispatcher("ag_stale", workdir)

		h.enqueue(QueueItem{Channel: "feishu", Thread: "t-new", MessageID: "m-new", Text: "add pagination"})
		res := h.drain()
		if res.Applied != 1 || res.Failed != 0 {
			t.Fatalf("drain = %+v, want applied=1 failed=0", res)
		}
		runs := f.verbs("run")
		if len(runs) != 1 {
			t.Fatalf("run calls = %v, want exactly one", runs)
		}
		for _, want := range []string{
			"run --no-wait --json",
			"--title t-new",
			"--cwd " + workdir,
			"--provider " + DefaultWorkerProvider,
			"--model " + DefaultWorkerModel,
		} {
			if !strings.Contains(runs[0], want) {
				t.Fatalf("run argv = %q, want it to contain %q", runs[0], want)
			}
		}
		sends := f.verbs("send")
		if len(sends) != 1 || !strings.HasPrefix(sends[0], "send ag_new ") {
			t.Fatalf("sends = %v, want one send to the created worker ag_new", sends)
		}
		binding := h.binding("feishu", "t-new")
		if binding == nil || binding.AgentID != "ag_new" || binding.State != StateBound {
			t.Fatalf("binding = %+v, want bound/ag_new (the parsed agentId)", binding)
		}

		// The next message on the same thread reuses the created worker: no second
		// `run`, and the binding is what carries the id.
		h.enqueue(QueueItem{Channel: "feishu", Thread: "t-new", MessageID: "m-new-2", Text: "also add sorting"})
		res = h.drain()
		if res.Applied != 1 {
			t.Fatalf("second drain = %+v, want applied=1", res)
		}
		if got := f.verbs("run"); len(got) != 1 {
			t.Fatalf("a second message created another worker: %v", got)
		}
		sends = f.verbs("send")
		if len(sends) != 2 || !strings.HasPrefix(sends[1], "send ag_new ") {
			t.Fatalf("sends = %v, want the second message to go to ag_new", sends)
		}
		if got := f.verbs("inspect"); len(got) < 3 {
			t.Fatalf("inspect calls = %v, want the bound worker inspected before reuse", got)
		}
	})

	// ④ Creation failure is a dispatch failure: no send, a kept attempt, the
	// dispatch_failed reason code and no trace line (the item is not applied).
	t.Run("creation_failure_is_dispatch_failed", func(t *testing.T) {
		f := newFakePaseo(t)
		f.exists()
		t.Setenv("FAKE_PASEO_RUN_EXIT", "1")
		h := newWorkerHarness(t, f, "")

		h.enqueue(QueueItem{Channel: "feishu", Thread: "t-dead", MessageID: "m-dead", Text: "rotate the keys"})
		res := h.drain()
		if res.Applied != 0 || res.Failed != 1 || res.Queued != 1 {
			t.Fatalf("drain = %+v, want applied=0 failed=1 queued=1", res)
		}
		if got := f.verbs("send"); len(got) != 0 {
			t.Fatalf("a failed worker creation still sent work: %v", got)
		}
		if got := h.trace(); len(got) != 0 {
			t.Fatalf("a backed-off item wrote trace lines: %+v", got)
		}
		q := h.queue()
		if len(q) != 1 || q[0].Attempts != 1 || q[0].ReasonCode != ReasonDispatchFailed {
			t.Fatalf("queue = %+v, want one item with attempts=1 and reason_code=%s", q, ReasonDispatchFailed)
		}

		// The failure is typed, so a caller can tell "no worker" from "refused".
		_, err := f.dispatcher("", t.TempDir()).EnsureWorker("t-dead", "")
		var resolveErr *WorkerResolveError
		if !errors.As(err, &resolveErr) {
			t.Fatalf("EnsureWorker error = %v, want a *WorkerResolveError", err)
		}
		if resolveErr.Op != "run" {
			t.Fatalf("resolve error op = %q, want run", resolveErr.Op)
		}

		// Once paseo can start a worker again, the same item goes through.
		t.Setenv("FAKE_PASEO_RUN_EXIT", "0")
		res = h.drain()
		if res.Applied != 1 || res.Queued != 0 {
			t.Fatalf("retry drain = %+v, want applied=1 queued=0", res)
		}
		if got := f.verbs("send"); len(got) != 1 {
			t.Fatalf("sends after recovery = %v, want one", got)
		}
	})

	// ⑤ "main" is never a target, on any path.
	t.Run("main_is_never_a_target", func(t *testing.T) {
		// (a) a binding that carries the placeholder is replaced by a real worker.
		f := newFakePaseo(t)
		f.exists(DefaultRouterAgentID) // even if paseo knows it, it stays a placeholder
		h := newWorkerHarness(t, f, "")
		seedBinding(t, h, "feishu", "t-main", DefaultRouterAgentID)

		h.enqueue(QueueItem{Channel: "feishu", Thread: "t-main", MessageID: "m-main", Text: "ship the loader"})
		res := h.drain()
		if res.Applied != 1 {
			t.Fatalf("drain = %+v, want applied=1", res)
		}
		assertNoSendTo(t, f, DefaultRouterAgentID)
		binding := h.binding("feishu", "t-main")
		if binding == nil || binding.AgentID != "ag_new" {
			t.Fatalf("binding = %+v, want the placeholder replaced by ag_new", binding)
		}

		// (b) the resolver itself never answers with the placeholder, even when the
		// candidate is exactly that.
		id, err := f.dispatcher("", t.TempDir()).EnsureWorker("t-main", DefaultRouterAgentID)
		if err != nil {
			t.Fatalf("EnsureWorker(main) = %v, want a created worker", err)
		}
		if id == DefaultRouterAgentID || id == "" {
			t.Fatalf("EnsureWorker(main) = %q, want a real worker id", id)
		}
		assertNoSendTo(t, f, DefaultRouterAgentID)

		// (c) a dispatcher that cannot resolve workers is not allowed to invent a
		// target: no bound id, no configured id -> a failure, not a send to main.
		plain := newRouteHarness(t, RuleDecider{})
		plain.router.AgentID = ""
		plain.router.Dispatcher = &stubDispatcher{}
		plain.enqueue(QueueItem{Channel: "feishu", Thread: "t-plain", MessageID: "m-plain", Text: "build the exporter"})
		res = plain.drain()
		if res.Applied != 0 || res.Failed != 1 {
			t.Fatalf("unresolvable drain = %+v, want applied=0 failed=1", res)
		}
		if got := len(plain.dispatcher.calls); got != 0 {
			t.Fatalf("dispatcher calls = %d, want 0 (%#v)", got, plain.dispatcher.calls)
		}
		q := plain.queue()
		if len(q) != 1 || q[0].Attempts != 1 || q[0].ReasonCode != ReasonDispatchFailed {
			t.Fatalf("queue = %+v, want attempts=1 reason_code=%s", q, ReasonDispatchFailed)
		}
		if got := plain.binding("feishu", "t-plain"); got != nil {
			t.Fatalf("an unresolved dispatch bound the thread: %+v", got)
		}

		// (d) the router never dispatches to the placeholder even when a decider
		// tries to route with it already in the binding and paseo is down.
		f2 := newFakePaseo(t)
		t.Setenv("FAKE_PASEO_RUN_EXIT", "1")
		h2 := newWorkerHarness(t, f2, "")
		seedBinding(t, h2, "feishu", "t-dead-main", DefaultRouterAgentID)
		h2.enqueue(QueueItem{Channel: "feishu", Thread: "t-dead-main", MessageID: "m-dead-main", Text: "resurrect the old job"})
		h2.drain()
		if got := f2.verbs("send"); len(got) != 0 {
			t.Fatalf("a failed creation sent work anyway: %v", got)
		}
		assertNoSendTo(t, f2, DefaultRouterAgentID)
	})
}

// ---------------------------------------------------------------------------
// The dispatcher alone: inspect / run / send argv and typed errors
// ---------------------------------------------------------------------------

func TestWorkerResolverArgv(t *testing.T) {
	f := newFakePaseo(t)

	// An id paseo knows is returned untouched, without creating anything.
	f.exists("ag_live")
	id, err := f.dispatcher("", t.TempDir()).EnsureWorker("t-argv", "ag_live")
	if err != nil || id != "ag_live" {
		t.Fatalf("EnsureWorker(ag_live) = %q, %v; want ag_live, nil", id, err)
	}
	if got := f.verbs("run"); len(got) != 0 {
		t.Fatalf("run calls = %v, want none for a live id", got)
	}

	// An unknown id falls back to creating one, and the returned id is the parsed
	// agentId -- not the candidate, not a placeholder.
	id, err = f.dispatcher("", t.TempDir()).EnsureWorker("t-argv", "ag_missing")
	if err != nil {
		t.Fatalf("EnsureWorker(ag_missing) = %v, want a created worker", err)
	}
	if id != "ag_new" {
		t.Fatalf("EnsureWorker(ag_missing) = %q, want the parsed ag_new", id)
	}

	// A run that prints no JSON is an error, never a guess.
	broken := filepath.Join(t.TempDir(), "broken-paseo.sh")
	if err := os.WriteFile(broken, []byte("#!/bin/sh\ncase \"$1\" in run) printf '%s\\n' 'not json'; exit 0 ;; *) exit 1 ;; esac\n"), 0o755); err != nil {
		t.Fatalf("write broken paseo: %v", err)
	}
	_, err = PaseoDispatcher{Bin: broken, Provider: DefaultWorkerProvider, Model: DefaultWorkerModel}.EnsureWorker("t-broken", "")
	var resolveErr *WorkerResolveError
	if !errors.As(err, &resolveErr) {
		t.Fatalf("EnsureWorker over unusable output = %v, want a *WorkerResolveError", err)
	}

	// Dispatch keeps its contract: `paseo send <id> <text>`, text as one argv.
	d := f.dispatcher("", t.TempDir())
	if err := d.Dispatch("t-send", "ag_live", "hello"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	sends := f.verbs("send")
	if len(sends) != 1 || sends[0] != "send ag_live hello" {
		t.Fatalf("sends = %v, want [send ag_live hello]", sends)
	}

	// The defaults match the frozen config table when nothing is configured.
	if DefaultWorkerProvider != "opencode" || DefaultWorkerModel != "deepseek/deepseek-flash" {
		t.Fatalf("worker defaults = %q/%q, want opencode + deepseek/deepseek-flash",
			DefaultWorkerProvider, DefaultWorkerModel)
	}
}

// TestWorkerResolutionConfig pins the three AC-009 config keys and their defaults.
func TestWorkerResolutionConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		"adbd_allow_channels=feishu",
		"adbd_worker_agent_id=ag_cfg",
		"adbd_worker_model=deepseek/deepseek-flash",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.WorkerAgentID != "ag_cfg" {
		t.Fatalf("WorkerAgentID = %q, want ag_cfg", cfg.WorkerAgentID)
	}
	if cfg.WorkerProvider != DefaultWorkerProvider {
		t.Fatalf("WorkerProvider = %q, want the default %q", cfg.WorkerProvider, DefaultWorkerProvider)
	}
	if cfg.WorkerModel != "deepseek/deepseek-flash" {
		t.Fatalf("WorkerModel = %q, want deepseek/deepseek-flash", cfg.WorkerModel)
	}

	// An empty/missing config leaves no worker id at all -- and certainly not the
	// former "main" placeholder.
	empty, err := LoadConfig(filepath.Join(dir, "absent"))
	if err != nil {
		t.Fatalf("LoadConfig(absent): %v", err)
	}
	if empty.WorkerAgentID != "" || empty.RouterAgent != "" {
		t.Fatalf("default worker ids = %q/%q, want both empty", empty.WorkerAgentID, empty.RouterAgent)
	}
	if empty.WorkerProvider != DefaultWorkerProvider || empty.WorkerModel != DefaultWorkerModel {
		t.Fatalf("default provider/model = %q/%q, want %q/%q",
			empty.WorkerProvider, empty.WorkerModel, DefaultWorkerProvider, DefaultWorkerModel)
	}
	if isPlaceholderAgentID(DefaultRouterAgentID) != true {
		t.Fatalf("%q is not treated as a placeholder", DefaultRouterAgentID)
	}
	// A real id is never mistaken for a placeholder, at any time of day.
	if isPlaceholderAgentID("ag_live") {
		t.Fatal("ag_live was treated as a placeholder")
	}
}
