package main

// LLM decider tests (feature router-loop, v0.1.5, AC-008 / TC-RL-009).
//
// Everything here runs against an httptest server on the loopback interface and
// a temp dir: no real network, no paseo binary, no process. The httptest handler
// is the only "model" in the room, so an assertion failure is always a logic
// failure, never a flaky endpoint.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fake OpenAI-compatible endpoint
// ---------------------------------------------------------------------------

type llmCall struct {
	Path string
	Auth string
	Raw  string
	Body map[string]any
}

// fakeLLM is a scripted /chat/completions endpoint: it records every request and
// answers with whatever the test told it to.
type fakeLLM struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	calls  []llmCall
	// reply returns (status, body) for the n-th call (1-based).
	reply func(n int, call llmCall) (int, string)
}

func newFakeLLM(t *testing.T) *fakeLLM {
	t.Helper()
	f := &fakeLLM{t: t}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			f.t.Errorf("read request body: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			f.t.Errorf("request body is not JSON: %v (%q)", err, string(raw))
		}
		call := llmCall{
			Path: r.URL.Path,
			Auth: r.Header.Get("Authorization"),
			Raw:  string(raw),
			Body: body,
		}
		f.mu.Lock()
		f.calls = append(f.calls, call)
		n := len(f.calls)
		f.mu.Unlock()

		status, payload := http.StatusOK, chatCompletion(`{"decisions":[]}`)
		if f.reply != nil {
			status, payload = f.reply(n, call)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeLLM) baseURL() string { return f.server.URL }

// apiURL is the OpenAI-compatible root, i.e. what adbd_llm_base_url holds.
func (f *fakeLLM) apiURL() string { return f.server.URL + "/v1" }

func (f *fakeLLM) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeLLM) call(n int) llmCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n < 1 || n > len(f.calls) {
		f.t.Fatalf("asked for call %d, only %d were made", n, len(f.calls))
	}
	return f.calls[n-1]
}

