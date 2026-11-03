// The worker management surface (truth.md, feature worker-crud, v0.1.5).
//
// paseo already knows how to list, start, rename and archive an agent. What it
// does not know is which project an agent belongs to, and it has no notion of
// "do not touch this one". This file adds exactly that much and no more:
//
//	list     one screen of workers with the project their cwd maps to
//	show     the nine facts about one worker
//	create   start a worker inside a registered project directory, born labelled
//	update   rename / relabel / change mode (nothing that would need a rebuild)
//	archive  soft-delete, with an audit line and an ownership precondition
//
// Three decisions hold the whole feature together.
//
//  1. paseo is driven through the CLI only (PaseoCliRunner, argv, no import).
//     Every call in this file goes through that one seam, so there is exactly
//     one place to read to see what adbd can do to an agent.
//  2. Ownership has two sources and no third: the labels `paseo ls --json`
//     happens to return, and the `ls --label k=v` lookups for the agents whose
//     name/title is the work item. An agent that answers neither is `unknown`,
//     and an unknown agent is displayed but never written to (AC-007).
//  3. There is no hard-delete verb anywhere on this surface (AC-006). The word
//     is answered with reason_code=delete_not_supported so an operator learns
//     the boundary instead of reaching for paseo directly.
//
// Project mapping lives in config/workspaces.json (AC-008) rather than in the
// project registry used by dispatch: dispatch maps a *thread* to a repository,
// this maps a *directory* back to the project that owns it, and the longest
// prefix wins so a checkout inside a checkout still lands on the inner project.
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
	"strconv"
	"strings"
	"sync"
	"time"
)

// Names, tokens and reason codes this feature owns (truth.md 配置项 / AC table).
const (
	// WorkerWorkspacesFile is the default worker -> project mapping table,
	// relative to the ADB root unless adbd_workspaces_file says otherwise.
	WorkerWorkspacesFile = "workspaces.json"
	// WorkerAuditFile is the append-only archive ledger inside .beacon/.
	WorkerAuditFile = "adbd-worker.jsonl"
	// WorkerProjectUnmatched is printed in the project column when a cwd maps to
	// no registered workspace.
	WorkerProjectUnmatched = "-"

	// WorkerOwned marks an agent that carries adbd.item.
	WorkerOwned = "owned"
	// WorkerForeign marks an agent that carries adbd.foreign.
	WorkerForeign = "foreign"
	// WorkerUnknown marks an agent that carries neither label.
	WorkerUnknown = "unknown"
	// WorkerStateArchived is the state token for an archived agent.
	WorkerStateArchived = "archived"

	// ReasonWorkspacesInvalid is AC-008: the mapping table cannot be used.
	ReasonWorkspacesInvalid = "workspaces_invalid"
	// ReasonAgentNotFound is AC-002: paseo does not know this id.
	ReasonAgentNotFound = "agent_not_found"
	// ReasonFieldNotUpdatable is AC-004: the field needs a new worker, not an edit.
	ReasonFieldNotUpdatable = "field_not_updatable"
	// ReasonOwnershipUnknown is AC-005/AC-007: nobody proved this agent is ours.
	ReasonOwnershipUnknown = "ownership_unknown"
	// ReasonDeleteNotSupported is AC-006: this surface archives, never deletes.
	ReasonDeleteNotSupported = "delete_not_supported"

	// workerLookupConcurrency bounds the parallel label lookups. One pass needs
	// one lookup per distinct agent name because paseo's `ls --json` drops
	// labels; running them in a small pool keeps a 176-agent machine bearable
	// without ever reporting a partial answer as a complete one.
	workerLookupConcurrency = 8
)

// workerClock is the clock seam. Ownership, age and the audit stamp all read it,
// so a test can pin "now" instead of hoping the wall clock cooperates.
var workerClock = func() time.Time { return time.Now() }

// ---------------------------------------------------------------------------
// project mapping: config/workspaces.json (AC-008)
// ---------------------------------------------------------------------------

// Workspace is one row of the mapping table. `path` is the directory workers are
// started in and what a cwd is matched against; `root` is an optional second
// directory of the same project (a checkout next to a docs root), matched the
// same way so either one resolves to this slug.
type Workspace struct {
	// Slug is the project slug named by `--project`.
	Slug string `json:"slug"`
	// Path is the worker directory, resolved to an absolute path on load.
	Path string `json:"path"`
	// Root is an optional second directory of the same project.
	Root string `json:"root"`
	// Aliases are extra spellings accepted by `--project`.
	Aliases []string `json:"aliases"`
}

// WorkspacesInvalidError is the typed AC-008 failure: the table exists but
// cannot be trusted, so every worker command stops instead of guessing.
type WorkspacesInvalidError struct {
	// Source is the file that was read.
	Source string
	// Err says what is wrong with it.
	Err error
}

func (e *WorkspacesInvalidError) Error() string {
	return fmt.Sprintf("workspaces %s: %v", e.Source, e.Err)
}

func (e *WorkspacesInvalidError) Unwrap() error { return e.Err }

// Workspaces is the loaded, validated mapping table. The zero value maps nothing.
type Workspaces struct {
	// Entries keeps the file order, which is the tie-break for equal path lengths.
	Entries []Workspace
	// byKey indexes slug and alias (lower-cased) onto an absolute directory.
	byKey map[string]string
	// Source is the file the table came from.
	Source string
}

// matchPaths is every directory that maps to this entry's slug.
func (w Workspace) matchPaths() []string {
	if strings.TrimSpace(w.Root) == "" {
		return []string{w.Path}
	}
	return []string{w.Path, w.Root}
}

// LoadWorkspaces reads and validates the mapping table (AC-008).
//
// Everything that would make a lookup silently wrong is an error and not a
// tolerated default: an unreadable or unparsable file, a duplicate slug, an
// entry without a slug or a path, and a path that is not a directory right now.
// A table that cannot answer "which project is this cwd?" must not answer "-"
// and look authoritative.
func LoadWorkspaces(configured, root string) (Workspaces, error) {
	source := workspacesFilePath(configured, root)
	table := Workspaces{byKey: map[string]string{}, Source: source}
	data, err := os.ReadFile(source)
	if err != nil {
		return table, &WorkspacesInvalidError{Source: source, Err: err}
	}
	var doc struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return table, &WorkspacesInvalidError{Source: source, Err: fmt.Errorf("not a JSON document: %w", err)}
	}
	if doc.Workspaces == nil {
		return table, &WorkspacesInvalidError{
			Source: source,
			Err:    errors.New(`no "workspaces" array`),
		}
	}
	base := absoluteBase(root)
	seen := map[string]bool{}
	for i, raw := range doc.Workspaces {
		slug := strings.TrimSpace(raw.Slug)
		if slug == "" {
			return table, &WorkspacesInvalidError{
				Source: source,
				Err:    fmt.Errorf("entry %d has no slug", i+1),
			}
		}
		key := strings.ToLower(slug)
		if seen[key] {
			return table, &WorkspacesInvalidError{
				Source: source,
				Err:    fmt.Errorf("slug %q is defined twice", slug),
			}
		}
		seen[key] = true
		declared := strings.TrimSpace(raw.Path)
		if declared == "" {
			return table, &WorkspacesInvalidError{
				Source: source,
				Err:    fmt.Errorf("entry %q has no path", slug),
			}
		}
		dir := resolveRepoPath(declared, base)
		if !isDirectory(dir) {
			return table, &WorkspacesInvalidError{
				Source: source,
				Err:    fmt.Errorf("entry %q points at %s, which is not a directory", slug, dir),
			}
		}
		entry := Workspace{Slug: slug, Path: dir}
		if declaredRoot := strings.TrimSpace(raw.Root); declaredRoot != "" {
			entry.Root = resolveRepoPath(declaredRoot, base)
		}
		for _, alias := range raw.Aliases {
			trimmed := strings.TrimSpace(alias)
			if trimmed == "" {
				continue
			}
			entry.Aliases = append(entry.Aliases, trimmed)
		}
		table.Entries = append(table.Entries, entry)
		table.index(slug, dir)
		for _, alias := range entry.Aliases {
			table.index(alias, dir)
		}
	}
	return table, nil
}

