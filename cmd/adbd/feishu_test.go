package main

// Feishu inbound mapping tests (feature feishu-inbound, v0.1.5):
// TC-FI-001 .. TC-FI-005.
//
// Every test drives the real CLI (`adbd feishu-feed`) against a temp root with a
// stub Dispatcher and an injected clock, and hands the command a fixture NDJSON
// stream instead of a terminal. No network, no lark-cli, no paseo binary: a
// failure here is always a mapping failure, never an environment one.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// harness: temp root + stub runner + injected clock + fixture stream
// ---------------------------------------------------------------------------

type feishuHarness struct {
	t          *testing.T
	root       string
	beacon     string
	statePath  string
	slowPath   string
	configPath string
	clock      *fakeClock
	runner     *stubRunner
	// stdout/stderr of the last feed call.
	stdout string
	stderr string
}

func newFeishuHarness(t *testing.T) *feishuHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatalf("mkdir .beacon: %v", err)
	}
	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h := &feishuHarness{
		t:          t,
		root:       root,
		beacon:     beacon,
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		configPath: filepath.Join(beacon, "config"),
		clock:      &fakeClock{t: start},
		runner:     &stubRunner{},
	}

	// Stub Dispatcher + injected clock: intake hands text to the stub, never to
	// paseo, and the dedupe/lease timestamps are exact.
	prevCoreFor := feishuCoreFor
	feishuCoreFor = func(cf *commonFlags) (*Core, error) {
		core, err := cf.core()
		if err != nil {
			return nil, err
		}
		core.Runner = h.runner
		core.Now = h.clock.now
		return core, nil
	}
	t.Cleanup(func() { feishuCoreFor = prevCoreFor })

	// The stdin seam: the CLI reads this fixture instead of a terminal.
	prevStdin := feishuStdin
	t.Cleanup(func() { feishuStdin = prevStdin })
	return h
}

// feed runs `adbd feishu-feed --root <root>` with the fixture on stdin and
// returns the exit code. stdout/stderr are kept on the harness.
func (h *feishuHarness) feed(fixture string, extra ...string) int {
	h.t.Helper()
	if err := os.MkdirAll(h.beacon, 0o755); err != nil {
		h.t.Fatalf("mkdir .beacon: %v", err)
	}
	feishuStdin = strings.NewReader(fixture)
	var out, errOut bytes.Buffer
	argv := append([]string{"feishu-feed", "--root", h.root}, extra...)
	code := run(argv, &out, &errOut)
	h.stdout, h.stderr = out.String(), errOut.String()
	return code
}

// bind creates a live binding through the public API with the injected clock, so
// the next event on that thread takes the fastpath.
func (h *feishuHarness) bind(channel, thread, agentID string) {
	h.t.Helper()
	core := NewCore(&Store{Path: h.statePath, SlowPath: h.slowPath}, nil)
	core.Now = h.clock.now
	if _, err := core.Bind(channel, thread, agentID, DefaultTTL); err != nil {
		h.t.Fatalf("bind %s/%s: %v", channel, thread, err)
	}
}

