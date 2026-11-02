package main

// Reply delivery tests (feature reply-delivery, v0.1.5): TC-RD-001 .. TC-RD-006.
//
// Every test here drives a REAL shell script as the sender binary, so the
// production seam (exec.Command inside LarkReplier) is exercised end to end --
// argument order included -- without ever touching lark-cli or the network. The
// script records its argv into a log file and can be told to fail with a given
// exit code and stderr, which is exactly what a refused send looks like.
//
// Everything else runs against a temp dir and the stub dispatcher from
// route_test.go: no paseo binary, no network, no shared state.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// fakeLarkCallMarker separates one invocation from the next in the argv log.
const fakeLarkCallMarker = "==CALL=="

// fakeLark is a stand-in lark-cli: a real executable that logs its argv and
// exits with the configured code, writing the configured text to stderr.
type fakeLark struct {
	bin    string
	log    string
	stderr string
	exit   int
}

// newFakeLark writes the script into a temp dir and returns the fake.
func newFakeLark(t *testing.T, exitCode int, stderr string) *fakeLark {
	t.Helper()
	dir := t.TempDir()
	f := &fakeLark{
		bin:    filepath.Join(dir, "lark-cli"),
		log:    filepath.Join(dir, "lark-args.log"),
		stderr: stderr,
		exit:   exitCode,
	}
	script := "#!/bin/sh\n" +
		"echo " + shellQuote(fakeLarkCallMarker) + " >> " + shellQuote(f.log) + "\n" +
		"for arg in \"$@\"; do printf '%s\\n' \"$arg\" >> " + shellQuote(f.log) + "; done\n" +
		"printf '%s' " + shellQuote(f.stderr) + " >&2\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(f.bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lark binary: %v", err)
	}
	return f
}

// calls returns one argv slice per invocation, in order.
func (f *fakeLark) calls(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile(f.log)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("read fake lark log: %v", err)
	}
	body := strings.TrimSuffix(string(data), "\n")
	if body == "" {
		return nil
	}
	var out [][]string
	var current []string
	for _, line := range strings.Split(body, "\n") {
		if line == fakeLarkCallMarker {
			if current != nil {
				out = append(out, current)
			}
			current = []string{}
			continue
		}
		current = append(current, line)
	}
	if current != nil {
		out = append(out, current)
	}
	return out
}

// shellQuote (fastpath_test.go) wraps a temp path or a message in single quotes
// so the shell can never reinterpret it.

// flagValue reads the value that follows one flag in an argv slice.
func flagValue(t *testing.T, argv []string, flag string) string {
	t.Helper()
	for i, arg := range argv {
		if arg == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	t.Fatalf("argv %#v carries no %s", argv, flag)
	return ""
}

// traceFields returns the raw JSON keys of every trace line, indexed by message id.
func traceFields(t *testing.T, path string) map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	out := map[string]map[string]any{}
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := map[string]any{}
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("trace line %d is not JSON: %v", i+1, err)
		}
		id, _ := fields["message_id"].(string)
		out[id] = fields
	}
	return out
}

// jsonKeys returns the sorted key set of one trace line.
func jsonKeys(fields map[string]any) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// replyRouter builds a router whose mouth is the fake lark binary.
func replyRouter(t *testing.T, fake *fakeLark, identity string) *routeHarness {
	t.Helper()
	h := newRouteHarness(t, RuleDecider{})
	h.router.Replier = LarkReplier{Bin: fake.bin, Identity: identity}
	h.router.ReplyChannel = DefaultReplyChannel
	return h
}

