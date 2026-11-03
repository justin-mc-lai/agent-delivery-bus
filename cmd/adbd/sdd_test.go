package main

// worker-sdd-binding tests (v0.1.5): TC-SB-001 .. TC-SB-007.
//
// Everything here runs against a temp ADB root, a temp project repository and a
// fake `paseo` script on disk that records its own argv. No real paseo is
// spawned and nothing touches the network, so a failure here is always a logic
// failure, never an environment one.
//
// The fixture is deliberately the shape the gate is about: a repository with a
// docs/beacon tree holding two features -- one whose truth.md is `frozen` and one
// whose truth.md is not -- plus a repository with no Beacon tree at all.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// sddRepoSlug is the project slug the fixtures register the temp repository
// under; the threads below address it as skill:<slug>:<feature>.
const sddRepoSlug = "studio"

// writeTruth writes <repo>/<docsRoot>/<version>/features/<slug>/truth.md with the
// given frontmatter status and returns its path.
func writeTruth(t *testing.T, repo, docsRoot, version, slug, status string) string {
	t.Helper()
	dir := filepath.Join(repo, docsRoot, version, beaconFeaturesDir, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir feature package: %v", err)
	}
	body := "---\nslug: " + slug + "\nversion: " + version + "\nstatus: " + status + "\n---\n\n# Requirement Truth: " + slug + "\n"
	path := filepath.Join(dir, beaconTruthFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// writeFakeSDDPaseo writes the recording stand-in for the paseo CLI. Every
// invocation is recorded as two files in a calls directory -- `<n>.verb` and
// `<n>.args` (NUL separated) -- so an argument carrying newlines survives the
// round trip intact and a test can read back exactly what a worker was told.
func writeFakeSDDPaseo(t *testing.T, callsDir string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "fake-sdd-paseo.sh")
	body := "#!/bin/sh\n" +
		"dir=" + shellQuote(callsDir) + "\n" +
		"mkdir -p \"$dir\"\n" +
		"verb=\"$1\"\n" +
		"n=$(cat \"$dir/counter\" 2>/dev/null || printf 0)\n" +
		"n=$((n + 1))\n" +
		"printf '%s\\n' \"$n\" > \"$dir/counter\"\n" +
		"printf '%s' \"$verb\" > \"$dir/$n.verb\"\n" +
		"shift\n" +
		": > \"$dir/$n.args\"\n" +
		"for a in \"$@\"; do printf '%s' \"$a\" >> \"$dir/$n.args\"; printf '\\0' >> \"$dir/$n.args\"; done\n" +
		"if [ \"$verb\" = \"run\" ]; then printf '%s\\n' '{\"agentId\": \"ag-sdd-worker\"}'; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake paseo: %v", err)
	}
	return script
}

// sddCall is one recorded fake-paseo invocation.
type sddCall struct {
	verb string
	args []string
}

// text is the argv joined by newlines, which is what a test needs when it looks
// for a sentence that may span several arguments.
func (c sddCall) text() string { return strings.Join(c.args, "\n") }

// payload is the text of a `paseo send <agent-id> <text>` call: the agent id is
// argv[0] and everything after it is the message.
func (c sddCall) payload() string {
	if len(c.args) <= 1 {
		return ""
	}
	return strings.Join(c.args[1:], "\n")
}

type sddHarness struct {
	t          *testing.T
	root       string
	bin        string
	callsDir   string
	configPath string
	statePath  string
	slowPath   string
	tracePath  string
	clock      *fakeClock
	projects   map[string]string
	dispatcher PaseoDispatcher
	router     *Router
}

