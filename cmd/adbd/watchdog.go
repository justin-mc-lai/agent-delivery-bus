// Watchdog: the out-of-process night watchman (truth.md, feature watchdog,
// v0.1.5).
//
// The one rule this file exists for: judge PROGRESS, never liveness (AC-001). A
// worker that is still running but has produced nothing for adbd_stall_after is
// stalled; a thread whose binding moved a minute ago is fine even when the
// process that moved it is gone. Nothing here reads a pid, a socket or a process
// table -- the only evidence is what the state document, the decision trace and
// the router heartbeat recorded.
//
// Silence is the default (AC-003): a clean scan prints exactly "NOTHING" and
// writes nothing at all. Every intervention appends one fixed-shape line to
// .beacon/adbd-watchdog.jsonl (AC-004), and a stall that will not clear escalates
// instead of nagging (AC-005).
//
// Two ledgers, deliberately separate:
//
//   - .beacon/adbd-watchdog.jsonl is the alert ledger. Its six keys are frozen by
//     AC-004, which is why it carries no field for "this is the 3rd scan".
//   - .beacon/adbd-watch-state.json is the scan ledger: the consecutive-stall
//     counter AC-005 needs. It is written only by a scan that found something
//     wrong, so a clean scan still touches nothing (AC-003).
//
// os/exec appears nowhere in this file: the watchdog has to work when adbd itself
// is dead (AC-006), and the only process seam in this feature is the launchctl
// status probe in agent.go.
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
	"sort"
	"strings"
	"time"
)

// Files this feature owns inside .beacon/.
const (
	// watchdogFile is the alert ledger (AC-004).
	watchdogFile = "adbd-watchdog.jsonl"
	// heartbeatFile is what `adbd route` refreshes after every drain (AC-002).
	heartbeatFile = "adbd-heartbeat.json"
	// watchStateFile is the scan ledger: consecutive-stall counters (AC-005).
	watchStateFile = "adbd-watch-state.json"
)

// Alert kinds (truth.md 状态模型: alert none -> stalled | router_stalled |
// repeat_stall).
const (
	KindStalled       = "stalled"
	KindRouterStalled = "router_stalled"
	KindRepeatStall   = "repeat_stall"
)

// Reason codes are stable machine tokens, never prose.
const (
	ReasonWatchdogStalled       = "stalled"
	ReasonWatchdogRouterStalled = "router_stalled"
	ReasonWatchdogRepeatStall   = "repeat_stall"
	// ReasonWatchdogRequeue tags the queue line a --apply requeue puts back, so
	// the front desk can tell a re-armed message from a fresh intake.
	ReasonWatchdogRequeue = "watchdog_requeue"
	// ReasonLaunchctlUnavailable means the probe could not run at all, which is a
	// different operator fact from "not loaded" (AC-007).
	ReasonLaunchctlUnavailable = "launchctl_unavailable"
)

// The only two actions a watchdog intervention may record (AC-004).
const (
	ActionObserved = "observed"
	ActionRequeue  = "requeue"
	ActionEscalate = "escalate"
)

// Scan phases (truth.md 状态模型: idle -> scanning -> clean | alerted -> acted).
const (
	PhaseIdle     = "idle"
	PhaseScanning = "scanning"
	PhaseClean    = "clean"
	PhaseAlerted  = "alerted"
	PhaseActed    = "acted"
)

// Frozen defaults from truth.md 配置项.
const (
	DefaultStallAfter     = 20 * time.Minute
	DefaultHeartbeatAfter = 10 * time.Minute
	DefaultRepeatWindow   = 6 * time.Hour
	DefaultAgentInterval  = 10 * time.Minute
	// DefaultRepeatThreshold is AC-005's "3 consecutive scans".
	DefaultRepeatThreshold = 3
)

// watchdogClock is the CLI clock seam. Production reads the wall clock; tests
// inject a fake so "25 minutes of silence" is an exact instant, not a sleep.
var watchdogClock = time.Now

// heartbeatRecord is the whole content of .beacon/adbd-heartbeat.json. One key,
// because the only question the watchdog asks it is "when".
type heartbeatRecord struct {
	At string `json:"at"`
}

// watchAlert is one line of .beacon/adbd-watchdog.jsonl. AC-004 froze the field
// set at exactly these six keys, emitted in this order, so they are spelled out
// with no omitempty: `thread` is the empty string for router_stalled, and the key
// is still there.
type watchAlert struct {
	Kind       string `json:"kind"`
	Thread     string `json:"thread"`
	ReasonCode string `json:"reason_code"`
	Evidence   string `json:"evidence"`
	Action     string `json:"action"`
	At         string `json:"at"`
}