// index records one lookup key, keeping the first registration so an alias that
// collides with another slug can never steal it.
func (w *Workspaces) index(key, dir string) {
	if w.byKey == nil {
		w.byKey = map[string]string{}
	}
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" || dir == "" {
		return
	}
	if _, taken := w.byKey[normalized]; taken {
		return
	}
	w.byKey[normalized] = dir
}

// PathForSlug answers `--project <slug>` (AC-003). ok=false means "not
// registered here", which the caller turns into project_not_registered without
// calling paseo at all.
func (w Workspaces) PathForSlug(slug string) (string, bool) {
	dir, ok := w.byKey[strings.ToLower(strings.TrimSpace(slug))]
	if !ok || dir == "" {
		return "", false
	}
	return dir, true
}

// CanonicalSlug folds an alias onto the slug it belongs to, so `--project b` and
// `--project beta` mean the same thing. A key the table does not know is
// returned unchanged (and then simply matches nothing).
func (w Workspaces) CanonicalSlug(key string) string {
	trimmed := strings.TrimSpace(key)
	dir, ok := w.PathForSlug(trimmed)
	if !ok {
		return trimmed
	}
	for _, entry := range w.Entries {
		if filepath.Clean(entry.Path) == filepath.Clean(dir) {
			return entry.Slug
		}
	}
	return trimmed
}

// SlugForPath answers "which project owns this cwd?" by longest prefix, so a
// directory below a registered path still resolves and an inner registration
// wins over the outer one (AC-001).
func (w Workspaces) SlugForPath(cwd string) string {
	target := workerAbsPath(cwd)
	if target == "" {
		return ""
	}
	best := ""
	bestLen := -1
	for _, entry := range w.Entries {
		for _, dir := range entry.matchPaths() {
			if strings.TrimSpace(dir) == "" {
				continue
			}
			if !pathIsUnder(dir, target) {
				continue
			}
			if len(dir) > bestLen {
				best, bestLen = entry.Slug, len(dir)
			}
		}
	}
	return best
}

// pathIsUnder reports whether target is dir itself or below it. A plain string
// prefix is not enough: /work/app must not own /work/application.
func pathIsUnder(dir, target string) bool {
	dir = filepath.Clean(dir)
	target = filepath.Clean(target)
	if dir == target {
		return true
	}
	prefix := dir
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(target, prefix)
}

// workerAbsPath normalizes a directory for comparison: `~` expanded (paseo's
// `ls` prints the home directory shortened) and made absolute.
func workerAbsPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "-" {
		return ""
	}
	expanded := expandHome(trimmed)
	if abs, err := filepath.Abs(expanded); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(expanded)
}

// workspacesFilePath resolves adbd_workspaces_file, defaulting to
// <root>/config/workspaces.json. A relative override is read against the root,
// the same way every other daemon path is.
func workspacesFilePath(configured, root string) string {
	base := absoluteBase(root)
	path := strings.TrimSpace(configured)
	if path == "" {
		return filepath.Join(base, "config", WorkerWorkspacesFile)
	}
	expanded := expandHome(path)
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	return filepath.Join(base, expanded)
}

// ---------------------------------------------------------------------------
// the observed worker (a superset of what supervision needs)
// ---------------------------------------------------------------------------

// WorkerAgent is one agent as this feature reads it. It differs from PaseoAgent
// by carrying cwd and the created stamp, which `list` needs and supervision
// never did.
type WorkerAgent struct {
	// ID is the paseo agent id.
	ID string
	// Name is the agent title; paseo prints "-" for an untitled agent.
	Name string
	// Status is paseo's raw status string (running / idle / error / ...).
	Status string
	// Cwd is the working directory, `~` expanded.
	Cwd string
	// Labels are the labels paseo happened to return inline (often none).
	Labels map[string]string
	// PendingPermissions are the permission requests still open.
	PendingPermissions []PendingPermission
	// Created is paseo's created stamp, verbatim (`ls` prints a relative
	// phrase, `inspect` prints RFC3339).
	Created string
	// Updated is paseo's updated stamp, verbatim.
	Updated string
	// Archived is true when paseo says the agent is archived.
	Archived bool
	// ArchivedAt is when, when paseo says.
	ArchivedAt string
}

// Label reads one label; "" means "not set".
func (a WorkerAgent) Label(key string) string {
	if a.Labels == nil {
		return ""
	}
	return strings.TrimSpace(a.Labels[key])
}

// State is the one state token the list and show surfaces print. Archiving wins
// over the raw status because "archived" is the fact an operator acts on.
func (a WorkerAgent) State() string {
	if a.Archived || strings.TrimSpace(a.ArchivedAt) != "" || a.Status == WorkerStateArchived {
		return WorkerStateArchived
	}
	if strings.TrimSpace(a.Status) == "" {
		return "-"
	}
	return a.Status
}

// toolNames is the deduped, sorted list of tools the agent is waiting on.
func (a WorkerAgent) toolNames() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a.PendingPermissions))
	for _, perm := range a.PendingPermissions {
		tool := strings.TrimSpace(perm.Tool)
		if tool == "" {
			continue
		}
		if _, done := seen[tool]; done {
			continue
		}
		seen[tool] = struct{}{}
		out = append(out, tool)
	}
	sort.Strings(out)
	return out
}

// rawWorkerAgent is the decoding shape of one row of `paseo ls --json`. The tags
// are PascalCase-tolerant because encoding/json matches keys ignoring case, so
// `Created` also reads `created` and `CreatedAt` reads `createdAt`.
type rawWorkerAgent struct {
	AgentID            string          `json:"agentId"`
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Title              string          `json:"title"`
	Status             string          `json:"status"`
	Cwd                string          `json:"cwd"`
	Created            string          `json:"created"`
	CreatedAt          string          `json:"createdAt"`
	UpdatedAt          string          `json:"updatedAt"`
	Archived           bool            `json:"archived"`
	ArchivedAt         string          `json:"archivedAt"`
	Labels             json.RawMessage `json:"labels"`
	PendingPermissions json.RawMessage `json:"pendingPermissions"`
}