// newSDDHarness builds a temp ADB root whose structural gate is on or off, with
// no project registered yet (tests register the repository they build).
func newSDDHarness(t *testing.T, gate bool) *sddHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h := &sddHarness{
		t:          t,
		root:       root,
		callsDir:   filepath.Join(root, "paseo-calls"),
		configPath: filepath.Join(beacon, "config"),
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		tracePath:  filepath.Join(beacon, decisionsFile),
		clock:      &fakeClock{t: start},
		projects:   map[string]string{},
	}
	h.bin = writeFakeSDDPaseo(t, h.callsDir)
	h.dispatcher = PaseoDispatcher{
		Bin:             h.bin,
		Registry:        ProjectRegistry{},
		FallbackWorkdir: root,
	}
	h.router = NewRouter(
		&Store{Path: h.statePath, SlowPath: h.slowPath},
		RuleDecider{},
		h.dispatcher,
	)
	h.router.TracePath = h.tracePath
	h.router.Now = h.clock.now
	h.router.SDDGate = gate
	h.router.BeaconDocsRoot = DefaultBeaconDocsRoot
	// A skill (or github) thread is a work request, so the dispatched text is the
	// request itself plus whatever the SDD gate adds -- no UNTRUSTED_INPUT line.
	h.router.TrustedChannels = []string{"skill", "github"}
	// No standing worker id: every dispatch creates a worker, which is what makes
	// the bootstrap prompt observable in the fake paseo's argv.
	h.router.AgentID = ""
	return h
}

// register maps a project slug onto a repository directory and reloads the
// registry into both seams (the gate and the dispatcher read the same one).
func (h *sddHarness) register(slug, repo string) {
	h.t.Helper()
	h.projects[slug] = repo
	entries := make([]map[string]string, 0, len(h.projects))
	for name, dir := range h.projects {
		entries = append(entries, map[string]string{"slug": name, "repo": dir})
	}
	data, err := json.MarshalIndent(map[string]any{"projects": entries}, "", "  ")
	if err != nil {
		h.t.Fatalf("marshal projects: %v", err)
	}
	path := filepath.Join(h.root, "config", "projects.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		h.t.Fatalf("write projects: %v", err)
	}
	registry := loadProjectRegistry(path, h.root)
	h.dispatcher.Registry = registry
	h.router.Dispatcher = h.dispatcher
	h.router.Registry = registry
}

func (h *sddHarness) enqueue(items ...QueueItem) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.slowPath), 0o755); err != nil {
		h.t.Fatalf("mkdir queue dir: %v", err)
	}
	if err := writeQueue(h.slowPath, items); err != nil {
		h.t.Fatalf("write queue: %v", err)
	}
}

func (h *sddHarness) drain() DrainResult {
	h.t.Helper()
	res, err := h.router.DrainOnce()
	if err != nil {
		h.t.Fatalf("DrainOnce: %v", err)
	}
	return res
}

func (h *sddHarness) queue() []QueueItem {
	h.t.Helper()
	items, err := readQueue(h.slowPath)
	if err != nil {
		h.t.Fatalf("read queue: %v", err)
	}
	return items
}

func (h *sddHarness) binding(channel, thread string) *Binding {
	h.t.Helper()
	snap, err := h.router.Store.load()
	if err != nil {
		h.t.Fatalf("load state: %v", err)
	}
	return snap.Bindings[bindingKey(channel, thread)]
}

