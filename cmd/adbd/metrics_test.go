package main

// Metrics tests (feature metrics, v0.1.5): TC-MT-001 .. TC-MT-005.
//
// Every test builds a temp root and writes the three ledgers with timestamps it
// chose itself, so the expected p50/p95 can be hand-computed and asserted
// exactly. Nothing here reads the real .beacon/ of this repository, and nothing
// here tolerates a percentile the code did not actually compute: the "absent"
// assertions decode the JSON into raw maps so a missing p95_ms cannot hide behind
// a zero value.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// metricsBase is the fixed instant every fixture counts from, so an expected
// duration is just a subtraction a reader can redo by eye.
var metricsBase = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// harness: a temp root holding the three ledgers
// ---------------------------------------------------------------------------

type metricsHarness struct {
	t      *testing.T
	root   string
	beacon string
}

func newMetricsHarness(t *testing.T) *metricsHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatalf("create .beacon: %v", err)
	}
	return &metricsHarness{t: t, root: root, beacon: beacon}
}

func (h *metricsHarness) path(name string) string {
	return filepath.Join(h.beacon, name)
}

// writeLines appends the marshaled rows to one ledger, using the production types
// so a fixture can never drift from the format the daemon writes.
func (h *metricsHarness) writeLines(name string, rows [][]byte) {
	h.t.Helper()
	var buf bytes.Buffer
	for _, row := range rows {
		buf.Write(row)
		buf.WriteByte('\n')
	}
	h.writeRaw(name, buf.String())
}

func (h *metricsHarness) writeRaw(name, body string) {
	h.t.Helper()
	if err := os.WriteFile(h.path(name), []byte(body), 0o644); err != nil {
		h.t.Fatalf("write %s: %v", name, err)
	}
}

// writeDecisions writes .beacon/adbd-decisions.jsonl from the frozen trace shape.
func (h *metricsHarness) writeDecisions(recs []traceRecord) {
	h.t.Helper()
	rows := make([][]byte, 0, len(recs))
	for _, rec := range recs {
		data, err := json.Marshal(rec)
		if err != nil {
			h.t.Fatalf("marshal decision: %v", err)
		}
		rows = append(rows, data)
	}
	h.writeLines(decisionsFile, rows)
}

// writeSlowPath writes .beacon/adbd-slowpath.jsonl.
func (h *metricsHarness) writeSlowPath(recs []slowPathRecord) {
	h.t.Helper()
	rows := make([][]byte, 0, len(recs))
	for _, rec := range recs {
		data, err := json.Marshal(rec)
		if err != nil {
			h.t.Fatalf("marshal slow-path record: %v", err)
		}
		rows = append(rows, data)
	}
	h.writeLines("adbd-slowpath.jsonl", rows)
}

// writeReports writes .beacon/adbd-reports.jsonl.
func (h *metricsHarness) writeReports(recs []workerReport) {
	h.t.Helper()
	rows := make([][]byte, 0, len(recs))
	for _, rec := range recs {
		data, err := json.Marshal(rec)
		if err != nil {
			h.t.Fatalf("marshal worker report: %v", err)
		}
		rows = append(rows, data)
	}
	h.writeLines(reportsFile, rows)
}

// run invokes the CLI exactly as a launchd job would.
func (h *metricsHarness) run(extra ...string) (code int, stdout, stderr string) {
	h.t.Helper()
	args := append([]string{"metrics", "--once", "--root", h.root}, extra...)
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// ---------------------------------------------------------------------------
// decoding: values AND the absence of keys
// ---------------------------------------------------------------------------

// metricsDoc keeps every top-level value raw, so a test can tell "key missing"
// apart from "key present and zero".
type metricsDoc struct {
	raw map[string]json.RawMessage
}

func decodeMetrics(t *testing.T, stdout string) metricsDoc {
	t.Helper()
	doc := metricsDoc{raw: map[string]json.RawMessage{}}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc.raw); err != nil {
		t.Fatalf("metrics output is not JSON: %v\n%s", err, stdout)
	}
	return doc
}

func (d metricsDoc) has(name string) bool {
	_, ok := d.raw[name]
	return ok
}

func (d metricsDoc) status(t *testing.T) string {
	t.Helper()
	raw, ok := d.raw["status"]
	if !ok {
		t.Fatalf("metrics output has no status key")
	}
	var status string
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("status is not a string: %v", err)
	}
	return status
}