// workerAgentFromRaw narrows one decoded row onto WorkerAgent.
func workerAgentFromRaw(raw rawWorkerAgent) (WorkerAgent, error) {
	id := strings.TrimSpace(raw.AgentID)
	if id == "" {
		id = strings.TrimSpace(raw.ID)
	}
	if id == "" {
		return WorkerAgent{}, errors.New("paseo printed an agent row without an id")
	}
	labels, err := parseLabels(raw.Labels)
	if err != nil {
		return WorkerAgent{}, fmt.Errorf("agent %s: %w", id, err)
	}
	perms, err := parsePermissions(raw.PendingPermissions)
	if err != nil {
		return WorkerAgent{}, fmt.Errorf("agent %s: %w", id, err)
	}
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = strings.TrimSpace(raw.Title)
	}
	if name == "-" {
		name = ""
	}
	created := strings.TrimSpace(raw.Created)
	if created == "" {
		created = strings.TrimSpace(raw.CreatedAt)
	}
	return WorkerAgent{
		ID:                 id,
		Name:               name,
		Status:             strings.TrimSpace(raw.Status),
		Cwd:                expandHome(strings.TrimSpace(raw.Cwd)),
		Labels:             labels,
		PendingPermissions: perms,
		Created:            created,
		Updated:            strings.TrimSpace(raw.UpdatedAt),
		Archived:           raw.Archived,
		ArchivedAt:         strings.TrimSpace(raw.ArchivedAt),
	}, nil
}

// parseWorkerAgentList reads `paseo ls --json`. The three accepted shapes match
// parseAgentList (object wrapper, bare array, NDJSON), because both read the same
// external contract; a row without an id is still an error, never a skipped row.
func parseWorkerAgentList(out []byte) ([]WorkerAgent, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, errors.New("paseo ls --json printed nothing")
	}
	switch trimmed[0] {
	case '{':
		var doc struct {
			Agents []rawWorkerAgent `json:"agents"`
		}
		if err := json.Unmarshal(trimmed, &doc); err == nil && doc.Agents != nil {
			return convertWorkerAgents(doc.Agents)
		}
	case '[':
		var rows []rawWorkerAgent
		if err := json.Unmarshal(trimmed, &rows); err == nil {
			return convertWorkerAgents(rows)
		}
		return nil, fmt.Errorf("paseo ls --json is not a JSON array (output: %s)", preview(trimmed))
	}
	var rows []rawWorkerAgent
	for i, line := range strings.Split(string(trimmed), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row rawWorkerAgent
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("paseo ls --json line %d is not an agent object: %w", i+1, err)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("paseo ls --json printed no agent list (output: %s)", preview(trimmed))
	}
	return convertWorkerAgents(rows)
}

func convertWorkerAgents(rows []rawWorkerAgent) ([]WorkerAgent, error) {
	out := make([]WorkerAgent, 0, len(rows))
	for i, row := range rows {
		agent, err := workerAgentFromRaw(row)
		if err != nil {
			return nil, fmt.Errorf("paseo ls --json row %d: %w", i+1, err)
		}
		out = append(out, agent)
	}
	return out, nil
}

// parseWorkerInspect reads `paseo inspect <id> --json`.
//
// The real CLI prints PascalCase keys (Id, Name, Status, Archived, Cwd,
// CreatedAt, UpdatedAt, PendingPermissions) and encoding/json matches tags
// ignoring case, so the same struct also reads the camelCase spellings an older
// or future build may use.
func parseWorkerInspect(out []byte) (WorkerAgent, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return WorkerAgent{}, errors.New("paseo inspect --json printed nothing")
	}
	var raw rawWorkerAgent
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		// inspect also has an NDJSON/table past; take the last object that parses.
		var found bool
		for _, line := range strings.Split(string(trimmed), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
				continue
			}
			if err := json.Unmarshal([]byte(line), &raw); err == nil {
				found = true
			}
		}
		if !found {
			return WorkerAgent{}, fmt.Errorf("paseo inspect --json is not a JSON object (output: %s)", preview(trimmed))
		}
	}
	return workerAgentFromRaw(raw)
}

// ---------------------------------------------------------------------------
// ownership: two labels, two sources, no guessing
// ---------------------------------------------------------------------------

// workerLabelIndex remembers which agents carry the ownership labels, learned
// from `ls --label k=v` because `ls --json` drops labels.
type workerLabelIndex struct {
	// items maps an adbd.item value onto the agent ids that carry it.
	items map[string]map[string]bool
	// order is the sorted key set of items, so "which item owns this agent?" has
	// one answer instead of whichever map order the runtime felt like.
	order []string
	// foreign is the set of agents carrying adbd.foreign.
	foreign map[string]bool
}

// emptyWorkerLabelIndex is what a caller gets when no lookup source exists.
func emptyWorkerLabelIndex() workerLabelIndex {
	return workerLabelIndex{items: map[string]map[string]bool{}, foreign: map[string]bool{}}
}

// loadWorkerLabelIndex asks paseo which agents carry adbd.item=<candidate> and
// adbd.foreign=1.
//
// The candidate set is every distinct agent name/title plus any inline
// adbd.item value: a worker created by `adbd worker create` (or by the router)
// is titled after its work item and labelled with it, so the two agree. A lookup
// failure is returned with whatever was learned so far -- `list` reports it as a
// warning, while update/archive fail closed on it rather than assuming "not
// owned" or "owned".
func loadWorkerLabelIndex(lookup LabelLookup, agents []WorkerAgent) (workerLabelIndex, error) {
	index := emptyWorkerLabelIndex()
	if lookup == nil {
		return index, nil
	}
	foreign, err := lookup.LabeledIDs(ForeignLabel, ForeignLabelValue)
	if err != nil {
		return index, err
	}
	index.foreign = foreign

	candidates := make([]string, 0, len(agents))
	seen := map[string]bool{}
	for _, agent := range agents {
		for _, candidate := range []string{agent.Label(OwnershipLabel), agent.Name} {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" || seen[candidate] {
				continue
			}
			seen[candidate] = true
			candidates = append(candidates, candidate)
		}
	}
	sort.Strings(candidates)

	var (
		mu       sync.Mutex
		firstErr error
		wg       sync.WaitGroup
	)
	sem := make(chan struct{}, workerLookupConcurrency)
	for _, candidate := range candidates {
		candidate := candidate
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			ids, err := lookup.LabeledIDs(OwnershipLabel, candidate)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			index.items[candidate] = ids
		}()
	}
	wg.Wait()

	index.order = make([]string, 0, len(index.items))
	for item := range index.items {
		index.order = append(index.order, item)
	}
	sort.Strings(index.order)
	if firstErr != nil {
		return index, firstErr
	}
	return index, nil
}

// classify answers "is this agent ours?" from both label sources (AC-001/AC-007).
func (idx workerLabelIndex) classify(agent WorkerAgent) (ownership, item string) {
	if label := agent.Label(OwnershipLabel); label != "" {
		return WorkerOwned, label
	}
	if agent.Label(ForeignLabel) != "" {
		return WorkerForeign, ""
	}
	if idx.foreign[agent.ID] {
		return WorkerForeign, ""
	}
	for _, candidate := range idx.order {
		if idx.items[candidate][agent.ID] {
			return WorkerOwned, candidate
		}
	}
	return WorkerUnknown, ""
}

// labelsFor is what `show` prints: the labels paseo returned inline, plus the
// ownership label the lookups proved (AC-002 allows reporting them from there).
func (idx workerLabelIndex) labelsFor(agent WorkerAgent) map[string]string {
	labels := map[string]string{}
	for key, value := range agent.Labels {
		if key = strings.TrimSpace(key); key != "" {
			labels[key] = value
		}
	}
	ownership, item := idx.classify(agent)
	if ownership == WorkerOwned && labels[OwnershipLabel] == "" {
		labels[OwnershipLabel] = item
	}
	if ownership == WorkerForeign && labels[ForeignLabel] == "" {
		labels[ForeignLabel] = ForeignLabelValue
	}
	return labels
}