func (h *sddHarness) trace() []traceRecord {
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
			h.t.Fatalf("trace line %q is not JSON: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func (h *sddHarness) traceFor(messageID string) traceRecord {
	h.t.Helper()
	for _, rec := range h.trace() {
		if rec.MessageID == messageID {
			return rec
		}
	}
	h.t.Fatalf("no trace line for message %q (trace=%+v)", messageID, h.trace())
	return traceRecord{}
}

// invocations lists every fake-paseo process the router started, whatever the
// verb. Empty is the proof that a refusal never reached out of the process.
func (h *sddHarness) invocations() []string {
	h.t.Helper()
	var verbs []string
	for _, call := range h.blocks() {
		verbs = append(verbs, call.verb)
	}
	return verbs
}

// blocks parses the recorded call directory back into calls, in invocation
// order.
func (h *sddHarness) blocks() []sddCall {
	h.t.Helper()
	entries, err := os.ReadDir(h.callsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read calls dir: %v", err)
	}
	type orderIndex map[int]bool
	indices := orderIndex{}
	calls := map[int]sddCall{}
	for _, entry := range entries {
		suffix, ok := strings.CutSuffix(entry.Name(), ".verb")
		if !ok {
			continue
		}
		index, convErr := strconv.Atoi(suffix)
		if convErr != nil {
			continue
		}
		verbData, readErr := os.ReadFile(filepath.Join(h.callsDir, entry.Name()))
		if readErr != nil {
			h.t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		argsData, readErr := os.ReadFile(filepath.Join(h.callsDir, suffix+".args"))
		if readErr != nil && !os.IsNotExist(readErr) {
			h.t.Fatalf("read %s.args: %v", suffix, readErr)
		}
		var args []string
		for _, arg := range strings.Split(string(argsData), "\x00") {
			if arg != "" {
				args = append(args, arg)
			}
		}
		indices[index] = true
		calls[index] = sddCall{verb: strings.TrimSpace(string(verbData)), args: args}
	}
	ordered := make([]int, 0, len(indices))
	for index := range indices {
		ordered = append(ordered, index)
	}
	sort.Ints(ordered)
	out := make([]sddCall, 0, len(ordered))
	for _, index := range ordered {
		out = append(out, calls[index])
	}
	return out
}

// blocksFor returns the recorded calls of one verb.
func (h *sddHarness) blocksFor(verb string) []sddCall {
	h.t.Helper()
	var out []sddCall
	for _, call := range h.blocks() {
		if call.verb == verb {
			out = append(out, call)
		}
	}
	return out
}

func (h *sddHarness) writeConfig(body string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0o755); err != nil {
		h.t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(h.configPath, []byte(body), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

// defaultConfig is the operator config the CLI tests run with: the gate on, the
// frozen docs root, and skill/github as trusted sources.
const sddTestConfig = "adbd_allow_all=true\n" +
	"adbd_trusted_channels=skill,github\n" +
	"adbd_sdd_gate=on\n" +
	"adbd_beacon_docs_root=" + DefaultBeaconDocsRoot + "\n"

func (h *sddHarness) cli(extra ...string) (int, string, string) {
	h.t.Helper()
	args := []string{
		"route", "--once",
		"--root", h.root,
		"--state", h.statePath,
		"--slow-path", h.slowPath,
		"--config", h.configPath,
		"--trace", h.tracePath,
		"--runner-bin", h.bin,
	}
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// ---------------------------------------------------------------------------
// TC-SB-001 (AC-001): a Beacon-managed project is recognised, and the grade is
// written into the trace field `sdd`
// ---------------------------------------------------------------------------

func TestSddDetectsBeaconProject(t *testing.T) {
	// Detection: the highest version directory that carries features/ wins, and
	// it is compared numerically, so v0.1.10 is not beaten by v0.1.9.
	repo := t.TempDir()
	writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.4", "alpha", "frozen")
	writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.10", "beta", "frozen")
	if version, ok := beaconManaged(repo, DefaultBeaconDocsRoot); !ok || version != "v0.1.10" {
		t.Fatalf("beaconManaged = (%q, %v), want (v0.1.10, true): highest version wins numerically",
			version, ok)
	}

	// A version directory without features/ is not a Beacon tree.
	shallow := t.TempDir()
	if err := os.MkdirAll(filepath.Join(shallow, DefaultBeaconDocsRoot, "v0.1.5"), 0o755); err != nil {
		t.Fatalf("mkdir shallow tree: %v", err)
	}
	if version, ok := beaconManaged(shallow, DefaultBeaconDocsRoot); ok {
		t.Fatalf("beaconManaged = (%q, true) for a tree with no features/, want not managed", version)
	}
	// No docs/beacon at all is the plain "not a Beacon project" answer.
	if version, ok := beaconManaged(t.TempDir(), DefaultBeaconDocsRoot); ok {
		t.Fatalf("beaconManaged = (%q, true) for a repo with no docs/beacon, want not managed", version)
	}

	// Feature resolution from a thread.
	if slug, ok := featureSlugFromThread("skill:studio:ac-003"); !ok || slug != "ac-003" {
		t.Fatalf("featureSlugFromThread(skill:studio:ac-003) = (%q, %v), want (ac-003, true)", slug, ok)
	}
	for _, thread := range []string{"gh:owner/repo#7", "feishu:oc_123", "skill:studio", "skill:Studio:Ac-003"} {
		if slug, ok := featureSlugFromThread(thread); ok {
			t.Fatalf("featureSlugFromThread(%q) = (%q, true), want no feature", thread, slug)
		}
	}
	wantPath := filepath.Join(repo, DefaultBeaconDocsRoot, "v0.1.10", beaconFeaturesDir, "beta", beaconTruthFile)
	if got := featureTruthPath(repo, DefaultBeaconDocsRoot, "v0.1.10", "beta"); got != wantPath {
		t.Fatalf("featureTruthPath = %q, want %q", got, wantPath)
	}

	// Lane 1: Beacon project + skill thread + frozen truth -> gated, dispatched.
	gated := newSDDHarness(t, true)
	gatedRepo := t.TempDir()
	writeTruth(t, gatedRepo, DefaultBeaconDocsRoot, "v0.1.5", "ac-003", "frozen")
	gated.register(sddRepoSlug, gatedRepo)
	gated.enqueue(QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-003", MessageID: "m-gated",
		Text: "add a health endpoint",
	})
	if res := gated.drain(); res.Applied != 1 {
		t.Fatalf("gated drain = %+v, want applied=1", res)
	}
	if rec := gated.traceFor("m-gated"); rec.SDD != SDDLaneGated {
		t.Fatalf("gated trace = %+v, want sdd=%s", rec, SDDLaneGated)
	}

	// Lane 2: a project with no Beacon tree -> not_applicable.
	plain := newSDDHarness(t, true)
	plainRepo := t.TempDir()
	plain.register(sddRepoSlug, plainRepo)
	plain.enqueue(QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-003", MessageID: "m-plain",
		Text: "add a health endpoint",
	})
	if res := plain.drain(); res.Applied != 1 {
		t.Fatalf("plain drain = %+v, want applied=1", res)
	}
	if rec := plain.traceFor("m-plain"); rec.SDD != SDDLaneNotApplicable {
		t.Fatalf("plain trace = %+v, want sdd=%s", rec, SDDLaneNotApplicable)
	}

	// Lane 3: a thread that names no project the bus knows -> guard_only.
	unknown := newSDDHarness(t, true)
	unknownRepo := t.TempDir()
	writeTruth(t, unknownRepo, DefaultBeaconDocsRoot, "v0.1.5", "ac-003", "frozen")
	unknown.register(sddRepoSlug, unknownRepo)
	unknown.enqueue(QueueItem{
		Channel: "feishu", Thread: "feishu:oc_guard", MessageID: "m-guard",
		Text: "add a health endpoint",
	})
	if res := unknown.drain(); res.Applied != 1 {
		t.Fatalf("guard drain = %+v, want applied=1", res)
	}
	if rec := unknown.traceFor("m-guard"); rec.SDD != SDDLaneGuardOnly {
		t.Fatalf("guard trace = %+v, want sdd=%s", rec, SDDLaneGuardOnly)
	}
}

// ---------------------------------------------------------------------------
// TC-SB-002 (AC-002): a registered but unfrozen truth refuses the dispatch
// ---------------------------------------------------------------------------

func TestSddRefusesUnfrozenTruth(t *testing.T) {
	h := newSDDHarness(t, true)
	repo := t.TempDir()
	writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.5", "ac-003", "draft")
	h.register(sddRepoSlug, repo)
	h.writeConfig(sddTestConfig)
	h.enqueue(QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-003", MessageID: "m-unfrozen",
		Text: "add a health endpoint",
	})

	code, out, stderr := h.cli()
	if code != 0 {
		// AC-002: a refusal is not an error, the daemon keeps breathing.
		t.Fatalf("route --once exit = %d, stderr = %q (a refusal must exit 0)", code, stderr)
	}
	if !strings.Contains(out, "reason_code="+ReasonTruthNotFrozen) {
		t.Fatalf("route output = %q, want reason_code=%s", out, ReasonTruthNotFrozen)
	}

	// Zero paseo calls: not even `inspect` ran, so no worker was created or
	// looked up.
	if got := h.invocations(); len(got) != 0 {
		t.Fatalf("paseo was called %v, want zero calls on a refusal", got)
	}
	// The item keeps its place in the slow-path queue.
	if q := h.queue(); len(q) != 1 || q[0].MessageID != "m-unfrozen" {
		t.Fatalf("queue after refusal = %+v, want the item kept", q)
	}
	// No binding was written.
	if b := h.binding("skill", "skill:studio:ac-003"); b != nil {
		t.Fatalf("binding after refusal = %+v, want none", b)
	}
	// The refusal is auditable.
	rec := h.traceFor("m-unfrozen")
	if rec.Decision != DecisionRefuse || rec.ReasonCode != ReasonTruthNotFrozen || rec.SDD != SDDLaneGated {
		t.Fatalf("refusal trace = %+v, want decision=%s reason_code=%s sdd=%s",
			rec, DecisionRefuse, ReasonTruthNotFrozen, SDDLaneGated)
	}

	// `--json` carries the same verdict, so a scheduler can branch on it.
	code, out, stderr = h.cli("--outcome", "json")
	if code != 0 {
		t.Fatalf("route --once --outcome json exit = %d, stderr = %q", code, stderr)
	}
	var result DrainResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("route --json output is not JSON: %v (%q)", err, out)
	}
	if result.ReasonCode != ReasonTruthNotFrozen {
		t.Fatalf("json reason_code = %q, want %q", result.ReasonCode, ReasonTruthNotFrozen)
	}
	if len(result.Traces) != 1 || result.Traces[0].SDD != SDDLaneGated {
		t.Fatalf("json traces = %+v, want one row with sdd=%s", result.Traces, SDDLaneGated)
	}
}

// ---------------------------------------------------------------------------
// TC-SB-003 (AC-003): a feature with no truth.md at all refuses the dispatch
// ---------------------------------------------------------------------------

func TestSddRefusesUnknownFeature(t *testing.T) {
	h := newSDDHarness(t, true)
	repo := t.TempDir()
	// The tree exists and another feature is frozen, so the project *is* Beacon
	// managed; only this thread's slug is unregistered.
	writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.5", "ac-001", "frozen")
	h.register(sddRepoSlug, repo)
	h.writeConfig(sddTestConfig)
	h.enqueue(QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-999", MessageID: "m-unknown",
		Text: "add a health endpoint",
	})

	code, out, stderr := h.cli()
	if code != 0 {
		t.Fatalf("route --once exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(out, "reason_code="+ReasonFeatureNotRegistered) {
		t.Fatalf("route output = %q, want reason_code=%s", out, ReasonFeatureNotRegistered)
	}
	if got := h.invocations(); len(got) != 0 {
		t.Fatalf("paseo was called %v, want zero calls on feature_not_registered", got)
	}
	if q := h.queue(); len(q) != 1 {
		t.Fatalf("queue after refusal = %+v, want the item kept", q)
	}
	if b := h.binding("skill", "skill:studio:ac-999"); b != nil {
		t.Fatalf("binding after refusal = %+v, want none", b)
	}
	rec := h.traceFor("m-unknown")
	if rec.Decision != DecisionRefuse || rec.ReasonCode != ReasonFeatureNotRegistered {
		t.Fatalf("refusal trace = %+v, want decision=%s reason_code=%s",
			rec, DecisionRefuse, ReasonFeatureNotRegistered)
	}
}

// ---------------------------------------------------------------------------
// TC-SB-004 (AC-004): a refusal keeps every piece of state, and only a frozen
// truth may turn the same item into a dispatch
// ---------------------------------------------------------------------------

func TestSddRefusalKeepsState(t *testing.T) {
	h := newSDDHarness(t, true)
	repo := t.TempDir()
	truthPath := writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.5", "ac-003", "draft")
	h.register(sddRepoSlug, repo)
	item := QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-003", MessageID: "m-state",
		Text: "add a health endpoint",
	}
	h.enqueue(item)

	for pass := 1; pass <= 2; pass++ {
		res := h.drain()
		if res.Applied != 0 || res.Failed != 1 {
			t.Fatalf("refusal drain %d = %+v, want applied=0 failed=1", pass, res)
		}
		if q := h.queue(); len(q) != 1 {
			t.Fatalf("queue after refusal %d = %+v, want one item kept", pass, q)
		}
		if b := h.binding("skill", "skill:studio:ac-003"); b != nil {
			t.Fatalf("binding after refusal %d = %+v, want none", pass, b)
		}
	}
	// Retrying never bypasses the gate: still zero paseo calls after two drains.
	if got := h.invocations(); len(got) != 0 {
		t.Fatalf("paseo was called %v after two refusals, want zero", got)
	}
	// The refusal row is *not* an applied verdict, so the second drain decided
	// the item again instead of silently dropping it.
	if got := len(h.trace()); got != 2 {
		t.Fatalf("trace rows = %d, want 2 (one per refused drain)", got)
	}

	// Freezing the truth is what opens the gate -- not the retries.
	if frozenPath := writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.5", "ac-003", "frozen"); frozenPath != truthPath {
		t.Fatalf("frozen truth path = %q, want the same package %q", frozenPath, truthPath)
	}
	res := h.drain()
	if res.Applied != 1 || res.Queued != 0 {
		t.Fatalf("post-freeze drain = %+v, want applied=1 queued=0", res)
	}
	if got := h.blocksFor("send"); len(got) != 1 {
		t.Fatalf("send calls = %d, want exactly 1", len(got))
	}
	binding := h.binding("skill", "skill:studio:ac-003")
	if binding == nil || binding.AgentID != "ag-sdd-worker" {
		t.Fatalf("binding after dispatch = %+v, want the confirmed worker", binding)
	}

	// Idempotency still holds: a replayed queue line is deduped, not re-sent.
	if err := writeQueue(h.slowPath, []QueueItem{item}); err != nil {
		t.Fatalf("replay queue: %v", err)
	}
	res = h.drain()
	if res.Skipped != 1 || res.Decided != 0 {
		t.Fatalf("replay drain = %+v, want skipped=1 decided=0", res)
	}
	if got := h.blocksFor("send"); len(got) != 1 {
		t.Fatalf("send calls after replay = %d, want still 1", len(got))
	}
}