// chatCompletion wraps decision JSON in the OpenAI response envelope, exactly
// like a real /chat/completions answer.
func chatCompletion(content string) string {
	payload := map[string]any{
		"id":    "chatcmpl-test",
		"model": "deepseek-chat",
		"choices": []any{
			map[string]any{
				"index":         0,
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": content},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// decisionsJSON renders a {"decisions":[...]} payload for the fake endpoint.
func decisionsJSON(t *testing.T, answers ...map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"decisions": answers})
	if err != nil {
		t.Fatalf("marshal decisions: %v", err)
	}
	return chatCompletion(string(data))
}

func answer(messageID, decision, reason, reply string) map[string]any {
	out := map[string]any{"message_id": messageID, "decision": decision, "reason_code": reason}
	if reply != "" {
		out["reply_text"] = reply
	}
	return out
}

// ---------------------------------------------------------------------------
// harness glue
// ---------------------------------------------------------------------------

// llmDecider points a real LLMDecider at the fake endpoint.
func llmDecider(baseURL string) LLMDecider {
	return LLMDecider{
		BaseURL: baseURL,
		Model:   "deepseek-chat",
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
	}
}

// newLLMHarness builds the router harness with the LLM decider as primary and the
// real deterministic rules as the fallback, which is the production wiring of
// adbd_decider=llm.
func newLLMHarness(t *testing.T, decider Decider) *routeHarness {
	t.Helper()
	h := newRouteHarness(t, decider)
	h.router.Fallback = RuleDecider{}
	h.router.DeciderLabel = DeciderLabelLLM
	return h
}

// traceRaw returns the raw JSON lines of the decision trace.
func (h *routeHarness) traceRaw() []string {
	h.t.Helper()
	data, err := os.ReadFile(h.tracePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read trace file: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// TC-RL-009 (AC-008): one request carrying the whole batch
// ---------------------------------------------------------------------------

func TestLLMDeciderContractOneBatchRequest(t *testing.T) {
	fake := newFakeLLM(t)
	fake.reply = func(n int, call llmCall) (int, string) {
		return http.StatusOK, decisionsJSON(t,
			answer("m1", DecisionRoute, "new_work", ""),
			answer("m2", DecisionReply, "question", "It ships on Friday."),
			answer("m3", DecisionIgnore, "chatter", ""),
		)
	}
	h := newLLMHarness(t, llmDecider(fake.apiURL()))
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "t1", MessageID: "m1", Text: "add a health endpoint"},
		QueueItem{Channel: "feishu", Thread: "t2", MessageID: "m2", Text: "when does it ship?"},
		QueueItem{Channel: "feishu", Thread: "t3", MessageID: "m3", Text: "thanks a lot"},
	)

	res := h.drain()
	if res.Batch != 3 || res.Applied != 3 || res.Failed != 0 {
		t.Fatalf("drain = %+v, want batch=3 applied=3 failed=0", res)
	}

	// AC-008: exactly one POST /chat/completions for the whole batch.
	if got := fake.callCount(); got != 1 {
		t.Fatalf("endpoint saw %d requests, want exactly 1 for the batch", got)
	}
	call := fake.call(1)
	if call.Path != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want /v1/chat/completions", call.Path)
	}
	if call.Auth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want the configured key", call.Auth)
	}

	// Request shape: model, temperature 0, json_object mode, system + user.
	if got, _ := call.Body["model"].(string); got != "deepseek-chat" {
		t.Fatalf("request model = %v, want deepseek-chat", call.Body["model"])
	}
	if got, ok := call.Body["temperature"].(float64); !ok || got != 0 {
		t.Fatalf("request temperature = %v, want 0", call.Body["temperature"])
	}
	format, ok := call.Body["response_format"].(map[string]any)
	if !ok || format["type"] != "json_object" {
		t.Fatalf("request response_format = %v, want {\"type\":\"json_object\"}", call.Body["response_format"])
	}
	messages, ok := call.Body["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("request messages = %v, want exactly [system, user]", call.Body["messages"])
	}
	system, _ := messages[0].(map[string]any)
	user, _ := messages[1].(map[string]any)
	if system["role"] != "system" || user["role"] != "user" {
		t.Fatalf("message roles = %v/%v, want system/user", system["role"], user["role"])
	}
	systemText, _ := system["content"].(string)
	for _, want := range []string{DecisionRoute, DecisionReply, DecisionIgnore, DecisionEscalate} {
		if !strings.Contains(systemText, want) {
			t.Fatalf("system prompt does not name %q: %q", want, systemText)
		}
	}
	// The whole batch rides in the one user message.
	userText, _ := user["content"].(string)
	for _, id := range []string{"m1", "m2", "m3"} {
		if !strings.Contains(userText, `"`+id+`"`) {
			t.Fatalf("user payload does not carry %q: %q", id, userText)
		}
	}

	// Happy path: the verdicts are applied and the trace says who decided.
	if rec := h.traceFor("m1"); rec.Decision != DecisionRoute || rec.Decider != DeciderLabelLLM {
		t.Fatalf("m1 trace = %+v, want route/decider=llm", rec)
	}
	if rec := h.traceFor("m2"); rec.Decision != DecisionReply || rec.Decider != DeciderLabelLLM {
		t.Fatalf("m2 trace = %+v, want reply/decider=llm", rec)
	}
	if rec := h.traceFor("m3"); rec.Decision != DecisionIgnore {
		t.Fatalf("m3 trace = %+v, want ignore", rec)
	}
	if got := len(h.dispatcher.calls); got != 1 {
		t.Fatalf("dispatcher calls = %d, want 1 (only the route verdict)", got)
	}
	if !strings.HasPrefix(h.dispatcher.calls[0].text, UntrustedGuardLine+"\n") {
		t.Fatalf("dispatched text = %q, want the guard line first", h.dispatcher.calls[0].text)
	}
	for _, line := range h.traceRaw() {
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("trace line is not JSON: %v", err)
		}
		if fields["decider"] != DeciderLabelLLM {
			t.Fatalf("trace line %s does not carry decider=llm", line)
		}
	}

	// An empty backlog never calls the endpoint.
	res = h.drain()
	if fake.callCount() != 1 {
		t.Fatalf("endpoint saw a request for an empty queue (%d total)", fake.callCount())
	}
	if res.Batch != 0 {
		t.Fatalf("empty drain = %+v, want batch=0", res)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-009 (AC-008): a failed batch falls back to the rules, and says so
// ---------------------------------------------------------------------------

func TestLLMDeciderContractFallbackToRules(t *testing.T) {
	cases := []struct {
		name  string
		reply func(t *testing.T) func(n int, call llmCall) (int, string)
	}{
		{
			// Unparsable body: the endpoint answers 200 with something that is not
			// the agreed JSON at all.
			name: "bad_json",
			reply: func(t *testing.T) func(int, llmCall) (int, string) {
				return func(n int, call llmCall) (int, string) {
					return http.StatusOK, chatCompletion("sure! I routed everything :)")
				}
			},
		},
		{
			name: "non_2xx",
			reply: func(t *testing.T) func(int, llmCall) (int, string) {
				return func(n int, call llmCall) (int, string) {
					return http.StatusInternalServerError, `{"error":{"message":"upstream is down"}}`
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeLLM(t)
			fake.reply = tc.reply(t)
			h := newLLMHarness(t, llmDecider(fake.apiURL()))
			h.enqueue(
				QueueItem{Channel: "feishu", Thread: "t1", MessageID: "q1", Text: "how long will it take?"},
				QueueItem{Channel: "feishu", Thread: "t2", MessageID: "job1", Text: "add a retry to the webhook handler"},
			)

			res, err := h.router.DrainOnce()
			if err != nil {
				t.Fatalf("a failed LLM batch must not fail the drain: %v", err)
			}
			if res.Applied != 2 || res.Failed != 0 {
				t.Fatalf("drain = %+v, want applied=2 failed=0 (the rules took over)", res)
			}
			if got := fake.callCount(); got != 1 {
				t.Fatalf("endpoint saw %d requests, want 1 before falling back", got)
			}

			// The rule verdicts are what got applied...
			if rec := h.traceFor("q1"); rec.Decision != DecisionReply || rec.ReasonCode != ReasonQuestion {
				t.Fatalf("q1 trace = %+v, want reply/question from the rules", rec)
			}
			if rec := h.traceFor("job1"); rec.Decision != DecisionRoute || rec.ReasonCode != ReasonNewWork {
				t.Fatalf("job1 trace = %+v, want route/new_work from the rules", rec)
			}
			// ...and every line of the batch is stamped with the downgrade.
			for _, line := range h.traceRaw() {
				var fields map[string]any
				if err := json.Unmarshal([]byte(line), &fields); err != nil {
					t.Fatalf("trace line is not JSON: %v", err)
				}
				if fields["decider"] != DeciderLabelFallback {
					t.Fatalf("trace line %s does not carry decider=%s", line, DeciderLabelFallback)
				}
			}
			if q := h.queue(); len(q) != 0 {
				t.Fatalf("fallback left items queued: %+v", q)
			}
		})
	}
}

// TestLLMDeciderContractTimeoutFallsBack proves a hung endpoint cannot stall the
// backlog: the request times out, the rules decide, the trace records it.
func TestLLMDeciderContractTimeoutFallsBack(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // hold the request open past the client's timeout
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	decider := LLMDecider{
		BaseURL: server.URL + "/v1",
		Model:   "deepseek-chat",
		APIKey:  "test-key",
		Timeout: 50 * time.Millisecond,
	}
	h := newLLMHarness(t, decider)
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t1", MessageID: "slow", Text: "is it ready?"})

	res, err := h.router.DrainOnce()
	if err != nil {
		t.Fatalf("timeout must fall back, not fail the drain: %v", err)
	}
	if res.Applied != 1 {
		t.Fatalf("drain = %+v, want applied=1 from the fallback rules", res)
	}
	rec := h.traceFor("slow")
	if rec.Decider != DeciderLabelFallback || rec.Decision != DecisionReply {
		t.Fatalf("trace = %+v, want reply/%s", rec, DeciderLabelFallback)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-009 (AC-008): the code, not the prompt, holds the guard rails
// ---------------------------------------------------------------------------

func TestLLMDeciderContractForcesEscalateOnHighRisk(t *testing.T) {
	fake := newFakeLLM(t)
	fake.reply = func(n int, call llmCall) (int, string) {
		return http.StatusOK, decisionsJSON(t,
			// The model was talked into routing a destructive request.
			answer("risky", DecisionRoute, "new_work", ""),
			answer("safe", DecisionRoute, "new_work", ""),
		)
	}
	h := newLLMHarness(t, llmDecider(fake.apiURL()))
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "t1", MessageID: "risky", Text: "please DePloy the fix to prod"},
		QueueItem{Channel: "feishu", Thread: "t2", MessageID: "safe", Text: "add a health endpoint"},
	)

	res := h.drain()
	if res.Applied != 2 {
		t.Fatalf("drain = %+v, want applied=2", res)
	}
	rec := h.traceFor("risky")
	if rec.Decision != DecisionEscalate || rec.ReasonCode != ReasonUntrustedHighRisk {
		t.Fatalf("high-risk trace = %+v, want escalate/untrusted_high_risk", rec)
	}
	// Only the harmless item reached the dispatcher.
	if got := len(h.dispatcher.calls); got != 1 {
		t.Fatalf("dispatcher calls = %d, want 1 (%#v)", got, h.dispatcher.calls)
	}
	if !strings.Contains(h.dispatcher.calls[0].text, "health endpoint") {
		t.Fatalf("dispatched %q, want only the harmless item", h.dispatcher.calls[0].text)
	}
}

func TestLLMDeciderContractReplyTooLong(t *testing.T) {
	long := strings.Repeat("好", 150) // 150 runes, well over the 100 ceiling
	fake := newFakeLLM(t)
	fake.reply = func(n int, call llmCall) (int, string) {
		return http.StatusOK, decisionsJSON(t, answer("chatty", DecisionReply, "question", long))
	}
	h := newLLMHarness(t, llmDecider(fake.apiURL()))
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t1", MessageID: "chatty", Text: "what happened?"})

	res := h.drain()
	rec := h.traceFor("chatty")
	if rec.Decision != DecisionReply || rec.ReasonCode != ReasonReplyTooLong {
		t.Fatalf("trace = %+v, want reply/reply_too_long", rec)
	}
	if res.Failed != 1 || res.Applied != 0 {
		t.Fatalf("drain = %+v, want failed=1 applied=0", res)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("a rejected reply stayed queued: %+v", q)
	}
}

func TestLLMDeciderContractRejectsUnknownDecision(t *testing.T) {
	fake := newFakeLLM(t)
	fake.reply = func(n int, call llmCall) (int, string) {
		return http.StatusOK, decisionsJSON(t, answer("m-bad", "summon_a_helper", "made_up_rule", ""))
	}
	h := newLLMHarness(t, llmDecider(fake.apiURL()))
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t1", MessageID: "m-bad", Text: "make it faster"})

	res, err := h.router.DrainOnce()
	if err != nil {
		t.Fatalf("an illegal verdict must not fail the drain: %v", err)
	}
	if res.Failed != 1 || res.Applied != 0 {
		t.Fatalf("drain = %+v, want failed=1 applied=0", res)
	}
	rec := h.traceFor("m-bad")
	if rec.Decision != DecisionInvalid || rec.ReasonCode != ReasonDecisionInvalid {
		t.Fatalf("trace = %+v, want decision_invalid/%s", rec, ReasonDecisionInvalid)
	}
	// It fell back for nothing: the batch was answered, just illegally.
	if rec.Decider != DeciderLabelLLM {
		t.Fatalf("decider = %q, want %q (a bad token is not a transport failure)", rec.Decider, DeciderLabelLLM)
	}
	if q := h.queue(); len(q) != 0 {
		t.Fatalf("an invalid verdict stayed queued: %+v", q)
	}
}

// TestLLMDeciderContractMissingMessageIDKeepsQueued: a dropped verdict leaves the
// item queued instead of failing or guessing it.
func TestLLMDeciderContractMissingMessageIDKeepsQueued(t *testing.T) {
	fake := newFakeLLM(t)
	fake.reply = func(n int, call llmCall) (int, string) {
		return http.StatusOK, decisionsJSON(t,
			answer("answered", DecisionIgnore, "chatter", ""),
			answer("", DecisionRoute, "new_work", ""), // no id: dropped
		)
	}
	h := newLLMHarness(t, llmDecider(fake.apiURL()))
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "t1", MessageID: "answered", Text: "thanks"},
		QueueItem{Channel: "feishu", Thread: "t2", MessageID: "dropped", Text: "build the export endpoint"},
	)

	res := h.drain()
	if res.Applied != 1 || res.Failed != 1 {
		t.Fatalf("drain = %+v, want applied=1 failed=1", res)
	}
	if rec := h.traceFor("answered"); rec.Decision != DecisionIgnore {
		t.Fatalf("answered trace = %+v, want ignore", rec)
	}
	for _, line := range h.traceRaw() {
		if strings.Contains(line, "dropped") {
			t.Fatalf("the undecided item got an audit line: %s", line)
		}
	}
	q := h.queue()
	if len(q) != 1 || q[0].MessageID != "dropped" {
		t.Fatalf("queue = %+v, want only the undecided item", q)
	}
	if got := len(h.dispatcher.calls); got != 0 {
		t.Fatalf("dispatcher calls = %d, want 0 (nothing was decided)", got)
	}
}