// ---------------------------------------------------------------------------
// the paseo CLI, as this feature drives it
// ---------------------------------------------------------------------------

// workerSpec is one `paseo run` invocation (AC-003).
type workerSpec struct {
	// Item is the work item id: the title and the adbd.item value.
	Item string
	// Cwd is the project directory the worker starts in.
	Cwd string
	// Prompt is the first thing the worker is told; empty means the bootstrap line.
	Prompt string
	// Provider / Model / Mode are passed only when the operator named them, so
	// paseo keeps its own defaults instead of adbd inventing one here.
	Provider string
	Model    string
	Mode     string
}

// listWorkerAgents reads `paseo ls --json`.
func (r PaseoCliRunner) listWorkerAgents() ([]WorkerAgent, error) {
	out, err := r.run("ls", "--json")
	if err != nil {
		return nil, err
	}
	return parseWorkerAgentList(out)
}

// listWorkerAgentsIncludingArchived reads `paseo ls -a --json`, the one form that
// still answers for an agent that was archived. It exists so `worker list
// --state archived` is a real answer instead of a silently empty screen.
func (r PaseoCliRunner) listWorkerAgentsIncludingArchived() ([]WorkerAgent, error) {
	out, err := r.run("ls", "-a", "--json")
	if err != nil {
		return nil, err
	}
	return parseWorkerAgentList(out)
}

// workerLookupAll is the LabelLookup shape over `ls -a --label k=v`. Plain `ls`
// hides archived agents, so this is what classifies them when the operator asks
// for the archive view.
type workerLookupAll struct {
	runner PaseoCliRunner
}

// LabeledIDs lists the ids of agents carrying key=value, archived included.
func (l workerLookupAll) LabeledIDs(key, value string) (map[string]bool, error) {
	out, err := l.runner.run("ls", "-a", "--label", key+"="+value, "--json")
	if err != nil {
		return nil, err
	}
	agents, err := parseWorkerAgentList(out)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(agents))
	for _, agent := range agents {
		ids[agent.ID] = true
	}
	return ids, nil
}

// inspectWorker reads `paseo inspect <id> --json`.
func (r PaseoCliRunner) inspectWorker(agentID string) (WorkerAgent, error) {
	out, err := r.run("inspect", agentID, "--json")
	if err != nil {
		return WorkerAgent{}, err
	}
	return parseWorkerInspect(out)
}

// createWorker starts a worker in a project directory and returns the id paseo
// printed. The argv is the frozen AC-003 contract:
//
//	paseo run --no-wait --json --new-workspace local --cwd <path>
//	          --title <item> --label adbd.item=<item> [...flags] <prompt>
//
// --new-workspace is not decoration: without it paseo reuses the caller
// workspace and ignores --cwd, so the worker would land in whichever repository
// adbd happened to be started from.
func (r PaseoCliRunner) createWorker(spec workerSpec) (string, error) {
	args := []string{
		"run", "--no-wait", "--json",
		"--new-workspace", "local",
		"--cwd", spec.Cwd,
		"--title", spec.Item,
		"--label", OwnershipLabel + "=" + spec.Item,
	}
	if provider := strings.TrimSpace(spec.Provider); provider != "" {
		args = append(args, "--provider", provider)
	}
	if model := strings.TrimSpace(spec.Model); model != "" {
		args = append(args, "--model", model)
	}
	if mode := strings.TrimSpace(spec.Mode); mode != "" {
		args = append(args, "--mode", mode)
	}
	prompt := strings.TrimSpace(spec.Prompt)
	if prompt == "" {
		// paseo run takes the task as a positional argument; a worker started
		// with no task gets the same one-line brief the router gives.
		prompt = workerBootstrapPrompt(spec.Item)
	}
	args = append(args, prompt)
	out, err := r.run(args...)
	if err != nil {
		return "", err
	}
	return parseAgentID(out)
}

// updateWorker renames an agent and/or writes labels (AC-004).
func (r PaseoCliRunner) updateWorker(agentID, name string, labels []string) error {
	args := []string{"agent", "update", agentID}
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		args = append(args, "--name", trimmed)
	}
	for _, label := range labels {
		args = append(args, "--label", label)
	}
	_, err := r.run(args...)
	return err
}

// setWorkerMode changes the provider mode of an existing agent (AC-004).
func (r PaseoCliRunner) setWorkerMode(agentID, mode string) error {
	_, err := r.run("agent", "mode", agentID, mode)
	return err
}

// archiveWorker soft-deletes one agent (AC-005). It is the only destructive verb
// this surface has, it is named after what it does, and there is no sibling that
// removes anything for good.
func (r PaseoCliRunner) archiveWorker(agentID string, force bool) error {
	args := []string{"archive", agentID}
	if force {
		// paseo refuses to archive a running agent unless --force is given; the
		// flag here means exactly that and nothing more.
		args = append(args, "--force")
	}
	_, err := r.run(args...)
	return err
}

// ---------------------------------------------------------------------------
// CLI plumbing
// ---------------------------------------------------------------------------

// workerEnv is everything a worker subcommand needs and nothing it could call.
type workerEnv struct {
	runner PaseoCliRunner
	root   string
	beacon string
	// statePath is the binding document. Archive must release the bindings that
	// point at the worker it just archived (AC-005); without that a follow-up on
	// the thread wakes the archived agent right back up.
	statePath string
	cfg       Config
	asJSON    bool
}

// addWorkerCommonFlags mirrors addCommonFlags for this surface only.
//
// Two shared flags are deliberately absent: `--state` is the agent-state filter
// here (AC-001) rather than the binding document, and the slow-path queue is not
// something worker management reads or writes. Everything else -- the root, the
// config file, the runner binary, the outcome -- is the same surface.
func addWorkerCommonFlags(fs *flag.FlagSet) *commonFlags {
	cf := &commonFlags{}
	fs.StringVar(&cf.root, "root", envOr("ADBD_ROOT", "."), "ADB root directory")
	fs.StringVar(&cf.configPath, "config", "", "config file path")
	fs.StringVar(&cf.runnerBin, "runner-bin", "", "runner binary path")
	fs.StringVar(&cf.outcome, "outcome", "text", "text or json")
	return cf
}

// newWorkerEnv resolves the common flags into the runner seam. It performs no
// paseo call, so a command that must fail before touching paseo can fail first.
func newWorkerEnv(cf *commonFlags, jsonOut bool, stderr io.Writer) (workerEnv, int) {
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd worker: %v\n", err)
		return workerEnv{}, 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd worker: %v\n", err)
		return workerEnv{}, 2
	}
	bin := cf.runnerBin
	if bin == "" {
		bin = envOr("ADBD_PASEO_BIN", cfg.PaseoBin)
	}
	if bin == "" {
		bin = "paseo"
	}
	root := absoluteBase(cf.root)
	return workerEnv{
		runner:    PaseoCliRunner{Bin: bin},
		root:      root,
		beacon:    filepath.Join(root, ".beacon"),
		statePath: cf.statePath,
		cfg:       cfg,
		asJSON:    jsonOut || cf.outcome == "json",
	}, 0
}

// workspaces loads the mapping table this command run is bound to.
func (e workerEnv) workspaces() (Workspaces, error) {
	configured := envOr("ADBD_WORKSPACES_FILE", e.cfg.WorkspacesFile)
	return LoadWorkspaces(configured, e.root)
}

