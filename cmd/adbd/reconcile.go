// Reconcile: worker receipts and exec_state write-back (truth.md, feature
// worker-report, v0.1.5).
//
// The gap this file closes: a dispatched thread that finished was
// indistinguishable from one that hung. The watchdog called it stalled and the
// requirement index kept saying exec_state=unknown, because nothing ever looked
// at the worker after the hand-off.
//
// The whole design rests on one decision (Q1 of the truth package): completion
// is *observed*, never *reported*. adbd asks paseo for the agent's Status,
// remembers the last answer on the binding (Binding.LastSeenStatus), and treats
// `running -> idle` as the completion event. No worker cooperation, no callback,
// no new protocol.
//
// What one pass does, in order:
//
//  1. read every binding that still holds a lease (Store.View, no write);
//  2. ask the injectable status runner for each distinct agent id;
//  3. append one receipt per newly finished agent to .beacon/adbd-reports.jsonl;
//  4. merge their exec_state into .beacon/adbd-exec-state.json (atomic write);
//  5. only then flip the bindings to `done`.
//
// The order is deliberate. A receipt is never lost to a crash between two writes
// because the ledgers are written before the state moves, and every ledger write
// is idempotent (receipts dedupe on agent_id, exec_state is keyed by req_id), so
// replaying the pass cannot double-count anything (AC-005).
//
// AC-006 is the other half of that: when paseo cannot be asked -- missing binary,
// non-zero exit, unparsable output -- the pass stops before the first write and
// reports reason_code=paseo_unavailable with exit code 2. Nothing is mutated, so
// the next pass starts from exactly the same place. A silent "no news" would be
// the worst outcome: it would look like "nothing finished" forever.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Files this feature owns inside .beacon/ (truth.md AC-002/AC-003).
const (
	// reportsFile is the worker receipt ledger: one JSON line per finished agent.
	reportsFile = "adbd-reports.jsonl"
	// execStateFile is what `adb index generate` merges (AC-004).
	execStateFile = "adbd-exec-state.json"
)

// Observed agent statuses, normalised (truth.md 状态模型: exec unknown -> running
// -> done | failed).
const (
	// StatusRunning is the observed "the agent is working" status.
	StatusRunning = "running"
	// StatusIdle is the observed "the agent went quiet" status.
	StatusIdle = "idle"
	// ReconcileGrace is how long after a dispatch an idle agent may still be
	// counted as "finished". A worker that starts and stops between two passes
	// would otherwise never be observed as running and never report completion.
	ReconcileGrace = 60 * time.Second
	// StatusMissing marks an agent paseo no longer knows. It is not an observed
	// agent status; the binder uses it to release the stale binding.
	StatusMissing = "missing"
)

// ReasonPaseoUnavailable is the AC-006 machine token: the status source could not
// be read, so the pass did nothing at all.
const ReasonPaseoUnavailable = "paseo_unavailable"

// reconcileClock is the CLI clock seam, mirroring watchdogClock. Tests inject a
// fake so finished_at and updated_at are exact instants, not wall-clock noise.
var reconcileClock = func() time.Time { return time.Now() }

// ---------------------------------------------------------------------------
// the status source (the only subprocess seam in this feature)
// ---------------------------------------------------------------------------

// StatusRunner answers "what is this agent doing right now?".
//
// It is an interface for the same reason the router's Dispatcher is one: the
// reconcile loop must be testable without a real paseo, and a test must be able
// to prove the AC-006 failure paths (missing binary, non-zero exit, unparsable
// output) for real.
type StatusRunner interface {
	// Status returns paseo's raw Status string for one agent id.
	Status(agentID string) (string, error)
}

// PaseoStatusRunner reads the status through `paseo inspect <id> --json`.
//
// paseo is treated as an opaque external CLI: argv only, no import, no forking.
// The JSON contract this file depends on is exactly one field, `Status`.
type PaseoStatusRunner struct {
	// Bin is the paseo executable; empty means "paseo" from PATH.
	Bin string
}

