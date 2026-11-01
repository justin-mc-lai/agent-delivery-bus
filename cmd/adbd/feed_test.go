package main

// Skill-trigger tests (feature skill-trigger, v0.1.5): TC-ST-001 .. TC-ST-006.
//
// Everything here runs against a temp root, an in-memory stub runner acting as the
// Dispatcher seam and an injected clock. No paseo binary is needed and no
// launchd job, heartbeat or resident process is involved: a failure here is
// always a logic failure, never an environment one.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// harness: temp root + stub runner + injected clock
// ---------------------------------------------------------------------------

// stubRunner is the stub Dispatcher on the fastpath side: it records what a fed
// message would hand a worker, in memory, and can be told to fail.
type stubRunner struct {
	calls   []dispatchCall
	failFor func(call dispatchCall) error
}

func (r *stubRunner) Send(agentID, text string) error {
	call := dispatchCall{agentID: agentID, text: text}
	r.calls = append(r.calls, call)
	if r.failFor != nil {
		return r.failFor(call)
	}
	return nil
}

// texts returns the text of every recorded hand-off, in order.
func (r *stubRunner) texts() []string {
	out := make([]string, 0, len(r.calls))
	for _, call := range r.calls {
		out = append(out, call.text)
	}
	return out
}

// feedHarness owns one temp ADB root plus the seams feed and route read.
type feedHarness struct {
	t          *testing.T
	root       string
	statePath  string
	slowPath   string
	configPath string
	tracePath  string
	argvLog    string
	runnerBin  string
	clock      *fakeClock
	runner     *stubRunner
}

func newFeedHarness(t *testing.T) *feedHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	h := &feedHarness{
		t:          t,
		root:       root,
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		configPath: filepath.Join(beacon, "config"),
		tracePath:  filepath.Join(beacon, decisionsFile),
		argvLog:    filepath.Join(root, "runner-argv.log"),
		runner:     &stubRunner{},
	}
	h.runnerBin = writeRecordingRunner(t, h.argvLog)

	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h.clock = &fakeClock{t: start}

	// Injected clock: feed stamps leases, dedupe keys and trace lines with it.
	prevNow := feedNow
	feedNow = h.clock.now
	t.Cleanup(func() { feedNow = prevNow })

	// Stub Dispatcher: the core feed delegates to hands text here, never to paseo.
	prevCoreFor := feedCoreFor
	feedCoreFor = func(cf *commonFlags) (*Core, error) {
		core, err := cf.core()
		if err != nil {
			return nil, err
		}
		core.Runner = h.runner
		return core, nil
	}
	t.Cleanup(func() { feedCoreFor = prevCoreFor })
	return h
}

// feed runs `adbd feed` against this root and returns exit code, stdout, stderr.
func (h *feedHarness) feed(args ...string) (int, string, string) {
	h.t.Helper()
	var out, errOut bytes.Buffer
	argv := append([]string{"feed", "--root", h.root}, args...)
	code := run(argv, &out, &errOut)
	return code, out.String(), errOut.String()
}

// feedProject is the common case: project, slug and text only.
func (h *feedHarness) feedProject(project, slug, text string) (int, string, string) {
	h.t.Helper()
	return h.feed("--project", project, "--slug", slug, "--text", text)
}

// routeOnce drains the slow path through the real router with a stub binary that
// only records its argv: paseo itself is never involved.
func (h *feedHarness) routeOnce() (int, string, string) {
	h.t.Helper()
	return h.routeOnceWith(h.runnerBin)
}

// routeOnceWith is routeOnce with an explicit runner binary, for a test that wants
// the dispatched text captured verbatim rather than as argv lines.
func (h *feedHarness) routeOnceWith(runnerBin string) (int, string, string) {
	h.t.Helper()
	var out, errOut bytes.Buffer
	code := run([]string{
		"route", "--once",
		"--root", h.root,
		"--runner-bin", runnerBin,
		"--trace", h.tracePath,
	}, &out, &errOut)
	return code, out.String(), errOut.String()
}

// bind creates a live binding through the public API with the injected clock, so
// feed takes the fastpath for that thread.
func (h *feedHarness) bind(channel, thread, agentID string) {
	h.t.Helper()
	core := NewCore(&Store{Path: h.statePath, SlowPath: h.slowPath}, nil)
	core.Now = h.clock.now
	if _, err := core.Bind(channel, thread, agentID, DefaultTTL); err != nil {
		h.t.Fatalf("bind %s/%s: %v", channel, thread, err)
	}
}

