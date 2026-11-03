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
//   - Every inbound text carries a source trust (AC-007, AC-010). A trusted
//     source -- a channel named in adbd_trusted_channels -- is a work request:
//     its trace says source_trust=trusted, its text reaches the worker with no
//     guard line, and a high-risk verb in it is not by itself an escalation.
//     Everything else is untrusted: it is tagged in the trace, the dispatched
//     text is prefixed with the guard line, high-risk verbs are escalated
//     instead of dispatched, and a reply over the word limit is rejected.
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
	// ReasonMaxActiveReached is AC-008 of the agent-supervisor lake: the ceiling
	// on owned, working agents is reached, so no new worker is created and the
	// queue item keeps its place instead of being lost.
	ReasonMaxActiveReached = "max_active_reached"
)

const (
	// UntrustedGuardLine starts every text handed to a worker for an untrusted
	// source (AC-007). A trusted source never gets it (AC-010).
	UntrustedGuardLine = "UNTRUSTED_INPUT"
	// SourceTrustTrusted / SourceTrustUntrusted are the only two values the trace
	// may carry for source_trust (AC-010).
	SourceTrustTrusted   = "trusted"
	SourceTrustUntrusted = "untrusted"
	// DefaultMaxReplyRunes is the reply ceiling from AC-007.
	DefaultMaxReplyRunes = 100
	// DefaultMaxDispatchAttempts is the retry ceiling from AC-005.
	DefaultMaxDispatchAttempts = 3
	// DefaultRouterAgentID is a placeholder, NOT a worker.
	//
	// AC-009: the router may never send work to an id it has not seen paseo
	// confirm, and "main" is exactly such an unverified name. The constant stays
	// only so the placeholder can be recognized (and refused) by name; it is
	// never used as a dispatch target.
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

// decidersFor builds the pair the router drains with (AC-008): the primary
// decider, and -- only when the operator chose adbd_decider=llm -- the
// deterministic rules that take over a batch the model failed to answer. label
// is what the trace records; the rule path stays implicit (see traceDecider).
var decidersFor = func(cfg Config) (Decider, Decider, string, error) {
	if strings.EqualFold(strings.TrimSpace(cfg.Decider), DeciderLabelLLM) {
		llm, err := NewLLMDecider(cfg)
		if err != nil {
			return nil, nil, "", err
		}
		return llm, RuleDecider{}, DeciderLabelLLM, nil
	}
	return deciderFor(), nil, DeciderLabelRule, nil
}

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
	// Trusted is drain-time only too: it is recomputed on every drain from
	// adbd_trusted_channels (AC-010), so changing the config re-classifies queued
	// text instead of being frozen at the moment it arrived.
	Trusted bool `json:"-"`
}

