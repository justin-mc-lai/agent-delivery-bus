// Router loop for the Agent Delivery Bus (truth.md, feature router-loop,
// v0.1.5).
//
// The router is the slow-path front desk: it drains the backlog as ONE batch,
// asks an injected Decider for exactly one verdict per message (route / reply /
// ignore / escalate), applies the verdict and appends one audit line per applied
// verdict.
//
// Two hard constraints live in this file:
//
//   - The router owns no execution tool (AC-003). This file never spawns a
//     process: the only way work leaves it is the injected Dispatcher interface,
//     process spawning lives in dispatch.go, and `adbd route --list-tools`
//     enumerates the toolset at runtime, where it is empty.
//   - Every inbound text is untrusted (AC-007): it is tagged in the trace, the
//     dispatched text is prefixed with the guard line, high-risk verbs are
//     escalated instead of dispatched, and a reply over the word limit is
//     rejected.
//
// Storage note: the queue and the trace are the only state this loop owns. Both
// are plain JSONL so an operator can read them with `tail -f`, and the queue is
// rewritten atomically (temp file + rename) under the same lock the fastpath
// already uses, so a killed process loses nothing.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// The four decisions the router may take (truth.md 状态模型). Nothing else is
// accepted from a decider.
const (
	DecisionRoute    = "route"
	DecisionReply    = "reply"
	DecisionIgnore   = "ignore"
	DecisionEscalate = "escalate"

	// DecisionInvalid is never produced by a decider: it is the audit token the
	// router writes when a decider returns something outside the four above.
	DecisionInvalid = "decision_invalid"
)

// Reason codes are stable machine tokens, never prose: a caller can branch on
// them without parsing a sentence.
const (
	ReasonUntrustedHighRisk         = "untrusted_high_risk"
	ReasonBoundThread               = "bound_thread"
	ReasonQuestion                  = "question"
	ReasonNewWork                   = "new_work"
	ReasonReplyTooLong              = "reply_too_long"
	ReasonDecisionInvalid           = "decision_invalid"
	ReasonDispatchAttemptsExhausted = "dispatch_attempts_exhausted"
)

const (
	// UntrustedGuardLine starts every text handed to a worker (AC-007).
	UntrustedGuardLine = "UNTRUSTED_INPUT"
	// DefaultMaxReplyRunes is the reply ceiling from AC-007.
	DefaultMaxReplyRunes = 100
	// DefaultMaxDispatchAttempts is the retry ceiling from AC-005.
	DefaultMaxDispatchAttempts = 3
	// DefaultRouterAgentID is the worker the router hands new work to when the
	// thread brings no live binding of its own.
	DefaultRouterAgentID = "main"
	// decisionsFile is the audit trail name inside .beacon/.
	decisionsFile = "adbd-decisions.jsonl"
)

// highRiskVerbs never reach a worker: the router escalates them to a human.
var highRiskVerbs = []string{"delete", "push", "release", "deploy"}

// routerToolset is the router's execution toolset. It is empty by construction
// and it is all `adbd route --list-tools` can ever print (AC-003).
var routerToolset = []string{}

// deciderFor builds the decider the CLI uses. M2 is deterministic rules; M2.5
// swaps the body for an LLM decider without touching the loop above it. Tests
// override it to inject a stub.
var deciderFor = func() Decider { return RuleDecider{} }

// QueueItem is one slow-path queue line plus the drain-time view of it.
type QueueItem struct {
	Channel    string `json:"channel"`
	Thread     string `json:"thread"`
	MessageID  string `json:"message_id"`
	Text       string `json:"text"`
	ReasonCode string `json:"reason_code,omitempty"`
	RecordedAt string `json:"recorded_at,omitempty"`
	// Attempts counts failed dispatches (AC-005) and is persisted with the item,
	// so a restart keeps counting instead of retrying forever.
	Attempts int `json:"attempts,omitempty"`
	// Bound is drain-time only: the decider sees it, the queue never stores it.
	Bound bool `json:"-"`
}

