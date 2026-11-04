package main

// gh-bug-binding tests (v0.1.5): TC-GB-001 .. TC-GB-007.
//
// The gate this file exercises sits on top of the worker-sdd-binding gate: a
// GitHub issue is a bug report, not a requirement, so it has to declare which
// frozen acceptance criterion it violates. Everything below runs against the same
// temp ADB root and recording fake `paseo` the SDD tests use -- no real paseo, no
// network, no GitHub.
//
// Fixture shape:
//
//	<repo>/docs/beacon/v0.1.5/features/bug-baseline/truth.md
//	gh:acme/bug#<n>            (registry key "acme/bug" -> <repo>)
//	beacon:ac=bug-baseline#AC-001 / beacon:no-ac labels
//
// The one thing the gate must never do is guess: an issue that cannot say which
// criterion it breaks is refused and kept in the queue, so the fix has to go back
// through truth change -> refreeze before it can be dispatched.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

const (
	// bugVersion is the version directory the fixtures freeze their truth in.
	bugVersion = "v0.1.5"
	// bugSlug is the requirement package a bound issue points at.
	bugSlug = "bug-baseline"
	// bugRepoKey is the registry key of the fixture repository. It is also the
	// owner/repo of the gh thread, which is how thread-workdir resolves it.
	bugRepoKey = "acme/bug"
	// bugThread is one issue in that repository.
	bugThread = "gh:acme/bug#7"
	// bugAC is the criterion the package freezes by default.
	bugAC = "AC-001"
)

// bugLabel builds the binding label for one criterion.
func bugLabel(slug, acID string) string {
	return "beacon:ac=" + slug + "#" + acID
}

