// Metrics (feature metrics, v0.1.5): four effect metrics aggregated from the
// three ledgers adbd already writes.
//
// Nothing in this file adds an instrumentation point. The inputs are exactly the
// files the daemon was already appending to:
//
//	.beacon/adbd-decisions.jsonl  router verdicts        -> decided_at
//	.beacon/adbd-reports.jsonl    worker receipts        -> dispatched_at / finished_at
//	.beacon/adbd-slowpath.jsonl   slow-path queue        -> recorded_at
//
// The rule that outranks every nice-looking number: a percentile is printed only
// when it was computed from real samples (AC-005). A missing or empty ledger is
// count=0 plus status=insufficient_data, never a fabricated figure; fewer than
// MetricsMinSamples samples omit p95_ms and label that metric insufficient_data.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MetricsReport status values (truth.md AC-005).
const (
	// MetricsOK means every metric was computed from a full sample set.
	MetricsOK = "ok"
	// MetricsInsufficientData means at least one metric could not be computed
	// honestly. It is never promoted to ok (truth.md "禁止的迁移").
	MetricsInsufficientData = "insufficient_data"
)

// MetricsMinSamples is the floor below which a percentile is not reported at all
// (truth.md: 样本数 < 5 时不输出该分位并标 insufficient_data).
const MetricsMinSamples = 5

// fastpathReasons are the reason codes that count as "the fastpath answered this
// one" (AC-004). They are the two verdicts that skip a fresh Router/LLM
// decision: a replay that was deduped, and a message a live binding already owns.
var fastpathReasons = map[string]bool{
	"deduped":         true, // RouteDeduped
	ReasonBoundThread: true,
}

// latencyMetric is one latency summary: AC-002 / AC-003.
//
// P50MS and P95MS are pointers so "not computed" is representable as an absent
// key instead of a zero: count=0 must print no percentile at all.
type latencyMetric struct {
	Count            int      `json:"count"`
	P50MS            *float64 `json:"p50_ms,omitempty"`
	P95MS            *float64 `json:"p95_ms,omitempty"`
	InsufficientData bool     `json:"insufficient_data,omitempty"`
}

// decisionsPerThreadMetric is AC-004 part one: decisions / distinct threads.
type decisionsPerThreadMetric struct {
	Decisions int     `json:"decisions"`
	Threads   int     `json:"threads"`
	Ratio     float64 `json:"ratio"`
}

// fastpathShareMetric is AC-004 part two: deduped+bound_thread rows / all rows.
type fastpathShareMetric struct {
	Fast  int     `json:"fast"`
	Total int     `json:"total"`
	Ratio float64 `json:"ratio"`
}

// MetricsReport is the whole CLI outcome: exactly four metrics plus a status.
type MetricsReport struct {
	TimeToFirstWorker  latencyMetric            `json:"time_to_first_worker"`
	TaskCompletion     latencyMetric            `json:"task_completion"`
	DecisionsPerThread decisionsPerThreadMetric `json:"decisions_per_thread"`
	FastpathShare      fastpathShareMetric      `json:"fastpath_share"`
	Status             string                   `json:"status"`
}

// MetricsCollector reads the three ledgers and aggregates them. Every field is a
// path so a test (or an operator) can point at any root.
type MetricsCollector struct {
	// DecisionsPath defaults to <beacon>/adbd-decisions.jsonl.
	DecisionsPath string
	// ReportsPath defaults to <beacon>/adbd-reports.jsonl.
	ReportsPath string
	// SlowPathPath defaults to <beacon>/adbd-slowpath.jsonl.
	SlowPathPath string
	// Since filters records by their own event time; zero means everything.
	Since time.Time
}