// Decision is one decider verdict for one queue item.
type Decision struct {
	Channel    string
	Thread     string
	MessageID  string
	Decision   string
	ReasonCode string
	// Reply is the text the router would say back. reply is recorded, never
	// delivered (truth.md Open Question Q2: channel delivery lands in M4).
	Reply string
}

// Decider turns a whole batch of queue items into verdicts. AC-001: one call per
// drain, so a decider may look at the batch as a whole (cost, priority, dedupe)
// instead of being woken per message.
type Decider interface {
	Decide(items []QueueItem) ([]Decision, error)
}

// Dispatcher hands one guarded message to one worker. It is the router's only
// way to cause work outside this process (AC-003).
type Dispatcher interface {
	Dispatch(thread, agentID, text string) error
}

// traceRecord is one line of .beacon/adbd-decisions.jsonl. The field set is
// frozen by AC-004: exactly these eight keys, emitted in this order.
type traceRecord struct {
	Channel    string `json:"channel"`
	Thread     string `json:"thread"`
	MessageID  string `json:"message_id"`
	Decision   string `json:"decision"`
	ReasonCode string `json:"reason_code"`
	BatchSize  int    `json:"batch_size"`
	Untrusted  bool   `json:"untrusted"`
	DecidedAt  string `json:"decided_at"`
}

// DrainResult is one drain, in the shape the CLI prints.
type DrainResult struct {
	Batch   int           `json:"batch"`
	Decided int           `json:"decided"`
	Applied int           `json:"applied"`
	Failed  int           `json:"failed"`
	Skipped int           `json:"skipped"`
	Queued  int           `json:"queued"`
	Traces  []traceRecord `json:"traces"`
}

// Router is the resident front desk.
type Router struct {
	// Store carries the queue path and the lock shared with the fastpath.
	Store *Store
	// Decider is the injected brain; the router never decides on its own.
	Decider Decider
	// Dispatcher is the injected hands; the router owns no execution tool.
	Dispatcher Dispatcher
	// TracePath defaults to <queue dir>/adbd-decisions.jsonl.
	TracePath string
	// AgentID is the worker new work is dispatched to.
	AgentID string
	// TTL is the lease granted by the binding a route decision creates.
	TTL time.Duration
	// Limit caps one drain at N items; 0 drains the whole queue (AC-001).
	Limit int
	// MaxAttempts is the retry ceiling before escalation (AC-005).
	MaxAttempts int
	// MaxReplyRunes is the reply ceiling (AC-007).
	MaxReplyRunes int
	// Now is injectable so the trace timestamp is exact in tests.
	Now func() time.Time
}

// NewRouter builds a Router with the frozen defaults.
func NewRouter(store *Store, decider Decider, dispatcher Dispatcher) *Router {
	return &Router{
		Store:         store,
		Decider:       decider,
		Dispatcher:    dispatcher,
		AgentID:       DefaultRouterAgentID,
		TTL:           DefaultTTL,
		MaxAttempts:   DefaultMaxDispatchAttempts,
		MaxReplyRunes: DefaultMaxReplyRunes,
		Now:           time.Now,
	}
}

func (r *Router) now() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

func (r *Router) ttl() time.Duration {
	if r.TTL <= 0 {
		return DefaultTTL
	}
	return r.TTL
}

func (r *Router) agentID() string {
	if id := strings.TrimSpace(r.AgentID); id != "" {
		return id
	}
	return DefaultRouterAgentID
}

func (r *Router) limit() int {
	if r.Limit < 0 {
		return 0
	}
	return r.Limit
}

func (r *Router) maxAttempts() int {
	if r.MaxAttempts <= 0 {
		return DefaultMaxDispatchAttempts
	}
	return r.MaxAttempts
}

func (r *Router) maxReplyRunes() int {
	if r.MaxReplyRunes <= 0 {
		return DefaultMaxReplyRunes
	}
	return r.MaxReplyRunes
}