// writeBugTruth writes a frozen-or-not requirement package whose Acceptance
// Criteria table lists exactly acIDs.
func writeBugTruth(t *testing.T, repo, status string, acIDs ...string) string {
	t.Helper()
	dir := filepath.Join(repo, DefaultBeaconDocsRoot, bugVersion, beaconFeaturesDir, bugSlug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir feature package: %v", err)
	}
	var body strings.Builder
	body.WriteString("---\nslug: " + bugSlug + "\nversion: " + bugVersion + "\nstatus: " + status + "\n---\n\n")
	body.WriteString("# Requirement Truth: " + bugSlug + "\n\n## Acceptance Criteria\n\n")
	body.WriteString("| AC ID | Criterion |\n|-------|-----------|\n")
	for _, acID := range acIDs {
		fmt.Fprintf(&body, "| %s | shipped behaviour |\n", acID)
	}
	path := filepath.Join(dir, beaconTruthFile)
	if err := os.WriteFile(path, []byte(body.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// writeBugChange records the change a human is required to make before a refused
// bug can pass (AC-007). The router never reads it -- the gate only reads the
// refrozen package -- but the artifact is what the process is audited by, so the
// test writes one and asserts the version type is recorded.
func writeBugChange(t *testing.T, repo, versionType, note string) string {
	t.Helper()
	dir := filepath.Join(repo, DefaultBeaconDocsRoot, bugVersion, beaconFeaturesDir, bugSlug, "changes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir changes: %v", err)
	}
	body := "---\nversion_type: " + versionType + "\n---\n\n# Change\n\n" + note + "\n"
	path := filepath.Join(dir, "001-"+versionType+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write change file: %v", err)
	}
	return path
}

// bugConfig is the operator config the CLI runs with. binding is "on" or "off".
func bugConfig(binding string) string {
	return "adbd_allow_all=true\n" +
		"adbd_trusted_channels=skill,github\n" +
		"adbd_sdd_gate=on\n" +
		"adbd_beacon_docs_root=" + DefaultBeaconDocsRoot + "\n" +
		"adbd_bug_binding=" + binding + "\n"
}

// newBugHarness builds the SDD harness with the bug-binding switch set. The
// fake paseo records every invocation, so "zero calls" is literal evidence that a
// refusal never reached out of the process.
func newBugHarness(t *testing.T, gate, binding bool) *sddHarness {
	t.Helper()
	h := newSDDHarness(t, gate)
	h.router.BugBinding = binding
	return h
}

// bugIssue is one queued gh issue with the given labels.
func bugIssue(messageID string, labels ...string) QueueItem {
	return QueueItem{
		Channel:   DefaultGitHubChannel,
		Thread:    bugThread,
		MessageID: messageID,
		Text:      "#7 the export hangs on big files",
		Labels:    labels,
	}
}

// lastTraceFor returns the newest audit row for a message id, which is what the
// change -> refreeze tests need once an item has been decided twice.
func (h *sddHarness) lastTraceFor(messageID string) traceRecord {
	h.t.Helper()
	var found traceRecord
	seen := false
	for _, rec := range h.trace() {
		if rec.MessageID == messageID {
			found, seen = rec, true
		}
	}
	if !seen {
		h.t.Fatalf("no trace line for message %q (trace=%+v)", messageID, h.trace())
	}
	return found
}

// ---------------------------------------------------------------------------
// TC-GB-001 (AC-001): the label grammar
// ---------------------------------------------------------------------------

func TestParseBugBindingLabels(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		want   bugBinding
	}{
		{
			name:   "one well-formed ac label",
			labels: []string{"bug", bugLabel(bugSlug, bugAC)},
			want:   bugBinding{Bound: true, Slug: bugSlug, ACID: bugAC},
		},
		{
			name:   "surrounding whitespace and mixed case are tolerated",
			labels: []string{" Beacon:AC=" + bugSlug + "#" + bugAC + " "},
			want:   bugBinding{Bound: true, Slug: bugSlug, ACID: bugAC},
		},
		{
			name:   "no-ac is an explicit exemption",
			labels: []string{"chore", "beacon:no-ac"},
			want:   bugBinding{Exempt: true},
		},
		{
			name:   "no label at all is unbound",
			labels: nil,
			want:   bugBinding{},
		},
		{
			name:   "unrelated labels are unbound",
			labels: []string{"bug", "help wanted", "priority:high"},
			want:   bugBinding{},
		},
		{
			name:   "missing AC id is malformed",
			labels: []string{"beacon:ac=" + bugSlug + "#"},
			want:   bugBinding{},
		},
		{
			name:   "missing slug is malformed",
			labels: []string{"beacon:ac=#AC-001"},
			want:   bugBinding{},
		},
		{
			name:   "missing separator is malformed",
			labels: []string{"beacon:ac=" + bugSlug},
			want:   bugBinding{},
		},
		{
			name:   "two separators are malformed",
			labels: []string{"beacon:ac=" + bugSlug + "#AC-001#extra"},
			want:   bugBinding{},
		},
		{
			name:   "invalid characters are malformed",
			labels: []string{"beacon:ac=bug baseline#AC-001"},
			want:   bugBinding{},
		},
		{
			name:   "the ac keyword without a value is malformed",
			labels: []string{"beacon:ac"},
			want:   bugBinding{},
		},
		{
			name:   "two ac labels are ambiguous",
			labels: []string{bugLabel(bugSlug, "AC-001"), bugLabel(bugSlug, "AC-002")},
			want:   bugBinding{},
		},
		{
			name:   "a malformed label next to a good one is ambiguous",
			labels: []string{bugLabel(bugSlug, bugAC), "beacon:ac=" + bugSlug},
			want:   bugBinding{},
		},
		{
			name:   "an exemption next to a binding is ambiguous",
			labels: []string{"beacon:no-ac", bugLabel(bugSlug, bugAC)},
			want:   bugBinding{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseBugBindingLabels(tc.labels); got != tc.want {
				t.Fatalf("parseBugBindingLabels(%v) = %+v, want %+v", tc.labels, got, tc.want)
			}
		})
	}

	// ref() is the exact token the trace and the dispatch text carry.
	if got, want := (bugBinding{Bound: true, Slug: bugSlug, ACID: bugAC}).ref(), bugSlug+"#"+bugAC; got != want {
		t.Fatalf("ref() = %q, want %q", got, want)
	}
	if got := (bugBinding{Exempt: true}).ref(); got != "" {
		t.Fatalf("ref() of an exemption = %q, want empty", got)
	}

	// The AC token match is not a substring match: AC-001 must not be satisfied
	// by AC-0010 sitting in the package.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, beaconTruthFile),
		[]byte("| AC-0010 | shipped |\n"), 0o644); err != nil {
		t.Fatalf("write truth: %v", err)
	}
	if packageHasAC(dir, bugAC) {
		t.Fatal("packageHasAC matched AC-001 inside AC-0010: token matching is broken")
	}
	if err := os.WriteFile(filepath.Join(dir, beaconTruthFile),
		[]byte("| AC-001 | shipped |\n"), 0o644); err != nil {
		t.Fatalf("write truth: %v", err)
	}
	if !packageHasAC(dir, bugAC) {
		t.Fatal("packageHasAC did not find the AC-001 row")
	}
}