// Line is what one alert prints on stdout. `stalled thread=<t>` and
// `repeat_stall thread=<t>` keep the thread reachable by `grep`; router_stalled
// has no thread to name.
func (a watchAlert) Line() string {
	if a.Thread == "" {
		return a.Kind
	}
	return a.Kind + " thread=" + a.Thread
}

// watchThreadState is the per-thread scan ledger entry (AC-005).
type watchThreadState struct {
	// Streak counts consecutive stalled scans in the current stall episode.
	Streak int `json:"streak"`
	// LastStalledAt closes the episode: a gap longer than adbd_repeat_window
	// means the thread recovered at some point in between, so the count restarts.
	LastStalledAt string `json:"last_stalled_at,omitempty"`
}

// watchState is the whole scan ledger document.
type watchState struct {
	Version int                          `json:"version"`
	Threads map[string]*watchThreadState `json:"threads"`
}

func newWatchState() *watchState {
	return &watchState{Version: stateVersion, Threads: map[string]*watchThreadState{}}
}

// WatchResult is one scan in the shape the CLI prints.
type WatchResult struct {
	Phase   string       `json:"phase"`
	Clean   bool         `json:"clean"`
	Lines   []string     `json:"lines"`
	Alerts  []watchAlert `json:"alerts"`
	Applied int          `json:"applied"`
}

// Watchdog is the night watchman. It owns no daemon, no goroutine and no timer:
// it runs once per invocation and exits, which is exactly what makes it
// survivable (AC-006).
type Watchdog struct {
	// Store carries the state document (bindings) and the slow-path queue.
	Store *Store

	// Paths default to their .beacon/ neighbours of Store.Path.
	DecisionsPath  string
	WatchPath      string
	HeartbeatPath  string
	WatchStatePath string

	// StallAfter: how long a bound thread may go without real activity.
	StallAfter time.Duration
	// HeartbeatAfter: how old a router heartbeat may be.
	HeartbeatAfter time.Duration
	// RepeatWindow: the silence window for a repeated alert on one thread.
	RepeatWindow time.Duration
	// RepeatThreshold: consecutive stalled scans that turn into repeat_stall.
	RepeatThreshold int

	// Apply turns observation into action (requeue / escalate).
	Apply bool
	// Now is injectable so tests drive an exact clock.
	Now func() time.Time

	// Phase is the current slot of the watch state machine (truth.md 状态模型).
	Phase string
	// OnPhase observes the phase walk; tests use it to prove TC-WD-008.
	OnPhase func(string)
}

// NewWatchdog builds a Watchdog with the frozen defaults.
func NewWatchdog(store *Store) *Watchdog {
	return &Watchdog{
		Store:           store,
		StallAfter:      DefaultStallAfter,
		HeartbeatAfter:  DefaultHeartbeatAfter,
		RepeatWindow:    DefaultRepeatWindow,
		RepeatThreshold: DefaultRepeatThreshold,
		Now:             time.Now,
		Phase:           PhaseIdle,
	}
}

func (w *Watchdog) now() time.Time {
	if w.Now == nil {
		return time.Now().UTC()
	}
	return w.Now().UTC()
}

func (w *Watchdog) stallAfter() time.Duration {
	if w.StallAfter <= 0 {
		return DefaultStallAfter
	}
	return w.StallAfter
}

func (w *Watchdog) heartbeatAfter() time.Duration {
	if w.HeartbeatAfter <= 0 {
		return DefaultHeartbeatAfter
	}
	return w.HeartbeatAfter
}

func (w *Watchdog) repeatWindow() time.Duration {
	if w.RepeatWindow <= 0 {
		return DefaultRepeatWindow
	}
	return w.RepeatWindow
}

func (w *Watchdog) repeatThreshold() int {
	if w.RepeatThreshold < 2 {
		return DefaultRepeatThreshold
	}
	return w.RepeatThreshold
}

// beaconDir is the directory this scan reads and (only on an alert) writes.
func (w *Watchdog) beaconDir() string {
	return filepath.Dir(w.Store.Path)
}

func (w *Watchdog) decisionsPath() string {
	if path := strings.TrimSpace(w.DecisionsPath); path != "" {
		return path
	}
	return filepath.Join(w.beaconDir(), decisionsFile)
}