func (r *Router) tracePath() string {
	if path := strings.TrimSpace(r.TracePath); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(r.Store.SlowPath), decisionsFile)
}

// DrainOnce claims the backlog, decides it, applies the verdicts and rewrites
// the queue with what is still unapplied (AC-001, AC-006).
//
// It returns an error only for infrastructure failures (unreadable queue, dead
// decider, unwritable trace). A bad decision is not an error: it is recorded and
// the item is marked failed (AC-002), so the daemon keeps breathing.
func (r *Router) DrainOnce() (DrainResult, error) {
	res := DrainResult{Traces: []traceRecord{}}
	if r == nil || r.Store == nil || strings.TrimSpace(r.Store.SlowPath) == "" {
		return res, errors.New("adbd route: slow-path queue path is empty")
	}
	if r.Decider == nil {
		return res, errors.New("adbd route: no decider configured")
	}

	// One lock for the whole drain: claim, trace and rewrite are one critical
	// section, so a second drain can never see a half-consumed queue.
	release, err := r.Store.acquireLock()
	if err != nil {
		return res, err
	}
	defer release()

	items, err := readQueue(r.Store.SlowPath)
	if err != nil {
		return res, err
	}
	if len(items) == 0 {
		// AC-001: an empty backlog calls the decider zero times and prints batch=0.
		return res, nil
	}

	batch, tail := items, []QueueItem(nil)
	if limit := r.limit(); limit > 0 && len(items) > limit {
		batch, tail = items[:limit], items[limit:]
	}

	snap, err := r.Store.load()
	if err != nil {
		return res, err
	}
	for i := range batch {
		binding := snap.Bindings[bindingKey(batch[i].Channel, batch[i].Thread)]
		batch[i].Bound = binding != nil && isLiveState(binding.State)
	}

	seen, err := readTraceIndex(r.tracePath())
	if err != nil {
		return res, err
	}

	// AC-001: exactly one decider call for the whole batch.
	decisions, err := r.Decider.Decide(batch)
	if err != nil {
		return res, err
	}
	byKey := make(map[string]Decision, len(decisions))
	for _, d := range decisions {
		byKey[dedupeKey(d.Channel, d.Thread, d.MessageID)] = d
	}

	now := r.now()
	res.Batch = len(batch)
	pending := make([]QueueItem, 0, len(batch))
	lines := make([]traceRecord, 0, len(batch))
	stateDirty := false

	for _, item := range batch {
		key := dedupeKey(item.Channel, item.Thread, item.MessageID)

		// AC-006: a crash between "trace written" and "queue rewritten" replays
		// the item. The trace key is the idempotency boundary: a message that
		// already has an audit line is never decided or dispatched again.
		if _, done := seen[key]; done {
			res.Skipped++
			continue
		}

		decision, ok := byKey[key]
		if !ok {
			decision = Decision{
				Channel:   item.Channel,
				Thread:    item.Thread,
				MessageID: item.MessageID,
				Decision:  DecisionInvalid,
			}
		}
		decision, reject, usable := r.validate(item, decision)
		if !usable {
			// AC-002 / AC-007: an unusable verdict is recorded and the item is
			// left failed; it never re-enters the backlog.
			lines = append(lines, newTrace(item, decision.Decision, reject, res.Batch, now))
			seen[key] = struct{}{}
			res.Failed++
			continue
		}

		switch decision.Decision {
		case DecisionRoute:
			agentID := r.agentID()
			if binding := snap.Bindings[bindingKey(item.Channel, item.Thread)]; binding != nil &&
				isLiveState(binding.State) && strings.TrimSpace(binding.AgentID) != "" {
				// A bound thread keeps its own worker: route to the owner.
				agentID = binding.AgentID
			}
			if item.Attempts >= r.maxAttempts() {
				// AC-005: the retries are spent. Escalate instead of dispatching
				// a fourth time.
				lines = append(lines, newTrace(item, DecisionEscalate, ReasonDispatchAttemptsExhausted, res.Batch, now))
				seen[key] = struct{}{}
				res.Applied++
				continue
			}
			if err := r.dispatch(item, agentID); err != nil {
				// AC-005: backed_off -- the item keeps its attempts and stays
				// queued, so the next drain retries it.
				item.Attempts++
				if item.Attempts >= r.maxAttempts() {
					lines = append(lines, newTrace(item, DecisionEscalate, ReasonDispatchAttemptsExhausted, res.Batch, now))
					seen[key] = struct{}{}
					res.Applied++
					continue
				}
				pending = append(pending, item)
				res.Failed++
				continue
			}
			if ensureBinding(snap, item, agentID, now, r.ttl()) {
				stateDirty = true
			}
			lines = append(lines, newTrace(item, DecisionRoute, decision.ReasonCode, res.Batch, now))
			seen[key] = struct{}{}
			res.Applied++

		case DecisionReply, DecisionIgnore, DecisionEscalate:
			// reply is recorded, not delivered (Q2); ignore and escalate are
			// accounting only. All three are terminal, so the item leaves the
			// queue.
			lines = append(lines, newTrace(item, decision.Decision, decision.ReasonCode, res.Batch, now))
			seen[key] = struct{}{}
			res.Applied++

		default:
			// validate() rejects everything outside the four, so this is only a
			// belt-and-braces branch.
			lines = append(lines, newTrace(item, DecisionInvalid, ReasonDecisionInvalid, res.Batch, now))
			seen[key] = struct{}{}
			res.Failed++
		}
	}

	res.Decided = res.Applied + res.Failed
	res.Traces = lines

	// Order matters: trace first, queue second. A crash in between replays the
	// item into the dedupe path above, which drops it without dispatching. The
	// reverse order would lose the audit line for work that did happen.
	if err := appendTrace(r.tracePath(), lines); err != nil {
		return res, err
	}
	if stateDirty {
		if err := r.Store.save(snap); err != nil {
			return res, err
		}
	}
	next := append(pending, tail...)
	if err := writeQueue(r.Store.SlowPath, next); err != nil {
		return res, err
	}
	res.Queued = len(next)
	return res, nil
}