// writeConfig writes .beacon/config for the feed command to read.
func (h *feedHarness) writeConfig(body string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0o755); err != nil {
		h.t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(h.configPath, []byte(body), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

func (h *feedHarness) slowPaths() []slowPathRecord {
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
			h.t.Fatalf("parse slow-path line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func (h *feedHarness) traces() []traceRecord {
	h.t.Helper()
	data, err := os.ReadFile(h.tracePath)
	if err != nil {
		if os.IsNotExist(err) {
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
			h.t.Fatalf("parse trace line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// feedFields splits the single-line text outcome into its key=value pairs.
func feedFields(t *testing.T, out string) map[string]string {
	t.Helper()
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(out), "\n", 2)[0])
	fields := map[string]string{}
	for _, part := range strings.Fields(line) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		fields[key] = value
	}
	return fields
}

// writeTextLogRunner writes a real #!/bin/sh stub that appends every dispatched
// text verbatim to one file, so a multi-line UNTRUSTED_INPUT payload stays
// readable instead of being cut at its newline.
//
// Like writeRecordingRunner it speaks the AC-009 worker-resolution verbs: any
// `inspect` succeeds and `run --json` answers with an agent id, so only `send`
// calls (the text the worker actually receives) reach the log.
func writeTextLogRunner(t *testing.T, logPath string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "stub-paseo-text.sh")
	body := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  inspect) exit 0 ;;\n" +
		"  run) printf '%s\\n' '{\"agentId\":\"ag-textlog-worker\"}'; exit 0 ;;\n" +
		"esac\n" +
		"printf '%s\\n' '--- call ---' >> " + shellQuote(logPath) + "\n" +
		"printf '%s\\n' \"$3\" >> " + shellQuote(logPath) + "\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write text-log runner: %v", err)
	}
	return script
}

// readTextLog returns one entry per recorded dispatch, in order.
func readTextLog(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read text log: %v", err)
	}
	var out []string
	for _, chunk := range strings.Split(string(data), "--- call ---\n") {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		out = append(out, strings.TrimSuffix(chunk, "\n"))
	}
	return out
}

// ---------------------------------------------------------------------------
// TC-ST-001 (AC-001): the skill document teaches the single entry point
// ---------------------------------------------------------------------------

func TestSkillDocContract(t *testing.T) {
	path := filepath.Join("..", "..", "skills", "agent-delivery-bus", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc := string(data)

	const (
		heading   = "## 触发 router（skill intake）"
		shorthand = "adbd feed --project <p> --slug <s> --text <t>"
	)

	// 1. The heading exists exactly, as its own line.
	var lines []string
	headingAt := -1
	for i, line := range strings.Split(doc, "\n") {
		lines = append(lines, line)
		if strings.TrimRight(line, " \t") == heading {
			if headingAt >= 0 {
				t.Fatalf("%q appears twice in %s", heading, path)
			}
			headingAt = i
		}
	}
	if headingAt < 0 {
		t.Fatalf("%s is missing the frozen heading %q", path, heading)
	}

	// 2. The recommended command shape lives inside that section.
	sectionEnd := len(lines)
	for i := headingAt + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			sectionEnd = i
			break
		}
	}
	section := strings.Join(lines[headingAt:sectionEnd], "\n")
	if !strings.Contains(section, shorthand) {
		t.Fatalf("section %q does not contain the command shape %q", heading, shorthand)
	}
	if !strings.Contains(section, "唯一入口") {
		t.Fatalf("section %q does not declare the command as the single entry", heading)
	}

	// 3. Nothing in the document teaches assembling adbd intake by hand: the
	//    trigger contract has exactly one entry, and a copy-pasteable
	//    "adbd intake --channel" line is what AC-001 forbids.
	for i, line := range lines {
		if strings.Contains(line, "adbd intake --channel") {
			t.Fatalf("%s line %d still teaches hand-assembled intake: %q", path, i+1, line)
		}
		instruction := strings.TrimSpace(strings.TrimLeft(line, "-*`> "))
		if strings.HasPrefix(instruction, "adbd intake") {
			t.Fatalf("%s line %d presents adbd intake as a runnable instruction: %q", path, i+1, line)
		}
	}
}

// ---------------------------------------------------------------------------
// TC-ST-002 (AC-002): default channel, derived idempotency key, dedupe
// ---------------------------------------------------------------------------