// Collect reads each ledger once and returns the four metrics.
func (c *MetricsCollector) Collect() (MetricsReport, error) {
	decisions, err := loadDecisionSamples(c.DecisionsPath)
	if err != nil {
		return MetricsReport{}, err
	}
	recorded, err := loadSlowPathTimes(c.SlowPathPath)
	if err != nil {
		return MetricsReport{}, err
	}
	reports, err := loadReportSamples(c.ReportsPath)
	if err != nil {
		return MetricsReport{}, err
	}

	rep := MetricsReport{}
	rep.TimeToFirstWorker = summarizeLatency(firstWorkerLatencies(decisions, recorded, c.Since))
	rep.TaskCompletion = summarizeLatency(completionLatencies(reports, c.Since))
	rep.DecisionsPerThread, rep.FastpathShare = countDecisions(decisions, c.Since)
	rep.Status = rep.status()
	return rep, nil
}

// status is ok only when every metric stands on real samples.
func (r MetricsReport) status() string {
	if r.TimeToFirstWorker.InsufficientData || r.TaskCompletion.InsufficientData {
		return MetricsInsufficientData
	}
	if r.FastpathShare.Total == 0 || r.DecisionsPerThread.Threads == 0 {
		return MetricsInsufficientData
	}
	return MetricsOK
}

// decisionSample is one decision-trace row reduced to the fields metrics needs.
type decisionSample struct {
	Thread     string
	MessageID  string
	ReasonCode string
	DecidedAt  time.Time
	HasTime    bool
}

// reportSample is one worker receipt reduced to the two instants AC-003 needs.
type reportSample struct {
	DispatchedAt time.Time
	FinishedAt   time.Time
	HasTime      bool
}

// loadDecisionSamples reads .beacon/adbd-decisions.jsonl. A line whose
// decided_at does not parse keeps HasTime=false: it still counts as a decision
// (it is a real audit line) but it can never contribute a fabricated latency.
func loadDecisionSamples(path string) ([]decisionSample, error) {
	var out []decisionSample
	err := forEachJSONLine(path, func(line []byte, lineNo int) error {
		var rec traceRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("adbd metrics: %s line %d is not a decision trace: %w", path, lineNo, err)
		}
		sample := decisionSample{
			Thread:     strings.TrimSpace(rec.Thread),
			MessageID:  strings.TrimSpace(rec.MessageID),
			ReasonCode: strings.TrimSpace(rec.ReasonCode),
		}
		if at, err := time.Parse(time.RFC3339, strings.TrimSpace(rec.DecidedAt)); err == nil {
			sample.DecidedAt = at
			sample.HasTime = true
		}
		out = append(out, sample)
		return nil
	})
	return out, err
}

// loadReportSamples reads .beacon/adbd-reports.jsonl. A receipt needs both
// instants to describe a duration; a line missing either one contributes nothing
// (AC-003: 无回执时 count=0 且不编造分位).
func loadReportSamples(path string) ([]reportSample, error) {
	var out []reportSample
	err := forEachJSONLine(path, func(line []byte, lineNo int) error {
		var rec workerReport
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("adbd metrics: %s line %d is not a worker report: %w", path, lineNo, err)
		}
		dispatched, errD := time.Parse(time.RFC3339, strings.TrimSpace(rec.DispatchedAt))
		finished, errF := time.Parse(time.RFC3339, strings.TrimSpace(rec.FinishedAt))
		if errD != nil || errF != nil {
			out = append(out, reportSample{})
			return nil
		}
		out = append(out, reportSample{DispatchedAt: dispatched, FinishedAt: finished, HasTime: true})
		return nil
	})
	return out, err
}

// loadSlowPathTimes returns the earliest recorded_at per message_id. The queue is
// append-only, so the earliest instant is the moment the message first lost the
// fastpath -- the honest start of "time to first worker".
func loadSlowPathTimes(path string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	err := forEachJSONLine(path, func(line []byte, lineNo int) error {
		var rec slowPathRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("adbd metrics: %s line %d is not a slow-path record: %w", path, lineNo, err)
		}
		id := strings.TrimSpace(rec.MessageID)
		if id == "" {
			return nil
		}
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(rec.RecordedAt))
		if err != nil {
			return nil // an unparsable clock field yields no sample, never a guess
		}
		if prev, ok := out[id]; !ok || at.Before(prev) {
			out[id] = at
		}
		return nil
	})
	return out, err
}