func (w *Watchdog) watchPath() string {
	if path := strings.TrimSpace(w.WatchPath); path != "" {
		return path
	}
	return filepath.Join(w.beaconDir(), watchdogFile)
}

func (w *Watchdog) heartbeatPath() string {
	if path := strings.TrimSpace(w.HeartbeatPath); path != "" {
		return path
	}
	return filepath.Join(w.beaconDir(), heartbeatFile)
}

func (w *Watchdog) watchStatePath() string {
	if path := strings.TrimSpace(w.WatchStatePath); path != "" {
		return path
	}
	return filepath.Join(w.beaconDir(), watchStateFile)
}

func (w *Watchdog) setPhase(phase string) {
	w.Phase = phase
	if w.OnPhase != nil {
		w.OnPhase(phase)
	}
}

// ---------------------------------------------------------------------------
// activity: what counts as progress (AC-001)
// ---------------------------------------------------------------------------

// threadActivity is the newest decision the router recorded for a thread.
type threadActivity struct {
	At        time.Time
	Channel   string
	MessageID string
}

// readThreadActivity returns the newest decided_at per thread. A line whose
// decided_at is unparsable is skipped rather than fatal: a damaged clock field in
// one audit line must never blind the night watchman.
func readThreadActivity(path string) (map[string]threadActivity, error) {
	out := map[string]threadActivity{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return out, nil
		}
		return nil, err
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec traceRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("adbd watchdog: %s line %d is not a decision trace: %w", path, i+1, err)
		}
		at, err := time.Parse(time.RFC3339, rec.DecidedAt)
		if err != nil {
			continue
		}
		prev, ok := out[rec.Thread]
		if !ok || at.After(prev.At) {
			out[rec.Thread] = threadActivity{At: at, Channel: rec.Channel, MessageID: rec.MessageID}
		}
	}
	return out, nil
}

// threadView is one bound thread as a single scan sees it.
type threadView struct {
	Channel string
	Thread  string
	AgentID string
	State   string
	// LastActivity is max(binding.UpdatedAt, newest decided_at for the thread).
	// A worker report moves binding.UpdatedAt (Core.Report), so the three sources
	// AC-001 names -- decision, binding update, worker report -- collapse into
	// these two clocks without the watchdog needing a third file.
	LastActivity time.Time
	// Source names which of the two clocks won, for the evidence string.
	Source        string
	LastMessageID string
	Idle          time.Duration
}

// evidence is one line of key=value facts an operator can act on without
// reading any other file.
func (v threadView) evidence(now time.Time, threshold time.Duration) string {
	return fmt.Sprintf("last_activity=%s idle=%s threshold=%s source=%s binding=%s agent_id=%s message=%s",
		v.LastActivity.Format(time.RFC3339), formatDuration(v.Idle), formatDuration(threshold),
		v.Source, v.State, orNone(v.AgentID), orNone(v.LastMessageID))
}

func orNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

// watchedThreads builds the thread table this scan judges, newest binding first
// by thread id so the output is deterministic (map order is not).
func watchedThreads(snap *snapshot, activity map[string]threadActivity, now time.Time) []threadView {
	views := map[string]threadView{}
	for _, binding := range snap.Bindings {
		if binding == nil || binding.State == StateDone || !isLiveState(binding.State) {
			// Only live leases are watched: a released or unbound thread owes
			// nobody progress, and watching it would nag forever. A `done`
			// binding (feature worker-report) is finished work, not a stall:
			// skipping it here is what stops the false alarm (AC-007).
			continue
		}
		last := binding.UpdatedAt
		source := "binding"
		// "从未有过动作的 thread 按绑定创建时间算": this schema stamps creation into
		// UpdatedAt, and the creation time is derivable from the lease otherwise.
		if last.IsZero() {
			if binding.TTL > 0 {
				last = binding.ExpiresAt.Add(-binding.TTL)
			} else {
				last = binding.ExpiresAt
			}
		}
		if dec, ok := activity[binding.Thread]; ok && dec.At.After(last) {
			last = dec.At
			source = "decision"
		}
		view := threadView{
			Channel:       binding.Channel,
			Thread:        binding.Thread,
			AgentID:       binding.AgentID,
			State:         binding.State,
			LastActivity:  last,
			Source:        source,
			LastMessageID: activity[binding.Thread].MessageID,
			Idle:          now.Sub(last),
		}
		if prev, ok := views[binding.Thread]; !ok || view.Idle < prev.Idle {
			// Two channels may reuse a thread id; AC-004 froze the alert shape at
			// one `thread` key, so a thread is one identity here. Keep the healthiest
			// view of it, so a shared name cannot raise a false alarm.
			views[binding.Thread] = view
		}
	}
	out := make([]threadView, 0, len(views))
	for _, view := range views {
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Thread < out[j].Thread })
	return out
}