// ---------------------------------------------------------------------------
// TC-SB-005 (AC-005): the guard paragraph reaches both the dispatched text and
// the bootstrap prompt
// ---------------------------------------------------------------------------

func TestSddGuardInDispatchedText(t *testing.T) {
	// The two sentences AC-005 names, checked literally.
	const spine = "beacon-go spine show"
	const rule = "Do not modify code that no frozen acceptance criterion covers."
	if !strings.Contains(sddGuard, spine) || !strings.Contains(sddGuard, rule) {
		t.Fatalf("sddGuard = %q, want it to name %q and %q", sddGuard, spine, rule)
	}
	if !strings.Contains(workerBootstrapPrompt("skill:studio:ac-003"), spine) ||
		!strings.Contains(workerBootstrapPrompt("skill:studio:ac-003"), rule) {
		t.Fatalf("workerBootstrapPrompt = %q, want the SDD guard paragraph",
			workerBootstrapPrompt("skill:studio:ac-003"))
	}

	// sdd=gated: the dispatched text starts with the guard paragraph.
	h := newSDDHarness(t, true)
	repo := t.TempDir()
	writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.5", "ac-003", "frozen")
	h.register(sddRepoSlug, repo)
	const request = "add a health endpoint"
	h.enqueue(QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-003", MessageID: "m-guard-text", Text: request,
	})
	if res := h.drain(); res.Applied != 1 {
		t.Fatalf("gated drain = %+v, want applied=1", res)
	}
	sends := h.blocksFor("send")
	if len(sends) != 1 {
		t.Fatalf("send calls = %d, want 1", len(sends))
	}
	text := sends[0].payload()
	if !strings.HasPrefix(text, sddGuard) {
		t.Fatalf("dispatched text = %q, want it to start with the SDD guard", text)
	}
	if !strings.HasSuffix(text, request) {
		t.Fatalf("dispatched text = %q, want the original request kept after the guard", text)
	}
	// A newly created worker is told the same thing up front.
	runs := h.blocksFor("run")
	if len(runs) != 1 {
		t.Fatalf("run calls = %d, want 1", len(runs))
	}
	if !strings.Contains(runs[0].text(), spine) || !strings.Contains(runs[0].text(), rule) {
		t.Fatalf("worker bootstrap = %q, want the SDD guard paragraph", runs[0].text())
	}

	// sdd=guard_only keeps UNTRUSTED_INPUT first and the SDD guard second.
	guard := newSDDHarness(t, true)
	guard.enqueue(QueueItem{
		Channel: "feishu", Thread: "feishu:oc_guard", MessageID: "m-guard-only", Text: request,
	})
	if res := guard.drain(); res.Applied != 1 {
		t.Fatalf("guard_only drain = %+v, want applied=1", res)
	}
	guardedSends := guard.blocksFor("send")
	if len(guardedSends) != 1 {
		t.Fatalf("guard_only send calls = %d, want 1", len(guardedSends))
	}
	guardedTextValue := guardedSends[0].payload()
	if !strings.HasPrefix(guardedTextValue, UntrustedGuardLine+"\n"+sddGuard) {
		t.Fatalf("guard_only text = %q, want %q first, then the SDD guard",
			guardedTextValue, UntrustedGuardLine)
	}
	if !strings.HasSuffix(guardedTextValue, request) {
		t.Fatalf("guard_only text = %q, want the original request kept", guardedTextValue)
	}
}