// writeReplyConfig points the CLI at the fake sender, optionally naming the
// identity (an empty identity exercises the "bot" default).
func (h *routeHarness) writeReplyConfig(bin, identity string) {
	h.t.Helper()
	lines := []string{"adbd_allow_all=true", "adbd_reply_bin=" + bin}
	if identity != "" {
		lines = append(lines, "adbd_reply_identity="+identity)
	}
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0o755); err != nil {
		h.t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(h.configPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TC-RD-001 (AC-001): a feishu reply really leaves through lark-cli
// ---------------------------------------------------------------------------

func TestReplyDeliversOnFeishu(t *testing.T) {
	// A successful send exits 0 and says nothing on stderr.
	fake := newFakeLark(t, 0, "")
	h := newRouteHarness(t, RuleDecider{})
	h.writeReplyConfig(fake.bin, "") // no identity -> the "bot" default

	const (
		thread = "feishu:oc_reply_001"
		msgID  = "m-reply-001"
		text   = "is it ready?"
	)
	h.enqueue(QueueItem{Channel: "feishu", Thread: thread, MessageID: msgID, Text: text})

	code, stdout, stderr := h.cli(h.routeArgs()...)
	if code != 0 {
		t.Fatalf("route exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "applied=1") {
		t.Fatalf("route stdout = %q, want applied=1", stdout)
	}

	calls := fake.calls(t)
	if len(calls) != 1 {
		t.Fatalf("sender was called %d time(s), want exactly 1 (%v)", len(calls), calls)
	}
	// AC-001: the exact command, in the exact order.
	assertArgv(t, calls[0], []string{
		"im", "+messages-send",
		// AC-001: the chat id is the thread without its channel prefix.
		"--chat-id", strings.TrimPrefix(thread, "feishu:"),
		"--text", text,
		"--as", DefaultReplyIdentity,
		"--idempotency-key", msgID,
	})

	rec := h.traceFor(msgID)
	if rec.Decision != DecisionReply || rec.ReasonCode != ReasonQuestion {
		t.Fatalf("trace = %+v, want reply/question", rec)
	}
	if rec.Delivered == nil || !*rec.Delivered || rec.DeliveryReason != DeliverySent {
		t.Fatalf("trace delivery = %v/%q, want true/sent", rec.Delivered, rec.DeliveryReason)
	}
	if rec.DeliveryDetail != "" {
		t.Fatalf("a successful send recorded delivery_detail=%q, want empty", rec.DeliveryDetail)
	}
}

// ---------------------------------------------------------------------------
// TC-RD-002 (AC-002): the trace gains two fields on reply rows, nothing else
// ---------------------------------------------------------------------------

func TestReplyTraceFields(t *testing.T) {
	fake := newFakeLark(t, 0, "")
	h := replyRouter(t, fake, "bot")
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "feishu:oc_reply", MessageID: "m-reply", Text: "any news?"},
		QueueItem{Channel: "feishu", Thread: "feishu:oc_route", MessageID: "m-route", Text: "add a health endpoint"},
	)
	res := h.drain()
	if res.Applied != 2 {
		t.Fatalf("drain = %+v, want applied=2", res)
	}

	fields := traceFields(t, h.tracePath)
	replyKeys := jsonKeys(fields["m-reply"])
	wantReply := []string{
		"batch_size", "channel", "decided_at", "decision",
		"delivered", "delivery_reason", "message_id", "reason_code",
		"thread", "untrusted",
	}
	if strings.Join(replyKeys, ",") != strings.Join(wantReply, ",") {
		t.Fatalf("reply trace keys = %v, want exactly %v", replyKeys, wantReply)
	}

	// AC-002 "其余字段不变": a verdict with no outbound step keeps the frozen
	// eight-key shape (route.go AC-004), so old expectations cannot drift.
	wantRoute := []string{
		"batch_size", "channel", "decided_at", "decision",
		"message_id", "reason_code", "thread", "untrusted",
	}
	if got := jsonKeys(fields["m-route"]); strings.Join(got, ",") != strings.Join(wantRoute, ",") {
		t.Fatalf("route trace keys = %v, want exactly %v", got, wantRoute)
	}

	delivered, ok := fields["m-reply"]["delivered"].(bool)
	if !ok || !delivered {
		t.Fatalf("reply delivered = %#v, want true", fields["m-reply"]["delivered"])
	}
	if reason, _ := fields["m-reply"]["delivery_reason"].(string); reason != DeliverySent {
		t.Fatalf("reply delivery_reason = %#v, want %q", fields["m-reply"]["delivery_reason"], DeliverySent)
	}
}

// ---------------------------------------------------------------------------
// TC-RD-003 (AC-003): a failed send does not undo the decision
// ---------------------------------------------------------------------------

func TestReplySendFailureKeepsDecision(t *testing.T) {
	// 300 characters of stderr: more than the 200-character cap on the trace.
	noisy := strings.Repeat("boom-", 60)
	fake := newFakeLark(t, 1, noisy)
	h := newRouteHarness(t, RuleDecider{})
	h.writeReplyConfig(fake.bin, "user")

	const msgID = "m-fail-003"
	h.enqueue(QueueItem{Channel: "feishu", Thread: "feishu:oc_fail", MessageID: msgID, Text: "still there?"})

	code, stdout, stderr := h.cli(h.routeArgs()...)
	// AC-003: the process exit code is untouched by a refused send.
	if code != 0 {
		t.Fatalf("route exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "applied=1") || !strings.Contains(stdout, "failed=0") {
		t.Fatalf("route stdout = %q, want applied=1 failed=0", stdout)
	}
	// The configured identity is what the sender was asked to speak as.
	calls := fake.calls(t)
	if len(calls) != 1 {
		t.Fatalf("sender called %d time(s), want 1 (%v)", len(calls), calls)
	}
	if got := flagValue(t, calls[0], "--as"); got != "user" {
		t.Fatalf("--as = %q, want user (adbd_reply_identity)", got)
	}

	rec := h.traceFor(msgID)
	if rec.Decision != DecisionReply {
		t.Fatalf("trace decision = %q, want reply (a failed send never rewrites the verdict)", rec.Decision)
	}
	if rec.Delivered == nil || *rec.Delivered {
		t.Fatalf("trace delivered = %v, want false", rec.Delivered)
	}
	if rec.DeliveryReason != DeliverySendFailed {
		t.Fatalf("trace delivery_reason = %q, want %q", rec.DeliveryReason, DeliverySendFailed)
	}
	if got := utf8.RuneCountInString(rec.DeliveryDetail); got != maxDeliveryDetailRunes {
		t.Fatalf("delivery_detail is %d characters, want %d (%q)", got, maxDeliveryDetailRunes, rec.DeliveryDetail)
	}
	if !strings.HasPrefix(rec.DeliveryDetail, "boom-boom-") {
		t.Fatalf("delivery_detail = %q, want the sender's stderr", rec.DeliveryDetail)
	}
	// AC-003: applied, not retried. The item left the queue with no attempt spent.
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("failed send left the item queued: %+v", q)
	}
	if got := len(h.trace()); got != 1 {
		t.Fatalf("trace lines = %d, want 1 (no retry line)", got)
	}
}

// ---------------------------------------------------------------------------
// TC-RD-004 (AC-004): the same message_id always carries the same key
// ---------------------------------------------------------------------------

func TestReplyIdempotencyKeyStable(t *testing.T) {
	fake := newFakeLark(t, 0, "")
	h := replyRouter(t, fake, "bot")

	const (
		thread = "feishu:oc_dup"
		msgID  = "m-dup-004"
	)
	item := QueueItem{Channel: "feishu", Thread: thread, MessageID: msgID, Text: "one more time?"}

	h.enqueue(item)
	h.drain()

	// Simulate the crash the key exists for: the send happened, the audit line was
	// lost, so the item is replayed and sent a second time.
	if err := os.Remove(h.tracePath); err != nil {
		t.Fatalf("remove trace to replay the item: %v", err)
	}
	h.restoreQueue(item)
	h.drain()

	calls := fake.calls(t)
	if len(calls) != 2 {
		t.Fatalf("sender called %d time(s), want 2 (%v)", len(calls), calls)
	}
	first := flagValue(t, calls[0], "--idempotency-key")
	second := flagValue(t, calls[1], "--idempotency-key")
	if first != second {
		t.Fatalf("idempotency keys differ: %q != %q", first, second)
	}
	if first != msgID {
		t.Fatalf("idempotency key = %q, want the queue message_id %q", first, msgID)
	}
}

// ---------------------------------------------------------------------------
// TC-RD-005 (AC-005): non-feishu threads are never sent to
// ---------------------------------------------------------------------------

func TestReplySkipsNonFeishu(t *testing.T) {
	fake := newFakeLark(t, 0, "")
	h := replyRouter(t, fake, "bot")
	h.enqueue(
		QueueItem{Channel: "skill", Thread: "skill:adbd", MessageID: "m-skill-005", Text: "are you there?"},
		QueueItem{Channel: "github", Thread: "gh:owner/repo#1", MessageID: "m-gh-005", Text: "is it merged?"},
	)
	res := h.drain()
	if res.Applied != 2 || res.Failed != 0 {
		t.Fatalf("drain = %+v, want applied=2 failed=0", res)
	}
	if calls := fake.calls(t); len(calls) != 0 {
		t.Fatalf("sender was called for a non-feishu thread: %v", calls)
	}
	for _, msgID := range []string{"m-skill-005", "m-gh-005"} {
		rec := h.traceFor(msgID)
		if rec.Decision != DecisionReply {
			t.Fatalf("%s decision = %q, want reply", msgID, rec.Decision)
		}
		if rec.Delivered == nil || *rec.Delivered {
			t.Fatalf("%s delivered = %v, want false", msgID, rec.Delivered)
		}
		if rec.DeliveryReason != DeliveryUnsupportedChannel {
			t.Fatalf("%s delivery_reason = %q, want %q", msgID, rec.DeliveryReason, DeliveryUnsupportedChannel)
		}
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("unsupported-channel items stayed queued: %+v", q)
	}
}

// ---------------------------------------------------------------------------
// TC-RD-006 (AC-006): with no sender configured nothing is sent, and the
// verdict is still recorded exactly as before
// ---------------------------------------------------------------------------

func TestReplyDisabledByDefault(t *testing.T) {
	// ① The CLI with a config that never mentions a sender: the replier stays nil.
	h := newRouteHarness(t, RuleDecider{})
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(h.configPath, []byte("adbd_allow_all=true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	const msgID = "m-off-006"
	h.enqueue(QueueItem{Channel: "feishu", Thread: "feishu:oc_off", MessageID: msgID, Text: "is it done?"})

	code, stdout, stderr := h.cli(h.routeArgs()...)
	if code != 0 {
		t.Fatalf("route exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	rec := h.traceFor(msgID)
	if rec.Decision != DecisionReply || rec.ReasonCode != ReasonQuestion {
		t.Fatalf("trace = %+v, want reply/question (behaviour unchanged)", rec)
	}
	if rec.Delivered == nil || *rec.Delivered {
		t.Fatalf("delivered = %v, want false", rec.Delivered)
	}
	if rec.DeliveryReason != DeliveryNotConfigured {
		t.Fatalf("delivery_reason = %q, want %q", rec.DeliveryReason, DeliveryNotConfigured)
	}
	if rec.DeliveryDetail != "" {
		t.Fatalf("delivery_detail = %q, want empty when nothing was attempted", rec.DeliveryDetail)
	}

	// ② The router itself: a nil Replier is the default and spawns nothing.
	plain := newRouteHarness(t, RuleDecider{})
	plain.enqueue(QueueItem{Channel: "feishu", Thread: "feishu:oc_off2", MessageID: "m-off-006b", Text: "hello?"})
	if res := plain.drain(); res.Applied != 1 {
		t.Fatalf("drain = %+v, want applied=1", res)
	}
	if got := plain.traceFor("m-off-006b").DeliveryReason; got != DeliveryNotConfigured {
		t.Fatalf("nil replier delivery_reason = %q, want %q", got, DeliveryNotConfigured)
	}
}

// ---------------------------------------------------------------------------
// config plumbing for the two new keys (AC-001 / AC-006)
// ---------------------------------------------------------------------------

func TestReplyConfigDefaults(t *testing.T) {
	h := newRouteHarness(t, RuleDecider{})
	// No reply keys at all: delivery stays off and the identity keeps its default.
	cfg, err := LoadConfig(h.configPath)
	if err != nil {
		t.Fatalf("LoadConfig (missing file): %v", err)
	}
	if cfg.ReplyBin != "" || cfg.ReplyIdentity != DefaultReplyIdentity {
		t.Fatalf("defaults = bin=%q identity=%q, want empty/%q", cfg.ReplyBin, cfg.ReplyIdentity, DefaultReplyIdentity)
	}

	h.writeReplyConfig("/opt/fake/lark-cli", "user")
	cfg, err = LoadConfig(h.configPath)
	if err != nil {
		t.Fatalf("LoadConfig (configured): %v", err)
	}
	if cfg.ReplyBin != "/opt/fake/lark-cli" || cfg.ReplyIdentity != "user" {
		t.Fatalf("configured = bin=%q identity=%q, want the written values", cfg.ReplyBin, cfg.ReplyIdentity)
	}
}

// feishuThread is the frozen channel prefix check (AC-005); pin it directly so a
// future refactor of the delivery gate cannot widen it silently.
func TestFeishuThreadPrefix(t *testing.T) {
	for _, tc := range []struct {
		thread string
		want   bool
	}{
		{"feishu:oc_123", true},
		{" feishu:oc_123 ", true},
		{"skill:adbd", false},
		{"gh:owner/repo#1", false},
		{"feishu", false},
		{"", false},
	} {
		if got := feishuThread(tc.thread); got != tc.want {
			t.Fatalf("feishuThread(%q) = %v, want %v", tc.thread, got, tc.want)
		}
	}
}