// workerFailure is the machine answer to "this command refused".
type workerFailure struct {
	ReasonCode string `json:"reason_code"`
	AgentID    string `json:"agent_id,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

// fail prints the refusal and returns exit code 2. The reason code goes to
// stdout because that is where every other adbd command puts it; the human
// sentence goes to stderr.
func (e workerEnv) fail(stdout, stderr io.Writer, failure workerFailure) int {
	if e.asJSON {
		if code := writeJSON(stdout, stderr, failure); code != 0 {
			return code
		}
	} else {
		line := "reason_code=" + failure.ReasonCode
		if failure.AgentID != "" {
			line += " agent_id=" + failure.AgentID
		}
		fmt.Fprintln(stdout, line)
	}
	if failure.Detail != "" {
		fmt.Fprintf(stderr, "adbd worker: %s\n", failure.Detail)
	}
	if failure.Hint != "" {
		fmt.Fprintf(stderr, "adbd worker: %s\n", failure.Hint)
	}
	return 2
}

// labelFlag collects repeatable `--label k=v` values.
type labelFlag []string

func (l *labelFlag) String() string { return strings.Join(*l, ",") }

func (l *labelFlag) Set(value string) error {
	key, val, ok := strings.Cut(strings.TrimSpace(value), "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return fmt.Errorf("--label %q is not key=value", value)
	}
	*l = append(*l, key+"="+val)
	return nil
}

// runWorker dispatches the worker surface. There is a branch for every verb the
// feature offers, plus the one branch that answers "no" (AC-006).
func runWorker(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "adbd worker: list, show, create, update or archive is required")
		return 2
	}
	switch args[0] {
	case "list":
		return runWorkerList(args[1:], stdout, stderr)
	case "show":
		return runWorkerShow(args[1:], stdout, stderr)
	case "create":
		return runWorkerCreate(args[1:], stdout, stderr)
	case "update":
		return runWorkerUpdate(args[1:], stdout, stderr)
	case "archive":
		return runWorkerArchive(args[1:], stdout, stderr)
	case "delete":
		return runWorkerDelete(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "adbd worker: unknown subcommand %q (want list, show, create, update or archive)\n", args[0])
		return 2
	}
}

// runWorkerDelete is the refusal (AC-006). The surface offers archive; an
// operator who asks for the hard verb is told the boundary and given the exit
// code a refusal deserves, instead of a command that quietly does something else.
func runWorkerDelete(args []string, stdout, stderr io.Writer) int {
	env := workerEnv{asJSON: workerJSONRequested(args)}
	return env.fail(stdout, stderr, workerFailure{
		ReasonCode: ReasonDeleteNotSupported,
		Detail:     "adbd worker has no such verb",
		Hint:       "archive instead: adbd worker archive <id>",
	})
}

// workerJSONRequested answers the pre-flag question for the one subcommand that
// has no flag set of its own.
func workerJSONRequested(args []string) bool {
	for i, arg := range args {
		switch {
		case arg == "--json":
			return true
		case arg == "--outcome=json":
			return true
		case arg == "--outcome" && i+1 < len(args) && args[i+1] == "json":
			return true
		}
	}
	return false
}

// workerID parses the positional id of show/update/archive.
func workerID(name string, positional []string, stderr io.Writer) (string, bool) {
	if len(positional) == 0 {
		fmt.Fprintf(stderr, "adbd worker %s: <id> is required\n", name)
		return "", false
	}
	id := strings.TrimSpace(positional[0])
	if id == "" {
		fmt.Fprintf(stderr, "adbd worker %s: <id> is required\n", name)
		return "", false
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "adbd worker %s: unexpected argument %q\n", name, positional[1])
		return "", false
	}
	return id, true
}

// workerArgs separates flags from positionals so `adbd worker show <id> --json`
// parses the way its documented shape reads. Go's flag package stops at the
// first positional, so the two lists are split here and the flags are handed to
// the flag set on their own.
//
// A flag is a token starting with "-"; it consumes the next token as its value
// unless it is one of the known boolean flags or already carries `=value`.
func workerArgs(args []string) (flags, positional []string) {
	boolean := map[string]bool{"json": true, "force": true, "owned": true, "help": true, "h": true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") || boolean[name] {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return flags, positional
}

// ---------------------------------------------------------------------------
// list (AC-001)
// ---------------------------------------------------------------------------

// WorkerRow is one line of `worker list`.
type WorkerRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Project   string `json:"project"`
	Ownership string `json:"ownership"`
	State     string `json:"state"`
	Age       string `json:"age"`
	// Item is the work item the ownership label names, when one was proven.
	Item string `json:"item,omitempty"`
}

// WorkerList is the whole answer of `worker list`.
type WorkerList struct {
	Workspaces string      `json:"workspaces"`
	Count      int         `json:"count"`
	Workers    []WorkerRow `json:"workers"`
}

func runWorkerList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var project, state string
	var owned, jsonOut bool
	fs.StringVar(&project, "project", "", "only workers whose cwd maps to this project slug")
	fs.StringVar(&state, "state", "", "only workers in this state (running, idle, error, archived)")
	fs.BoolVar(&owned, "owned", false, "only workers that carry adbd.item")
	fs.BoolVar(&jsonOut, "json", false, "machine output")
	cf := addWorkerCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	env, code := newWorkerEnv(cf, jsonOut, stderr)
	if code != 0 {
		return code
	}
	workspaces, err := env.workspaces()
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonWorkspacesInvalid,
			Detail:     err.Error(),
			Hint:       "fix the mapping table: " + workspacesFilePath(env.cfg.WorkspacesFile, env.root),
		})
	}
	agents, err := env.runner.listWorkerAgents()
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonPaseoUnavailable,
			Detail:     err.Error(),
		})
	}
	// Asking for the archived state has to look at the states paseo hides by
	// default, and to classify those agents the label lookups need the same view.
	var lookup LabelLookup = env.runner
	if strings.EqualFold(strings.TrimSpace(state), WorkerStateArchived) {
		if agents, err = env.runner.listWorkerAgentsIncludingArchived(); err != nil {
			return env.fail(stdout, stderr, workerFailure{
				ReasonCode: ReasonPaseoUnavailable,
				Detail:     err.Error(),
			})
		}
		lookup = workerLookupAll{runner: env.runner}
	}
	index, lookupErr := loadWorkerLabelIndex(lookup, agents)
	if lookupErr != nil {
		// list is read-only: an unanswered lookup downgrades the ownership column
		// to unknown for the agents it covers, and says so, instead of failing the
		// whole screen or pretending the answer is complete.
		fmt.Fprintf(stderr, "adbd worker list: ownership lookup incomplete: %v\n", lookupErr)
	}

	now := workerClock()
	rows := make([]WorkerRow, 0, len(agents))
	// An alias in --project means the project it is an alias of.
	project = workspaces.CanonicalSlug(project)
	for _, agent := range agents {
		slug := workspaces.SlugForPath(agent.Cwd)
		if slug == "" {
			slug = WorkerProjectUnmatched
		}
		ownership, item := index.classify(agent)
		if !workerRowMatches(project, state, owned, slug, agent.State(), ownership) {
			continue
		}
		rows = append(rows, WorkerRow{
			ID:        agent.ID,
			Name:      agent.Name,
			Project:   slug,
			Ownership: ownership,
			State:     agent.State(),
			Age:       workerAge(agent.Created, now),
			Item:      item,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	if env.asJSON {
		return writeJSON(stdout, stderr, WorkerList{
			Workspaces: workspaces.Source,
			Count:      len(rows),
			Workers:    rows,
		})
	}
	for _, row := range rows {
		line := fmt.Sprintf("id=%s name=%s project=%s ownership=%s state=%s age=%s",
			row.ID, workerText(row.Name), row.Project, row.Ownership, row.State, row.Age)
		if row.Item != "" {
			line += " item=" + row.Item
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// workerRowMatches applies the three filters, which compose (AC-001).
func workerRowMatches(project, state string, owned bool, slug, stateToken, ownership string) bool {
	if project = strings.TrimSpace(project); project != "" {
		if !strings.EqualFold(slug, project) {
			return false
		}
	}
	if state = strings.TrimSpace(state); state != "" {
		if !strings.EqualFold(stateToken, state) {
			return false
		}
	}
	if owned && ownership != WorkerOwned {
		return false
	}
	return true
}

// workerText renders an empty name as "-" so a line keeps its column count.
func workerText(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return "-"
}

// workerAge turns paseo's created stamp into the age column.
//
// `ls --json` prints a relative phrase ("5 minutes ago") while `inspect` prints
// RFC3339; both are accepted and both end up as one compact token, so the column
// means the same thing no matter which verb answered.
func workerAge(created string, now time.Time) string {
	raw := strings.TrimSpace(created)
	if raw == "" || raw == "-" {
		return "-"
	}
	if at := parseInstant(raw); !at.IsZero() {
		return formatWorkerAge(now.Sub(at))
	}
	if normalized := normalizeRelativeAge(raw); normalized != "" {
		return normalized
	}
	return raw
}

// normalizeRelativeAge folds paseo's phrase onto the compact token, keeping the
// number so "3 days ago" does not become the vague "long ago".
func normalizeRelativeAge(raw string) string {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	if lowered == "just now" {
		return "0s"
	}
	phrase := strings.TrimSuffix(lowered, " ago")
	fields := strings.Fields(phrase)
	if len(fields) < 2 {
		return ""
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil || count < 0 {
		return ""
	}
	unit := strings.TrimSuffix(fields[1], "s")
	suffix := map[string]string{
		"second": "s",
		"minute": "m",
		"hour":   "h",
		"day":    "d",
		"week":   "w",
		"month":  "mo",
		"year":   "y",
	}[unit]
	if suffix == "" {
		return ""
	}
	return strconv.Itoa(count) + suffix
}

// formatWorkerAge renders a duration as the same compact token.
func formatWorkerAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd%dh", int(d/(24*time.Hour)), int((d%(24*time.Hour))/time.Hour))
	case d >= time.Hour:
		return fmt.Sprintf("%dh%dm", int(d/time.Hour), int((d%time.Hour)/time.Minute))
	case d >= time.Minute:
		return fmt.Sprintf("%dm%ds", int(d/time.Minute), int((d%time.Minute)/time.Second))
	}
	return strconv.Itoa(int(d/time.Second)) + "s"
}

// ---------------------------------------------------------------------------
// show (AC-002)
// ---------------------------------------------------------------------------

// WorkerShow is the nine facts of `worker show`.
type WorkerShow struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Cwd                string            `json:"cwd"`
	Project            string            `json:"project"`
	Status             string            `json:"status"`
	PendingPermissions []string          `json:"pending_permissions"`
	Labels             map[string]string `json:"labels"`
	Created            string            `json:"created"`
	Updated            string            `json:"updated"`
	// Ownership is not one of the nine fields; it is what makes "labels" readable
	// at a glance and what update/archive enforce.
	Ownership string `json:"ownership"`
}

func runWorkerShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "machine output")
	cf := addWorkerCommonFlags(fs)
	flags, positional := workerArgs(args)
	if err := fs.Parse(flags); err != nil {
		return flagExit(err, stderr)
	}
	id, ok := workerID("show", positional, stderr)
	if !ok {
		return 2
	}
	env, code := newWorkerEnv(cf, jsonOut, stderr)
	if code != 0 {
		return code
	}
	workspaces, err := env.workspaces()
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonWorkspacesInvalid,
			Detail:     err.Error(),
		})
	}
	agent, err := env.runner.inspectWorker(id)
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonAgentNotFound,
			AgentID:    id,
			Detail:     err.Error(),
		})
	}
	if strings.TrimSpace(agent.ID) == "" {
		agent.ID = id
	}
	index, lookupErr := loadWorkerLabelIndex(env.runner, []WorkerAgent{agent})
	if lookupErr != nil {
		fmt.Fprintf(stderr, "adbd worker show: label lookup incomplete: %v\n", lookupErr)
	}
	ownership, _ := index.classify(agent)
	slug := workspaces.SlugForPath(agent.Cwd)
	if slug == "" {
		slug = WorkerProjectUnmatched
	}
	show := WorkerShow{
		ID:                 agent.ID,
		Name:               agent.Name,
		Cwd:                agent.Cwd,
		Project:            slug,
		Status:             agent.State(),
		PendingPermissions: agent.toolNames(),
		Labels:             index.labelsFor(agent),
		Created:            agent.Created,
		Updated:            agent.Updated,
		Ownership:          ownership,
	}
	if show.PendingPermissions == nil {
		show.PendingPermissions = []string{}
	}
	if env.asJSON {
		return writeJSON(stdout, stderr, show)
	}
	fmt.Fprintf(stdout, "id=%s name=%s cwd=%s project=%s status=%s pending_permissions=%s labels=%s created=%s updated=%s ownership=%s\n",
		show.ID, workerText(show.Name), workerText(show.Cwd), show.Project, show.Status,
		joinOrDash(show.PendingPermissions), formatLabels(show.Labels),
		workerText(show.Created), workerText(show.Updated), show.Ownership)
	return 0
}

// joinOrDash renders a list as a,b or "-" when empty.
func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

// formatLabels renders labels as a stable k=v,k=v string or "-" when empty.
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ",")
}

// ---------------------------------------------------------------------------
// create (AC-003)
// ---------------------------------------------------------------------------

// WorkerCreated is the answer of `worker create`.
type WorkerCreated struct {
	AgentID string `json:"agent_id"`
	Project string `json:"project"`
	Cwd     string `json:"cwd"`
	Item    string `json:"item"`
}

func runWorkerCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var project, item, prompt, provider, model, mode string
	var jsonOut bool
	fs.StringVar(&project, "project", "", "project slug the worker belongs to")
	fs.StringVar(&item, "item", "", "work item id; becomes the title and adbd.item label")
	fs.StringVar(&prompt, "prompt", "", "task the worker is started with")
	fs.StringVar(&provider, "provider", "", "provider for the new worker")
	fs.StringVar(&model, "model", "", "model for the new worker")
	fs.StringVar(&mode, "mode", "", "provider mode for the new worker")
	fs.BoolVar(&jsonOut, "json", false, "machine output")
	cf := addWorkerCommonFlags(fs)
	flags, positional := workerArgs(args)
	if err := fs.Parse(flags); err != nil {
		return flagExit(err, stderr)
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "adbd worker create: unexpected argument %q\n", positional[0])
		return 2
	}
	if strings.TrimSpace(project) == "" {
		fmt.Fprintln(stderr, "adbd worker create: --project <slug> is required")
		return 2
	}
	if strings.TrimSpace(item) == "" {
		fmt.Fprintln(stderr, "adbd worker create: --item <id> is required")
		return 2
	}
	env, code := newWorkerEnv(cf, jsonOut, stderr)
	if code != 0 {
		return code
	}
	workspaces, err := env.workspaces()
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonWorkspacesInvalid,
			Detail:     err.Error(),
		})
	}
	slug := strings.TrimSpace(project)
	dir, registered := workspaces.PathForSlug(slug)
	if !registered {
		// AC-003 names adbd_projects_file as a table create resolves against. The
		// mapping table answers first because it is the one `worker list` uses to
		// name a project: a worker must start where the project says its workers
		// live, or list would immediately disagree with create. The project
		// registry is the fallback, so a slug registered only for dispatch still
		// works.
		registry := loadProjectRegistry(env.cfg.ProjectsFile, env.root)
		if resolved, hit := registry.Resolve(slug); hit {
			dir, registered = resolved, true
		}
	}
	if !registered || !isDirectory(dir) {
		// AC-003: no directory, no paseo call. The worker is never created
		// somewhere else and then reported as a near miss.
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonProjectNotRegistered,
			Detail:     fmt.Sprintf("project %q has no registered directory", slug),
			Hint:       "register it in " + workspacesFilePath(env.cfg.WorkspacesFile, env.root),
		})
	}
	agentID, err := env.runner.createWorker(workerSpec{
		Item:     strings.TrimSpace(item),
		Cwd:      dir,
		Prompt:   prompt,
		Provider: provider,
		Model:    model,
		Mode:     mode,
	})
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonPaseoUnavailable,
			Detail:     err.Error(),
		})
	}
	created := WorkerCreated{
		AgentID: agentID,
		Project: slug,
		Cwd:     dir,
		Item:    strings.TrimSpace(item),
	}
	if env.asJSON {
		return writeJSON(stdout, stderr, created)
	}
	fmt.Fprintf(stdout, "agent_id=%s project=%s cwd=%s item=%s\n",
		created.AgentID, created.Project, created.Cwd, created.Item)
	return 0
}

// ---------------------------------------------------------------------------
// update (AC-004)
// ---------------------------------------------------------------------------

// WorkerUpdated is the answer of `worker update`.
type WorkerUpdated struct {
	ID        string   `json:"id"`
	Ownership string   `json:"ownership"`
	Item      string   `json:"item,omitempty"`
	Name      string   `json:"name,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Mode      string   `json:"mode,omitempty"`
}