func (r PaseoStatusRunner) bin() string {
	if bin := strings.TrimSpace(r.Bin); bin != "" {
		return bin
	}
	return "paseo"
}

// Status runs one `paseo inspect <id> --json` and parses the Status field.
func (r PaseoStatusRunner) Status(agentID string) (string, error) {
	bin := r.bin()
	out, err := exec.Command(bin, "inspect", agentID, "--json").Output()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail = strings.TrimSpace(string(exitErr.Stderr))
		}
		if isAgentMissing(detail) {
			return "", &AgentMissingError{AgentID: agentID}
		}
		return "", fmt.Errorf("runner %s inspect %s --json: %w (output: %s)",
			bin, agentID, err, detail)
	}
	return parseInspectStatus(out)
}

// AgentMissingError means paseo answered "that agent does not exist". It is a
// per-binding condition (the worker was deleted or archived), not a transport
// failure: one stale binding must not abort the whole pass.
type AgentMissingError struct {
	AgentID string
}

func (e *AgentMissingError) Error() string {
	return "paseo has no agent " + e.AgentID
}

func isAgentMissing(detail string) bool {
	d := strings.ToLower(detail)
	return strings.Contains(d, "agent_not_found") || strings.Contains(d, "agent not found")
}

// parseInspectStatus reads the Status field out of `paseo inspect --json`.
//
// The field name is matched case-insensitively because paseo's own casing is not
// this repo's to freeze; everything else is strict on purpose. No Status field, a
// non-string Status or a non-JSON document is an error, not an empty answer --
// guessing "idle" here would silently invent a completion (AC-006).
func parseInspectStatus(out []byte) (string, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(out), &payload); err != nil {
		return "", fmt.Errorf("paseo inspect --json printed no JSON object (output: %s)",
			strings.TrimSpace(string(out)))
	}
	for key, raw := range payload {
		if !strings.EqualFold(key, "status") {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("paseo inspect --json printed a non-string Status (output: %s)",
				strings.TrimSpace(string(out)))
		}
		if value = strings.TrimSpace(value); value == "" {
			return "", fmt.Errorf("paseo inspect --json printed an empty Status")
		}
		return value, nil
	}
	return "", fmt.Errorf("paseo inspect --json printed no Status field (output: %s)",
		strings.TrimSpace(string(out)))
}

// normaliseStatus folds paseo's vocabulary onto the two statuses this feature
// acts on. An unrecognised status normalises to "" -- observed, but not decisive:
// a "waiting for permission" agent is neither finished nor making progress, and
// inventing one of the two would be a lie in both directions.
func normaliseStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "running", "working", "busy", "thinking", "in_progress", "in-progress":
		return StatusRunning
	case "idle", "done", "complete", "completed", "finished", "stopped", "exited":
		return StatusIdle
	}
	return ""
}

// PaseoUnavailableError marks a pass that could not read the status source at
// all. The CLI maps it to reason_code=paseo_unavailable and exit code 2.
type PaseoUnavailableError struct {
	// AgentID is the agent whose status could not be read, when there was one.
	AgentID string
	// Err is the underlying cause.
	Err error
}

func (e *PaseoUnavailableError) Error() string {
	if e.AgentID != "" {
		return fmt.Sprintf("%s: %v (agent %s)", ReasonPaseoUnavailable, e.Err, e.AgentID)
	}
	return fmt.Sprintf("%s: %v", ReasonPaseoUnavailable, e.Err)
}

func (e *PaseoUnavailableError) Unwrap() error { return e.Err }

// ---------------------------------------------------------------------------
// the two ledgers
// ---------------------------------------------------------------------------

// workerReport is one line of .beacon/adbd-reports.jsonl. AC-002 froze the field
// set at exactly these five keys, in this order, so none of them is omitempty:
// a receipt with a missing field is not a receipt.
type workerReport struct {
	Thread       string `json:"thread"`
	AgentID      string `json:"agent_id"`
	DispatchedAt string `json:"dispatched_at"`
	FinishedAt   string `json:"finished_at"`
	Status       string `json:"status"`
}

