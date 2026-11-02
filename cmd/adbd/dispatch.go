// Process spawning lives here and nowhere else in the router loop.
//
// Why a separate file: the router must be provably unable to execute anything on
// its own (truth.md AC-003). route.go imports no subprocess package at all; the
// single seam where the router reaches the outside world is the Dispatcher
// interface, and the only production implementation is this one -- `paseo send`,
// `paseo inspect` and `paseo run`, treated as an opaque external CLI (no import,
// no fork of its code, argv only).
//
// AC-009: sending is not enough. Before a message leaves the router, the target
// must be a worker paseo itself confirms exists -- a live bound id, the configured
// `adbd_worker_agent_id`, or one created on the spot by `paseo run`. A hardcoded
// name like "main" is never a target.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Defaults for creating a worker (truth.md AC-009 config table).
const (
	// DefaultWorkerProvider is adbd_worker_provider's default.
	DefaultWorkerProvider = "opencode"
	// DefaultWorkerModel is adbd_worker_model's default.
	DefaultWorkerModel = "deepseek/deepseek-flash"
	// DefaultWorkerWorkdir is used only when no root is configured; the daemon
	// always passes its real root.
	DefaultWorkerWorkdir = "."
	// DefaultWorkerMode is the operational mode a created worker runs in. The
	// provider default asks a human before touching anything (external_directory
	// requests pile up and the worker sits there), so unattended workers must be
	// created in an edit-capable mode.
	DefaultWorkerMode = "build"
)

// WorkerResolveError is the typed failure of the worker-resolution seam. Callers
// (the router) treat it exactly like a failed dispatch: attempts are kept and the
// item is retried under the AC-005 budget. It is a distinct type so a test or an
// operator can tell "no worker" apart from "worker refused the message".
type WorkerResolveError struct {
	// Op is the paseo verb that failed: "inspect" or "run".
	Op string
	// AgentID is the id being inspected, when there was one.
	AgentID string
	// Err is the underlying cause.
	Err error
}

func (e *WorkerResolveError) Error() string {
	if e.AgentID != "" {
		return fmt.Sprintf("paseo %s %s: %v", e.Op, e.AgentID, e.Err)
	}
	return fmt.Sprintf("paseo %s: %v", e.Op, e.Err)
}

func (e *WorkerResolveError) Unwrap() error { return e.Err }

// PaseoDispatcher hands a message to a paseo worker agent.
type PaseoDispatcher struct {
	// Bin is the paseo executable; empty means "paseo" from PATH.
	Bin string
	// WorkerID is adbd_worker_agent_id: the standing worker to reuse when the
	// thread brings no live binding of its own. Empty means "create one".
	WorkerID string
	// Provider is adbd_worker_provider, passed to `paseo run` for a new worker.
	Provider string
	// Model is adbd_worker_model, passed to `paseo run` for a new worker.
	Model string
	// Workdir is the repository root a new worker is pointed at (`--cwd`) when
	// the thread names no registered project. Kept alongside FallbackWorkdir so
	// the older spelling keeps working.
	Workdir string
	// Mode is the provider mode for a new worker (opencode: build|plan,
	// claude: bypass, codex: full-access). Empty means DefaultWorkerMode.
	Mode string
	// Registry maps a thread's project key onto a repository directory
	// (feature thread-workdir). The zero value resolves nothing, so a daemon
	// with no registry keeps its single fixed directory.
	Registry ProjectRegistry
	// FallbackWorkdir is the directory a worker gets when the thread resolves to
	// no existing project directory (AC-004). Empty falls back to Workdir, then
	// DefaultWorkerWorkdir.
	FallbackWorkdir string
	// ReportsPath is the ledger a workdir fallback is recorded in
	// (.beacon/adbd-reports.jsonl). Empty means no record is written.
	ReportsPath string
}

// compile-time proof that the production dispatcher satisfies both seams.
var (
	_ Dispatcher     = PaseoDispatcher{}
	_ WorkerResolver = PaseoDispatcher{}
)

