package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// harness: temp state dir + temp config + a REAL recording runner executable
// ---------------------------------------------------------------------------

// fakeClock makes lease moves exact; the runner stays a real process.
type fakeClock struct {
	t time.Time
}

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

type harness struct {
	t          *testing.T
	dir        string
	statePath  string
	slowPath   string
	configPath string
	runnerBin  string
	argvLog    string
	clock      *fakeClock
	core       *Core
}

const baseTime = "2026-02-06T09:00:00Z"

// writeRecordingRunner writes a real #!/bin/sh executable that records its own
// argv, one call per line as `arg|arg|arg|`. It never touches real paseo.
//
// It also speaks the three verbs worker resolution and the dispatch check need
// (truth.md AC-009, dispatch-liveness AC-002): every `inspect` succeeds,
// `run --json` answers with an agent id, and `logs`/`send` are answered by the
// shared liveness arms, so a routed message looks verified instead of unreadable.
func writeRecordingRunner(t *testing.T, argvLog string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "recording-paseo.sh")
	var body strings.Builder
	body.WriteString("#!/bin/sh\n")
	body.WriteString("case \"$1\" in\n")
	body.WriteString("  inspect) exit 0 ;;\n")
	body.WriteString("  run) printf '%s\\n' '{\"agentId\":\"ag-recording-worker\"}'; exit 0 ;;\n")
	body.WriteString(livenessProbeCases(filepath.Dir(argvLog) + "/timeline"))
	body.WriteString("esac\n")
	body.WriteString("printf '%s|' \"$@\" >> " + shellQuote(argvLog) + "\n")
	body.WriteString("printf '\\n' >> " + shellQuote(argvLog) + "\n")
	body.WriteString("exit 0\n")
	if err := os.WriteFile(script, []byte(body.String()), 0o755); err != nil {
		t.Fatalf("write recording runner: %v", err)
	}
	return script
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	beacon := filepath.Join(dir, ".beacon")
	h := &harness{
		t:          t,
		dir:        dir,
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		configPath: filepath.Join(beacon, "config"),
		argvLog:    filepath.Join(dir, "runner-argv.log"),
	}
	h.runnerBin = writeRecordingRunner(t, h.argvLog)
	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h.clock = &fakeClock{t: start}
	h.core = NewCore(
		&Store{Path: h.statePath, SlowPath: h.slowPath},
		PaseoRunner{Bin: h.runnerBin},
	)
	h.core.Now = h.clock.now
	return h
}

// calls returns the argv of every runner process invocation, in order.
func (h *harness) calls() [][]string {
	h.t.Helper()
	data, err := os.ReadFile(h.argvLog)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read argv log: %v", err)
	}
	var out [][]string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		argv := strings.Split(line, "|")
		if n := len(argv); n > 0 && argv[n-1] == "" {
			argv = argv[:n-1]
		}
		out = append(out, argv)
	}
	return out
}