// validate keeps a decider honest (AC-002, AC-007). The fourth return value says
// whether the verdict may be applied; when it is false the third value is the
// reason code the trace must record.
func (r *Router) validate(item QueueItem, d Decision) (Decision, string, bool) {
	switch d.Decision {
	case DecisionRoute, DecisionIgnore, DecisionEscalate:
		return d, "", true
	case DecisionReply:
		if utf8.RuneCountInString(replyText(item, d)) > r.maxReplyRunes() {
			// AC-007: a wall of text is not a reply. Reject it here rather than
			// in the decider, so no decider (M2.5 included) can bypass the limit.
			d.ReasonCode = ReasonReplyTooLong
			return d, ReasonReplyTooLong, false
		}
		return d, "", true
	}
	// AC-002: anything outside the four is refused, whatever its spelling.
	bad := Decision{
		Channel:    d.Channel,
		Thread:     d.Thread,
		MessageID:  d.MessageID,
		Decision:   DecisionInvalid,
		ReasonCode: ReasonDecisionInvalid,
	}
	return bad, ReasonDecisionInvalid, false
}

// replyText is the text the limit applies to: the decider's reply, or the
// inbound text when the decider echoed nothing.
func replyText(item QueueItem, d Decision) string {
	if strings.TrimSpace(d.Reply) != "" {
		return d.Reply
	}
	return item.Text
}

// dispatch is the only exit from the router to the outside world. The guard line
// is added here, so no dispatcher can bypass it (AC-007).
func (r *Router) dispatch(item QueueItem, agentID string) error {
	if r.Dispatcher == nil {
		return errors.New("adbd route: no dispatcher configured")
	}
	return r.Dispatcher.Dispatch(item.Thread, agentID, guardedText(item.Text))
}

// guardedText marks inbound text as untrusted data, not instructions.
func guardedText(text string) string {
	return UntrustedGuardLine + "\n" + text
}