// ---------------------------------------------------------------------------
// TC-GB-002 (AC-002): a bound bug is dispatched against its frozen AC
// ---------------------------------------------------------------------------

func TestBoundBugDispatches(t *testing.T) {
	h := newBugHarness(t, true, true)
	repo := t.TempDir()
	writeBugTruth(t, repo, "frozen", bugAC, "AC-002")
	h.register(bugRepoKey, repo)
	const messageID = "m-bound"
	h.enqueue(bugIssue(messageID, "bug", bugLabel(bugSlug, bugAC)))

	res := h.drain()
	if res.Applied != 1 || res.Failed != 0 || res.Queued != 0 {
		t.Fatalf("drain = %+v, want applied=1 failed=0 queued=0", res)
	}

	// The trace states the lane and the claim.
	rec := h.traceFor(messageID)
	if rec.Decision != DecisionRoute {
		t.Fatalf("trace decision = %q, want %q (%+v)", rec.Decision, DecisionRoute, rec)
	}
	if rec.SDD != SDDLaneGated {
		t.Fatalf("trace sdd = %q, want %q", rec.SDD, SDDLaneGated)
	}
	const wantRef = bugSlug + "#" + bugAC
	if rec.ACRef != wantRef {
		t.Fatalf("trace ac_ref = %q, want %q", rec.ACRef, wantRef)
	}
	if rec.Binding != BindingOn {
		t.Fatalf("trace binding = %q, want %q", rec.Binding, BindingOn)
	}

	// The dispatched text carries the AC reference and the regression rule, after
	// the SDD guard and before the request itself.
	sends := h.blocksFor("send")
	if len(sends) != 1 {
		t.Fatalf("send calls = %d, want 1", len(sends))
	}
	text := sends[0].payload()
	if !strings.HasPrefix(text, sddGuard) {
		t.Fatalf("dispatched text = %q, want it to start with the SDD guard", text)
	}
	if !strings.Contains(text, wantRef) {
		t.Fatalf("dispatched text = %q, want the AC reference %q", text, wantRef)
	}
	if !strings.Contains(text, bugBindingClause) {
		t.Fatalf("dispatched text = %q, want the sentence %q", text, bugBindingClause)
	}
	if !strings.HasSuffix(text, "#7 the export hangs on big files") {
		t.Fatalf("dispatched text = %q, want the issue text kept after the binding block", text)
	}

	// The thread now owns the confirmed worker.
	if b := h.binding(DefaultGitHubChannel, bugThread); b == nil || b.AgentID != "ag-sdd-worker" {
		t.Fatalf("binding after dispatch = %+v, want the confirmed worker", b)
	}
}

// ---------------------------------------------------------------------------
// TC-GB-003 (AC-003): an unbound bug is refused and stays in the queue
// ---------------------------------------------------------------------------