func (h *harness) slowPathRecords() []slowPathRecord {
	h.t.Helper()
	data, err := os.ReadFile(h.slowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read slow path: %v", err)
	}
	var out []slowPathRecord
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec slowPathRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			h.t.Fatalf("slow path line %q is not JSON: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func (h *harness) writeConfig(body string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0o755); err != nil {
		h.t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(h.configPath, []byte(body), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

func (h *harness) mustBind(channel, thread, agentID string, ttl time.Duration) Binding {
	h.t.Helper()
	binding, err := h.core.Bind(channel, thread, agentID, ttl)
	if err != nil {
		h.t.Fatalf("bind %s/%s: %v", channel, thread, err)
	}
	return binding
}

func (h *harness) mustIntake(channel, thread, messageID, text string) IntakeResult {
	h.t.Helper()
	res, err := h.core.Intake(IntakeInput{Channel: channel, Thread: thread, MessageID: messageID, Text: text})
	if err != nil {
		h.t.Fatalf("intake %s/%s/%s: %v", channel, thread, messageID, err)
	}
	return res
}

func assertArgv(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q (whole argv %#v)", i, got[i], want[i], got)
		}
	}
}

func (h *harness) status(channel, thread string) StatusResult {
	h.t.Helper()
	st, err := h.core.Status(channel, thread)
	if err != nil {
		h.t.Fatalf("status %s/%s: %v", channel, thread, err)
	}
	return st
}

// ---------------------------------------------------------------------------
// TC-IF-001 (AC-001): module exists, builds, --version is non-empty
// ---------------------------------------------------------------------------

func TestModuleAndVersionFlag(t *testing.T) {
	gomod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("go.mod missing: %v", err)
	}
	if !strings.Contains(string(gomod), "module adbd") {
		t.Fatalf("go.mod does not declare module adbd: %q", string(gomod))
	}
	if strings.Contains(string(gomod), "require ") {
		t.Fatalf("adbd must stay stdlib-only, go.mod has requires: %q", string(gomod))
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("adbd --version exit = %d, stderr = %q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatal("adbd --version printed nothing")
	}
	if !strings.Contains(stdout.String(), "adbd") {
		t.Fatalf("adbd --version output = %q, want it to name adbd", stdout.String())
	}

	// no subcommand: usage on stderr, non-zero exit.
	stdout.Reset()
	stderr.Reset()
	if code := run(nil, &stdout, &stderr); code == 0 {
		t.Fatal("adbd with no arguments should not exit 0")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("usage missing from stderr: %q", stderr.String())
	}
}

// ---------------------------------------------------------------------------
// TC-IF-002 (AC-002): bound thread forwards once through the real runner process
// ---------------------------------------------------------------------------

func TestFastpathForwardsToBoundWorker(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("adbd_allow_channels=feishu,slack\n")

	var stdout, stderr bytes.Buffer
	common := []string{
		"--channel", "feishu", "--thread", "thread-42",
		"--state", h.statePath, "--slow-path", h.slowPath, "--config", h.configPath,
	}
	if code := run(append([]string{"bind", "--agent-id", "agent-abc", "--ttl", "24h"}, common...), &stdout, &stderr); code != 0 {
		t.Fatalf("bind exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "state=bound") {
		t.Fatalf("bind output = %q, want state=bound", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	intakeArgs := append([]string{
		"intake", "--message-id", "msg-1", "--text", "hello world",
		"--runner-bin", h.runnerBin,
	}, common...)
	if code := run(intakeArgs, &stdout, &stderr); code != 0 {
		t.Fatalf("intake exit = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "route=fast") {
		t.Fatalf("intake output = %q, want route=fast", out)
	}
	if !strings.Contains(out, "lease=ok") {
		t.Fatalf("intake output = %q, want lease=ok", out)
	}

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("runner invoked %d times, want exactly 1 (calls=%#v)", len(calls), calls)
	}
	assertArgv(t, calls[0], []string{"send", "agent-abc", "hello world"})

	// the lease was renewed to now+TTL and the state moved to leasing
	stdout.Reset()
	stderr.Reset()
	statusArgs := append([]string{"status", "--outcome", "json"}, common...)
	if code := run(statusArgs, &stdout, &stderr); code != 0 {
		t.Fatalf("status exit = %d, stderr = %q", code, stderr.String())
	}
	var st StatusResult
	if err := json.Unmarshal(stdout.Bytes(), &st); err != nil {
		t.Fatalf("status json %q: %v", stdout.String(), err)
	}
	if st.State != StateLeasing {
		t.Fatalf("status state = %q, want %q", st.State, StateLeasing)
	}
	if st.AgentID != "agent-abc" {
		t.Fatalf("status agent_id = %q, want agent-abc", st.AgentID)
	}
	if st.RemainingSeconds < 23*3600 {
		t.Fatalf("lease not renewed, remaining = %ds", st.RemainingSeconds)
	}
}

// ---------------------------------------------------------------------------
// TC-IF-003 (AC-003): replayed message dedupes, runner fires once
// ---------------------------------------------------------------------------

func TestDuplicateMessageDeduped(t *testing.T) {
	h := newHarness(t)
	h.mustBind("feishu", "thread-1", "agent-1", DefaultTTL)

	first := h.mustIntake("feishu", "thread-1", "msg-dup", "first delivery")
	if first.Route != RouteFast {
		t.Fatalf("first route = %q, want fast", first.Route)
	}
	if len(h.calls()) != 1 {
		t.Fatalf("runner calls after first intake = %d, want 1", len(h.calls()))
	}

	second := h.mustIntake("feishu", "thread-1", "msg-dup", "first delivery")
	if second.Route != RouteDeduped {
		t.Fatalf("second route = %q, want deduped", second.Route)
	}
	if second.ReasonCode != ReasonDuplicateMessage {
		t.Fatalf("second reason = %q, want %q", second.ReasonCode, ReasonDuplicateMessage)
	}
	if second.Forwarded {
		t.Fatal("deduped message reported forwarded=true")
	}
	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("runner calls after replay = %d, want 1 (calls=%#v)", len(calls), calls)
	}

	// A distinct message id on the same thread still delivers.
	third := h.mustIntake("feishu", "thread-1", "msg-new", "second delivery")
	if third.Route != RouteFast {
		t.Fatalf("third route = %q, want fast", third.Route)
	}
	if got := len(h.calls()); got != 2 {
		t.Fatalf("runner calls after new message = %d, want 2", got)
	}

	// The state document carries the idempotency keys (unique by construction).
	data, err := os.ReadFile(h.statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !strings.Contains(string(data), "dedupe_keys") {
		t.Fatalf("state document has no dedupe_keys: %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// TC-IF-004 (AC-004): renew on forward, expiring label, slow path after expiry
// ---------------------------------------------------------------------------

func TestLeaseRenewAndExpiry(t *testing.T) {
	h := newHarness(t)
	// A 2h lease proves renewal keeps the binding's own TTL instead of jumping
	// to the 24h daemon default.
	const lease = 2 * time.Hour
	h.mustBind("feishu", "thread-l", "agent-l", lease)

	// Forward at T0: the whole lease is ahead -> lease=ok, expires = T0+lease.
	first := h.mustIntake("feishu", "thread-l", "m1", "one")
	if first.Route != RouteFast || first.Lease != LeaseOK {
		t.Fatalf("first = %+v, want route=fast lease=ok", first)
	}
	wantExpiry := h.clock.now().Add(lease).Format(time.RFC3339)
	if first.ExpiresAt != wantExpiry {
		t.Fatalf("first expires_at = %s, want %s", first.ExpiresAt, wantExpiry)
	}

	// 1h30m later only 30m remain: renewing forward must report expiring.
	h.clock.advance(lease - 30*time.Minute)
	second := h.mustIntake("feishu", "thread-l", "m2", "two")
	if second.Route != RouteFast {
		t.Fatalf("second route = %q, want fast", second.Route)
	}
	if second.Lease != LeaseExpiring {
		t.Fatalf("second lease = %q, want expiring", second.Lease)
	}
	if second.State != StateExpiring {
		t.Fatalf("second state = %q, want expiring", second.State)
	}
	// The renew is relative to the new now, and it keeps the 2h lease length.
	wantRenewed := h.clock.now().Add(lease).Format(time.RFC3339)
	if second.ExpiresAt != wantRenewed {
		t.Fatalf("second expires_at = %s, want %s", second.ExpiresAt, wantRenewed)
	}

	// Past expiry the binding is dropped and the next message is slow.
	h.clock.advance(lease + time.Minute)
	third := h.mustIntake("feishu", "thread-l", "m3", "three")
	if third.Route != RouteSlow {
		t.Fatalf("third route = %q, want slow", third.Route)
	}
	if third.ReasonCode != ReasonLeaseExpired {
		t.Fatalf("third reason = %q, want %q", third.ReasonCode, ReasonLeaseExpired)
	}
	if third.State != StateUnbound {
		t.Fatalf("third state = %q, want unbound", third.State)
	}
	if got := len(h.calls()); got != 2 {
		t.Fatalf("runner calls = %d, want 2 (expired message must not forward)", got)
	}
	if st := h.status("feishu", "thread-l"); st.State != StateUnbound {
		t.Fatalf("status after expiry = %q, want unbound", st.State)
	}
}

// ---------------------------------------------------------------------------
// TC-IF-005 (AC-005): no binding -> slow, unknown channel -> rejected
// ---------------------------------------------------------------------------

func TestSlowPathAndRejected(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("# adbd dev config\nadbd_allow_channels=feishu, slack\n")

	cfg, err := LoadConfig(h.configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	h.core.AllowAll = cfg.AllowAll
	h.core.Allowed = map[string]bool{}
	for _, channel := range cfg.AllowedChannels {
		h.core.Allowed[channel] = true
	}
	if h.core.AllowAll {
		t.Fatal("config with an allow-list must not set AllowAll")
	}

	// unknown source: rejected, runner untouched, no state written, no queue entry
	rejected := h.mustIntake("slack-unknown", "thread-x", "msg-x", "nope")
	if rejected.Route != RouteRejected {
		t.Fatalf("unknown channel route = %q, want rejected", rejected.Route)
	}
	if rejected.ReasonCode != ReasonSourceNotAllowed {
		t.Fatalf("unknown channel reason = %q, want %q", rejected.ReasonCode, ReasonSourceNotAllowed)
	}
	if len(h.calls()) != 0 {
		t.Fatalf("runner called for a rejected source: %#v", h.calls())
	}
	if recs := h.slowPathRecords(); len(recs) != 0 {
		t.Fatalf("rejected source was queued on the slow path: %#v", recs)
	}

	// allowed source, no binding: slow path queue entry, runner untouched
	slow := h.mustIntake("feishu", "thread-none", "msg-slow", "unbound text")
	if slow.Route != RouteSlow {
		t.Fatalf("unbound thread route = %q, want slow", slow.Route)
	}
	if slow.ReasonCode != ReasonNoBinding {
		t.Fatalf("unbound thread reason = %q, want %q", slow.ReasonCode, ReasonNoBinding)
	}
	if slow.Forwarded {
		t.Fatal("slow path reported forwarded=true")
	}
	if len(h.calls()) != 0 {
		t.Fatalf("runner called for an unbound thread: %#v", h.calls())
	}
	recs := h.slowPathRecords()
	if len(recs) != 1 {
		t.Fatalf("slow path records = %d, want 1 (%#v)", len(recs), recs)
	}
	if recs[0].Channel != "feishu" || recs[0].Thread != "thread-none" ||
		recs[0].MessageID != "msg-slow" || recs[0].Text != "unbound text" {
		t.Fatalf("slow path record = %#v, want the intake event", recs[0])
	}

	// AC-005 fail-closed: an absent allow-list allows nothing. Only an
	// explicit adbd_allow_all=true opts out.
	h.writeConfig("")
	cfg, err = LoadConfig(h.configPath)
	if err != nil {
		t.Fatalf("LoadConfig(empty): %v", err)
	}
	if cfg.AllowAll || len(cfg.AllowedChannels) != 0 {
		t.Fatal("empty allow-list must be fail-closed")
	}
	h.writeConfig("adbd_allow_all=true")
	cfg, err = LoadConfig(h.configPath)
	if err != nil {
		t.Fatalf("LoadConfig(allow_all): %v", err)
	}
	if !cfg.AllowAll {
		t.Fatal("adbd_allow_all=true must opt out of the allow-list")
	}
}

// ---------------------------------------------------------------------------
// TC-IF-006 (AC-006): threads never cross, released binding cannot be revived
// ---------------------------------------------------------------------------

func TestNoCrossThreadLeak(t *testing.T) {
	h := newHarness(t)
	h.mustBind("feishu", "thread-a", "agent-a", DefaultTTL)
	h.mustBind("feishu", "thread-b", "agent-b", DefaultTTL)

	// Interleaved delivery: each worker only ever sees its own text.
	h.mustIntake("feishu", "thread-a", "a1", "alpha")
	h.mustIntake("feishu", "thread-b", "b1", "beta")
	h.mustIntake("feishu", "thread-a", "a2", "gamma")
	h.mustIntake("feishu", "thread-b", "b2", "delta")

	calls := h.calls()
	if len(calls) != 4 {
		t.Fatalf("runner calls = %d, want 4 (%#v)", len(calls), calls)
	}
	assertArgv(t, calls[0], []string{"send", "agent-a", "alpha"})
	assertArgv(t, calls[1], []string{"send", "agent-b", "beta"})
	assertArgv(t, calls[2], []string{"send", "agent-a", "gamma"})
	assertArgv(t, calls[3], []string{"send", "agent-b", "delta"})

	for _, call := range calls {
		if call[1] == "agent-a" && (call[2] == "beta" || call[2] == "delta") {
			t.Fatalf("thread-b text leaked to agent-a: %#v", call)
		}
		if call[1] == "agent-b" && (call[2] == "alpha" || call[2] == "gamma") {
			t.Fatalf("thread-a text leaked to agent-b: %#v", call)
		}
	}

	// One worker per thread: a second bind on a live thread is refused.
	if _, err := h.core.Bind("feishu", "thread-a", "agent-a2", DefaultTTL); err == nil {
		t.Fatal("second bind on a live thread was accepted")
	} else if !IsTransitionError(err, "binding_exists") {
		t.Fatalf("second bind error = %v, want code binding_exists", err)
	}

	// released -> bound is illegal: explicit release must be final.
	if _, err := h.core.Release("feishu", "thread-a"); err != nil {
		t.Fatalf("release thread-a: %v", err)
	}
	if st := h.status("feishu", "thread-a"); st.State != StateReleased {
		t.Fatalf("status after release = %q, want released", st.State)
	}
	if _, err := h.core.Bind("feishu", "thread-a", "agent-a", DefaultTTL); err == nil {
		t.Fatal("reviving a released binding was accepted")
	} else if !IsTransitionError(err, ReasonBindingReleased) {
		t.Fatalf("revival error = %v, want code %q", err, ReasonBindingReleased)
	}
	// ... and the surviving thread is unaffected.
	if st := h.status("feishu", "thread-b"); st.State != StateLeasing || st.AgentID != "agent-b" {
		t.Fatalf("thread-b status = %+v, want leasing/agent-b", st)
	}
	if got := len(h.calls()); got != 4 {
		t.Fatalf("release/revival attempts invoked the runner: %d calls", got)
	}
}

// ---------------------------------------------------------------------------
// TC-IF-007 (AC-004): bound -> leasing -> expiring -> unbound
// ---------------------------------------------------------------------------

func TestLeaseExpiryWalksToUnbound(t *testing.T) {
	h := newHarness(t)
	h.mustBind("feishu", "thread-walk", "agent-w", DefaultTTL)

	var walk []string
	walk = append(walk, h.status("feishu", "thread-walk").State)

	// Forward renews into leasing while the lease is comfortably long.
	first := h.mustIntake("feishu", "thread-walk", "w1", "stage one")
	if first.Route != RouteFast || first.State != StateLeasing {
		t.Fatalf("first intake = %+v, want fast/leasing", first)
	}
	walk = append(walk, h.status("feishu", "thread-walk").State)

	// 23h15m later only 45m remain: reading status flips it to expiring.
	h.clock.advance(23*time.Hour + 15*time.Minute)
	st := h.status("feishu", "thread-walk")
	if st.State != StateExpiring {
		t.Fatalf("state with 45m left = %q, want expiring", st.State)
	}
	if st.Lease != LeaseExpiring {
		t.Fatalf("lease with 45m left = %q, want expiring", st.Lease)
	}
	walk = append(walk, st.State)

	// Past the expiry the binding dissolves into unbound, no release needed.
	h.clock.advance(time.Hour)
	st = h.status("feishu", "thread-walk")
	if st.State != StateUnbound {
		t.Fatalf("state past expiry = %q, want unbound", st.State)
	}
	if st.Lease != LeaseExpired {
		t.Fatalf("lease past expiry = %q, want expired", st.Lease)
	}
	walk = append(walk, st.State)

	want := []string{StateBound, StateLeasing, StateExpiring, StateUnbound}
	if strings.Join(walk, ">") != strings.Join(want, ">") {
		t.Fatalf("state walk = %v, want %v", walk, want)
	}

	// The worker is gone: the next message may not resurrect the fastpath.
	res := h.mustIntake("feishu", "thread-walk", "w2", "stage two")
	if res.Route != RouteSlow || res.State != StateUnbound {
		t.Fatalf("post-expiry intake = %+v, want slow/unbound", res)
	}
	if got := len(h.calls()); got != 1 {
		t.Fatalf("runner calls = %d, want 1", got)
	}

	// A fresh bind is the only way back, and it starts at bound.
	h.mustBind("feishu", "thread-walk", "agent-w2", DefaultTTL)
	if st := h.status("feishu", "thread-walk"); st.State != StateBound || st.AgentID != "agent-w2" {
		t.Fatalf("rebind status = %+v, want bound/agent-w2", st)
	}
}

// ---------------------------------------------------------------------------
// TC-IF-008 (AC-006): the three illegal moves are rejected
// ---------------------------------------------------------------------------

func TestIllegalTransitionsRejected(t *testing.T) {
	cases := []struct {
		name  string
		from  string
		event Event
		code  string
	}{
		{"released_to_bound", StateReleased, EventBind, ReasonBindingReleased},
		{"unbound_to_leasing", StateUnbound, EventIntake, "unbound_cannot_lease"},
		{"expiring_to_released", StateExpiring, EventRelease, "illegal_transition"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			to, err := Transition(tc.from, tc.event)
			if err == nil {
				t.Fatalf("%s --%s--> was accepted as %q, want rejection", tc.from, tc.event, to)
			}
			if to != "" {
				t.Fatalf("rejected transition still returned state %q", to)
			}
			if !IsTransitionError(err, tc.code) {
				t.Fatalf("error = %v, want code %q", err, tc.code)
			}
		})
	}

	// Legal counterparts of the same edges still work.
	for _, legal := range []struct {
		from  string
		event Event
		to    string
	}{
		{StateUnbound, EventBind, StateBound},
		{StateBound, EventIntake, StateLeasing},
		{StateBound, EventRelease, StateReleased},
		{StateLeasing, EventExpire, StateExpiring},
		{StateExpiring, EventExpireRelease, StateUnbound},
	} {
		to, err := Transition(legal.from, legal.event)
		if err != nil {
			t.Fatalf("%s --%s--> rejected: %v", legal.from, legal.event, err)
		}
		if to != legal.to {
			t.Fatalf("%s --%s--> = %q, want %q", legal.from, legal.event, to, legal.to)
		}
	}

	// ... and the same rules hold through the public API, not just the table.
	h := newHarness(t)
	h.mustBind("feishu", "thread-i", "agent-i", 30*time.Minute)
	// 30m TTL: forwarding lands straight in expiring.
	res := h.mustIntake("feishu", "thread-i", "i1", "hi")
	if res.State != StateExpiring {
		t.Fatalf("state with a 30m lease = %q, want expiring", res.State)
	}
	if _, err := h.core.Release("feishu", "thread-i"); err == nil {
		t.Fatal("releasing an expiring binding was accepted")
	} else if !IsTransitionError(err, "illegal_transition") {
		t.Fatalf("expiring release error = %v, want illegal_transition", err)
	}
	// A released thread refuses both revival and further fastpath traffic.
	h2 := newHarness(t)
	h2.mustBind("feishu", "thread-r", "agent-r", DefaultTTL)
	if _, err := h2.core.Release("feishu", "thread-r"); err != nil {
		t.Fatalf("release: %v", err)
	}
	revived := h2.mustIntake("feishu", "thread-r", "r1", "after release")
	if revived.Route != RouteSlow || revived.ReasonCode != ReasonBindingReleased {
		t.Fatalf("post-release intake = %+v, want slow/binding_released", revived)
	}
	if got := len(h2.calls()); got != 0 {
		t.Fatalf("released thread forwarded: %d runner calls", got)
	}
}