// ---------------------------------------------------------------------------
// heartbeat (AC-002)
// ---------------------------------------------------------------------------

// readHeartbeat returns the router's last drain time. A missing, empty or
// unreadable heartbeat is reported as absent: "心跳文件不存在等同过期".
func readHeartbeat(path string) (time.Time, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}
	var rec heartbeatRecord
	if err := json.Unmarshal(bytes.TrimSpace(data), &rec); err != nil {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(rec.At))
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

// writeHeartbeat stamps one drain (AC-002). It is called by the router, never by
// the watchdog.
func writeHeartbeat(path string, now time.Time) error {
	data, err := json.Marshal(heartbeatRecord{At: now.Format(time.RFC3339)})
	if err != nil {
		return err
	}
	return writeAtomicFile(path, append(data, '\n'))
}

// ---------------------------------------------------------------------------
// scan ledger (AC-005)
// ---------------------------------------------------------------------------

func readWatchState(path string) (*watchState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newWatchState(), nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return newWatchState(), nil
	}
	state := newWatchState()
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("adbd watchdog: corrupt scan ledger %s: %w", path, err)
	}
	if state.Threads == nil {
		state.Threads = map[string]*watchThreadState{}
	}
	return state, nil
}

func writeWatchState(path string, state *watchState) error {
	if state.Threads == nil {
		state.Threads = map[string]*watchThreadState{}
	}
	state.Version = stateVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomicFile(path, append(data, '\n'))
}

// nextStreak continues the current stall episode or starts a new one: a gap
// longer than the repeat window means the thread was healthy in between.
func (w *Watchdog) nextStreak(state *watchThreadState, now time.Time) int {
	if state == nil || state.Streak <= 0 || state.LastStalledAt == "" {
		return 1
	}
	last, err := time.Parse(time.RFC3339, state.LastStalledAt)
	if err != nil || now.Sub(last) > w.repeatWindow() {
		return 1
	}
	return state.Streak + 1
}

// ---------------------------------------------------------------------------
// alert ledger (AC-004, AC-005 dedupe)
// ---------------------------------------------------------------------------

type alertHistory struct {
	Kind string
	At   time.Time
}

// readAlertHistory returns the newest alert per thread. The alert ledger itself is
// the dedupe source: losing the scan ledger can then cost a streak restart but
// never a duplicate alert inside the window.
func readAlertHistory(path string) (map[string]alertHistory, error) {
	history := map[string]alertHistory{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return history, nil
		}
		return nil, err
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var alert watchAlert
		if err := json.Unmarshal([]byte(line), &alert); err != nil {
			return nil, fmt.Errorf("adbd watchdog: %s line %d is not a watchdog trace: %w", path, i+1, err)
		}
		at, err := time.Parse(time.RFC3339, alert.At)
		if err != nil {
			continue
		}
		prev, ok := history[alert.Thread]
		if !ok || at.After(prev.At) {
			history[alert.Thread] = alertHistory{Kind: alert.Kind, At: at}
		}
	}
	return history, nil
}

// suppressed reports whether this alert repeats the last one of the same kind for
// the same thread inside the silence window (AC-005). A different kind -- the
// escalated repeat_stall after a stalled -- is a new fact and is always written.
func suppressed(history map[string]alertHistory, thread, kind string, now time.Time, window time.Duration) bool {
	last, ok := history[thread]
	if !ok {
		return false
	}
	return last.Kind == kind && now.Sub(last.At) < window
}