func (d metricsDoc) metric(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, ok := d.raw[name]
	if !ok {
		t.Fatalf("metrics output has no %q key; keys = %v", name, d.keys())
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%q is not an object: %v", name, err)
	}
	return out
}

func (d metricsDoc) keys() []string {
	out := make([]string, 0, len(d.raw))
	for key := range d.raw {
		out = append(out, key)
	}
	return out
}

// numberField reads a numeric field and fails loudly when the key is absent.
func numberField(t *testing.T, metric map[string]any, key string) float64 {
	t.Helper()
	value, ok := metric[key]
	if !ok {
		t.Fatalf("metric has no %q key: %v", key, metric)
	}
	number, ok := value.(float64)
	if !ok {
		t.Fatalf("%q is not a number: %#v", key, value)
	}
	return number
}

func requireAbsent(t *testing.T, metric map[string]any, key string) {
	t.Helper()
	if _, ok := metric[key]; ok {
		t.Fatalf("metric must not carry %q, got %v", key, metric)
	}
}

// decisionRow builds one frozen-shape decision trace line.
func decisionRow(thread, messageID, reason string, decidedAt time.Time) traceRecord {
	return traceRecord{
		Channel:    "feishu",
		Thread:     thread,
		MessageID:  messageID,
		Decision:   DecisionRoute,
		ReasonCode: reason,
		BatchSize:  1,
		Untrusted:  true,
		DecidedAt:  decidedAt.Format(time.RFC3339),
	}
}

// slowPathRow builds one slow-path queue line recorded at at.
func slowPathRow(messageID string, at time.Time) slowPathRecord {
	return slowPathRecord{
		Channel:    "feishu",
		Thread:     "t-1",
		MessageID:  messageID,
		Text:       "hello",
		ReasonCode: ReasonNoBinding,
		RecordedAt: at.Format(time.RFC3339),
	}
}

// receiptRow builds one worker receipt with a known duration.
func receiptRow(agentID string, dispatchedAt, finishedAt time.Time) workerReport {
	return workerReport{
		Thread:       "t-1",
		AgentID:      agentID,
		DispatchedAt: dispatchedAt.Format(time.RFC3339),
		FinishedAt:   finishedAt.Format(time.RFC3339),
		Status:       StateDone,
	}
}

// ---------------------------------------------------------------------------
// TC-MT-001 (AC-001): reads the existing streams, honours --since
// ---------------------------------------------------------------------------