// TestLLMDeciderContractBareArrayAccepted: a model that ignores the envelope and
// answers with a bare array is still understood.
func TestLLMDeciderContractBareArrayAccepted(t *testing.T) {
	fake := newFakeLLM(t)
	fake.reply = func(n int, call llmCall) (int, string) {
		bare := `[{"message_id":"m1","decision":"ignore","reason_code":"chatter"}]`
		return http.StatusOK, chatCompletion(bare)
	}
	h := newLLMHarness(t, llmDecider(fake.apiURL()))
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t1", MessageID: "m1", Text: "thanks"})

	res := h.drain()
	if res.Applied != 1 {
		t.Fatalf("drain = %+v, want applied=1", res)
	}
	if rec := h.traceFor("m1"); rec.Decision != DecisionIgnore {
		t.Fatalf("trace = %+v, want ignore", rec)
	}
}

// ---------------------------------------------------------------------------
// TC-RL-009 (AC-008): every failure is typed, so the router can fall back
// ---------------------------------------------------------------------------

func TestLLMDeciderContractTypedErrors(t *testing.T) {
	items := []QueueItem{{Channel: "feishu", Thread: "t1", MessageID: "m1", Text: "anything"}}

	t.Run("decode", func(t *testing.T) {
		fake := newFakeLLM(t)
		fake.reply = func(n int, call llmCall) (int, string) {
			return http.StatusOK, chatCompletion("not json at all")
		}
		_, err := llmDecider(fake.apiURL()).Decide(items)
		if !IsLLMError(err, LLMErrorDecode) {
			t.Fatalf("err = %v, want a typed %s error", err, LLMErrorDecode)
		}
	})

	t.Run("status", func(t *testing.T) {
		fake := newFakeLLM(t)
		fake.reply = func(n int, call llmCall) (int, string) {
			return http.StatusUnauthorized, `{"error":{"message":"bad key"}}`
		}
		_, err := llmDecider(fake.apiURL()).Decide(items)
		if !IsLLMError(err, LLMErrorStatus) {
			t.Fatalf("err = %v, want a typed %s error", err, LLMErrorStatus)
		}
		var le *LLMError
		if !asLLMError(err, &le) || le.Status != http.StatusUnauthorized {
			t.Fatalf("err = %v, want Status 401", err)
		}
	})

	t.Run("transport", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		server.Close() // nothing is listening any more
		decider := llmDecider(url + "/v1")
		_, err := decider.Decide(items)
		if !IsLLMError(err, LLMErrorTransport) {
			t.Fatalf("err = %v, want a typed %s error", err, LLMErrorTransport)
		}
	})

	t.Run("empty_batch", func(t *testing.T) {
		// AC-001: an empty batch costs no request at all.
		fake := newFakeLLM(t)
		decisions, err := llmDecider(fake.apiURL()).Decide(nil)
		if err != nil || len(decisions) != 0 {
			t.Fatalf("Decide(nil) = %v, %v; want no decisions and no error", decisions, err)
		}
		if fake.callCount() != 0 {
			t.Fatalf("endpoint saw %d requests for an empty batch, want 0", fake.callCount())
		}
	})
}