func runWorkerUpdate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var name, mode string
	var labels labelFlag
	var cwdFlag, providerFlag, modelFlag string
	var jsonOut, force bool
	fs.StringVar(&name, "name", "", "new name for the worker")
	fs.Var(&labels, "label", "label in k=v form (repeatable)")
	fs.StringVar(&mode, "mode", "", "new provider mode")
	// These three are declared only so the refusal can name them: paseo cannot
	// move, re-provider or re-model an existing agent.
	fs.StringVar(&cwdFlag, "cwd", "", "not updatable; create a new worker instead")
	fs.StringVar(&providerFlag, "provider", "", "not updatable; create a new worker instead")
	fs.StringVar(&modelFlag, "model", "", "not updatable; create a new worker instead")
	fs.BoolVar(&jsonOut, "json", false, "machine output")
	fs.BoolVar(&force, "force", false, "allow editing a worker tagged "+ForeignLabel)
	cf := addWorkerCommonFlags(fs)
	flags, positional := workerArgs(args)
	if err := fs.Parse(flags); err != nil {
		return flagExit(err, stderr)
	}
	id, ok := workerID("update", positional, stderr)
	if !ok {
		return 2
	}
	env, code := newWorkerEnv(cf, jsonOut, stderr)
	if code != 0 {
		return code
	}
	// AC-004: the un-updatable fields are refused before any paseo call, and the
	// answer says what to do instead.
	if blocked := firstUnupdatableField(fs); blocked != "" {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonFieldNotUpdatable,
			AgentID:    id,
			Detail:     fmt.Sprintf("--%s cannot be changed on an existing worker", blocked),
			Hint:       "create a worker with the field instead: adbd worker create --project <slug> --item <id> --" + blocked + " ...",
		})
	}
	if strings.TrimSpace(name) == "" && len(labels) == 0 && strings.TrimSpace(mode) == "" {
		fmt.Fprintln(stderr, "adbd worker update: nothing to change (give --name, --label or --mode)")
		return 2
	}
	agent, err := env.runner.inspectWorker(id)
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonAgentNotFound,
			AgentID:    id,
			Detail:     err.Error(),
		})
	}
	if strings.TrimSpace(agent.ID) == "" {
		agent.ID = id
	}
	index, err := loadWorkerLabelIndex(env.runner, []WorkerAgent{agent})
	if err != nil {
		// Failing closed: "we could not read the labels" is not "it is not ours",
		// and it is certainly not permission to write.
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonPaseoUnavailable,
			AgentID:    id,
			Detail:     err.Error(),
		})
	}
	ownership, item := index.classify(agent)
	if ownership == WorkerForeign && !force {
		// AC-004: foreign is a deliberate "not ours"; editing it needs --force.
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonOwnershipUnknown,
			AgentID:    id,
			Detail:     id + " is tagged " + ForeignLabel + "; editing it needs --force",
			Hint:       "adbd worker update " + id + " --force ...",
		})
	}
	if ownership == WorkerUnknown {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonOwnershipUnknown,
			AgentID:    id,
			Detail:     fmt.Sprintf("%s carries neither %s nor %s", id, OwnershipLabel, ForeignLabel),
			Hint:       "claim it first: adbd adopt --apply",
		})
	}
	if trimmed := strings.TrimSpace(name); trimmed != "" || len(labels) > 0 {
		if err := env.runner.updateWorker(agent.ID, name, labels); err != nil {
			return env.fail(stdout, stderr, workerFailure{
				ReasonCode: ReasonPaseoUnavailable,
				AgentID:    agent.ID,
				Detail:     err.Error(),
			})
		}
	}
	if trimmed := strings.TrimSpace(mode); trimmed != "" {
		if err := env.runner.setWorkerMode(agent.ID, trimmed); err != nil {
			return env.fail(stdout, stderr, workerFailure{
				ReasonCode: ReasonPaseoUnavailable,
				AgentID:    agent.ID,
				Detail:     err.Error(),
			})
		}
	}
	updated := WorkerUpdated{
		ID:        agent.ID,
		Ownership: ownership,
		Item:      item,
		Name:      strings.TrimSpace(name),
		Labels:    labels,
		Mode:      strings.TrimSpace(mode),
	}
	if env.asJSON {
		return writeJSON(stdout, stderr, updated)
	}
	line := fmt.Sprintf("id=%s ownership=%s", updated.ID, updated.Ownership)
	if updated.Item != "" {
		line += " item=" + updated.Item
	}
	if updated.Name != "" {
		line += " name=" + updated.Name
	}
	if len(updated.Labels) > 0 {
		line += " labels=" + strings.Join(updated.Labels, ",")
	}
	if updated.Mode != "" {
		line += " mode=" + updated.Mode
	}
	fmt.Fprintln(stdout, line)
	return 0
}