func TestFeedDefaultsAndIdempotency(t *testing.T) {
	h := newFeedHarness(t)
	text := "把 studio 的 AC-003 做掉"

	code, out, errOut := h.feedProject("studio", "ac-003", text)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	got := feedFields(t, out)
	if got["route"] != "slow" {
		t.Fatalf("route = %q, want slow (out: %s)", got["route"], strings.TrimSpace(out))
	}
	// The channel defaults to "skill" with no config and no --channel.
	if got["channel"] != DefaultFeedChannel {
		t.Fatalf("channel = %q, want %q", got["channel"], DefaultFeedChannel)
	}
	if got["thread"] != "skill:studio:ac-003" {
		t.Fatalf("thread = %q, want skill:studio:ac-003", got["thread"])
	}

	// The key is sha256(channel|thread|text)[:16], computed here independently.
	sum := sha256.Sum256([]byte(DefaultFeedChannel + "|skill:studio:ac-003|" + text))
	want := hex.EncodeToString(sum[:])[:feedMessageIDHexLen]
	if got["message_id"] != want {
		t.Fatalf("message_id = %q, want %q (sha256 of channel|thread|text)", got["message_id"], want)
	}
	if FeedMessageID(DefaultFeedChannel, "skill:studio:ac-003", text) != want {
		t.Fatalf("FeedMessageID disagrees with sha256(channel|thread|text)[:16] = %q", want)
	}
	if len(want) != 16 {
		t.Fatalf("derived key length = %d, want 16 hex chars", len(want))
	}

	// Same input twice -> same key, deduped, and still just one queued item.
	h.clock.advance(2 * time.Minute)
	code2, out2, errOut2 := h.feedProject("studio", "ac-003", text)
	if code2 != 0 {
		t.Fatalf("second feed exit = %d, want 0 (stderr: %s)", code2, errOut2)
	}
	got2 := feedFields(t, out2)
	if got2["route"] != "deduped" {
		t.Fatalf("second route = %q, want deduped (out: %s)", got2["route"], strings.TrimSpace(out2))
	}
	if got2["message_id"] != want {
		t.Fatalf("second message_id = %q, want the same %q", got2["message_id"], want)
	}
	if reason := got2["reason_code"]; reason != ReasonDuplicateMessage {
		t.Fatalf("dedupe reason_code = %q, want %q", reason, ReasonDuplicateMessage)
	}
	if queued := h.slowPaths(); len(queued) != 1 {
		t.Fatalf("slow-path records after a replayed feed = %d, want 1", len(queued))
	}
	if calls := len(h.runner.calls); calls != 0 {
		t.Fatalf("unbound feed forwarded %d time(s), want 0", calls)
	}

	// adbd_feed_project_channel is the config-driven default channel.
	other := newFeedHarness(t)
	other.writeConfig("adbd_feed_project_channel=skillx\n")
	code3, out3, errOut3 := other.feedProject("studio", "ac-003", text)
	if code3 != 0 {
		t.Fatalf("config-channel feed exit = %d, want 0 (stderr: %s)", code3, errOut3)
	}
	if got := feedFields(t, out3); got["channel"] != "skillx" || got["thread"] != "skill:studio:ac-003" {
		t.Fatalf("config default channel = %q thread = %q, want skillx / skill:studio:ac-003",
			got["channel"], got["thread"])
	}
	// A different channel is a different event, so it is not deduped against the
	// skill-channel copy.
	if got := feedFields(t, out3); got["route"] != "slow" {
		t.Fatalf("first feed on a new channel = %q, want slow", got["route"])
	}

	// An explicit --message-id is honoured verbatim.
	explicit := newFeedHarness(t)
	code4, out4, errOut4 := explicit.feed("--project", "studio", "--slug", "ac-003", "--text", text, "--message-id", "m-42")
	if code4 != 0 {
		t.Fatalf("explicit message-id feed exit = %d, want 0 (stderr: %s)", code4, errOut4)
	}
	if got := feedFields(t, out4); got["message_id"] != "m-42" {
		t.Fatalf("explicit message_id = %q, want m-42", got["message_id"])
	}
}

// ---------------------------------------------------------------------------
// TC-ST-003 (AC-003): frozen thread shape, rejection, per-project isolation
// ---------------------------------------------------------------------------