// writeConfig writes .beacon/config for the command to read.
func (h *feishuHarness) writeConfig(body string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0o755); err != nil {
		h.t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(h.configPath, []byte(body), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

// slowPaths returns the slow-path queue as records.
func (h *feishuHarness) slowPaths() []slowPathRecord {
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

// summary parses the LAST printed line of the text outcome, which is the one the
// contract freezes: accepted=<n> skipped=<m> deduped=<k>.
func (h *feishuHarness) summary() map[string]string {
	h.t.Helper()
	lines := strings.Split(strings.TrimRight(h.stdout, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		h.t.Fatalf("no output to parse (stderr: %s)", h.stderr)
	}
	return kvFields(h.t, lines[len(lines)-1])
}

// kvFields splits one key=value line into a map.
func kvFields(t *testing.T, line string) map[string]string {
	t.Helper()
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

// summaryCount reads one integer count out of the summary line.
func (h *feishuHarness) summaryCount(key string) int {
	h.t.Helper()
	fields := h.summary()
	raw, ok := fields[key]
	if !ok {
		h.t.Fatalf("summary line %q has no %s= field", strings.TrimSpace(h.stdout), key)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		h.t.Fatalf("%s=%q is not an integer: %v", key, raw, err)
	}
	return value
}

// feishuEvent builds one NDJSON event line.
func feishuEvent(chatID, messageID, content, messageType, senderType string) string {
	payload := map[string]string{
		"chat_id":      chatID,
		"message_id":   messageID,
		"content":      content,
		"message_type": messageType,
		"sender_type":  senderType,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// ndjson joins lines into one stream, newline-terminated like lark-cli prints it.
func ndjson(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// ---------------------------------------------------------------------------
// TC-FI-001 (AC-001): one text event becomes one threaded intake event.
// ---------------------------------------------------------------------------

func TestFeishuFeedMapsText(t *testing.T) {
	t.Run("unbound thread takes the slow path with the frozen identity", func(t *testing.T) {
		h := newFeishuHarness(t)
		h.writeConfig("adbd_allow_channels=feishu\n")

		code := h.feed(ndjson(feishuEvent("oc_x", "om_1", "你好，帮我看看", "text", "user")))
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, h.stderr)
		}
		if got := h.summaryCount("accepted"); got != 1 {
			t.Fatalf("accepted = %d, want 1 (output: %s)", got, h.stdout)
		}
		if got := h.summaryCount("skipped"); got != 0 {
			t.Fatalf("skipped = %d, want 0 (output: %s)", got, h.stdout)
		}
		if got := h.summaryCount("deduped"); got != 0 {
			t.Fatalf("deduped = %d, want 0 (output: %s)", got, h.stdout)
		}

		// The mapping itself, as the queue holds it: channel/thread/message-id/text.
		recs := h.slowPaths()
		if len(recs) != 1 {
			t.Fatalf("slow-path records = %d, want 1", len(recs))
		}
		rec := recs[0]
		if rec.Channel != DefaultFeishuChannel {
			t.Fatalf("channel = %q, want %q", rec.Channel, DefaultFeishuChannel)
		}
		if rec.Thread != "feishu:oc_x" {
			t.Fatalf("thread = %q, want feishu:oc_x", rec.Thread)
		}
		if rec.MessageID != "om_1" {
			t.Fatalf("message_id = %q, want om_1", rec.MessageID)
		}
		if !strings.Contains(rec.Text, "你好，帮我看看") {
			t.Fatalf("text = %q, want the event content", rec.Text)
		}
		// Inbound chat text is untrusted data, so the guard line is on it before
		// intake sees it (the same boundary `feed` and `poll-github` use).
		if !strings.HasPrefix(rec.Text, UntrustedGuardLine) {
			t.Fatalf("text = %q, want the %s guard prefix", rec.Text, UntrustedGuardLine)
		}
	})

	t.Run("bound thread takes the fastpath and hands text to the worker", func(t *testing.T) {
		h := newFeishuHarness(t)
		h.bind(DefaultFeishuChannel, "feishu:oc_x", "ag-feishu")

		code := h.feed(ndjson(feishuEvent("oc_x", "om_2", "继续", "text", "user")))
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, h.stderr)
		}
		if got := h.summaryCount("accepted"); got != 1 {
			t.Fatalf("accepted = %d, want 1 (output: %s)", got, h.stdout)
		}
		if len(h.runner.calls) != 1 {
			t.Fatalf("dispatches = %d, want 1", len(h.runner.calls))
		}
		if got := h.runner.calls[0].agentID; got != "ag-feishu" {
			t.Fatalf("dispatched to %q, want ag-feishu", got)
		}
		if !strings.Contains(h.runner.calls[0].text, "继续") {
			t.Fatalf("dispatched text = %q, want the event content", h.runner.calls[0].text)
		}
		if len(h.slowPaths()) != 0 {
			t.Fatalf("a fastpath delivery must not queue anything, got %d record(s)", len(h.slowPaths()))
		}
	})

	t.Run("json outcome carries the same counts", func(t *testing.T) {
		h := newFeishuHarness(t)
		code := h.feed(ndjson(
			feishuEvent("oc_x", "om_3", "hello", "text", "user"),
			feishuEvent("oc_y", "om_4", "world", "text", "user"),
		), "--outcome", "json")
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, h.stderr)
		}
		var res FeishuFeedResult
		if err := json.Unmarshal([]byte(strings.TrimSpace(h.stdout)), &res); err != nil {
			t.Fatalf("json outcome is not a FeishuFeedResult: %v (output: %s)", err, h.stdout)
		}
		if res.Accepted != 2 || res.Skipped != 0 || res.Deduped != 0 || res.Lines != 2 {
			t.Fatalf("counts = accepted %d / skipped %d / deduped %d / lines %d, want 2/0/0/2",
				res.Accepted, res.Skipped, res.Deduped, res.Lines)
		}
	})
}

// ---------------------------------------------------------------------------
// TC-FI-002 (AC-002): only text is taken; anything else is skipped by reason.
// ---------------------------------------------------------------------------

func TestFeishuFeedSkipsNonText(t *testing.T) {
	h := newFeishuHarness(t)
	code := h.feed(ndjson(
		feishuEvent("oc_x", "om_img", "https://example.invalid/a.png", "image", "user"),
		feishuEvent("oc_x", "om_post", "a rich post", "post", "user"),
		feishuEvent("oc_x", "om_txt", "真的文本", "text", "user"),
	))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, h.stderr)
	}
	if got := h.summaryCount("skipped"); got != 2 {
		t.Fatalf("skipped = %d, want 2 (output: %s)", got, h.stdout)
	}
	if got := h.summaryCount("accepted"); got != 1 {
		t.Fatalf("accepted = %d, want 1: the text line after the skips must still be taken (output: %s)", got, h.stdout)
	}
	if got := h.summary()["reasons"]; got != ReasonFeishuUnsupportedMessageType+"=2" {
		t.Fatalf("reasons = %q, want %s=2 (output: %s)", got, ReasonFeishuUnsupportedMessageType, h.stdout)
	}
	if !strings.Contains(h.stdout, "reason="+ReasonFeishuUnsupportedMessageType) {
		t.Fatalf("output does not name the skip reason (output: %s)", h.stdout)
	}

	// A skipped event is not work: nothing may reach the queue.
	recs := h.slowPaths()
	if len(recs) != 1 {
		t.Fatalf("slow-path records = %d, want 1 (only the text line)", len(recs))
	}
	if recs[0].MessageID != "om_txt" {
		t.Fatalf("queued message_id = %q, want om_txt", recs[0].MessageID)
	}
}