// firstUnupdatableField reports the first of --cwd/--provider/--model that was
// actually given, so `--cwd ""` cannot slip through as "not set".
func firstUnupdatableField(fs *flag.FlagSet) string {
	blocked := ""
	fs.Visit(func(f *flag.Flag) {
		if blocked != "" {
			return
		}
		switch f.Name {
		case "cwd", "provider", "model":
			blocked = f.Name
		}
	})
	return blocked
}

// ---------------------------------------------------------------------------
// archive (AC-005)
// ---------------------------------------------------------------------------

// workerAuditRecord is one line of .beacon/adbd-worker.jsonl. One line per
// successful archive, and nothing per refused or repeated call.
type workerAuditRecord struct {
	At        string `json:"at"`
	Action    string `json:"action"`
	AgentID   string `json:"agent_id"`
	Ownership string `json:"ownership"`
	Item      string `json:"item,omitempty"`
	Forced    bool   `json:"forced,omitempty"`
	// ReleasedBindings counts the bindings this archive let go (AC-005).
	ReleasedBindings int `json:"released_bindings,omitempty"`
}

// WorkerArchived is the answer of `worker archive`.
type WorkerArchived struct {
	ID               string `json:"id"`
	ReleasedBindings int    `json:"released_bindings,omitempty"`
	Ownership        string `json:"ownership"`
	Item             string `json:"item,omitempty"`
	Archived         bool   `json:"archived"`
	AlreadyArchived  bool   `json:"already_archived"`
	Forced           bool   `json:"forced"`
	AuditPath        string `json:"audit_path,omitempty"`
}