// Decision is one decider verdict for one queue item.
type Decision struct {
	Channel    string
	Thread     string
	MessageID  string
	Decision   string
	ReasonCode string
	// Reply is the text the router says back. reply-delivery AC-001: the router
	// hands it to the injected Replier; with no Replier it stays trace-only.
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

// WorkerResolver is the optional second capability of a Dispatcher: it turns a
// thread plus a candidate id into a worker paseo has actually confirmed exists
// (AC-009). A Dispatcher that does not implement it can still be dispatched to,
// but the router then refuses to invent a target: no bound and no configured id
// is an error, never "main".
type WorkerResolver interface {
	EnsureWorker(thread, current string) (string, error)
}

// traceRecord is one line of .beacon/adbd-decisions.jsonl. The field set is
// frozen by AC-004: exactly these eight keys, emitted in this order. AC-008 adds
// a ninth key, decider, which is only materialized when the verdict did not come
// from the default deterministic rules (see traceDecider).
type traceRecord struct {
	Channel    string `json:"channel"`
	Thread     string `json:"thread"`
	MessageID  string `json:"message_id"`
	Decision   string `json:"decision"`
	ReasonCode string `json:"reason_code"`
	BatchSize  int    `json:"batch_size"`
	Untrusted  bool   `json:"untrusted"`
	DecidedAt  string `json:"decided_at"`
	// SourceTrust is AC-010's addition to the frozen AC-004 key set: "trusted" or
	// "untrusted", always written. A row written before this change simply has no
	// source_trust key when it is read back, which no reader depends on -- the
	// dedupe index only looks at channel / thread / message_id.
	SourceTrust string `json:"source_trust"`
	// Decider is "rule" (implicit, omitted), "llm", or "llm_fallback_rules".
	Decider string `json:"decider,omitempty"`
	// Delivered is nil for every verdict that has no outbound step (route,
	// ignore, escalate, and a reply the router refused), so the eight-key shape
	// frozen by AC-004 stays intact for those rows. A reply row always carries it:
	// true when the sender accepted the text, false when it did not (AC-002).
	Delivered *bool `json:"delivered,omitempty"`
	// DeliveryReason is sent / not_configured / unsupported_channel /
	// send_failed, and only on a reply row (AC-002).
	DeliveryReason string `json:"delivery_reason,omitempty"`
	// DeliveryDetail is the sender's own explanation, capped at 200 characters,
	// and only when the send failed (AC-003).
	DeliveryDetail string `json:"delivery_detail,omitempty"`
}

// traceDecider maps a decider label to the value the trace carries. The default
// rule path writes no field at all: AC-004 froze the trace key set at eight and
// TC-RL-004 asserts exactly those keys, so only a non-default decider is spelled
// out -- which is precisely the case an operator needs to see.
func traceDecider(label string) string {
	label = strings.TrimSpace(label)
	if label == "" || label == DeciderLabelRule {
		return ""
	}
	return label
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
	// ReasonCode is set when the drain deliberately did less than it could, so a
	// scheduler can tell "nothing to do" from "refused on purpose" (AC-008).
	ReasonCode string `json:"reason_code,omitempty"`
}

// Router is the resident front desk.
type Router struct {
	// Store carries the queue path and the lock shared with the fastpath.
	Store *Store
	// Decider is the injected brain; the router never decides on its own.
	Decider Decider
	// Fallback is the deterministic decider used for a whole batch when Decider
	// fails (AC-008). It is nil on the rule path, where a decider failure is a
	// real infrastructure error and must not be papered over.
	Fallback Decider
	// DeciderLabel names the primary decider in the trace: "rule", "llm" or
	// "llm_fallback_rules". Empty means the default rules.
	DeciderLabel string
	// Dispatcher is the injected hands; the router owns no execution tool.
	Dispatcher Dispatcher
	// Replier is the injected mouth (reply-delivery AC-001). nil means reply
	// delivery is switched off, which is the default (AC-006): the verdict is
	// still traced, and the trace says not_configured.
	Replier Replier
	// ReplyChannel is the only channel a reply may go out on. Empty means
	// DefaultReplyChannel (feishu).
	ReplyChannel string
	// TracePath defaults to <queue dir>/adbd-decisions.jsonl.
	TracePath string
	// HeartbeatPath defaults to <queue dir>/adbd-heartbeat.json. Every drain
	// refreshes it, and the watchdog reads it as "the router is still draining"
	// (watchdog AC-002).
	HeartbeatPath string
	// AgentID is the configured standing worker (adbd_worker_agent_id). It is a
	// candidate, not a verdict: AC-009 requires it to be confirmed with paseo
	// before use, and empty means "create a worker instead".
	AgentID string
	// TTL is the lease granted by the binding a route decision creates.
	TTL time.Duration
	// Limit caps one drain at N items; 0 drains the whole queue (AC-001).
	Limit int
	// MaxAttempts is the retry ceiling before escalation (AC-005).
	MaxAttempts int
	// MaxReplyRunes is the reply ceiling (AC-007).
	MaxReplyRunes int
	// TrustedChannels is adbd_trusted_channels (AC-010): the channels whose text
	// is a work request, not hostile input. Nil or empty trusts nothing.
	TrustedChannels []string
	// MaxActive is adbd_max_active: the ceiling on owned agents that are working
	// right now (agent-supervisor AC-008). Zero or negative disables the ceiling.
	MaxActive int
	// ActiveCounter answers "how many owned agents are working right now?". It is
	// called at most once per drain, only when a route verdict needs a new worker,
	// and it is handed the drain's own bindings so it never reaches back into the
	// locked state document. A nil counter or a failed read leaves the ceiling
	// unapplied rather than blocking work behind a broken probe.
	ActiveCounter ActiveCounter
	// Now is injectable so the trace timestamp is exact in tests.
	Now func() time.Time
}

// ActiveCounter is the router's window onto the agent population (AC-008 of the
// agent-supervisor lake). It is a function, not a number, because one drain asks
// exactly once and the answer must not be a stale snapshot from an earlier run.
type ActiveCounter func(bindings []Binding) (int, error)

// NewRouter builds a Router with the frozen defaults.
func NewRouter(store *Store, decider Decider, dispatcher Dispatcher) *Router {
	return &Router{
		Store:      store,
		Decider:    decider,
		Dispatcher: dispatcher,
		// AC-009: no placeholder default. An empty AgentID means "configure no
		// standing worker", which makes the router resolve one per message (bound
		// id first, then a freshly created worker) instead of guessing "main".
		AgentID: "",
		// AC-010: TrustedChannels stays nil here, which trusts nothing. The CLI is
		// what loads adbd_trusted_channels (default github,skill) and hands it in,
		// so a Router built by hand fails closed instead of inheriting a trust
		// list nobody configured.
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

// agentID is the operator-configured standing worker (adbd_worker_agent_id / --
// agent-id). Empty stays empty: a placeholder is never promoted to a target
// (AC-009 ④).
func (r *Router) agentID() string {
	id := strings.TrimSpace(r.AgentID)
	if isPlaceholderAgentID(id) {
		return ""
	}
	return id
}

// placeholderAgentIDs name no verified worker. Sending work to one of them is
// the bug AC-009 exists to kill, so every path that picks a target filters them.
var placeholderAgentIDs = map[string]struct{}{
	DefaultRouterAgentID: {},
}

// isPlaceholderAgentID reports whether id is a name that must never be used as a
// dispatch target. Empty counts as no id at all.
func isPlaceholderAgentID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return true
	}
	_, placeholder := placeholderAgentIDs[id]
	return placeholder
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

// isTrustedChannel answers whether this router treats the channel's text as a
// work request (AC-010). It is asked once per item, at drain time, so the answer
// always reflects the configuration that is live now.
func (r *Router) isTrustedChannel(channel string) bool {
	return channelInList(r.TrustedChannels, channel)
}

// maxActive is the ceiling from AC-008 of the agent-supervisor lake; zero or
// negative means "no ceiling set", never "ceiling of zero".
func (r *Router) maxActive() int {
	if r.MaxActive <= 0 {
		return 0
	}
	return r.MaxActive
}

// needsNewWorker reports whether routing this item would *create* a worker. It
// is AC-008's precondition: the ceiling only applies to creation, so a thread
// that already owns a live worker (or that has a configured standing worker to
// reuse) is never blocked by it.
func (r *Router) needsNewWorker(binding *Binding) bool {
	if binding != nil && isLiveState(binding.State) && !isPlaceholderAgentID(binding.AgentID) {
		return false
	}
	return r.agentID() == ""
}

func (r *Router) tracePath() string {
	if path := strings.TrimSpace(r.TracePath); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(r.Store.SlowPath), decisionsFile)
}

func (r *Router) heartbeatPath() string {
	if path := strings.TrimSpace(r.HeartbeatPath); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(r.Store.SlowPath), heartbeatFile)
}