func TestMetricsReadsExistingStreams(t *testing.T) {
	h := newMetricsHarness(t)

	// Five messages that lost the fastpath and were decided 1s..5s later.
	decisions := make([]traceRecord, 0, 5)
	queue := make([]slowPathRecord, 0, 5)
	for i := 1; i <= 5; i++ {
		messageID := "m-" + string(rune('0'+i))
		queue = append(queue, slowPathRow(messageID, metricsBase))
		decisions = append(decisions, decisionRow("t-1", messageID, ReasonNewWork, metricsBase.Add(time.Duration(i)*time.Second)))
	}
	h.writeSlowPath(queue)
	h.writeDecisions(decisions)
	h.writeReports([]workerReport{
		receiptRow("ag-1", metricsBase, metricsBase.Add(time.Second)),
	})

	code, stdout, stderr := h.run("--outcome", "json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	doc := decodeMetrics(t, stdout)
	for _, name := range []string{"time_to_first_worker", "task_completion", "decisions_per_thread", "fastpath_share"} {
		if !doc.has(name) {
			t.Fatalf("metrics output is missing %q; keys = %v", name, doc.keys())
		}
	}
	if got := doc.status(t); got != MetricsInsufficientData && got != MetricsOK {
		t.Fatalf("status = %q, want ok or insufficient_data", got)
	}
	if got := numberField(t, doc.metric(t, "time_to_first_worker"), "count"); got != 5 {
		t.Fatalf("time_to_first_worker.count = %v, want 5", got)
	}
	if got := numberField(t, doc.metric(t, "decisions_per_thread"), "decisions"); got != 5 {
		t.Fatalf("decisions_per_thread.decisions = %v, want 5", got)
	}

	// The text outcome prints every metric plus the status last.
	code, textOut, stderr := h.run()
	if code != 0 {
		t.Fatalf("text exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"time_to_first_worker count=5", "task_completion count=1", "decisions_per_thread decisions=5", "fastpath_share fast=0 total=5", "status="} {
		if !strings.Contains(textOut, want) {
			t.Fatalf("text outcome missing %q:\n%s", want, textOut)
		}
	}
	if lines := strings.Split(strings.TrimSpace(textOut), "\n"); len(lines) != 5 || !strings.HasPrefix(lines[4], "status=") {
		t.Fatalf("text outcome = %q, want five lines with status last", textOut)
	}

	// --since keeps only records at or after the instant, in every stream.
	code, stdout, stderr = h.run("--outcome", "json", "--since", metricsBase.Add(4*time.Second).Format(time.RFC3339))
	if code != 0 {
		t.Fatalf("--since exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	doc = decodeMetrics(t, stdout)
	first := doc.metric(t, "time_to_first_worker")
	if got := numberField(t, first, "count"); got != 2 {
		t.Fatalf("--since time_to_first_worker.count = %v, want 2", got)
	}
	if got := numberField(t, first, "p50_ms"); got != 4000 {
		t.Fatalf("--since time_to_first_worker.p50_ms = %v, want 4000", got)
	}
	requireAbsent(t, first, "p95_ms")
	if got := first["insufficient_data"]; got != true {
		t.Fatalf("--since time_to_first_worker.insufficient_data = %v, want true", got)
	}
	if got := numberField(t, doc.metric(t, "decisions_per_thread"), "decisions"); got != 2 {
		t.Fatalf("--since decisions_per_thread.decisions = %v, want 2", got)
	}
	// The receipt finished at +1s, so it falls outside the window.
	if got := numberField(t, doc.metric(t, "task_completion"), "count"); got != 0 {
		t.Fatalf("--since task_completion.count = %v, want 0", got)
	}

	// A malformed --since is a loud usage error, not a silent "no filter".
	var out, errOut bytes.Buffer
	if code := run([]string{"metrics", "--once", "--root", h.root, "--since", "yesterday"}, &out, &errOut); code != 2 {
		t.Fatalf("bad --since exit = %d, want 2", code)
	}
	// --once has no resident mode: omitting it is a usage error too.
	if code := run([]string{"metrics", "--root", h.root}, &out, &errOut); code != 2 {
		t.Fatalf("missing --once exit = %d, want 2", code)
	}
}

// ---------------------------------------------------------------------------
// TC-MT-002 (AC-002): time_to_first_worker from recorded_at + decided_at
// ---------------------------------------------------------------------------

func TestMetricsTimeToFirstWorker(t *testing.T) {
	h := newMetricsHarness(t)

	// Deltas 1s..5s; nearest-rank p50 = ceil(0.5*5) = 3rd = 3000, p95 = 5th = 5000.
	queue := make([]slowPathRecord, 0, 5)
	decisions := make([]traceRecord, 0, 6)
	for i := 1; i <= 5; i++ {
		messageID := "m-" + string(rune('0'+i))
		queue = append(queue, slowPathRow(messageID, metricsBase))
		decisions = append(decisions, decisionRow("t-1", messageID, ReasonNewWork, metricsBase.Add(time.Duration(i)*time.Second)))
	}
	// A decision with no queue row has no start instant: it must not be paired
	// with anything, so the count stays 5.
	decisions = append(decisions, decisionRow("t-1", "m-orphan", ReasonNewWork, metricsBase.Add(time.Hour)))
	h.writeSlowPath(queue)
	h.writeDecisions(decisions)

	code, stdout, stderr := h.run("--outcome", "json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	first := decodeMetrics(t, stdout).metric(t, "time_to_first_worker")
	if got := numberField(t, first, "count"); got != 5 {
		t.Fatalf("count = %v, want 5", got)
	}
	if got := numberField(t, first, "p50_ms"); got != 3000 {
		t.Fatalf("p50_ms = %v, want 3000 (nearest-rank 3rd of 5)", got)
	}
	if got := numberField(t, first, "p95_ms"); got != 5000 {
		t.Fatalf("p95_ms = %v, want 5000 (nearest-rank 5th of 5)", got)
	}
	requireAbsent(t, first, "insufficient_data")

	// Four samples: p50 is still computed (2nd of 4 = 2000) but p95 is omitted.
	h.writeDecisions(decisions[:4])
	code, stdout, stderr = h.run("--outcome", "json")
	if code != 0 {
		t.Fatalf("4-sample exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	first = decodeMetrics(t, stdout).metric(t, "time_to_first_worker")
	if got := numberField(t, first, "count"); got != 4 {
		t.Fatalf("4-sample count = %v, want 4", got)
	}
	if got := numberField(t, first, "p50_ms"); got != 2000 {
		t.Fatalf("4-sample p50_ms = %v, want 2000 (nearest-rank 2nd of 4)", got)
	}
	requireAbsent(t, first, "p95_ms")
	if got := first["insufficient_data"]; got != true {
		t.Fatalf("4-sample insufficient_data = %v, want true", got)
	}
}

// ---------------------------------------------------------------------------
// TC-MT-003 (AC-003): task_completion from dispatched_at + finished_at
// ---------------------------------------------------------------------------

func TestMetricsTaskCompletion(t *testing.T) {
	h := newMetricsHarness(t)

	reports := make([]workerReport, 0, 6)
	for i := 1; i <= 5; i++ {
		reports = append(reports, receiptRow(
			"ag-"+string(rune('0'+i)),
			metricsBase,
			metricsBase.Add(time.Duration(i)*time.Second),
		))
	}
	// A receipt without finished_at describes no duration: it contributes nothing.
	reports = append(reports, workerReport{
		Thread:       "t-2",
		AgentID:      "ag-incomplete",
		DispatchedAt: metricsBase.Format(time.RFC3339),
		Status:       StateDone,
	})
	h.writeReports(reports)

	code, stdout, stderr := h.run("--outcome", "json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	completion := decodeMetrics(t, stdout).metric(t, "task_completion")
	if got := numberField(t, completion, "count"); got != 5 {
		t.Fatalf("count = %v, want 5 (the incomplete receipt must be skipped)", got)
	}
	if got := numberField(t, completion, "p50_ms"); got != 3000 {
		t.Fatalf("p50_ms = %v, want 3000", got)
	}
	if got := numberField(t, completion, "p95_ms"); got != 5000 {
		t.Fatalf("p95_ms = %v, want 5000", got)
	}

	// No receipts at all: count=0 and no invented percentile (AC-003).
	h.writeRaw(reportsFile, "")
	code, stdout, stderr = h.run("--outcome", "json")
	if code != 0 {
		t.Fatalf("empty-receipts exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	doc := decodeMetrics(t, stdout)
	completion = doc.metric(t, "task_completion")
	if got := numberField(t, completion, "count"); got != 0 {
		t.Fatalf("empty receipts count = %v, want 0", got)
	}
	requireAbsent(t, completion, "p50_ms")
	requireAbsent(t, completion, "p95_ms")
	if got := completion["insufficient_data"]; got != true {
		t.Fatalf("empty receipts insufficient_data = %v, want true", got)
	}
	if got := doc.status(t); got != MetricsInsufficientData {
		t.Fatalf("empty receipts status = %q, want %q", got, MetricsInsufficientData)
	}
}

// ---------------------------------------------------------------------------
// TC-MT-004 (AC-004): decisions_per_thread and fastpath_share
// ---------------------------------------------------------------------------

func TestMetricsPerThreadAndFastShare(t *testing.T) {
	h := newMetricsHarness(t)

	// 6 decisions over 3 threads (t-1: 3, t-2: 2, t-3: 1) -> ratio 6/3 = 2.
	// 4 of them carry a fastpath reason code -> share 4/6 = 0.67.
	reasons := []string{
		ReasonBoundThread, ReasonBoundThread, RouteDeduped,
		RouteDeduped, ReasonNewWork, ReasonQuestion,
	}
	threads := []string{"t-1", "t-1", "t-1", "t-2", "t-2", "t-3"}
	decisions := make([]traceRecord, 0, len(reasons))
	for i := range reasons {
		decisions = append(decisions, decisionRow(threads[i], "m-per-"+string(rune('0'+i)), reasons[i], metricsBase))
	}
	h.writeDecisions(decisions)
	h.writeReports([]workerReport{})

	code, stdout, stderr := h.run("--outcome", "json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	doc := decodeMetrics(t, stdout)
	perThread := doc.metric(t, "decisions_per_thread")
	if got := numberField(t, perThread, "decisions"); got != 6 {
		t.Fatalf("decisions = %v, want 6", got)
	}
	if got := numberField(t, perThread, "threads"); got != 3 {
		t.Fatalf("threads = %v, want 3 (distinct only)", got)
	}
	if got := numberField(t, perThread, "ratio"); got != 2 {
		t.Fatalf("ratio = %v, want 2", got)
	}
	share := doc.metric(t, "fastpath_share")
	if got := numberField(t, share, "fast"); got != 4 {
		t.Fatalf("fast = %v, want 4 (deduped + bound_thread only)", got)
	}
	if got := numberField(t, share, "total"); got != 6 {
		t.Fatalf("total = %v, want 6", got)
	}
	if got := numberField(t, share, "ratio"); got != 0.67 {
		t.Fatalf("ratio = %v, want 0.67 (two decimals)", got)
	}

	// 1 decision over 3 threads: the ratio is rounded to two decimals, not
	// truncated and not invented.
	h.writeDecisions([]traceRecord{
		decisionRow("t-1", "m-a", ReasonNewWork, metricsBase),
		decisionRow("t-2", "m-b", ReasonNewWork, metricsBase),
		decisionRow("t-3", "m-c", ReasonNewWork, metricsBase),
	})
	code, stdout, stderr = h.run("--outcome", "json")
	if code != 0 {
		t.Fatalf("rounding exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	doc = decodeMetrics(t, stdout)
	if got := numberField(t, doc.metric(t, "decisions_per_thread"), "ratio"); got != 1 {
		t.Fatalf("3 decisions / 3 threads = %v, want 1", got)
	}
	share = doc.metric(t, "fastpath_share")
	if got := numberField(t, share, "fast"); got != 0 {
		t.Fatalf("fast = %v, want 0 (no fastpath reason codes)", got)
	}
	if got := numberField(t, share, "ratio"); got != 0 {
		t.Fatalf("ratio = %v, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// TC-MT-005 (AC-005): missing or empty ledgers are explicit, never invented
// ---------------------------------------------------------------------------

func TestMetricsEmptyIsExplicit(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(h *metricsHarness)
	}{
		{
			name:    "no files at all",
			prepare: func(h *metricsHarness) {},
		},
		{
			name: "empty files",
			prepare: func(h *metricsHarness) {
				h.writeRaw(decisionsFile, "")
				h.writeRaw("adbd-slowpath.jsonl", "")
				h.writeRaw(reportsFile, "")
			},
		},
		{
			name: "whitespace-only files",
			prepare: func(h *metricsHarness) {
				h.writeRaw(decisionsFile, "\n  \n\n")
				h.writeRaw("adbd-slowpath.jsonl", "\n\n")
				h.writeRaw(reportsFile, "\n   \n")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newMetricsHarness(t)
			tc.prepare(h)

			code, stdout, stderr := h.run("--outcome", "json")
			if code != 0 {
				t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
			}
			doc := decodeMetrics(t, stdout)
			if got := doc.status(t); got != MetricsInsufficientData {
				t.Fatalf("status = %q, want %q", got, MetricsInsufficientData)
			}
			for _, name := range []string{"time_to_first_worker", "task_completion"} {
				metric := doc.metric(t, name)
				if got := numberField(t, metric, "count"); got != 0 {
					t.Fatalf("%s.count = %v, want 0", name, got)
				}
				requireAbsent(t, metric, "p50_ms")
				requireAbsent(t, metric, "p95_ms")
				if got := metric["insufficient_data"]; got != true {
					t.Fatalf("%s.insufficient_data = %v, want true", name, got)
				}
			}
			perThread := doc.metric(t, "decisions_per_thread")
			if got := numberField(t, perThread, "decisions"); got != 0 {
				t.Fatalf("decisions = %v, want 0", got)
			}
			if got := numberField(t, perThread, "threads"); got != 0 {
				t.Fatalf("threads = %v, want 0", got)
			}
			if got := numberField(t, perThread, "ratio"); got != 0 {
				t.Fatalf("ratio = %v, want 0", got)
			}
			share := doc.metric(t, "fastpath_share")
			if got := numberField(t, share, "fast"); got != 0 {
				t.Fatalf("fast = %v, want 0", got)
			}
			if got := numberField(t, share, "total"); got != 0 {
				t.Fatalf("total = %v, want 0", got)
			}
			if got := numberField(t, share, "ratio"); got != 0 {
				t.Fatalf("ratio = %v, want 0", got)
			}

			// The text outcome is just as explicit and still exits 0.
			code, textOut, stderr := h.run()
			if code != 0 {
				t.Fatalf("text exit = %d, want 0 (stderr: %s)", code, stderr)
			}
			if strings.Contains(textOut, "p50_ms") || strings.Contains(textOut, "p95_ms") {
				t.Fatalf("text outcome invented a percentile:\n%s", textOut)
			}
			if !strings.Contains(textOut, "status="+MetricsInsufficientData) {
				t.Fatalf("text outcome = %q, want status=%s", textOut, MetricsInsufficientData)
			}
		})
	}
}