func TestThreadShapeAndIsolation(t *testing.T) {
	// The accepted shape.
	if thread, err := FeedThread("studio", "ac-003"); err != nil || thread != "skill:studio:ac-003" {
		t.Fatalf("FeedThread(studio, ac-003) = %q, %v; want skill:studio:ac-003, nil", thread, err)
	}
	if thread, err := FeedThread("my-proj_1.2", "a"); err != nil || thread != "skill:my-proj_1.2:a" {
		t.Fatalf("FeedThread with the full alphabet = %q, %v", thread, err)
	}

	bad := []struct{ name, project, slug string }{
		{"empty_project", "", "ac-003"},
		{"empty_slug", "studio", ""},
		{"uppercase_project", "Studio", "ac-003"},
		{"uppercase_slug", "studio", "AC-003"},
		{"space_inside", "studio one", "ac-003"},
		{"slash_inside", "studio", "ac/003"},
		{"colon_inside", "studio:sub", "ac-003"},
		{"non_ascii", "studío", "ac-003"},
		{"slug_too_long", "studio", strings.Repeat("a", 65)},
		{"project_too_long", strings.Repeat("p", 65), "ac-003"},
	}

	// The alphabet is an alphabet, not a path rule: ".." and a leading dash are
	// inside [a-z0-9._-] and are therefore accepted threads, not rejections.
	for _, tc := range []struct{ project, slug string }{{"studio", ".."}, {"studio", "-ac-003"}} {
		if _, err := FeedThread(tc.project, tc.slug); err != nil {
			t.Fatalf("FeedThread(%q, %q) rejected an alphabet-valid part: %v", tc.project, tc.slug, err)
		}
	}

	h := newFeedHarness(t)
	for _, tc := range bad {
		code, out, errOut := h.feedProject(tc.project, tc.slug, "do the thing")
		reason := feedFields(t, out)["reason_code"]
		if code != 2 {
			t.Fatalf("%s: exit = %d, want 2 (out: %s)", tc.name, code, strings.TrimSpace(out))
		}
		if reason != ReasonThreadInvalid {
			t.Fatalf("%s: reason_code = %q, want %q", tc.name, reason, ReasonThreadInvalid)
		}
		if !strings.Contains(errOut, ReasonThreadInvalid) {
			t.Fatalf("%s: stderr = %q, want it to name %q", tc.name, errOut, ReasonThreadInvalid)
		}
		if thread, err := FeedThread(tc.project, tc.slug); err == nil {
			t.Fatalf("%s: FeedThread(%q, %q) = %q, want a rejection", tc.name, tc.project, tc.slug, thread)
		}
		// A rejected trigger writes nothing at all.
		if queued := h.slowPaths(); len(queued) != 0 {
			t.Fatalf("%s: a rejected feed queued %d item(s)", tc.name, len(queued))
		}
		if traces := h.traces(); len(traces) != 0 {
			t.Fatalf("%s: a rejected feed wrote %d trace line(s)", tc.name, len(traces))
		}
	}

	// Different projects with the same slug never share a thread or a key.
	iso := newFeedHarness(t)
	if code, out, errOut := iso.feedProject("studio", "ac-003", "same words"); code != 0 {
		t.Fatalf("studio feed exit = %d (stderr: %s, out: %s)", code, errOut, out)
	}
	if code, out, errOut := iso.feedProject("blog", "ac-003", "same words"); code != 0 {
		t.Fatalf("blog feed exit = %d (stderr: %s, out: %s)", code, errOut, out)
	}
	queued := iso.slowPaths()
	if len(queued) != 2 {
		t.Fatalf("slow-path records = %d, want 2 (both projects queued)", len(queued))
	}
	if queued[0].Thread == queued[1].Thread {
		t.Fatalf("both projects landed on the same thread %q", queued[0].Thread)
	}
	if queued[0].MessageID == queued[1].MessageID {
		t.Fatalf("both projects produced the same message_id %q", queued[0].MessageID)
	}
	wantThreads := map[string]bool{"skill:studio:ac-003": true, "skill:blog:ac-003": true}
	for _, rec := range queued {
		if !wantThreads[rec.Thread] {
			t.Fatalf("unexpected thread %q in the queue", rec.Thread)
		}
		delete(wantThreads, rec.Thread)
	}
	if len(wantThreads) != 0 {
		t.Fatalf("threads never queued: %v", wantThreads)
	}
}

// ---------------------------------------------------------------------------
// TC-ST-004 (AC-004): end to end -- one line, guarded text, one of three routes
// ---------------------------------------------------------------------------