// execStateEntry is one value of .beacon/adbd-exec-state.json (AC-003).
type execStateEntry struct {
	ExecState string `json:"exec_state"`
	Worker    string `json:"worker"`
	Thread    string `json:"thread"`
	UpdatedAt string `json:"updated_at"`
}

// execStateDoc is the whole file: req_id -> entry.
type execStateDoc map[string]execStateEntry

// reqIDForThread maps an intake thread onto the requirement index primary key:
// “skill:<project>:<slug>“ -> “req:<project>:<slug>“ (AC-003).
//
// Only that exact three-part shape produces an id. Everything else -- a github
// thread, a feishu thread, a malformed skill thread -- returns "", which is what
// "非 skill thread 不产生条目" means: no entry at all, never a guessed one. The
// project/slug alphabet is the one feed.go already froze, so the reconcile side
// can never invent a req_id the index would not recognise.
func reqIDForThread(thread string) string {
	parts := strings.Split(strings.TrimSpace(thread), ":")
	if len(parts) != 3 || parts[0] != feedThreadPrefix {
		return ""
	}
	if !feedPartPattern.MatchString(parts[1]) || !feedPartPattern.MatchString(parts[2]) {
		return ""
	}
	return "req:" + parts[1] + ":" + parts[2]
}

// readReportAgentIDs returns every agent id that already has a receipt. The
// ledger itself is the dedupe source (AC-005): the file is append-only, so the
// set is rebuilt from it instead of from a second bookkeeping file that could
// drift.
func readReportAgentIDs(path string) (map[string]bool, error) {
	out := map[string]bool{}
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
		var report workerReport
		if err := json.Unmarshal([]byte(line), &report); err != nil {
			// A damaged ledger is loud, never silently skipped: quietly
			// re-appending receipts is how a ledger stops being trustworthy.
			return nil, fmt.Errorf("adbd reconcile: %s line %d is not a worker report: %w",
				path, i+1, err)
		}
		if id := strings.TrimSpace(report.AgentID); id != "" {
			out[id] = true
		}
	}
	return out, nil
}