func runWorkerArchive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker archive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var force, jsonOut bool
	fs.BoolVar(&force, "force", false, "let paseo interrupt a running agent before archiving it")
	fs.BoolVar(&jsonOut, "json", false, "machine output")
	cf := addWorkerCommonFlags(fs)
	flags, positional := workerArgs(args)
	if err := fs.Parse(flags); err != nil {
		return flagExit(err, stderr)
	}
	id, ok := workerID("archive", positional, stderr)
	if !ok {
		return 2
	}
	env, code := newWorkerEnv(cf, jsonOut, stderr)
	if code != 0 {
		return code
	}
	agent, err := env.runner.inspectWorker(id)
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonAgentNotFound,
			AgentID:    id,
			Detail:     err.Error(),
		})
	}
	if strings.TrimSpace(agent.ID) == "" {
		agent.ID = id
	}
	// AC-005: a repeated archive is a no-op success, not an error, and it is not
	// a second archive call either.
	if agent.State() == WorkerStateArchived {
		archived := WorkerArchived{
			ID:              agent.ID,
			Ownership:       WorkerUnknown,
			AlreadyArchived: true,
			Forced:          force,
		}
		if index, lookupErr := loadWorkerLabelIndex(env.runner, []WorkerAgent{agent}); lookupErr == nil {
			archived.Ownership, archived.Item = index.classify(agent)
		}
		return workerArchiveResult(stdout, stderr, env, archived)
	}
	index, err := loadWorkerLabelIndex(env.runner, []WorkerAgent{agent})
	if err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonPaseoUnavailable,
			AgentID:    id,
			Detail:     err.Error(),
		})
	}
	ownership, item := index.classify(agent)
	if ownership == WorkerForeign && !force {
		// AC-005: a foreign agent is explicitly not ours. Archive it only when the
		// operator says so out loud.
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonOwnershipUnknown,
			AgentID:    id,
			Detail:     id + " is tagged " + ForeignLabel + "; archiving it needs --force",
			Hint:       "adbd worker archive " + id + " --force",
		})
	}
	if ownership == WorkerUnknown {
		// AC-005/AC-007: nobody proved this agent is ours, so nothing is written.
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonOwnershipUnknown,
			AgentID:    id,
			Detail:     fmt.Sprintf("%s carries neither %s nor %s", id, OwnershipLabel, ForeignLabel),
			Hint:       "claim it first: adbd adopt --apply",
		})
	}
	if err := env.runner.archiveWorker(agent.ID, force); err != nil {
		return env.fail(stdout, stderr, workerFailure{
			ReasonCode: ReasonPaseoUnavailable,
			AgentID:    agent.ID,
			Detail:     err.Error(),
		})
	}
	archived := WorkerArchived{ID: agent.ID, Ownership: ownership, Item: item, Archived: true, Forced: force}
	released, relErr := releaseBindingsForAgent(env, agent.ID)
	if relErr != nil {
		// The archive happened; the binding release did not. Report it instead of
		// claiming a clean archive, because the stale binding wakes the agent up.
		fmt.Fprintf(stderr, "adbd worker archive: archived %s but could not release its bindings: %v\n", agent.ID, relErr)
	}
	archived.ReleasedBindings = released
	path, err := env.appendWorkerAudit(workerAuditRecord{
		At:               workerClock().UTC().Format(time.RFC3339),
		Action:           "archive",
		AgentID:          agent.ID,
		Ownership:        ownership,
		Item:             item,
		Forced:           force,
		ReleasedBindings: released,
	})
	archived.AuditPath = path
	if err != nil {
		// The archive happened; the record of it did not. Saying "archived" while
		// the ledger is missing would be the one silent lie this surface must not
		// tell, so the pass still succeeds but the failure is printed.
		fmt.Fprintf(stderr, "adbd worker archive: archived %s but could not write %s: %v\n", agent.ID, path, err)
	}
	return workerArchiveResult(stdout, stderr, env, archived)
}

// workerArchiveResult prints the archive outcome and returns its exit code.
func workerArchiveResult(stdout, stderr io.Writer, env workerEnv, archived WorkerArchived) int {
	if env.asJSON {
		return writeJSON(stdout, stderr, archived)
	}
	line := fmt.Sprintf("id=%s ownership=%s archived=%s already_archived=%s",
		archived.ID, archived.Ownership, boolToken(archived.Archived), boolToken(archived.AlreadyArchived))
	if archived.Item != "" {
		line += " item=" + archived.Item
	}
	if archived.Forced {
		line += " forced=true"
	}
	if archived.AuditPath != "" {
		line += " audit=" + archived.AuditPath
	}
	fmt.Fprintln(stdout, line)
	return 0
}

// appendWorkerAudit writes one line to .beacon/adbd-worker.jsonl and returns the
// path it tried, so the caller can name it even when the write fails.
func (e workerEnv) appendWorkerAudit(record workerAuditRecord) (string, error) {
	path := filepath.Join(e.beacon, WorkerAuditFile)
	if err := os.MkdirAll(e.beacon, 0o755); err != nil {
		return path, err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return path, err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return path, err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return path, err
	}
	return path, f.Close()
}

// releaseBindingsForAgent marks every binding that points at agentID as released.
//
// AC-005: an archived worker must not stay routable. The binding row stays (the
// thread keeps its history) but its state becomes released, so intake takes the
// slow path instead of waking the archived agent through the fast path.
func releaseBindingsForAgent(env workerEnv, agentID string) (int, error) {
	path := strings.TrimSpace(env.statePath)
	if path == "" {
		path = filepath.Join(env.beacon, "adbd-state.json")
	}
	if _, err := os.Stat(path); err != nil {
		return 0, nil // no state document yet: nothing is bound
	}
	store := &Store{Path: path, SlowPath: filepath.Join(env.beacon, "adbd-slowpath.jsonl")}
	released := 0
	err := store.Update(func(snap *snapshot) error {
		for _, binding := range snap.Bindings {
			if binding == nil || binding.AgentID != agentID || binding.State == StateReleased {
				continue
			}
			binding.State = StateReleased
			binding.UpdatedAt = workerClock().UTC()
			released++
		}
		return nil
	})
	return released, err
}