// Dispatch sends one guarded text to one agent. The text is passed as a single
// argv element, so no shell ever parses it.
func (d PaseoDispatcher) Dispatch(thread, agentID, text string) error {
	bin := d.bin()
	out, err := exec.Command(bin, "send", agentID, text).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dispatcher %s send %s (thread %s): %w (output: %s)",
			bin, agentID, thread, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureWorker returns an id paseo confirms exists (AC-009).
//
//	current is the candidate the router offered (bound id first, then the
//	configured default). Order:
//	  ① a verified current                       -> reuse it
//	  ② a verified configured adbd_worker_agent_id -> use it
//	  ③ `paseo run` a new worker and parse agentId
//
// It never returns "" or a placeholder: a failure is a typed error, never a
// silent fallback to a name nobody verified.
//
// AC-005 (thread-workdir): the first two steps return before any directory is
// resolved, so a thread that already owns a live worker is reused as-is -- no
// second workdir resolution and no second workdir_fallback record.
func (d PaseoDispatcher) EnsureWorker(thread, current string) (string, error) {
	current = strings.TrimSpace(current)
	if !isPlaceholderAgentID(current) {
		if err := d.inspect(current); err == nil {
			// The thread's own worker (or the configured one) is alive: reuse it,
			// so a thread keeps its history instead of spawning a new agent per
			// message.
			return current, nil
		}
		// A stale binding is not a reason to drop a message; fall through and get
		// a real worker instead.
	}

	if configured := strings.TrimSpace(d.WorkerID); !isPlaceholderAgentID(configured) {
		if err := d.inspect(configured); err == nil {
			return configured, nil
		}
	}

	return d.create(thread)
}

// inspect asks paseo whether an agent id exists. Exit 0 means yes.
func (d PaseoDispatcher) inspect(agentID string) error {
	bin := d.bin()
	out, err := exec.Command(bin, "inspect", agentID).CombinedOutput()
	if err != nil {
		return &WorkerResolveError{
			Op:      "inspect",
			AgentID: agentID,
			Err:     fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(out))),
		}
	}
	return nil
}

// create spawns a brand new worker with `paseo run --no-wait --json` and returns
// the agentId paseo printed. --no-wait keeps the router non-blocking (the message
// itself goes out right after with `paseo send`), and --json is the only contract
// this file relies on.
//
// AC-001 (agent-supervisor): the worker is created already claimed -- the argv
// carries `--label adbd.item=<thread>`, so supervision owns it from the first
// second instead of having to adopt it later.
func (d PaseoDispatcher) create(thread string) (string, error) {
	bin := d.bin()
	title := strings.TrimSpace(thread)
	if title == "" {
		title = "adbd"
	}
	provider := strings.TrimSpace(d.Provider)
	if provider == "" {
		provider = DefaultWorkerProvider
	}
	model := strings.TrimSpace(d.Model)
	if model == "" {
		model = DefaultWorkerModel
	}
	// AC-003/AC-004: the repository comes from the thread, not from wherever the
	// daemon happens to run; an unresolvable thread keeps the configured default
	// and leaves a record saying so.
	workdir, fallbackReason := d.createWorkdir(thread)
	if fallbackReason != "" {
		d.recordWorkdirFallback(thread, fallbackReason)
	}

	mode := strings.TrimSpace(d.Mode)
	if mode == "" {
		mode = DefaultWorkerMode
	}

	args := []string{
		"run", "--no-wait", "--json",
		"--mode", mode,
		// Without --new-workspace paseo reuses the caller workspace and
		// ignores --cwd, so the worker would land in whichever repo the
		// daemon happened to be started from.
		"--new-workspace", "local",
		"--title", title,
		"--cwd", workdir,
		"--provider", provider,
		"--model", model,
	}
	// AC-001 (agent-supervisor): a worker is born claimed. The label names the
	// work item, which is the thread string itself, so a freshly created worker
	// is owned from its first second and `adbd adopt` afterwards has nothing to
	// fix. An empty thread is not a work item, so no label is invented for it.
	if thread = strings.TrimSpace(thread); thread != "" {
		args = append(args, "--label", OwnershipLabel+"="+thread)
	}
	args = append(args, workerBootstrapPrompt(title))
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		detail := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", &WorkerResolveError{
			Op:  "run",
			Err: fmt.Errorf("%w (thread %s, output: %s)", err, thread, detail),
		}
	}
	id, parseErr := parseAgentID(out)
	if parseErr != nil {
		return "", &WorkerResolveError{Op: "run", Err: fmt.Errorf("%w (thread %s)", parseErr, thread)}
	}
	return id, nil
}