// appendReports writes every new receipt in one write, so a killed process never
// leaves half a line behind.
func appendReports(path string, reports []workerReport) error {
	if len(reports) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, report := range reports {
		data, err := json.Marshal(report)
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

// readExecState loads the exec_state document. A missing or empty file is an
// empty document (AC-004: "文件不存在时行为不变"); a corrupt one is an error,
// because merging into a document we failed to parse would silently drop every
// entry already written.
func readExecState(path string) (execStateDoc, error) {
	doc := execStateDoc{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("adbd reconcile: corrupt exec state %s: %w", path, err)
	}
	if doc == nil {
		doc = execStateDoc{}
	}
	return doc, nil
}

// writeExecState merges the new entries into the document and replaces it
// atomically (temp file + rename), so a reader -- `adb index generate` -- never
// sees a partially written file (AC-003).
func writeExecState(path string, entries execStateDoc) error {
	if len(entries) == 0 {
		return nil
	}
	doc, err := readExecState(path)
	if err != nil {
		return err
	}
	for reqID, entry := range entries {
		doc[reqID] = entry
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomicFile(path, append(data, '\n'))
}

// ---------------------------------------------------------------------------
// the pass
// ---------------------------------------------------------------------------

// ReconcileResult is one pass in the shape the CLI prints (AC-001).
//
// Checked is how many leased bindings were looked at, Running how many were seen
// working, Done how many finished *in this pass* (AC-005: an already-done binding
// is never counted again).
type ReconcileResult struct {
	Checked    int    `json:"checked"`
	Done       int    `json:"done"`
	Running    int    `json:"running"`
	ReasonCode string `json:"reason_code,omitempty"`
	// Completed names the threads that finished in this pass, for the audit
	// trail. The text output stays exactly the three AC-001 counters.
	Completed []string `json:"completed,omitempty"`
	// Released counts bindings whose worker no longer exists and were let go so
	// the next message resolves a live one.
	Released int `json:"released,omitempty"`
}

// Reconciler observes worker agents and writes the receipts nobody else writes.
// Like the watchdog it owns no daemon and no timer: one call, one pass, exit.
type Reconciler struct {
	// Store carries the state document (bindings).
	Store *Store
	// Runner reads one agent's status; nil is a configuration error, never a
	// silent no-op.
	Runner StatusRunner
	// Now is injectable so tests drive an exact clock.
	Now func() time.Time

	// Paths default to their .beacon/ neighbours of Store.Path.
	ReportsPath   string
	ExecStatePath string
}

// NewReconciler builds a Reconciler with the default paths.
func NewReconciler(store *Store, runner StatusRunner) *Reconciler {
	return &Reconciler{Store: store, Runner: runner, Now: reconcileClock}
}

func (r *Reconciler) now() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

func (r *Reconciler) beaconDir() string {
	return filepath.Dir(r.Store.Path)
}

func (r *Reconciler) reportsPath() string {
	if path := strings.TrimSpace(r.ReportsPath); path != "" {
		return path
	}
	return filepath.Join(r.beaconDir(), reportsFile)
}

func (r *Reconciler) execStatePath() string {
	if path := strings.TrimSpace(r.ExecStatePath); path != "" {
		return path
	}
	return filepath.Join(r.beaconDir(), execStateFile)
}

// reconcileTarget is one leased binding this pass will look at, copied out of the
// snapshot so no lock is held while a child process runs.
type reconcileTarget struct {
	// key is the binding map key, used to find the row again under the lock.
	key        string
	thread     string
	agentID    string
	state      string
	lastSeen   string
	dispatched time.Time
}

// leaseTargets lists the bindings that still hold a lease, thread-sorted so the
// output is deterministic (map order is not).
//
// "Holds a lease" is exact here: a live state *and* an unexpired ExpiresAt. An
// expired lease is already on its way out (the fastpath expires it on the next
// touch), and asking paseo about it would only produce a receipt for work nobody
// is waiting on any more.
func leaseTargets(snap *snapshot, now time.Time) []reconcileTarget {
	out := []reconcileTarget{}
	for key, binding := range snap.Bindings {
		if binding == nil || binding.State == StateDone || !isLiveState(binding.State) {
			continue
		}
		if !now.Before(binding.ExpiresAt) {
			continue
		}
		agentID := strings.TrimSpace(binding.AgentID)
		if isPlaceholderAgentID(agentID) {
			continue
		}
		out = append(out, reconcileTarget{
			key:        key,
			thread:     binding.Thread,
			agentID:    agentID,
			state:      binding.State,
			lastSeen:   binding.LastSeenStatus,
			dispatched: dispatchedAt(binding),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].thread != out[j].thread {
			return out[i].thread < out[j].thread
		}
		return out[i].agentID < out[j].agentID
	})
	return out
}

// dispatchedAt is the "we handed this over at" instant the receipt carries.
//
// The binding schema has one clock (UpdatedAt, stamped at bind/route time), and
// the lease carries the other two facts needed to recover it when UpdatedAt is
// absent. Nothing here invents a value: an unknown dispatch time is reported as
// such by the caller.
func dispatchedAt(binding *Binding) time.Time {
	if !binding.UpdatedAt.IsZero() {
		return binding.UpdatedAt
	}
	if binding.TTL > 0 {
		return binding.ExpiresAt.Add(-binding.TTL)
	}
	return binding.ExpiresAt
}

// Reconcile runs exactly one pass (AC-001..AC-006).
func (r *Reconciler) Reconcile() (ReconcileResult, error) {
	res := ReconcileResult{}
	if r.Store == nil || strings.TrimSpace(r.Store.Path) == "" {
		return res, errors.New("adbd reconcile: state path is empty")
	}
	if r.Runner == nil {
		return res, errors.New("adbd reconcile: no status runner configured")
	}
	now := r.now()

	var targets []reconcileTarget
	err := r.Store.View(func(snap *snapshot) error {
		targets = leaseTargets(snap, now)
		return nil
	})
	if err != nil {
		return res, err
	}
	if len(targets) == 0 {
		// AC-001: nothing leased -> checked=0, and nothing at all is written.
		return res, nil
	}
	res.Checked = len(targets)

	// Phase 1: read every status BEFORE writing anything. Two bindings may share
	// one agent, so each agent id is asked exactly once.
	statuses := map[string]string{}
	missingTargets := []reconcileTarget{}
	for _, target := range targets {
		if _, asked := statuses[target.agentID]; asked {
			continue
		}
		raw, err := r.Runner.Status(target.agentID)
		if err != nil {
			var missing *AgentMissingError
			if errors.As(err, &missing) {
				// The worker vanished (deleted/archived). Release the binding so
				// the next message resolves a live worker instead of retrying a
				// dead id forever, and keep the pass going.
				missingTargets = append(missingTargets, target)
				statuses[target.agentID] = StatusMissing
				continue
			}
			// AC-006: a transport failure mutates nothing -- not even for the
			// agents whose status was read successfully.
			res.ReasonCode = ReasonPaseoUnavailable
			return res, &PaseoUnavailableError{AgentID: target.agentID, Err: err}
		}
		statuses[target.agentID] = normaliseStatus(raw)
	}

	// Phase 2: judge. `running -> idle` is the only completion event (AC-002).
	done := []reconcileTarget{}
	for _, target := range targets {
		switch statuses[target.agentID] {
		case StatusRunning:
			res.Running++
		case StatusIdle:
			// Completion has two shapes: an observed running->idle edge, and an
			// agent that was dispatched, never caught running, and has been idle
			// past the grace window (it finished between two passes).
			if target.lastSeen == StatusRunning {
				done = append(done, target)
				continue
			}
			if target.lastSeen != StatusRunning && !target.dispatched.IsZero() &&
				now.Sub(target.dispatched) >= ReconcileGrace {
				done = append(done, target)
			}
		}
	}
	// Phase 3: receipts first (append-only, deduped by agent id), then exec_state
	// (merged, atomic), then the bindings. A crash in between loses nothing a
	// replay cannot produce, because both ledgers are idempotent.
	known, err := readReportAgentIDs(r.reportsPath())
	if err != nil {
		return res, err
	}
	reports := []workerReport{}
	entries := execStateDoc{}
	for _, target := range done {
		finishedAt := now.Format(time.RFC3339)
		if !known[target.agentID] {
			reports = append(reports, workerReport{
				Thread:       target.thread,
				AgentID:      target.agentID,
				DispatchedAt: target.dispatched.UTC().Format(time.RFC3339),
				FinishedAt:   finishedAt,
				Status:       StateDone,
			})
			known[target.agentID] = true
		}
		// exec_state is keyed by requirement, not by agent: the same worker may
		// finish several threads, and each of them has its own row (AC-003).
		if reqID := reqIDForThread(target.thread); reqID != "" {
			entries[reqID] = execStateEntry{
				ExecState: StateDone,
				Worker:    target.agentID,
				Thread:    target.thread,
				UpdatedAt: finishedAt,
			}
		}
	}
	if err := appendReports(r.reportsPath(), reports); err != nil {
		return res, err
	}
	if err := writeExecState(r.execStatePath(), entries); err != nil {
		return res, err
	}

	res.Done = len(done)
	for _, target := range done {
		res.Completed = append(res.Completed, target.thread)
	}
	if len(missingTargets) > 0 {
		if err := r.releaseMissing(missingTargets); err != nil {
			return res, err
		}
		res.Released = len(missingTargets)
	}
	doneKeys := map[string]bool{}
	for _, target := range done {
		doneKeys[target.key] = true
	}
	return r.rememberObservations(targets, statuses, now, res, doneKeys)
}

// rememberObservations flips the bindings and stores what was just observed.
//
// The update re-reads the snapshot under the lock and re-checks the binding: the
// status reads above happened outside the lock, so a concurrently released or
// re-bound thread is left alone rather than having another thread's evidence
// written onto it.
// releaseMissing drops bindings whose worker vanished. The thread keeps no
// lease: the next message re-resolves a worker (and may create one), which is
// strictly better than retrying a dead id forever.
func (r *Reconciler) releaseMissing(targets []reconcileTarget) error {
	return r.Store.Update(func(snap *snapshot) error {
		for _, target := range targets {
			binding := snap.Bindings[target.key]
			if binding == nil || binding.State == StateDone || !isLiveState(binding.State) {
				continue
			}
			if strings.TrimSpace(binding.AgentID) != target.agentID {
				continue
			}
			binding.State = StateReleased
			binding.UpdatedAt = time.Now().UTC()
			binding.LastSeenStatus = StatusMissing
		}
		return nil
	})
}

func (r *Reconciler) rememberObservations(
	targets []reconcileTarget,
	statuses map[string]string,
	now time.Time,
	res ReconcileResult,
	doneKeys map[string]bool,
) (ReconcileResult, error) {
	if err := r.Store.Update(func(snap *snapshot) error {
		for _, target := range targets {
			binding := snap.Bindings[target.key]
			if binding == nil || binding.State == StateDone || !isLiveState(binding.State) {
				// AC-005/AC-007: a done binding is terminal; nothing to observe.
				continue
			}
			if strings.TrimSpace(binding.AgentID) != target.agentID {
				// The thread was re-bound to another worker mid-pass.
				continue
			}
			status := statuses[target.agentID]
			if status == "" {
				// Unrecognised status: observed, but not worth remembering.
				continue
			}
			if doneKeys[target.key] || (status == StatusIdle && binding.LastSeenStatus == StatusRunning) {
				binding.LastSeenStatus = status
				// done -> working is illegal (truth.md 禁止的迁移): the only write
				// here is the one completion edge.
				binding.State = StateDone
				binding.UpdatedAt = now
				continue
			}
			// A running sighting deliberately does NOT renew UpdatedAt: the
			// watchdog judges progress, and "still running, still producing
			// nothing" must be able to reach adbd_stall_after.
			binding.LastSeenStatus = status
		}
		return nil
	}); err != nil {
		return res, err
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// CLI: adbd reconcile --once [--root] [--outcome json|text]
// ---------------------------------------------------------------------------

func runReconcile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var once bool
	var reportsPath, execStatePath string
	fs.BoolVar(&once, "once", false, "run one reconcile pass and exit")
	fs.StringVar(&reportsPath, "reports", "", "worker report ledger path")
	fs.StringVar(&execStatePath, "exec-state", "", "exec state document path")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if !once {
		fmt.Fprintln(stderr, "adbd reconcile: --once is required")
		return 2
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd reconcile: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd reconcile: %v\n", err)
		return 1
	}
	bin := cf.runnerBin
	if bin == "" {
		bin = envOr("ADBD_PASEO_BIN", cfg.PaseoBin)
	}
	if bin == "" {
		bin = "paseo"
	}
	reconciler := &Reconciler{
		Store:         &Store{Path: cf.statePath, SlowPath: cf.slowPath},
		Runner:        PaseoStatusRunner{Bin: bin},
		Now:           reconcileClock,
		ReportsPath:   reportsPath,
		ExecStatePath: execStatePath,
	}

	res, err := reconciler.Reconcile()

	// Print the outcome even when the pass failed: "exit 2" alone is exactly the
	// silent failure AC-006 forbids.
	if cf.outcome == "json" {
		if code := writeJSON(stdout, stderr, res); code != 0 {
			return code
		}
	} else if res.ReasonCode != "" {
		fmt.Fprintln(stdout, "reason_code="+res.ReasonCode)
	} else {
		fmt.Fprintf(stdout, "checked=%d done=%d running=%d\n", res.Checked, res.Done, res.Running)
	}

	if err != nil {
		var unavailable *PaseoUnavailableError
		if errors.As(err, &unavailable) {
			fmt.Fprintf(stderr, "adbd reconcile: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "adbd reconcile: %v\n", err)
		return 1
	}
	return 0
}