// firstWorkerLatencies pairs a slow-path recorded_at with the first decision for
// the same message_id (AC-002).
func firstWorkerLatencies(decisions []decisionSample, recorded map[string]time.Time, since time.Time) []float64 {
	type pair struct{ recordedAt, decidedAt time.Time }
	first := map[string]pair{}
	for _, d := range decisions {
		if !d.HasTime || d.MessageID == "" {
			continue
		}
		start, ok := recorded[d.MessageID]
		if !ok {
			continue
		}
		if prev, seen := first[d.MessageID]; seen && !d.DecidedAt.Before(prev.decidedAt) {
			continue
		}
		first[d.MessageID] = pair{recordedAt: start, decidedAt: d.DecidedAt}
	}
	out := make([]float64, 0, len(first))
	for _, p := range first {
		if !since.IsZero() && p.decidedAt.Before(since) {
			continue
		}
		out = append(out, millis(p.decidedAt.Sub(p.recordedAt)))
	}
	return out
}

// completionLatencies turns each receipt into finished_at - dispatched_at
// (AC-003).
func completionLatencies(reports []reportSample, since time.Time) []float64 {
	out := make([]float64, 0, len(reports))
	for _, r := range reports {
		if !r.HasTime {
			continue
		}
		if !since.IsZero() && r.FinishedAt.Before(since) {
			continue
		}
		out = append(out, millis(r.FinishedAt.Sub(r.DispatchedAt)))
	}
	return out
}

// countDecisions computes AC-004 from one pass over the decision rows.
func countDecisions(decisions []decisionSample, since time.Time) (decisionsPerThreadMetric, fastpathShareMetric) {
	threads := map[string]bool{}
	fast, total := 0, 0
	for _, d := range decisions {
		// A row filtered out by --since is not "before the window" for counting
		// purposes only when its time is unknown: an unparsable decided_at cannot
		// be proven in-window, so it is dropped instead of guessed in.
		if !since.IsZero() && (!d.HasTime || d.DecidedAt.Before(since)) {
			continue
		}
		total++
		if d.Thread != "" {
			threads[d.Thread] = true
		}
		if fastpathReasons[d.ReasonCode] {
			fast++
		}
	}
	return decisionsPerThreadMetric{
			Decisions: total,
			Threads:   len(threads),
			Ratio:     ratio(total, len(threads)),
		}, fastpathShareMetric{
			Fast:  fast,
			Total: total,
			Ratio: ratio(fast, total),
		}
}

// summarizeLatency turns raw millisecond samples into one latency metric using
// the nearest-rank percentile and the frozen insufficiency floor.
func summarizeLatency(values []float64) latencyMetric {
	m := latencyMetric{Count: len(values)}
	if len(values) == 0 {
		// count=0: no percentile is printed, because none was computed.
		m.InsufficientData = true
		return m
	}
	sort.Float64s(values)
	p50 := nearestRank(values, 50)
	m.P50MS = &p50
	if len(values) < MetricsMinSamples {
		m.InsufficientData = true
		return m
	}
	p95 := nearestRank(values, 95)
	m.P95MS = &p95
	return m
}

// nearestRank is the nearest-rank percentile of an ascending slice:
// rank = ceil(p/100 * n), clamped to [1, n]. Sorted input is the caller's
// contract, and an empty slice has no rank at all.
func nearestRank(sorted []float64, p int) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := (p*n + 99) / 100 // integer ceiling of p*n/100, no float rounding
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

// millis renders a duration as milliseconds, keeping sub-millisecond precision
// instead of truncating it away.
func millis(d time.Duration) float64 {
	return math.Round(float64(d.Microseconds())) / 1000
}

// ratio rounds num/den to two decimals (AC-004). A zero denominator yields 0,
// which the status field marks as insufficient rather than "0% fastpath".
func ratio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return math.Round(float64(num)/float64(den)*100) / 100
}