// ---------------------------------------------------------------------------
// TC-SB-006 (AC-006): a project that is not Beacon-managed is untouched
// ---------------------------------------------------------------------------

func TestSddNonBeaconUnaffected(t *testing.T) {
	h := newSDDHarness(t, true)
	repo := t.TempDir()
	// A content/research checkout: no docs/beacon anywhere.
	if err := os.MkdirAll(filepath.Join(repo, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	h.register(sddRepoSlug, repo)
	h.writeConfig(sddTestConfig)
	const request = "add a health endpoint"
	h.enqueue(QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-003", MessageID: "m-non-beacon", Text: request,
	})

	code, out, stderr := h.cli()
	if code != 0 {
		t.Fatalf("route --once exit = %d, stderr = %q", code, stderr)
	}
	if strings.Contains(out, "reason_code="+ReasonTruthNotFrozen) ||
		strings.Contains(out, "reason_code="+ReasonFeatureNotRegistered) {
		t.Fatalf("non-beacon project was refused: %q", out)
	}
	rec := h.traceFor("m-non-beacon")
	if rec.Decision != DecisionRoute || rec.SDD != SDDLaneNotApplicable {
		t.Fatalf("non-beacon trace = %+v, want decision=%s sdd=%s",
			rec, DecisionRoute, SDDLaneNotApplicable)
	}
	sends := h.blocksFor("send")
	if len(sends) != 1 {
		t.Fatalf("send calls = %d, want 1", len(sends))
	}
	if got := sends[0].payload(); got != request {
		t.Fatalf("dispatched text = %q, want the request verbatim %q (no SDD guard)", got, request)
	}
}

// ---------------------------------------------------------------------------
// TC-SB-007 (AC-007): the gate is configurable and both the setting and the
// verdict are visible in --json
// ---------------------------------------------------------------------------

func TestSddConfigAndTraces(t *testing.T) {
	// Defaults: the gate is on, the docs root is docs/beacon.
	if cfg, err := LoadConfig(""); err != nil {
		t.Fatalf("LoadConfig(\"\") error: %v", err)
	} else {
		if !cfg.SDDGate || cfg.BeaconDocsRoot != DefaultBeaconDocsRoot {
			t.Fatalf("default config sdd = (%v, %q), want (true, %q)",
				cfg.SDDGate, cfg.BeaconDocsRoot, DefaultBeaconDocsRoot)
		}
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	writeFile := func(body string) Config {
		t.Helper()
		if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		return cfg
	}
	if cfg := writeFile("adbd_sdd_gate=on\n"); !cfg.SDDGate {
		t.Fatalf("adbd_sdd_gate=on -> %v, want true", cfg.SDDGate)
	}
	if cfg := writeFile("adbd_sdd_gate=off\n"); cfg.SDDGate {
		t.Fatalf("adbd_sdd_gate=off -> %v, want false", cfg.SDDGate)
	}
	// A typo never disables the gate: the safe answer wins.
	if cfg := writeFile("adbd_sdd_gate=maybe\n"); !cfg.SDDGate {
		t.Fatalf("adbd_sdd_gate=maybe -> %v, want the gate to stay on", cfg.SDDGate)
	}
	if cfg := writeFile("adbd_beacon_docs_root=beacon\n"); cfg.BeaconDocsRoot != "beacon" {
		t.Fatalf("adbd_beacon_docs_root=beacon -> %q, want beacon", cfg.BeaconDocsRoot)
	}

	// Gate off: the unfrozen truth is dispatched anyway and the row says so.
	off := newSDDHarness(t, false)
	repo := t.TempDir()
	writeTruth(t, repo, DefaultBeaconDocsRoot, "v0.1.5", "ac-003", "draft")
	off.register(sddRepoSlug, repo)
	off.writeConfig(strings.Replace(sddTestConfig, "adbd_sdd_gate=on", "adbd_sdd_gate=off", 1))
	off.enqueue(QueueItem{
		Channel: "skill", Thread: "skill:studio:ac-003", MessageID: "m-off",
		Text: "add a health endpoint",
	})
	code, out, stderr := off.cli("--outcome", "json")
	if code != 0 {
		t.Fatalf("route --once (gate off) exit = %d, stderr = %q", code, stderr)
	}
	var result DrainResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("route --json output is not JSON: %v (%q)", err, out)
	}
	// AC-007: the configuration itself is visible in the JSON.
	if result.SDDGate {
		t.Fatalf("json sdd_gate = true, want false (adbd_sdd_gate=off)")
	}
	if result.BeaconDocsRoot != DefaultBeaconDocsRoot {
		t.Fatalf("json beacon_docs_root = %q, want %q", result.BeaconDocsRoot, DefaultBeaconDocsRoot)
	}
	if len(result.Traces) != 1 {
		t.Fatalf("json traces = %+v, want exactly one row", result.Traces)
	}
	if rec := result.Traces[0]; rec.SDD != SDDLaneGatedOff || rec.Decision != DecisionRoute {
		t.Fatalf("gate-off trace = %+v, want decision=%s sdd=%s",
			rec, DecisionRoute, SDDLaneGatedOff)
	}
	if result.ReasonCode == ReasonTruthNotFrozen || result.ReasonCode == ReasonFeatureNotRegistered {
		t.Fatalf("gate off refused the item: reason_code=%q", result.ReasonCode)
	}
	if sends := off.blocksFor("send"); len(sends) != 1 {
		t.Fatalf("gate-off send calls = %d, want 1", len(sends))
	}
	// The gate is off, so no guard paragraph is claimed.
	if sends := off.blocksFor("send"); strings.Contains(sends[0].payload(), "beacon-go spine show") {
		t.Fatalf("gate-off dispatch = %q, want no SDD guard", sends[0].payload())
	}
}