// ---------------------------------------------------------------------------
// TC-FI-003 (AC-003): the bot's own messages never re-enter the bus.
// ---------------------------------------------------------------------------

func TestFeishuFeedSkipsBot(t *testing.T) {
	h := newFeishuHarness(t)
	code := h.feed(ndjson(
		feishuEvent("oc_x", "om_bot", "我是机器人", "text", "bot"),
		feishuEvent("oc_x", "om_human", "我是人", "text", "user"),
	))
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, h.stderr)
	}
	if got := h.summaryCount("skipped"); got != 1 {
		t.Fatalf("skipped = %d, want 1 (output: %s)", got, h.stdout)
	}
	if got := h.summaryCount("accepted"); got != 1 {
		t.Fatalf("accepted = %d, want 1 (output: %s)", got, h.stdout)
	}
	if got := h.summary()["reasons"]; got != ReasonFeishuFromBot+"=1" {
		t.Fatalf("reasons = %q, want %s=1 (output: %s)", got, ReasonFeishuFromBot, h.stdout)
	}
	recs := h.slowPaths()
	if len(recs) != 1 || recs[0].MessageID != "om_human" {
		t.Fatalf("queued = %+v, want exactly the human message om_human", recs)
	}
}

// ---------------------------------------------------------------------------
// TC-FI-004 (AC-004): bad lines are data. They are skipped with a reason, the
// lines after them still count, and the exit code stays 0.
// ---------------------------------------------------------------------------

func TestFeishuFeedBadLines(t *testing.T) {
	h := newFeishuHarness(t)
	code := h.feed(ndjson(
		"not json at all",
		`{"chat_id":"oc_x","message_id":"om_a","content":"hi"`,                                                  // truncated JSON
		`{"message_id":"om_no_chat","content":"hi","message_type":"text","sender_type":"user"}`,                 // no chat_id
		`{"chat_id":"oc_x","content":"hi","message_type":"text","sender_type":"user"}`,                          // no message_id
		`{"chat_id":"oc_x","message_id":"om_no_content","message_type":"text","sender_type":"user"}`,            // no content
		`{"chat_id":"oc_x","message_id":"om_blank","content":"   ","message_type":"text","sender_type":"user"}`, // empty content
		"", // a blank line is formatting, not an event
		feishuEvent("oc_x", "om_good", "坏行之后仍然要处理", "text", "user"),
	))
	if code != 0 {
		t.Fatalf("exit = %d, want 0: bad lines are data, not a crash (stderr: %s)", code, h.stderr)
	}
	if got := h.summaryCount("skipped"); got != 6 {
		t.Fatalf("skipped = %d, want 6 (two invalid_json + four missing_field) (output: %s)", got, h.stdout)
	}
	if got := h.summaryCount("accepted"); got != 1 {
		t.Fatalf("accepted = %d, want 1: the valid line after the bad ones must still be taken (output: %s)", got, h.stdout)
	}
	reasons := h.summary()["reasons"]
	if !strings.Contains(reasons, ReasonFeishuInvalidJSON+"=2") {
		t.Fatalf("reasons = %q, want %s=2 (a non-JSON line and a truncated one)", reasons, ReasonFeishuInvalidJSON)
	}
	if !strings.Contains(reasons, ReasonFeishuMissingField+"=4") {
		t.Fatalf("reasons = %q, want %s=4 (no chat_id, no message_id, no content, blank content)", reasons, ReasonFeishuMissingField)
	}
	recs := h.slowPaths()
	if len(recs) != 1 || recs[0].MessageID != "om_good" {
		t.Fatalf("queued = %+v, want exactly om_good", recs)
	}
}