func TestUnboundBugIsRefused(t *testing.T) {
	h := newBugHarness(t, true, true)
	repo := t.TempDir()
	writeBugTruth(t, repo, "frozen", bugAC)
	h.register(bugRepoKey, repo)
	h.writeConfig(bugConfig("on"))
	const messageID = "m-unbound"
	// A bug label is not a binding: the issue says what it is, not what it breaks.
	h.enqueue(bugIssue(messageID, "bug"))

	code, out, stderr := h.cli()
	if code != 0 {
		t.Fatalf("route --once exit = %d, want 0 (a refusal is not an error; stderr=%q)", code, stderr)
	}
	if !strings.Contains(out, "reason_code="+ReasonIssueUnbound) {
		t.Fatalf("route output = %q, want reason_code=%s", out, ReasonIssueUnbound)
	}

	// No worker was created, looked up or sent to.
	if got := h.invocations(); len(got) != 0 {
		t.Fatalf("paseo was called %v, want zero calls on an unbound issue", got)
	}
	// The item keeps its place in the queue and no binding is written.
	if q := h.queue(); len(q) != 1 || q[0].MessageID != messageID {
		t.Fatalf("queue after refusal = %+v, want the item kept", q)
	}
	if b := h.binding(DefaultGitHubChannel, bugThread); b != nil {
		t.Fatalf("binding after refusal = %+v, want none", b)
	}

	// The row says refuse, names the reason, and claims no AC.
	rec := h.traceFor(messageID)
	if rec.Decision != DecisionRefuse || rec.ReasonCode != ReasonIssueUnbound {
		t.Fatalf("refusal trace = %+v, want decision=%s reason_code=%s",
			rec, DecisionRefuse, ReasonIssueUnbound)
	}
	if rec.SDD != SDDLaneGated || rec.ACRef != "" {
		t.Fatalf("refusal trace = %+v, want sdd=%s and no ac_ref", rec, SDDLaneGated)
	}

	// A scheduler can branch on the same verdict from --outcome json.
	code, out, stderr = h.cli("--outcome", "json")
	if code != 0 {
		t.Fatalf("route --once --outcome json exit = %d, stderr = %q", code, stderr)
	}
	var result DrainResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("route --json output is not JSON: %v (%q)", err, out)
	}
	if result.ReasonCode != ReasonIssueUnbound {
		t.Fatalf("json reason_code = %q, want %q", result.ReasonCode, ReasonIssueUnbound)
	}
}

// ---------------------------------------------------------------------------
// TC-GB-004 (AC-004): beacon:no-ac is an exemption, not a refusal
// ---------------------------------------------------------------------------

func TestNoAcExemptionDispatches(t *testing.T) {
	h := newBugHarness(t, true, true)
	repo := t.TempDir()
	writeBugTruth(t, repo, "frozen", bugAC)
	h.register(bugRepoKey, repo)
	const messageID = "m-exempt"
	h.enqueue(bugIssue(messageID, "chore", "beacon:no-ac"))

	res := h.drain()
	if res.Applied != 1 || res.Failed != 0 {
		t.Fatalf("drain = %+v, want applied=1 failed=0", res)
	}

	rec := h.traceFor(messageID)
	if rec.Decision != DecisionRoute {
		t.Fatalf("trace decision = %q, want %q", rec.Decision, DecisionRoute)
	}
	if rec.SDD != SDDLaneGuardOnly {
		t.Fatalf("trace sdd = %q, want %q", rec.SDD, SDDLaneGuardOnly)
	}
	if rec.ReasonCode != ReasonIssueExemptNoAC {
		t.Fatalf("trace reason_code = %q, want %q", rec.ReasonCode, ReasonIssueExemptNoAC)
	}
	if rec.Binding != BindingOn {
		t.Fatalf("trace binding = %q, want %q", rec.Binding, BindingOn)
	}
	if rec.ACRef != "" {
		t.Fatalf("trace ac_ref = %q, want none on an exemption", rec.ACRef)
	}

	// It is dispatched as a normal, guarded request -- no AC block invented.
	sends := h.blocksFor("send")
	if len(sends) != 1 {
		t.Fatalf("send calls = %d, want 1", len(sends))
	}
	text := sends[0].payload()
	if !strings.HasPrefix(text, sddGuard) {
		t.Fatalf("exempt text = %q, want the SDD guard paragraph", text)
	}
	if strings.Contains(text, bugBindingClause) {
		t.Fatalf("exempt text = %q, want no AC binding block", text)
	}
}

// ---------------------------------------------------------------------------
// TC-GB-005 (AC-005): a bound bug still needs a frozen truth
// ---------------------------------------------------------------------------