func TestFeedEndToEndTrace(t *testing.T) {
	const text = "add a retry to the webhook handler"

	// (a) Slow path: feed queues, the router drains, the worker sees the guard.
	slow := newFeedHarness(t)
	code, out, errOut := slow.feedProject("studio", "ac-003", text)
	if code != 0 {
		t.Fatalf("feed exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if got := feedFields(t, out); got["route"] != RouteSlow {
		t.Fatalf("feed route = %q, want slow (out: %s)", got["route"], strings.TrimSpace(out))
	}
	if traces := slow.traces(); len(traces) != 0 {
		t.Fatalf("feed wrote %d trace line(s) on the slow path, want 0 (the router traces it)", len(traces))
	}

	textLog := filepath.Join(slow.root, "route-texts.log")
	routeRunner := writeTextLogRunner(t, textLog)
	routeCode, routeOut, routeErr := slow.routeOnceWith(routeRunner)
	if routeCode != 0 {
		t.Fatalf("route exit = %d, want 0 (stderr: %s)", routeCode, routeErr)
	}
	if !strings.Contains(routeOut, "applied=1") {
		t.Fatalf("route drain did not apply the fed item: %q", strings.TrimSpace(routeOut))
	}

	messageID := feedFields(t, out)["message_id"]
	traces := slow.traces()
	if len(traces) != 1 {
		t.Fatalf("decision trace lines = %d, want exactly 1", len(traces))
	}
	line := traces[0]
	if line.MessageID != messageID || line.Thread != "skill:studio:ac-003" || line.Channel != DefaultFeedChannel {
		t.Fatalf("trace line = %+v, want the fed message (%s on skill:studio:ac-003)", line, messageID)
	}
	if !line.Untrusted {
		t.Fatalf("trace line did not mark the text untrusted: %+v", line)
	}
	dispatched := readTextLog(t, textLog)
	if len(dispatched) != 1 {
		t.Fatalf("dispatched texts = %d, want 1", len(dispatched))
	}
	if !strings.HasPrefix(dispatched[0], UntrustedGuardLine) {
		t.Fatalf("dispatched text = %q, want it to start with %q", dispatched[0], UntrustedGuardLine)
	}
	if strings.Count(dispatched[0], UntrustedGuardLine) != 1 {
		t.Fatalf("dispatched text carries the guard line %d times, want 1: %q",
			strings.Count(dispatched[0], UntrustedGuardLine), dispatched[0])
	}
	if !strings.Contains(dispatched[0], text) {
		t.Fatalf("dispatched text lost the utterance: %q", dispatched[0])
	}

	// (b) Fast path: feed delivers straight to the bound worker, and that one
	// delivery is the one trace line for the message.
	fast := newFeedHarness(t)
	fast.bind(DefaultFeedChannel, "skill:studio:ac-003", "agent-fast")
	fastCode, fastOut, fastErr := fast.feedProject("studio", "ac-003", text)
	if fastCode != 0 {
		t.Fatalf("fast feed exit = %d, want 0 (stderr: %s)", fastCode, fastErr)
	}
	fields := feedFields(t, fastOut)
	if fields["route"] != RouteFast {
		t.Fatalf("bound feed route = %q, want fast (out: %s)", fields["route"], strings.TrimSpace(fastOut))
	}
	if fields["agent_id"] != "agent-fast" {
		t.Fatalf("agent_id = %q, want agent-fast", fields["agent_id"])
	}
	if calls := len(fast.runner.calls); calls != 1 {
		t.Fatalf("fastpath hand-offs = %d, want 1", calls)
	}
	if got := fast.runner.texts()[0]; !strings.HasPrefix(got, UntrustedGuardLine) {
		t.Fatalf("fastpath text = %q, want it to start with %q", got, UntrustedGuardLine)
	}
	fastTraces := fast.traces()
	if len(fastTraces) != 1 {
		t.Fatalf("fastpath trace lines = %d, want exactly 1", len(fastTraces))
	}
	if fastTraces[0].MessageID != fields["message_id"] || fastTraces[0].Thread != "skill:studio:ac-003" {
		t.Fatalf("fastpath trace line = %+v, want the fed message", fastTraces[0])
	}
	if queued := fast.slowPaths(); len(queued) != 0 {
		t.Fatalf("fastpath feed also queued %d item(s)", len(queued))
	}

	// (c) Deduped: the replay adds no second line and never re-dispatches.
	replayCode, replayOut, replayErr := fast.feedProject("studio", "ac-003", text)
	if replayCode != 0 {
		t.Fatalf("replayed feed exit = %d, want 0 (stderr: %s)", replayCode, replayErr)
	}
	if got := feedFields(t, replayOut); got["route"] != RouteDeduped {
		t.Fatalf("replayed route = %q, want deduped", got["route"])
	}
	if traces := fast.traces(); len(traces) != 1 {
		t.Fatalf("trace lines after a replay = %d, want 1", len(traces))
	}
	if calls := len(fast.runner.calls); calls != 1 {
		t.Fatalf("replayed feed dispatched again: %d hand-offs", calls)
	}

	// Every successful outcome prints exactly one of the three routes.
	for _, out := range []string{out, fastOut, replayOut} {
		route := feedFields(t, out)["route"]
		switch route {
		case RouteFast, RouteSlow, RouteDeduped:
		default:
			t.Fatalf("feed printed route=%q, want fast, slow or deduped", route)
		}
	}
}

// ---------------------------------------------------------------------------
// TC-ST-005 (AC-005): three loud reason codes, all with exit 2
// ---------------------------------------------------------------------------

func TestFeedErrorCodes(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		project  string
		slug     string
		wantCode string
	}{
		{"empty_text", "", "studio", "ac-003", ReasonTextEmpty},
		{"whitespace_text", "   \n\t ", "studio", "ac-003", ReasonTextEmpty},
		{"invalid_thread", "do it", "Studio", "ac-003", ReasonThreadInvalid},
		{"invalid_slug", "do it", "studio", "AC-003", ReasonThreadInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newFeedHarness(t)
			code, out, errOut := h.feedProject(tc.project, tc.slug, tc.text)
			if code != 2 {
				t.Fatalf("exit = %d, want 2 (out: %s, stderr: %s)", code, strings.TrimSpace(out), errOut)
			}
			if got := feedFields(t, out)["reason_code"]; got != tc.wantCode {
				t.Fatalf("reason_code = %q, want %q (out: %s)", got, tc.wantCode, strings.TrimSpace(out))
			}
			if !strings.Contains(errOut, tc.wantCode) {
				t.Fatalf("stderr = %q, want it to name %q", errOut, tc.wantCode)
			}
			// Neither of these may touch the queue or the trace.
			if queued := h.slowPaths(); len(queued) != 0 {
				t.Fatalf("a rejected feed queued %d item(s)", len(queued))
			}
			if traces := h.traces(); len(traces) != 0 {
				t.Fatalf("a rejected feed wrote %d trace line(s)", len(traces))
			}
			if calls := len(h.runner.calls); calls != 0 {
				t.Fatalf("a rejected feed reached the worker %d time(s)", calls)
			}
		})
	}

	// dispatch_failed: the thread is bound, the worker is down.
	h := newFeedHarness(t)
	h.bind(DefaultFeedChannel, "skill:studio:ac-003", "agent-down")
	h.runner.failFor = func(call dispatchCall) error {
		return errors.New("worker refused the message")
	}
	const text = "ship the migration"
	code, out, errOut := h.feedProject("studio", "ac-003", text)
	if code != 2 {
		t.Fatalf("dispatch failure exit = %d, want 2 (out: %s)", code, strings.TrimSpace(out))
	}
	fields := feedFields(t, out)
	if fields["reason_code"] != ReasonDispatchFailed {
		t.Fatalf("reason_code = %q, want %q (out: %s)", fields["reason_code"], ReasonDispatchFailed, strings.TrimSpace(out))
	}
	if fields["queued"] != "true" {
		t.Fatalf("a failed dispatch did not report queued=true: %s", strings.TrimSpace(out))
	}
	if !strings.Contains(errOut, ReasonDispatchFailed) {
		t.Fatalf("stderr = %q, want it to name %q", errOut, ReasonDispatchFailed)
	}
	// AC-005: the message is not lost -- it is back on the slow path.
	queued := h.slowPaths()
	if len(queued) != 1 {
		t.Fatalf("slow-path records after a failed dispatch = %d, want 1", len(queued))
	}
	if queued[0].ReasonCode != ReasonDispatchFailed || queued[0].Thread != "skill:studio:ac-003" {
		t.Fatalf("requeued item = %+v, want dispatch_failed on skill:studio:ac-003", queued[0])
	}
	if !strings.HasPrefix(queued[0].Text, UntrustedGuardLine) {
		t.Fatalf("requeued text = %q, want it to keep the guard line", queued[0].Text)
	}

	// ... and the router picks it up on its next drain, so the retry budget stays
	// with the router instead of the item evaporating.
	h.runner.failFor = nil
	routeCode, routeOut, routeErr := h.routeOnce()
	if routeCode != 0 {
		t.Fatalf("route after a failed dispatch exit = %d, want 0 (stderr: %s)", routeCode, routeErr)
	}
	if !strings.Contains(routeOut, "applied=1") {
		t.Fatalf("the requeued item was not retried: %q", strings.TrimSpace(routeOut))
	}
	if traces := h.traces(); len(traces) != 1 {
		t.Fatalf("trace lines after the retry = %d, want 1", len(traces))
	}
}