// forEachJSONLine calls fn for every non-empty line of path. A missing or empty
// file calls fn zero times and returns no error: "no data yet" is a legitimate
// state (AC-005), not a failure.
func forEachJSONLine(path string, fn func(line []byte, lineNo int) error) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if err := fn([]byte(line), i+1); err != nil {
			return err
		}
	}
	return nil
}

// parseSince reads the --since flag: RFC3339 (fractional seconds allowed), and an
// empty value means "no window".
func parseSince(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("--since %q is not an RFC3339 instant (e.g. 2026-02-01T00:00:00Z)", raw)
	}
	return at, nil
}

// runMetrics is the CLI surface: adbd metrics --once [--since RFC3339]
// [--root DIR] [--outcome json|text] (AC-001).
func runMetrics(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("metrics", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var once bool
	var sinceRaw, decisionsPath, reportsPath string
	fs.BoolVar(&once, "once", false, "collect one metrics snapshot and exit")
	fs.StringVar(&sinceRaw, "since", "", "only count records at or after this RFC3339 instant")
	fs.StringVar(&decisionsPath, "decisions", "", "decision trace path")
	fs.StringVar(&reportsPath, "reports", "", "worker report ledger path")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	// Like reconcile and watchdog: the collector has no resident mode, every run
	// is exactly one pass, and saying so out loud keeps the launchd rhythm honest.
	if !once {
		fmt.Fprintln(stderr, "adbd metrics: --once is required")
		return 2
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd metrics: %v\n", err)
		return 2
	}
	since, err := parseSince(sinceRaw)
	if err != nil {
		fmt.Fprintf(stderr, "adbd metrics: %v\n", err)
		return 2
	}
	beacon := filepath.Dir(cf.statePath)
	if strings.TrimSpace(decisionsPath) == "" {
		decisionsPath = filepath.Join(beacon, decisionsFile)
	}
	if strings.TrimSpace(reportsPath) == "" {
		reportsPath = filepath.Join(beacon, reportsFile)
	}
	collector := &MetricsCollector{
		DecisionsPath: decisionsPath,
		ReportsPath:   reportsPath,
		SlowPathPath:  cf.slowPath,
		Since:         since,
	}
	report, err := collector.Collect()
	if err != nil {
		fmt.Fprintf(stderr, "adbd metrics: %v\n", err)
		return 1
	}
	if cf.outcome == "json" {
		return writeJSON(stdout, stderr, report)
	}
	fmt.Fprint(stdout, formatMetrics(report))
	return 0
}

// formatMetrics renders the text outcome: one line per metric, status last.
func formatMetrics(r MetricsReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "time_to_first_worker %s\n", formatLatency(r.TimeToFirstWorker))
	fmt.Fprintf(&b, "task_completion %s\n", formatLatency(r.TaskCompletion))
	fmt.Fprintf(&b, "decisions_per_thread decisions=%d threads=%d ratio=%s\n",
		r.DecisionsPerThread.Decisions, r.DecisionsPerThread.Threads, formatNumber(r.DecisionsPerThread.Ratio))
	fmt.Fprintf(&b, "fastpath_share fast=%d total=%d ratio=%s\n",
		r.FastpathShare.Fast, r.FastpathShare.Total, formatNumber(r.FastpathShare.Ratio))
	fmt.Fprintf(&b, "status=%s\n", r.Status)
	return b.String()
}

// formatLatency prints only the keys that were actually computed.
func formatLatency(m latencyMetric) string {
	parts := []string{fmt.Sprintf("count=%d", m.Count)}
	if m.P50MS != nil {
		parts = append(parts, "p50_ms="+formatNumber(*m.P50MS))
	}
	if m.P95MS != nil {
		parts = append(parts, "p95_ms="+formatNumber(*m.P95MS))
	}
	if m.InsufficientData {
		parts = append(parts, "insufficient_data=true")
	}
	return strings.Join(parts, " ")
}

// formatNumber prints a number in the same shortest round-tripping form
// encoding/json uses, so the text and JSON outcomes never disagree about a value.
func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