func TestBoundBugNeedsFrozenTruth(t *testing.T) {
	h := newBugHarness(t, true, true)
	repo := t.TempDir()
	writeBugTruth(t, repo, "draft", bugAC)
	h.register(bugRepoKey, repo)
	h.writeConfig(bugConfig("on"))
	const messageID = "m-unfrozen"
	h.enqueue(bugIssue(messageID, bugLabel(bugSlug, bugAC)))

	code, out, stderr := h.cli()
	if code != 0 {
		t.Fatalf("route --once exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	if !strings.Contains(out, "reason_code="+ReasonTruthNotFrozen) {
		t.Fatalf("route output = %q, want reason_code=%s", out, ReasonTruthNotFrozen)
	}
	if got := h.invocations(); len(got) != 0 {
		t.Fatalf("paseo was called %v, want zero calls on truth_not_frozen", got)
	}
	if q := h.queue(); len(q) != 1 {
		t.Fatalf("queue after refusal = %+v, want the item kept", q)
	}

	rec := h.traceFor(messageID)
	if rec.Decision != DecisionRefuse || rec.ReasonCode != ReasonTruthNotFrozen {
		t.Fatalf("refusal trace = %+v, want decision=%s reason_code=%s",
			rec, DecisionRefuse, ReasonTruthNotFrozen)
	}
	// AC-007: the row keeps the claim next to the verdict, for audit.
	if rec.ACRef != bugSlug+"#"+bugAC {
		t.Fatalf("refusal trace ac_ref = %q, want the declared reference kept", rec.ACRef)
	}
}

// ---------------------------------------------------------------------------
// TC-GB-006 (AC-006): a frozen package that does not contain the AC refuses
// ---------------------------------------------------------------------------

func TestBoundBugMissingACMismatch(t *testing.T) {
	h := newBugHarness(t, true, true)
	repo := t.TempDir()
	writeBugTruth(t, repo, "frozen", bugAC)
	h.register(bugRepoKey, repo)
	h.writeConfig(bugConfig("on"))
	const messageID = "m-missing-ac"
	h.enqueue(bugIssue(messageID, bugLabel(bugSlug, "AC-999")))

	code, out, stderr := h.cli()
	if code != 0 {
		t.Fatalf("route --once exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	if !strings.Contains(out, "reason_code="+ReasonACNotFound) {
		t.Fatalf("route output = %q, want reason_code=%s", out, ReasonACNotFound)
	}
	if got := h.invocations(); len(got) != 0 {
		t.Fatalf("paseo was called %v, want zero calls on ac_not_found", got)
	}
	if q := h.queue(); len(q) != 1 {
		t.Fatalf("queue after refusal = %+v, want the item kept", q)
	}
	if b := h.binding(DefaultGitHubChannel, bugThread); b != nil {
		t.Fatalf("binding after refusal = %+v, want none", b)
	}
	rec := h.traceFor(messageID)
	if rec.Decision != DecisionRefuse || rec.ReasonCode != ReasonACNotFound {
		t.Fatalf("refusal trace = %+v, want decision=%s reason_code=%s",
			rec, DecisionRefuse, ReasonACNotFound)
	}
	if rec.ACRef != bugSlug+"#AC-999" {
		t.Fatalf("refusal trace ac_ref = %q, want the stale claim kept for audit", rec.ACRef)
	}
}

// ---------------------------------------------------------------------------
// TC-GB-007 (AC-007): the switch, and the change -> refreeze path
// ---------------------------------------------------------------------------

func TestBindingGateCanBeDisabled(t *testing.T) {
	// Config: on by default, off only on an explicit token, and a typo keeps it on.
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig(\"\") error: %v", err)
	}
	if !cfg.BugBinding {
		t.Fatalf("default config bug_binding = %v, want true", cfg.BugBinding)
	}
	configPath := filepath.Join(t.TempDir(), "config")
	load := func(body string) Config {
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
	if cfg := load("adbd_bug_binding=on\n"); !cfg.BugBinding {
		t.Fatalf("adbd_bug_binding=on -> %v, want true", cfg.BugBinding)
	}
	if cfg := load("adbd_bug_binding=off\n"); cfg.BugBinding {
		t.Fatalf("adbd_bug_binding=off -> %v, want false", cfg.BugBinding)
	}
	if cfg := load("adbd_bug_binding=maybe\n"); !cfg.BugBinding {
		t.Fatalf("adbd_bug_binding=maybe -> %v, want the gate to stay on", cfg.BugBinding)
	}

	// Switch off: the unbound issue is dispatched anyway, as guard_only, and the
	// row records binding_off so a disabled gate never looks like a passing one.
	off := newBugHarness(t, true, false)
	offRepo := t.TempDir()
	writeBugTruth(t, offRepo, "frozen", bugAC)
	off.register(bugRepoKey, offRepo)
	off.writeConfig(bugConfig("off"))
	const messageID = "m-binding-off"
	off.enqueue(bugIssue(messageID, "bug")) // no binding label at all

	code, out, stderr := off.cli("--outcome", "json")
	if code != 0 {
		t.Fatalf("route --once (binding off) exit = %d, stderr = %q", code, stderr)
	}
	var result DrainResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("route --json output is not JSON: %v (%q)", err, out)
	}
	if result.BugBinding {
		t.Fatal("json bug_binding = true, want false (adbd_bug_binding=off)")
	}
	if len(result.Traces) != 1 {
		t.Fatalf("json traces = %+v, want exactly one row", result.Traces)
	}
	rec := result.Traces[0]
	if rec.SDD != SDDLaneGuardOnly || rec.Decision != DecisionRoute {
		t.Fatalf("binding-off trace = %+v, want decision=%s sdd=%s", rec, DecisionRoute, SDDLaneGuardOnly)
	}
	if rec.Binding != BindingOff {
		t.Fatalf("binding-off trace binding = %q, want %q", rec.Binding, BindingOff)
	}
	if result.ReasonCode == ReasonIssueUnbound {
		t.Fatalf("binding off still refused the issue: reason_code=%q", result.ReasonCode)
	}
	if sends := off.blocksFor("send"); len(sends) != 1 {
		t.Fatalf("binding-off send calls = %d, want 1", len(sends))
	}

	// Switch on: the refusal is undone only by change -> refreeze.
	on := newBugHarness(t, true, true)
	onRepo := t.TempDir()
	writeBugTruth(t, onRepo, "frozen", bugAC) // frozen, but AC-009 is not there yet
	on.register(bugRepoKey, onRepo)
	const refreezeID = "m-refreeze"
	on.enqueue(bugIssue(refreezeID, bugLabel(bugSlug, "AC-009")))

	if res := on.drain(); res.Applied != 0 || res.Failed != 1 {
		t.Fatalf("pre-change drain = %+v, want applied=0 failed=1", res)
	}
	refused := on.traceFor(refreezeID)
	if refused.ReasonCode != ReasonACNotFound || refused.ACRef != bugSlug+"#AC-009" {
		t.Fatalf("pre-change trace = %+v, want reason_code=%s and ac_ref kept",
			refused, ReasonACNotFound)
	}
	if got := on.blocksFor("send"); len(got) != 0 {
		t.Fatalf("send calls before the refreeze = %d, want 0", len(got))
	}

	// A human adds the criterion through truth change (adding an AC is a minor
	// change) and refreezes the package.
	changePath := writeBugChange(t, onRepo, "minor", "add AC-009 for the export hang")
	if data, err := os.ReadFile(changePath); err != nil {
		t.Fatalf("read change file: %v", err)
	} else if !strings.Contains(string(data), "version_type: minor") {
		t.Fatalf("change file = %q, want the version type recorded", data)
	}
	writeBugTruth(t, onRepo, "frozen", bugAC, "AC-009")

	res := on.drain()
	if res.Applied != 1 || res.Failed != 0 || res.Queued != 0 {
		t.Fatalf("post-refreeze drain = %+v, want applied=1 failed=0 queued=0", res)
	}
	dispatched := on.lastTraceFor(refreezeID)
	if dispatched.Decision != DecisionRoute || dispatched.SDD != SDDLaneGated {
		t.Fatalf("post-refreeze trace = %+v, want decision=%s sdd=%s",
			dispatched, DecisionRoute, SDDLaneGated)
	}
	if dispatched.ACRef != bugSlug+"#AC-009" {
		t.Fatalf("post-refreeze ac_ref = %q, want %q", dispatched.ACRef, bugSlug+"#AC-009")
	}
	// AC-007: both the refusal's reason and the dispatch's ac_ref survive in the
	// audit trail, so the whole sequence is explainable from the trace alone.
	reasons := []string{}
	refs := []string{}
	for _, row := range on.trace() {
		if row.MessageID != refreezeID {
			continue
		}
		reasons = append(reasons, row.ReasonCode)
		refs = append(refs, row.ACRef)
	}
	if len(reasons) != 2 {
		t.Fatalf("trace rows for the refrozen issue = %d, want 2 (%v)", len(reasons), reasons)
	}
	if reasons[0] != ReasonACNotFound || refs[0] != bugSlug+"#AC-009" ||
		refs[1] != bugSlug+"#AC-009" {
		t.Fatalf("audit sequence = reasons %v refs %v, want the refusal and the dispatch both explained",
			reasons, refs)
	}
	if sends := on.blocksFor("send"); len(sends) != 1 {
		t.Fatalf("send calls after the refreeze = %d, want 1", len(sends))
	}
}