// ensureBinding binds the thread to the worker that just took the work, so the
// next message on that thread takes the fastpath. A released thread stays
// released: released -> bound is illegal (fastpath truth).
func ensureBinding(snap *snapshot, item QueueItem, agentID string, now time.Time, ttl time.Duration) bool {
	key := bindingKey(item.Channel, item.Thread)
	binding := snap.Bindings[key]
	if binding != nil && binding.State == StateReleased {
		return false
	}
	if binding != nil && isLiveState(binding.State) {
		lease := ttl
		if binding.TTL > 0 {
			lease = binding.TTL
		}
		binding.ExpiresAt = now.Add(lease)
		binding.UpdatedAt = now
		return true
	}
	snap.Bindings[key] = &Binding{
		Channel:   item.Channel,
		Thread:    item.Thread,
		AgentID:   agentID,
		State:     StateBound,
		TTL:       ttl,
		ExpiresAt: now.Add(ttl),
		UpdatedAt: now,
	}
	return true
}

func newTrace(item QueueItem, decision, reason string, batch int, now time.Time) traceRecord {
	return traceRecord{
		Channel:    item.Channel,
		Thread:     item.Thread,
		MessageID:  item.MessageID,
		Decision:   decision,
		ReasonCode: reason,
		BatchSize:  batch,
		Untrusted:  true, // AC-007: every inbound text is untrusted.
		DecidedAt:  now.Format(time.RFC3339),
	}
}

// RuleDecider is the deterministic M2 decider (truth.md Open Question Q1).
//
// It reads a batch and answers each item in a fixed order, so the same backlog
// always produces the same trace:
//
//	high-risk verb -> escalate / untrusted_high_risk (never dispatch sabotage)
//	bound thread   -> route    / bound_thread
//	question       -> reply    / question
//	anything else  -> route    / new_work
type RuleDecider struct{}

// compile-time proof that the rule decider satisfies the injection point.
var _ Decider = RuleDecider{}

// Decide answers the whole batch. It never returns an error: the rules cannot
// fail, and every unusable verdict is caught by the router's validation.
func (d RuleDecider) Decide(items []QueueItem) ([]Decision, error) {
	out := make([]Decision, 0, len(items))
	for _, item := range items {
		out = append(out, decideWithRules(item))
	}
	return out, nil
}

func decideWithRules(item QueueItem) Decision {
	decision := Decision{Channel: item.Channel, Thread: item.Thread, MessageID: item.MessageID}
	switch {
	case hasHighRiskVerb(item.Text):
		// AC-007: untrusted text may not ask for a destructive verb.
		decision.Decision = DecisionEscalate
		decision.ReasonCode = ReasonUntrustedHighRisk
	case item.Bound:
		decision.Decision = DecisionRoute
		decision.ReasonCode = ReasonBoundThread
	case isQuestion(item.Text):
		decision.Decision = DecisionReply
		decision.ReasonCode = ReasonQuestion
		decision.Reply = item.Text
	default:
		decision.Decision = DecisionRoute
		decision.ReasonCode = ReasonNewWork
	}
	return decision
}

// hasHighRiskVerb reports whether the text asks for a destructive verb, in any
// casing. Substring matching is deliberate: "please DePloy now" must trip too.
func hasHighRiskVerb(text string) bool {
	lowered := strings.ToLower(text)
	for _, verb := range highRiskVerbs {
		if strings.Contains(lowered, verb) {
			return true
		}
	}
	return false
}

// isQuestion reports whether the text ends with a question mark (ASCII or full
// width).
func isQuestion(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasSuffix(trimmed, "?") || strings.HasSuffix(trimmed, "？")
}

// ---------------------------------------------------------------------------
// queue + trace storage
// ---------------------------------------------------------------------------

// readQueue loads the whole backlog in order. A corrupt line is a loud error,
// not a silent drop: losing one message quietly is worse than stopping the drain.
func readQueue(path string) ([]QueueItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var items []QueueItem
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var item QueueItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("adbd route: %s line %d is not a queue item: %w", path, i+1, err)
		}
		items = append(items, item)
	}
	return items, nil
}