// appendAlerts writes one line per alert, in one write, so a killed process never
// leaves half a line behind.
func appendAlerts(path string, alerts []watchAlert) error {
	if len(alerts) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, alert := range alerts {
		data, err := json.Marshal(alert)
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

// writeAtomicFile replaces a file with temp+rename, so a reader never sees a
// partially written heartbeat or ledger.
func writeAtomicFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".adbd-write-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
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

// ---------------------------------------------------------------------------
// the scan
// ---------------------------------------------------------------------------

// Scan runs exactly one watch pass: read the three sources, judge progress, and
// -- only when something is wrong -- leave a trace and, under Apply, act.
func (w *Watchdog) Scan() (WatchResult, error) {
	res := WatchResult{Phase: PhaseIdle, Lines: []string{}, Alerts: []watchAlert{}}
	w.setPhase(PhaseScanning)
	now := w.now()

	if w.Store == nil || strings.TrimSpace(w.Store.Path) == "" {
		return res, errors.New("adbd watchdog: state path is empty")
	}
	// Read-only load: no lock file, no rewrite, so a clean scan leaves the
	// directory byte-for-byte and mtime-for-mtime identical (AC-003).
	snap, err := w.Store.load()
	if err != nil {
		return res, err
	}
	activity, err := readThreadActivity(w.decisionsPath())
	if err != nil {
		return res, err
	}
	history, err := readAlertHistory(w.watchPath())
	if err != nil {
		return res, err
	}
	streaks, err := readWatchState(w.watchStatePath())
	if err != nil {
		return res, err
	}

	alerts := []watchAlert{}
	ledgerDirty := false

	// AC-001: progress, not liveness.
	for _, view := range watchedThreads(snap, activity, now) {
		if view.Idle <= w.stallAfter() {
			continue
		}
		streak := w.nextStreak(streaks.Threads[view.Thread], now)
		kind, reason := KindStalled, ReasonWatchdogStalled
		if streak >= w.repeatThreshold() {
			// AC-005: three scans stuck in a row is no longer "maybe it is
			// thinking", it is a repeat stall.
			kind, reason = KindRepeatStall, ReasonWatchdogRepeatStall
		}
		alert := watchAlert{
			Kind:       kind,
			Thread:     view.Thread,
			ReasonCode: reason,
			Evidence:   view.evidence(now, w.stallAfter()),
			Action:     ActionObserved,
			At:         now.Format(time.RFC3339),
		}
		if w.Apply {
			if kind == KindStalled {
				// The first stuck scan is re-armed: unbind the owner and put the
				// message back on the slow path, so the front desk decides again.
				note, err := w.requeue(view, now)
				if err != nil {
					return res, err
				}
				if note != "" {
					alert.Evidence += " " + note
				}
				alert.Action = ActionRequeue
				res.Applied++
			} else {
				// A repeat stall already burned a requeue: a human decides now.
				alert.Action = ActionEscalate
			}
		}
		streaks.Threads[view.Thread] = &watchThreadState{Streak: streak, LastStalledAt: alert.At}
		ledgerDirty = true
		res.Lines = append(res.Lines, alert.Line())
		// A requeue changed the world (it released a binding), so its line is never
		// silenced: the ledger has to be able to explain a release on its own. The
		// repeat window exists to stop nagging, not to hide interventions.
		if alert.Action == ActionRequeue || !suppressed(history, view.Thread, kind, now, w.repeatWindow()) {
			alerts = append(alerts, alert)
		}
	}

	// AC-002: the router is watched too. A heartbeat that is missing, unreadable
	// or older than adbd_heartbeat_after means the front desk stopped draining.
	beatAt, beatOK := readHeartbeat(w.heartbeatPath())
	if !beatOK || now.Sub(beatAt) > w.heartbeatAfter() {
		alert := watchAlert{
			Kind:       KindRouterStalled,
			Thread:     "",
			ReasonCode: ReasonWatchdogRouterStalled,
			Evidence:   heartbeatEvidence(beatAt, beatOK, now, w.heartbeatAfter()),
			Action:     ActionObserved,
			At:         now.Format(time.RFC3339),
		}
		if w.Apply {
			// Nothing this process can restart for the router: escalation is the
			// whole action, and it stays visible in the trace.
			alert.Action = ActionEscalate
		}
		res.Lines = append(res.Lines, alert.Line())
		if !suppressed(history, alert.Thread, alert.Kind, now, w.repeatWindow()) {
			alerts = append(alerts, alert)
		}
	}

	// Trace first, scan ledger second: if the ledger write fails, the next scan
	// may repeat an alert, and the dedupe above still silences it. The reverse
	// order could advance the streak without ever recording the alert.
	if err := appendAlerts(w.watchPath(), alerts); err != nil {
		return res, err
	}
	if ledgerDirty {
		if err := writeWatchState(w.watchStatePath(), streaks); err != nil {
			return res, err
		}
	}

	res.Alerts = alerts
	res.Clean = len(res.Lines) == 0
	switch {
	case res.Clean:
		res.Phase = PhaseClean
	case res.Applied > 0:
		res.Phase = PhaseActed
	default:
		res.Phase = PhaseAlerted
	}
	w.setPhase(res.Phase)
	return res, nil
}

func heartbeatEvidence(at time.Time, ok bool, now time.Time, threshold time.Duration) string {
	if !ok {
		return "heartbeat=missing threshold=" + formatDuration(threshold)
	}
	return fmt.Sprintf("heartbeat=%s age=%s threshold=%s",
		at.Format(time.RFC3339), formatDuration(now.Sub(at)), formatDuration(threshold))
}

// requeue is the only mutating intervention (AC-004 action=requeue): unbind the
// stalled thread and put its last message back on the slow path.
//
// The message text is deliberately not invented here: the decision trace records
// who/what/when but never the body (AC-004 of the router froze its field set), so
// a requeue re-arms the message identity and lets the front desk re-decide. On a
// purely fast-path thread there is no message at all to re-arm, and the honest
// outcome is unbind-only, which the evidence string says out loud.
func (w *Watchdog) requeue(view threadView, now time.Time) (string, error) {
	unbound := false
	err := w.Store.Update(func(snap *snapshot) error {
		binding := snap.Bindings[bindingKey(view.Channel, view.Thread)]
		if binding == nil || !isLiveState(binding.State) {
			return nil
		}
		next, err := Transition(binding.State, EventRelease)
		if err != nil {
			// expiring -> released is illegal; an expiring lease expires instead.
			next, err = Transition(binding.State, EventExpireRelease)
			if err != nil {
				return err
			}
		}
		applyState(binding, next, now)
		unbound = true
		return nil
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(view.LastMessageID) == "" {
		return "requeued=none unbound=" + boolToken(unbound), nil
	}
	// A fresh message id keeps the router's idempotency boundary honest: the
	// dispatched attempt keeps its own trace line, and the re-armed message gets a
	// second decision instead of being skipped as a replay.
	rec := slowPathRecord{
		Channel:    view.Channel,
		Thread:     view.Thread,
		MessageID:  view.LastMessageID + "#watchdog-" + now.UTC().Format("20060102T150405Z"),
		Text:       "",
		ReasonCode: ReasonWatchdogRequeue,
		RecordedAt: now.Format(time.RFC3339),
	}
	if err := w.Store.appendSlowPath(rec); err != nil {
		return "", err
	}
	return "requeued=" + rec.MessageID + " unbound=" + boolToken(unbound), nil
}

func boolToken(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

// ---------------------------------------------------------------------------
// CLI: adbd watchdog --once [--apply] [--outcome json|text]
// ---------------------------------------------------------------------------

func runWatchdog(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("watchdog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var once, apply bool
	var decisionsPath, watchPath, heartbeatPath, watchStatePath string
	fs.BoolVar(&once, "once", false, "run one watch pass and exit")
	fs.BoolVar(&apply, "apply", false, "act on a stall (requeue/escalate) instead of only observing")
	fs.StringVar(&decisionsPath, "decisions", "", "decision trace path")
	fs.StringVar(&watchPath, "watch", "", "watchdog trace path")
	fs.StringVar(&heartbeatPath, "heartbeat", "", "router heartbeat path")
	fs.StringVar(&watchStatePath, "watch-state", "", "watchdog scan ledger path")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if !once {
		fmt.Fprintln(stderr, "adbd watchdog: --once is required")
		return 2
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd watchdog: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd watchdog: %v\n", err)
		return 1
	}
	watch := NewWatchdog(&Store{Path: cf.statePath, SlowPath: cf.slowPath})
	watch.DecisionsPath = decisionsPath
	watch.WatchPath = watchPath
	watch.HeartbeatPath = heartbeatPath
	watch.WatchStatePath = watchStatePath
	watch.StallAfter = cfg.StallAfter
	watch.HeartbeatAfter = cfg.HeartbeatAfter
	watch.RepeatWindow = cfg.RepeatWindow
	watch.Apply = apply
	watch.Now = watchdogClock

	res, err := watch.Scan()
	if err != nil {
		fmt.Fprintf(stderr, "adbd watchdog: %v\n", err)
		return 1
	}
	if cf.outcome == "json" {
		return writeJSON(stdout, stderr, res)
	}
	if res.Clean {
		// AC-003: exactly one line, nothing else.
		fmt.Fprintln(stdout, "NOTHING")
		return 0
	}
	for _, line := range res.Lines {
		fmt.Fprintln(stdout, line)
	}
	return 0
}