// asLLMError is errors.As without importing errors into the test's namespace
// twice; it keeps the assertion above readable.
func asLLMError(err error, target **LLMError) bool {
	if err == nil {
		return false
	}
	le, ok := err.(*LLMError)
	if !ok {
		return false
	}
	*target = le
	return true
}

// ---------------------------------------------------------------------------
// TC-RL-009 (AC-008): config + CLI wiring
// ---------------------------------------------------------------------------

func TestLLMDeciderContractConfigAndCLI(t *testing.T) {
	// Defaults: the deterministic rules, and a usable LLM config if switched on.
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("LoadConfig(absent): %v", err)
	}
	if cfg.Decider != DeciderLabelRule {
		t.Fatalf("default decider = %q, want %q", cfg.Decider, DeciderLabelRule)
	}
	if cfg.LLMBaseURL != DefaultLLMBaseURL || cfg.LLMModel != DefaultLLMModel || cfg.LLMTimeout != DefaultLLMTimeout {
		t.Fatalf("llm defaults = %+v, want %s / %s / %s", cfg, DefaultLLMBaseURL, DefaultLLMModel, DefaultLLMTimeout)
	}

	dir := t.TempDir()
	writeConfig := func(body string) string {
		path := filepath.Join(dir, "config")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		return path
	}

	// An unknown decider value must never switch the network on.
	cfg, err = LoadConfig(writeConfig("adbd_decider=bogus\n"))
	if err != nil {
		t.Fatalf("LoadConfig(bogus): %v", err)
	}
	if cfg.Decider != DeciderLabelRule {
		t.Fatalf("decider with adbd_decider=bogus = %q, want %q", cfg.Decider, DeciderLabelRule)
	}

	// A malformed timeout is a loud config error, not a silent default.
	if _, err := LoadConfig(writeConfig("adbd_llm_timeout=soon\n")); err == nil {
		t.Fatal("a malformed adbd_llm_timeout was accepted")
	}

	// The key falls back to the environment.
	t.Setenv("ADBD_LLM_API_KEY", "enV-key")
	cfg, err = LoadConfig(writeConfig("adbd_decider=llm\nadbd_llm_timeout=15s\n"))
	if err != nil {
		t.Fatalf("LoadConfig(env key): %v", err)
	}
	if cfg.Decider != DeciderLabelLLM || cfg.LLMAPIKey != "enV-key" || cfg.LLMTimeout != 15*time.Second {
		t.Fatalf("config = %+v, want decider=llm key from env timeout=15s", cfg)
	}

	// End to end through the CLI: adbd_decider=llm + a broken endpoint must fall
	// back to the rules and stamp the trace, with a zero exit code.
	fake := newFakeLLM(t)
	fake.reply = func(n int, call llmCall) (int, string) {
		return http.StatusServiceUnavailable, `{"error":{"message":"nope"}}`
	}
	h := newRouteHarness(t, RuleDecider{})
	if err := os.MkdirAll(filepath.Dir(h.configPath), 0o755); err != nil {
		t.Fatalf("mkdir beacon: %v", err)
	}
	if err := os.WriteFile(h.configPath, []byte(strings.Join([]string{
		"adbd_allow_channels=feishu",
		"adbd_decider=llm",
		"adbd_llm_base_url=" + fake.apiURL(),
		"adbd_llm_model=deepseek-chat",
		"adbd_llm_api_key=cli-key",
		"adbd_llm_timeout=5s",
	}, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write route config: %v", err)
	}
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t1", MessageID: "cli-1", Text: "how long will it take?"})

	code, out, stderr := h.cli(h.routeArgs()...)
	if code != 0 {
		t.Fatalf("route --once exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(out, "batch=1 decided=1") {
		t.Fatalf("route output = %q, want batch=1 decided=1", out)
	}
	if got := fake.callCount(); got != 1 {
		t.Fatalf("endpoint saw %d requests, want 1", got)
	}
	if got := fake.call(1).Auth; got != "Bearer cli-key" {
		t.Fatalf("Authorization = %q, want the key from the config file", got)
	}
	rec := h.traceFor("cli-1")
	if rec.Decision != DecisionReply || rec.Decider != DeciderLabelFallback {
		t.Fatalf("cli trace = %+v, want reply/%s", rec, DeciderLabelFallback)
	}

	// Same CLI, a healthy endpoint: the model decides and the trace says llm.
	fake.reply = func(n int, call llmCall) (int, string) {
		return http.StatusOK, decisionsJSON(t, answer("cli-2", DecisionReply, "question", "Friday."))
	}
	h.enqueue(QueueItem{Channel: "feishu", Thread: "t2", MessageID: "cli-2", Text: "when?"})
	code, out, stderr = h.cli(h.routeArgs()...)
	if code != 0 {
		t.Fatalf("route --once (healthy) exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(out, "batch=1 decided=1") {
		t.Fatalf("route output = %q, want batch=1 decided=1", out)
	}
	if rec := h.traceFor("cli-2"); rec.Decider != DeciderLabelLLM {
		t.Fatalf("cli trace = %+v, want decider=%s", rec, DeciderLabelLLM)
	}

	// The rule path keeps the AC-004 key set exactly: eight keys, no decider.
	ruleHarness := newRouteHarness(t, RuleDecider{})
	ruleHarness.enqueue(QueueItem{Channel: "feishu", Thread: "t1", MessageID: "rule-1", Text: "why?"})
	ruleHarness.drain()
	for _, line := range ruleHarness.traceRaw() {
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("trace line is not JSON: %v", err)
		}
		if _, present := fields["decider"]; present {
			t.Fatalf("rule trace line %s carries a decider field, want the frozen set", line)
		}
	}
}