// ---------------------------------------------------------------------------
// TC-ST-006 (AC-006): one synchronous command, no daemon, no heartbeat
// ---------------------------------------------------------------------------

func TestFeedNeedsNoDaemon(t *testing.T) {
	const (
		text = "run the AC-003 slice"
		want = "skill:studio:ac-003"
	)

	// Two independent bare invocations of the same input agree, down to the
	// derived idempotency key: nothing in the answer depends on ambient state.
	var first, second string
	for i := 0; i < 2; i++ {
		h := newFeedHarness(t)
		code, out, errOut := h.feedProject("studio", "ac-003", text)
		if code != 0 {
			t.Fatalf("bare invocation %d exit = %d, want 0 (stderr: %s)", i+1, code, errOut)
		}
		if i == 0 {
			first = out
		} else {
			second = out
		}
		if got := feedFields(t, out); got["thread"] != want || got["route"] != RouteSlow {
			t.Fatalf("bare invocation %d = %s, want thread=%s route=slow", i+1, strings.TrimSpace(out), want)
		}
		// No daemon artifacts: the trigger never reads a heartbeat, never loads a
		// launchd job and never starts a resident process.
		for _, artifact := range []string{
			filepath.Join(h.root, ".beacon", heartbeatFile),
			filepath.Join(h.root, ".beacon", "adbd-watch-state.json"),
			filepath.Join(h.root, ".beacon", "adbd-watchdog.jsonl"),
			filepath.Join(h.root, ".beacon", "com.adbd.router.plist"),
		} {
			if _, err := os.Stat(artifact); err == nil {
				t.Fatalf("adbd feed created %s; it must depend on no daemon", artifact)
			} else if !os.IsNotExist(err) {
				t.Fatalf("stat %s: %v", artifact, err)
			}
		}
		if got := len(h.runner.calls); got != 0 {
			t.Fatalf("unbound feed started %d worker process(es), want 0", got)
		}
	}
	if first != second {
		t.Fatalf("two bare invocations disagreed:\n first: %s\nsecond: %s",
			strings.TrimSpace(first), strings.TrimSpace(second))
	}

	// Repeated calls in one root also stay consistent: same text, same key, one
	// queued item, and no lock or heartbeat is left behind afterwards.
	h := newFeedHarness(t)
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		code, out, errOut := h.feedProject("studio", "ac-003", text)
		if code != 0 {
			t.Fatalf("call %d exit = %d, want 0 (stderr: %s)", i+1, code, errOut)
		}
		seen[feedFields(t, out)["message_id"]] = true
	}
	if len(seen) != 1 {
		t.Fatalf("repeated calls produced %d different message ids, want 1", len(seen))
	}
	if queued := h.slowPaths(); len(queued) != 1 {
		t.Fatalf("slow-path records after two identical calls = %d, want 1", len(queued))
	}
	if _, err := os.Stat(h.statePath + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("feed left a state lock behind (%v); the command must be one-shot", err)
	}

	// Feed also runs in a root that has no .beacon directory yet: no installer, no
	// plist, no daemon bootstrapping required.
	bare := t.TempDir()
	var out, errOut bytes.Buffer
	code := run([]string{"feed", "--root", bare, "--project", "studio", "--slug", "ac-003", "--text", text}, &out, &errOut)
	if code != 0 {
		t.Fatalf("feed in a bare root exit = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(bare, ".beacon", "adbd-slowpath.jsonl")); err != nil {
		t.Fatalf("feed in a bare root did not queue the item: %v", err)
	}
}