// ---------------------------------------------------------------------------
// TC-FI-005 (AC-005): the message_id is the idempotency key. Fed twice, the
// event is taken in once -- queued once on the slow path, dispatched once on the
// fast path -- and the second line is reported as deduped.
// ---------------------------------------------------------------------------

func TestFeishuFeedDedupAndQueue(t *testing.T) {
	t.Run("slow path queues the message once", func(t *testing.T) {
		h := newFeishuHarness(t)
		open := feishuEvent("oc_x", "om_dup", "只处理一次", "text", "user")
		code := h.feed(ndjson(open, open))
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, h.stderr)
		}
		if got := h.summaryCount("accepted"); got != 1 {
			t.Fatalf("accepted = %d, want 1 (output: %s)", got, h.stdout)
		}
		if got := h.summaryCount("deduped"); got != 1 {
			t.Fatalf("deduped = %d, want 1 (output: %s)", got, h.stdout)
		}
		if got := h.summaryCount("skipped"); got != 0 {
			t.Fatalf("skipped = %d, want 0: a replay is deduped, not skipped (output: %s)", got, h.stdout)
		}
		recs := h.slowPaths()
		if len(recs) != 1 {
			t.Fatalf("slow-path records = %d, want 1: the replay must not queue a second copy", len(recs))
		}
		if recs[0].MessageID != "om_dup" {
			t.Fatalf("queued message_id = %q, want om_dup", recs[0].MessageID)
		}
	})

	t.Run("fast path dispatches the message once", func(t *testing.T) {
		h := newFeishuHarness(t)
		h.bind(DefaultFeishuChannel, "feishu:oc_x", "ag-feishu")
		open := feishuEvent("oc_x", "om_dup2", "只发一次", "text", "user")
		code := h.feed(ndjson(open, open, feishuEvent("oc_x", "om_next", "新的消息", "text", "user")))
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, h.stderr)
		}
		if got := h.summaryCount("accepted"); got != 2 {
			t.Fatalf("accepted = %d, want 2 (om_dup2 and om_next) (output: %s)", got, h.stdout)
		}
		if got := h.summaryCount("deduped"); got != 1 {
			t.Fatalf("deduped = %d, want 1 (output: %s)", got, h.stdout)
		}
		if len(h.runner.calls) != 2 {
			t.Fatalf("dispatches = %d, want 2: the repeated message_id may only be dispatched once", len(h.runner.calls))
		}
		if !strings.Contains(h.runner.calls[0].text, "只发一次") {
			t.Fatalf("first dispatch = %q, want the first event", h.runner.calls[0].text)
		}
		if !strings.Contains(h.runner.calls[1].text, "新的消息") {
			t.Fatalf("second dispatch = %q, want om_next, not the replay", h.runner.calls[1].text)
		}
		// The deduped line is still reported, so a replay is never invisible.
		if !strings.Contains(h.stdout, "reason_code="+ReasonDuplicateMessage) {
			t.Fatalf("output does not explain the dedupe (output: %s)", h.stdout)
		}
	})
}

// guard: the classifier is exercised directly too, so the check order (which is
// the contract) is pinned independently of the CLI plumbing.
func TestFeishuClassifyOrder(t *testing.T) {
	// Missing field wins over from_bot: an event without a message id cannot be
	// deduped, so it never reaches intake even when a bot sent it.
	if _, skip, _ := ClassifyFeishuLine(`{"chat_id":"oc_x","content":"hi","sender_type":"bot","message_type":"text"}`); skip != ReasonFeishuMissingField {
		t.Fatalf("skip = %q, want %q", skip, ReasonFeishuMissingField)
	}
	// from_bot wins over an unsupported type: both would drop the line, but the
	// reason has to name the first cause.
	if _, skip, _ := ClassifyFeishuLine(feishuEvent("oc_x", "om_1", "x", "image", "bot")); skip != ReasonFeishuFromBot {
		t.Fatalf("skip = %q, want %q", skip, ReasonFeishuFromBot)
	}
	in, skip, err := ClassifyFeishuLine(feishuEvent("oc_x", "om_1", "hi", "text", "user"))
	if skip != "" || err != nil {
		t.Fatalf("text event was dropped: skip=%q err=%v", skip, err)
	}
	if in.Channel != DefaultFeishuChannel || in.Thread != "feishu:oc_x" || in.MessageID != "om_1" {
		t.Fatalf("intake input = %+v, want channel feishu / thread feishu:oc_x / message_id om_1", in)
	}
}