// writeQueue atomically replaces the queue with the still-unapplied items.
// Temp file + rename means a reader never sees a half-written backlog.
func writeQueue(path string, items []QueueItem) error {
	var buf bytes.Buffer
	for _, item := range items {
		item.Bound = false // drain-time only, never persisted
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".adbd-slowpath-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// readTraceIndex returns the (channel, thread, message_id) keys that already have
// an audit line. Those items are applied, whatever the queue still says.
func readTraceIndex(path string) (map[string]struct{}, error) {
	seen := map[string]struct{}{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return seen, nil
		}
		return nil, err
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec traceRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("adbd route: %s line %d is not a decision trace: %w", path, i+1, err)
		}
		seen[dedupeKey(rec.Channel, rec.Thread, rec.MessageID)] = struct{}{}
	}
	return seen, nil
}

// appendTrace writes one line per applied decision (AC-004). The append is a
// single write so a killed process leaves whole lines behind, never half of one.
func appendTrace(path string, lines []traceRecord) error {
	if len(lines) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, line := range lines {
		data, err := json.Marshal(line)
		if err != nil {
			return err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ---------------------------------------------------------------------------
// CLI: adbd route --once [--limit N] [--outcome json|text]
// ---------------------------------------------------------------------------

func runRoute(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var once, listTools bool
	var limit int
	var agentID, tracePath string
	fs.BoolVar(&once, "once", false, "drain the queue once and exit")
	fs.BoolVar(&listTools, "list-tools", false, "print the router execution toolset and exit")
	fs.IntVar(&limit, "limit", 0, "drain at most N items, 0 drains the whole queue")
	fs.StringVar(&agentID, "agent-id", "", "worker agent id for new dispatches")
	fs.StringVar(&tracePath, "trace", "", "decision trace path")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}

	// AC-003: the toolset is enumerated at runtime, and it is empty.
	if listTools {
		return writeJSON(stdout, stderr, routerToolset)
	}
	if !once {
		fmt.Fprintln(stderr, "adbd route: --once or --list-tools is required")
		return 2
	}
	if limit < 0 {
		fmt.Fprintf(stderr, "adbd route: --limit must be >= 0, got %d\n", limit)
		return 2
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd route: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd route: %v\n", err)
		return 1
	}
	bin := cf.runnerBin
	if bin == "" {
		bin = envOr("ADBD_PASEO_BIN", cfg.PaseoBin)
	}
	if bin == "" {
		bin = "paseo"
	}
	if tracePath == "" {
		tracePath = envOr("ADBD_TRACE", filepath.Join(cf.root, ".beacon", decisionsFile))
	}
	if agentID == "" {
		agentID = envOr("ADBD_ROUTER_AGENT", cfg.RouterAgent)
	}

	router := &Router{
		Store:         &Store{Path: cf.statePath, SlowPath: cf.slowPath},
		Decider:       deciderFor(),
		Dispatcher:    PaseoDispatcher{Bin: bin},
		TracePath:     tracePath,
		AgentID:       agentID,
		TTL:           DefaultTTL,
		Limit:         limit,
		MaxAttempts:   DefaultMaxDispatchAttempts,
		MaxReplyRunes: DefaultMaxReplyRunes,
		Now:           time.Now,
	}
	res, err := router.DrainOnce()
	if err != nil {
		fmt.Fprintf(stderr, "adbd route: %v\n", err)
		return 1
	}
	if cf.outcome == "json" {
		return writeJSON(stdout, stderr, res)
	}
	fmt.Fprintf(stdout, "batch=%d decided=%d applied=%d failed=%d skipped=%d queued=%d\n",
		res.Batch, res.Decided, res.Applied, res.Failed, res.Skipped, res.Queued)
	// AC-002: a rejected decision never changes the exit code; it is recorded.
	return 0
}