// beat records that the router is alive and draining, for the watchdog to read
// (watchdog AC-002). It is stamped on every completed drain, including the empty
// one: "no backlog" is a heartbeat, "no drain at all" is the failure the
// watchdog is looking for.
func (r *Router) beat(now time.Time) error {
	return writeHeartbeat(r.heartbeatPath(), now)
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
	now := r.now()
	if len(items) == 0 {
		// AC-001: an empty backlog calls the decider zero times and prints batch=0.
		// It still refreshes the heartbeat: the router drained, which is exactly
		// what the night watchman needs to know.
		if err := r.beat(now); err != nil {
			return res, err
		}
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
		// AC-010: the source trust the decider and the dispatch seam both read.
		batch[i].Trusted = r.isTrustedChannel(batch[i].Channel)
	}

	seen, err := readTraceIndex(r.tracePath())
	if err != nil {
		return res, err
	}

	// AC-008 (agent-supervisor): the ceiling on owned, working agents. The count
	// is asked for lazily -- only when a route verdict actually needs a brand new
	// worker -- because the probe runs an external CLI and most drains create
	// nothing. It is asked at most once per drain, and a probe that fails leaves
	// the ceiling off: an unreadable agent list must never be read as "the bus is
	// full".
	counted, activeKnown, active := false, false, 0
	activeCount := func() (int, bool) {
		if !counted {
			counted = true
			if r.maxActive() > 0 && r.ActiveCounter != nil {
				if count, err := r.ActiveCounter(bindingRows(snap)); err == nil {
					active, activeKnown = count, true
				}
			}
		}
		return active, activeKnown
	}

	// AC-001: exactly one decider call for the whole batch.
	res.Batch = len(batch)
	label := r.DeciderLabel
	decisions, err := r.Decider.Decide(batch)
	if err != nil {
		if r.Fallback == nil {
			return res, err
		}
		// AC-008: one failed batch (transport, non-2xx, timeout, unparsable) must
		// never stall the backlog. The deterministic rules decide this batch and
		// every line says so, so a silent downgrade is impossible.
		fallbackDecisions, fallbackErr := r.Fallback.Decide(batch)
		if fallbackErr != nil {
			return res, fmt.Errorf("adbd route: llm decider failed (%w) and the rules failed too: %v", err, fallbackErr)
		}
		decisions = fallbackDecisions
		label = DeciderLabelFallback
	}
	// trace is the local helper that stamps the freeze-time fields of one line:
	// batch size, clock and the decider that actually produced the verdict.
	trace := func(item QueueItem, decision, reason string) traceRecord {
		rec := newTrace(item, decision, reason, res.Batch, now)
		rec.Decider = traceDecider(label)
		return rec
	}
	byKey := make(map[string]Decision, len(decisions))
	for _, d := range decisions {
		byKey[dedupeKey(d.Channel, d.Thread, d.MessageID)] = d
	}

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
			// AC-008: a decider that returns no verdict for an item has not
			// decided it -- a model that drops a message_id gets no guessing in
			// return. The item keeps its queued state and is asked again next
			// drain; without an audit line it is not counted as applied.
			pending = append(pending, item)
			res.Failed++
			continue
		}
		decision, reject, usable := r.validate(item, decision)
		if !usable {
			// AC-002 / AC-007: an unusable verdict is recorded and the item is
			// left failed; it never re-enters the backlog. A rejected reply is
			// never sent, so its row carries no delivered / delivery_reason: the
			// frozen vocabulary has no honest token for "nothing was attempted".
			lines = append(lines, trace(item, decision.Decision, reject))
			seen[key] = struct{}{}
			res.Failed++
			continue
		}

		switch decision.Decision {
		case DecisionRoute:
			if item.Attempts >= r.maxAttempts() {
				// AC-005: the retries are spent. Escalate instead of dispatching
				// a fourth time.
				lines = append(lines, trace(item, DecisionEscalate, ReasonDispatchAttemptsExhausted))
				seen[key] = struct{}{}
				res.Applied++
				continue
			}
			// AC-009: resolve a worker paseo confirms exists before anything is
			// sent. A stale binding is not a reason to drop the message, and an
			// unverified name is never a target.
			needsNew := r.needsNewWorker(snap.Bindings[bindingKey(item.Channel, item.Thread)])
			if needsNew && r.maxActive() > 0 {
				if count, known := activeCount(); known && count >= r.maxActive() {
					// AC-008 (agent-supervisor): the ceiling on owned, working
					// agents is reached. No new worker, no send, no attempt burned --
					// the item keeps its place in the queue and the next drain
					// re-asks.
					item.ReasonCode = ReasonMaxActiveReached
					pending = append(pending, item)
					res.Failed++
					res.ReasonCode = ReasonMaxActiveReached
					continue
				}
			}
			agentID, err := r.resolveWorker(item, snap.Bindings[bindingKey(item.Channel, item.Thread)])
			if err == nil {
				err = r.dispatch(item, agentID)
			}
			if err != nil {
				// AC-005: backed_off -- the item keeps its attempts and stays
				// queued, so the next drain retries it. AC-009 ④: a worker that
				// could not be resolved is a dispatch failure, recorded as such.
				item.Attempts++
				item.ReasonCode = ReasonDispatchFailed
				if item.Attempts >= r.maxAttempts() {
					lines = append(lines, trace(item, DecisionEscalate, ReasonDispatchAttemptsExhausted))
					seen[key] = struct{}{}
					res.Applied++
					continue
				}
				pending = append(pending, item)
				res.Failed++
				continue
			}
			// The resolved id is what the binding keeps: the next message on this
			// thread reuses the confirmed worker instead of creating another one.
			if ensureBinding(snap, item, agentID, now, r.ttl()) {
				stateDirty = true
			}
			if needsNew && activeKnown {
				// Keep the ceiling honest inside one drain: a pass that creates two
				// workers must not sail past the limit on a count it read once.
				active++
			}
			lines = append(lines, trace(item, DecisionRoute, decision.ReasonCode))
			seen[key] = struct{}{}
			res.Applied++

		case DecisionReply:
			// reply-delivery AC-002: the verdict is terminal whatever the channel
			// does next, so the row is built first and the delivery outcome is
			// stamped into it afterwards. A send failure never fails the item
			// (AC-003): attempts stay untouched and the process keeps breathing.
			rec := trace(item, decision.Decision, decision.ReasonCode)
			r.attachDelivery(&rec, item, decision)
			lines = append(lines, rec)
			seen[key] = struct{}{}
			res.Applied++

		case DecisionIgnore, DecisionEscalate:
			// Accounting only: both are terminal, so the item leaves the queue.
			lines = append(lines, trace(item, decision.Decision, decision.ReasonCode))
			seen[key] = struct{}{}
			res.Applied++

		default:
			// validate() rejects everything outside the four, so this is only a
			// belt-and-braces branch.
			lines = append(lines, trace(item, DecisionInvalid, ReasonDecisionInvalid))
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
	// Watchdog AC-002: one drain, one heartbeat.
	if err := r.beat(now); err != nil {
		return res, err
	}
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

// resolveWorker answers "which real worker takes this thread?" (AC-009):
//
//	① a live bound id (or the configured default) that paseo confirms -> reuse
//	② otherwise ask the dispatcher to create one and parse its agentId
//	③ with no resolver and no usable id -> error, never a placeholder target
//
// The candidate handed to the resolver is the bound id first, then the configured
// default (adbd_worker_agent_id); empty means "none", so the resolver creates a
// worker instead of guessing.
func (r *Router) resolveWorker(item QueueItem, binding *Binding) (string, error) {
	candidate := ""
	if binding != nil && isLiveState(binding.State) {
		if id := strings.TrimSpace(binding.AgentID); !isPlaceholderAgentID(id) {
			// A bound thread keeps its own worker: route to the owner.
			candidate = id
		}
	}
	if candidate == "" {
		candidate = r.agentID()
	}

	if resolver, ok := r.Dispatcher.(WorkerResolver); ok {
		id, err := resolver.EnsureWorker(item.Thread, candidate)
		if err != nil {
			return "", err
		}
		if isPlaceholderAgentID(id) {
			// Belt and braces: a resolver that hands back "main" has not resolved
			// anything, and AC-009 forbids sending there.
			return "", fmt.Errorf("adbd route: resolver returned no verified worker for thread %q (got %q)",
				item.Thread, id)
		}
		return id, nil
	}

	if candidate == "" {
		// No resolver and no id: the honest answer is "cannot dispatch", not a
		// send to a placeholder nobody verified.
		return "", fmt.Errorf("adbd route: no verified worker for thread %q: the dispatcher cannot resolve one and no bound or configured agent id is set",
			item.Thread)
	}
	return candidate, nil
}

// dispatch is the only exit from the router to the outside world. The guard line
// is decided here and nowhere else, so no dispatcher can bypass it (AC-007) and
// no caller can stack a second one (AC-010).
func (r *Router) dispatch(item QueueItem, agentID string) error {
	if r.Dispatcher == nil {
		return errors.New("adbd route: no dispatcher configured")
	}
	return r.Dispatcher.Dispatch(item.Thread, agentID, dispatchText(item))
}

// dispatchText is the one place the guard line is applied (AC-007, AC-010).
//
//   - untrusted source -> the text starts with the guard line, exactly once:
//     guardedText is idempotent, so a text the intake boundary already tagged
//     (feed.go / feishu.go / github.go) is not tagged twice.
//   - trusted source   -> the text goes to the worker without the guard line. Any
//     tag the intake boundary added on the way in is stripped here, because the
//     trust decision belongs to the router (it is the only component that knows
//     adbd_trusted_channels) and the worker must see the request verbatim.
func dispatchText(item QueueItem) string {
	if item.Trusted {
		return unguardedText(item.Text)
	}
	return guardedText(item.Text)
}

// guardedText marks inbound text as untrusted data, not instructions.
//
// It is idempotent: the guard is already present exactly when the first line of
// the text is the guard line. That matters because a fed message is tagged once at
// the trigger boundary (feed.go) and once again here on its way out of the router;
// stacking the tag would turn one message into two guard lines.
func guardedText(text string) string {
	if strings.HasPrefix(text, UntrustedGuardLine) {
		return text
	}
	return UntrustedGuardLine + "\n" + text
}

// unguardedText removes the guard line the intake boundary may have added, so a
// trusted source reaches the worker exactly as it was written. Only a leading
// guard *line* is removed -- the guard plus its terminator -- never a mention of
// the word inside the body, so a request that happens to quote the token keeps
// every character it had.
func unguardedText(text string) string {
	if text == UntrustedGuardLine {
		return ""
	}
	if rest, ok := strings.CutPrefix(text, UntrustedGuardLine+"\r\n"); ok {
		return rest
	}
	if rest, ok := strings.CutPrefix(text, UntrustedGuardLine+"\n"); ok {
		return rest
	}
	return text
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
		if id := strings.TrimSpace(agentID); !isPlaceholderAgentID(id) && binding.AgentID != id {
			// AC-009: a worker resolved for this message replaces whatever id the
			// binding carried, so the next message reuses the confirmed one.
			binding.AgentID = id
		}
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
	trust := SourceTrustUntrusted
	if item.Trusted {
		trust = SourceTrustTrusted
	}
	return traceRecord{
		Channel:    item.Channel,
		Thread:     item.Thread,
		MessageID:  item.MessageID,
		Decision:   decision,
		ReasonCode: reason,
		BatchSize:  batch,
		// AC-007 / AC-010: the source decides the trust, and the trace states it
		// twice -- the frozen AC-004 boolean and the AC-010 token -- so an old
		// reader and a new one both see the same thing.
		Untrusted:   !item.Trusted,
		SourceTrust: trust,
		DecidedAt:   now.Format(time.RFC3339),
	}
}

// RuleDecider is the deterministic M2 decider (truth.md Open Question Q1).
//
// It reads a batch and answers each item in a fixed order, so the same backlog
// always produces the same trace:
//
//	high-risk verb + untrusted -> escalate / untrusted_high_risk
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
	case hasHighRiskVerb(item.Text) && !item.Trusted:
		// AC-007 / AC-010: only untrusted text may not ask for a destructive
		// verb. A trusted source is a work request -- "release v0.1.5" from
		// github or from a skill session is the job, not an injection attempt.
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
		agentID = envOr("ADBD_ROUTER_AGENT", cfg.WorkerAgentID)
	}

	// AC-008: the operator chooses the brain; a failed LLM batch falls back to
	// the deterministic rules inside the router, so the CLI only builds the pair.
	decider, fallback, deciderLabel, err := decidersFor(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "adbd route: %v\n", err)
		return 1
	}

	// thread-workdir AC-002/AC-004: the project registry decides where a new
	// worker lands, and the reports ledger is where a fallback is recorded. Both
	// paths are root-relative unless the operator configured an absolute one.
	workerDir := workerWorkdir(cfg.WorkerWorkdir, cf.root)
	projectsFile := projectsFilePath(cfg.ProjectsFile, cf.root)

	// reply-delivery AC-006: no adbd_reply_bin means no mouth at all. The replier
	// stays nil, so every reply row records not_configured and nothing is spawned.
	var replier Replier
	if replyBin := strings.TrimSpace(cfg.ReplyBin); replyBin != "" {
		replier = LarkReplier{Bin: replyBin, Identity: cfg.ReplyIdentity}
	}

	router := &Router{
		Store:        &Store{Path: cf.statePath, SlowPath: cf.slowPath},
		Decider:      decider,
		Fallback:     fallback,
		DeciderLabel: deciderLabel,
		Dispatcher: PaseoDispatcher{
			Bin:             bin,
			WorkerID:        agentID,
			Provider:        cfg.WorkerProvider,
			Model:           cfg.WorkerModel,
			Workdir:         workerDir,
			Mode:            cfg.WorkerMode,
			Registry:        loadProjectRegistry(projectsFile, cf.root),
			FallbackWorkdir: workerDir,
			ReportsPath:     filepath.Join(cf.root, ".beacon", reportsFile),
		},
		TracePath:     tracePath,
		AgentID:       agentID,
		Replier:       replier,
		ReplyChannel:  DefaultReplyChannel,
		TTL:           DefaultTTL,
		Limit:         limit,
		MaxAttempts:   DefaultMaxDispatchAttempts,
		MaxReplyRunes: DefaultMaxReplyRunes,
		MaxActive:     cfg.MaxActive,
		// AC-010: source trust comes from the operator config, never from the
		// message itself. An empty list here means every source is untrusted.
		TrustedChannels: cfg.TrustedChannels,
		Now:             time.Now,
	}
	// AC-008 (agent-supervisor): the ceiling counts owned agents that are working
	// right now, so the counter reads the same ownership truth supervision uses --
	// `paseo ls --json` narrowed to bound work items. Only wired when a ceiling is
	// configured, so a daemon that never set adbd_max_active pays nothing.
	if cfg.MaxActive > 0 {
		runner := PaseoCliRunner{Bin: bin}
		router.ActiveCounter = func(bindings []Binding) (int, error) {
			return CountActiveOwned(runner, bindings)
		}
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
	if res.ReasonCode != "" {
		// AC-008: the drain stopped on purpose; say so instead of looking idle.
		fmt.Fprintln(stdout, "reason_code="+res.ReasonCode)
	}
	// AC-002: a rejected decision never changes the exit code; it is recorded.
	return 0
}

// workerWorkdir prefers the operator-configured directory and falls back to the
// daemon root, so a created worker never ends up in an unrelated checkout.
func workerWorkdir(configured, root string) string {
	if w := strings.TrimSpace(configured); w != "" {
		return w
	}
	return root
}