// createWorkdir decides the `--cwd` of a worker that is about to be created.
//
// A thread that resolves to an existing project directory gets exactly that
// directory. Every other outcome -- no project key, an unregistered project, a
// registered repo that is gone -- gets the configured fallback directory and a
// non-empty reason, which is what the caller turns into a ledger record. The
// two outcomes are exclusive: one dispatch either resolved or fell back, never
// both.
func (d PaseoDispatcher) createWorkdir(thread string) (workdir, fallbackReason string) {
	fallback := d.fallbackWorkdir()
	key := threadWorkdirKey(thread)
	if key == "" {
		return fallback, ReasonNoProjectKey
	}
	if dir, ok := d.Registry.Resolve(key); ok {
		return dir, ""
	}
	if _, registered := d.Registry.Lookup(key); registered {
		// Registered, but the directory is not there any more.
		return fallback, ReasonRepoMissing
	}
	return fallback, ReasonProjectNotRegistered
}

// fallbackWorkdir is the directory used when a thread names no usable project:
// FallbackWorkdir, then the older Workdir spelling, then the package default.
func (d PaseoDispatcher) fallbackWorkdir() string {
	for _, candidate := range []string{d.FallbackWorkdir, d.Workdir} {
		if dir := strings.TrimSpace(candidate); dir != "" {
			return dir
		}
	}
	return DefaultWorkerWorkdir
}

// recordWorkdirFallback appends one workdir_fallback line to the reports ledger.
//
// Writing is best effort: a read-only or missing ledger must not stop a
// dispatch, because the message is more valuable than the record of it being
// imperfect. The line carries exactly kind/thread/reason, and reconcile.go's
// receipt reader ignores rows without an agent_id, so the two shapes coexist.
func (d PaseoDispatcher) recordWorkdirFallback(thread, reason string) {
	path := strings.TrimSpace(d.ReportsPath)
	if path == "" {
		return
	}
	data, err := json.Marshal(workdirFallbackRecord{
		Kind:   WorkdirFallbackKind,
		Thread: thread,
		Reason: reason,
	})
	if err != nil {
		return
	}
	data = append(data, '\n')
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(data)
}

// workerBootstrapPrompt is the only text a freshly created worker receives. The
// actual message is delivered afterwards with `paseo send`, so this line just
// tells the new worker what it is and where it may work.
func workerBootstrapPrompt(thread string) string {
	return fmt.Sprintf("ADB worker for thread %q: wait for the next dispatch. Work only inside the repository that thread names; if the thread does not name one, ask before touching any file.", thread)
}

// parseAgentID reads the agentId out of `paseo run --json` output. The last
// parsable line wins, so a banner or a progress line before the payload cannot
// defeat the parse, and a payload carrying "main" is refused on principle.
func parseAgentID(out []byte) (string, error) {
	// paseo prints pretty-printed JSON. Try the whole document first; the
	// line-by-line scan below only exists for NDJSON-style streams.
	var whole struct {
		AgentID string `json:"agentId"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &whole); err == nil {
		if id := strings.TrimSpace(whole.AgentID); !isPlaceholderAgentID(id) {
			return id, nil
		}
	}
	lines := strings.Split(string(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var payload struct {
			AgentID string `json:"agentId"`
		}
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		id := strings.TrimSpace(payload.AgentID)
		if isPlaceholderAgentID(id) {
			continue
		}
		return id, nil
	}
	return "", fmt.Errorf("paseo run --json printed no usable agentId (output: %s)", strings.TrimSpace(string(out)))
}

// bin is the paseo executable this dispatcher drives.
func (d PaseoDispatcher) bin() string {
	if bin := strings.TrimSpace(d.Bin); bin != "" {
		return bin
	}
	return "paseo"
}
