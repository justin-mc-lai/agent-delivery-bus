// Supervision of paseo agents (truth.md, feature agent-supervisor, v0.1.5).
//
// The problem this file solves is not "is the agent running" -- paseo already
// answers that, and its answer is the useless one. A hundred stopped agents all
// say "idle": some finished, some are waiting for a human, some were abandoned
// mid-way. Only four facts separate them, and they are all observable:
//
//  1. the permission requests paseo is holding open for the agent
//  2. the agent status (running / idle / error)
//  3. the work item's own report file (.beacon/adbd-reports/<item>.json)
//  4. how long the agent has been quiet since its last real activity
//
// The judgement is a fixed priority over those four facts. No model is asked,
// no state is invented: truth.md froze the table, this file implements it, and
// every row names the fact that decided it (`because=`), so a human can check the
// judgement instead of trusting it.
//
// The other half of the invariant is ownership. An agent carries an `adbd.item`
// label naming the work item it serves, which is a thread string; an agent
// without one is foreign and nobody here may touch it. That is why every action
// in this file is either archive (never delete), a permission decision, or a
// single probe message.
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

// Supervision states (truth.md 状态判定). `archived` is not a fact paseo reports:
// it is what this file records after it has archived an agent, so a later pass
// spends zero actions on work it already reaped (AC-007).
const (
	SupStateWaitingPermission = "waiting_permission"
	SupStateWorking           = "working"
	SupStateDone              = "done"
	SupStateWaitingInput      = "waiting_input"
	SupStateGrace             = "grace"
	SupStateOrphan            = "orphan"
	SupStateFailed            = "failed"
	SupStateUnknown           = "unknown"
	SupStateArchived          = "archived"
)

// Supervision actions (truth.md 状态判定 third column, AC-005..AC-007).
const (
	SupActionAllow    = "allow"
	SupActionEscalate = "escalate"
	SupActionProbe    = "probe"
	SupActionFail     = "fail"
	SupActionArchive  = "archive"
	SupActionNone     = "none"
)

// Ownership labels. paseo labels are the only durable, paseo-native way to say
// "this agent belongs to this work item" (Open Question Q1), and the work item
// id is the thread string itself, so no second naming scheme is introduced.
const (
	// OwnershipLabel tags an agent with the work item it serves.
	OwnershipLabel = "adbd.item"
	// ForeignLabel tags an agent this tool has decided never to manage.
	ForeignLabel = "adbd.foreign"
	// ForeignLabelValue is the value written for ForeignLabel.
	ForeignLabelValue = "1"
)

// Work item report statuses (truth.md 状态判定 fact 3). A report says one of two
// things that change the judgement: the item is ready (done) or it needs a human
// (waiting_input). Any other status is silence, not a third verdict.
const (
	ReportReady     = "ready"
	ReportAttention = "attention"
)

// Defaults and file names this feature owns (truth.md 配置项).
const (
	// DefaultGraceAfter is adbd_grace_after: how long an idle agent without a
	// report may sit before it is called orphan.
	DefaultGraceAfter = 5 * time.Minute
	// DefaultMaxActive is adbd_max_active: the ceiling on owned, working agents
	// (AC-008).
	DefaultMaxActive = 8
	// superviseAuditFile is the append-only audit ledger (AC-005).
	superviseAuditFile = "adbd-supervise.jsonl"
	// superviseStateFile is what this feature remembers between passes: who was
	// probed, who was archived.
	superviseStateFile = "adbd-supervise-state.json"
	// reportsDirName is the per-work-item report directory (adbd_report_dir).
	reportsDirName = "adbd-reports"
	// probeMessage is the one line an orphan agent receives, ever (AC-006).
	probeMessage = "adbd: status?"
)

// ---------------------------------------------------------------------------
// the observed agent
// ---------------------------------------------------------------------------

// PendingPermission is one permission request paseo is holding open.
type PendingPermission struct {
	// Tool is the tool name the agent wants to run.
	Tool string
	// At is when the request was raised, when paseo says.
	At time.Time
}

// PaseoAgent is one row of `paseo ls --json`, narrowed to the facts supervision
// is allowed to use. Anything else paseo prints is ignored on purpose: an extra
// field must never become a fifth fact.
type PaseoAgent struct {
	// ID is the agent id (agentId).
	ID string
	// Name is the agent name/title. It is the second way an agent can be owned:
	// a name equal to a bound thread is a match (AC-002).
	Name string
	// Status is paseo's raw status string, unmodified.
	Status string
	// Labels are the agent labels, e.g. adbd.item=skill:studio:probe.
	Labels map[string]string
	// PendingPermissions are the permission requests still open.
	PendingPermissions []PendingPermission
	// LastActiveAt is the agent's last real activity, zero when paseo says
	// nothing. Grace and orphan are measured from it.
	LastActiveAt time.Time
}

// Label reads one label; "" means "not set".
func (a PaseoAgent) Label(key string) string {
	if a.Labels == nil {
		return ""
	}
	return strings.TrimSpace(a.Labels[key])
}

// toolNames is the deduped, sorted list of tools the agent is waiting on.
func (a PaseoAgent) toolNames() []string {
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

// rawAgent is the decoding shape of one `paseo ls --json` row. Alternative
// spellings are tolerated because the agent list is an external contract; the
// fields this file acts on are not.
type rawAgent struct {
	AgentID            string          `json:"agentId"`
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Title              string          `json:"title"`
	Status             string          `json:"status"`
	Labels             json.RawMessage `json:"labels"`
	PendingPermissions json.RawMessage `json:"pendingPermissions"`
	LastActivityAt     string          `json:"lastActivityAt"`
	LastActiveAt       string          `json:"lastActiveAt"`
	UpdatedAt          string          `json:"updatedAt"`
}

// rawPermission is one permission request. `tool` is the field this file acts
// on; the alternatives are tolerated spellings of the same fact.
type rawPermission struct {
	Tool        string `json:"tool"`
	ToolName    string `json:"toolName"`
	Name        string `json:"name"`
	At          string `json:"at"`
	RequestedAt string `json:"requestedAt"`
}

// parseAgentList reads `paseo ls --json`.
//
// Three shapes are accepted -- {"agents":[...]}, a bare [...], and NDJSON -- but
// a row without an agent id is an error, not a row to skip: silently dropping an
// agent would make supervision report a shorter world than the real one.
func parseAgentList(out []byte) ([]PaseoAgent, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, errors.New("paseo ls --json printed nothing")
	}
	switch trimmed[0] {
	case '{':
		var doc struct {
			Agents []rawAgent `json:"agents"`
		}
		if err := json.Unmarshal(trimmed, &doc); err == nil && doc.Agents != nil {
			return convertAgents(doc.Agents)
		}
		// A single NDJSON stream can also start with '{'. Fall through to the
		// line scan before calling it broken.
	case '[':
		var rows []rawAgent
		if err := json.Unmarshal(trimmed, &rows); err == nil {
			return convertAgents(rows)
		}
		return nil, fmt.Errorf("paseo ls --json is not a JSON array (output: %s)", preview(trimmed))
	}
	var rows []rawAgent
	for i, line := range strings.Split(string(trimmed), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row rawAgent
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("paseo ls --json line %d is not an agent object: %w", i+1, err)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("paseo ls --json printed no agent list (output: %s)", preview(trimmed))
	}
	return convertAgents(rows)
}

// convertAgents narrows the decoding shape onto PaseoAgent.
func convertAgents(rows []rawAgent) ([]PaseoAgent, error) {
	out := make([]PaseoAgent, 0, len(rows))
	for i, row := range rows {
		id := strings.TrimSpace(row.AgentID)
		if id == "" {
			id = strings.TrimSpace(row.ID)
		}
		if id == "" {
			return nil, fmt.Errorf("paseo ls --json row %d has no agentId", i+1)
		}
		labels, err := parseLabels(row.Labels)
		if err != nil {
			return nil, fmt.Errorf("paseo ls --json agent %s: %w", id, err)
		}
		perms, err := parsePermissions(row.PendingPermissions)
		if err != nil {
			return nil, fmt.Errorf("paseo ls --json agent %s: %w", id, err)
		}
		name := strings.TrimSpace(row.Name)
		if name == "" {
			name = strings.TrimSpace(row.Title)
		}
		out = append(out, PaseoAgent{
			ID:                 id,
			Name:               name,
			Status:             strings.TrimSpace(row.Status),
			Labels:             labels,
			PendingPermissions: perms,
			LastActiveAt:       firstInstant(row.LastActivityAt, row.LastActiveAt, row.UpdatedAt),
		})
	}
	return out, nil
}

// parseLabels accepts both the object form ({"adbd.item":"x"}) and the list form
// (["adbd.item=x"]) because paseo has used both spellings.
func parseLabels(raw json.RawMessage) (map[string]string, error) {
	labels := map[string]string{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return labels, nil
	}
	var asMap map[string]string
	if err := json.Unmarshal(trimmed, &asMap); err == nil {
		for key, value := range asMap {
			labels[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		return labels, nil
	}
	var asList []string
	if err := json.Unmarshal(trimmed, &asList); err != nil {
		return nil, errors.New("labels is neither an object nor a list of key=value strings")
	}
	for _, entry := range asList {
		key, value, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		labels[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return labels, nil
}

// parsePermissions accepts the object form ([{"tool":"bash"}]) and the plain
// string form (["bash"]).
func parsePermissions(raw json.RawMessage) ([]PendingPermission, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	var objects []rawPermission
	if err := json.Unmarshal(trimmed, &objects); err == nil {
		out := make([]PendingPermission, 0, len(objects))
		for _, obj := range objects {
			tool := firstToken(obj.Tool, obj.ToolName, obj.Name)
			if tool == "" {
				continue
			}
			out = append(out, PendingPermission{Tool: tool, At: firstInstant(obj.At, obj.RequestedAt)})
		}
		return out, nil
	}
	var names []string
	if err := json.Unmarshal(trimmed, &names); err != nil {
		return nil, errors.New("pendingPermissions is neither a list of objects nor a list of tool names")
	}
	out := make([]PendingPermission, 0, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, PendingPermission{Tool: name})
		}
	}
	return out, nil
}

// firstToken returns the first non-empty, trimmed candidate.
func firstToken(candidates ...string) string {
	for _, candidate := range candidates {
		if token := strings.TrimSpace(candidate); token != "" {
			return token
		}
	}
	return ""
}

// firstInstant returns the first candidate that parses as an instant.
func firstInstant(candidates ...string) time.Time {
	for _, candidate := range candidates {
		if at := parseInstant(candidate); !at.IsZero() {
			return at
		}
	}
	return time.Time{}
}

// parseInstant reads an RFC3339 timestamp (with or without fractional seconds).
func parseInstant(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if at, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return at
	}
	return time.Time{}
}

// preview shortens child output for an error message.
func preview(out []byte) string {
	text := strings.TrimSpace(string(out))
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	return text
}

// ---------------------------------------------------------------------------
// ownership
// ---------------------------------------------------------------------------

// ownedAgent is one agent plus the work item that owns it.
type ownedAgent struct {
	// Item is the work item id, which equals the thread string.
	Item string
	// Agent is the agent itself.
	Agent PaseoAgent
	// Source says which match won: "label" or "name".
	Source string
}

// classifyOwnership splits paseo's list into the agents this tool manages and
// the ones it must never touch (AC-002).
//
// A binding thread is the only thing that can own an agent: an `adbd.item` label
// naming a thread wins, an agent name equal to a thread is the fallback (that is
// how the pre-label backlog is recognized). Everything else -- including an
// agent whose label names a thread that is no longer bound -- is foreign.
func classifyOwnership(agents []PaseoAgent, threads []string) (owned []ownedAgent, foreign []PaseoAgent) {
	bound := make(map[string]struct{}, len(threads))
	for _, thread := range threads {
		if thread = strings.TrimSpace(thread); thread != "" {
			bound[thread] = struct{}{}
		}
	}
	for _, agent := range agents {
		if item := agent.Label(OwnershipLabel); item != "" {
			if _, ok := bound[item]; ok {
				owned = append(owned, ownedAgent{Item: item, Agent: agent, Source: "label"})
				continue
			}
		}
		if name := strings.TrimSpace(agent.Name); name != "" {
			if _, ok := bound[name]; ok {
				owned = append(owned, ownedAgent{Item: name, Agent: agent, Source: "name"})
				continue
			}
		}
		foreign = append(foreign, agent)
	}
	sort.Slice(owned, func(i, j int) bool {
		if owned[i].Item != owned[j].Item {
			return owned[i].Item < owned[j].Item
		}
		return owned[i].Agent.ID < owned[j].Agent.ID
	})
	sort.Slice(foreign, func(i, j int) bool { return foreign[i].ID < foreign[j].ID })
	return owned, foreign
}

// bindingThreads is the sorted, deduped list of threads the bindings name.
func bindingThreads(bindings []Binding) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		thread := strings.TrimSpace(binding.Thread)
		if thread == "" {
			continue
		}
		if _, done := seen[thread]; done {
			continue
		}
		seen[thread] = struct{}{}
		out = append(out, thread)
	}
	sort.Strings(out)
	return out
}

// bindingRows copies the bindings out of a snapshot in thread order, so a pass
// never depends on Go's map iteration order.
func bindingRows(snap *snapshot) []Binding {
	if snap == nil {
		return nil
	}
	out := make([]Binding, 0, len(snap.Bindings))
	for _, binding := range snap.Bindings {
		if binding != nil {
			out = append(out, *binding)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		return out[i].Thread < out[j].Thread
	})
	return out
}

// loadBindingRows reads the bindings under the store's lock.
func loadBindingRows(store *Store) ([]Binding, error) {
	if store == nil || strings.TrimSpace(store.Path) == "" {
		return nil, nil
	}
	var rows []Binding
	if err := store.View(func(snap *snapshot) error {
		rows = bindingRows(snap)
		return nil
	}); err != nil {
		return nil, err
	}
	return rows, nil
}

// ---------------------------------------------------------------------------
// the work item report (fact 3)
// ---------------------------------------------------------------------------

// WorkItemReport is one work item's own word about itself, read from
// .beacon/adbd-reports/<item>.json.
type WorkItemReport struct {
	Status string `json:"status"`
	At     string `json:"at"`
}

// reportFileName maps a work item id onto its report file name. A thread may
// contain a slash (gh:owner/repo#1), which is a directory separator, so the
// separator is folded rather than allowed to decide where the file goes.
func reportFileName(item string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_").Replace(strings.TrimSpace(item))
	return safe + ".json"
}

// readReport reads one work item's report. A missing file is (nil, nil): "the
// item has not reported" is a fact, not an error. A file that is not JSON is an
// error, because that is a broken producer, not silence.
func readReport(dir, item string) (*WorkItemReport, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" || strings.TrimSpace(item) == "" {
		return nil, nil
	}
	path := filepath.Join(dir, reportFileName(item))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}
	var report WorkItemReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("adbd supervise: report %s is not JSON: %w", path, err)
	}
	return &report, nil
}

// ---------------------------------------------------------------------------
// the judgement
// ---------------------------------------------------------------------------

// superviseStatus folds paseo's status vocabulary onto the three statuses that
// decide anything here. Unlike reconcile's normaliseStatus, "error" is its own
// answer and an unrecognised status stays unrecognised: AC-004 requires an
// unknown state to be reported as unknown, never guessed into idle.
func superviseStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "running", "working", "busy", "thinking", "in_progress", "in-progress":
		return StatusRunning
	case "idle", "waiting", "paused":
		return StatusIdle
	case "error", "errored", "failed":
		return "error"
	}
	return ""
}

// classifyItem applies the four facts in the frozen priority order and returns
// the state plus the fact that decided it.
func classifyItem(agent PaseoAgent, report *WorkItemReport, now time.Time, grace time.Duration) (state, because string) {
	// Fact 1 outranks everything: an agent blocked on a permission is not
	// "working", whatever its status says.
	if len(agent.PendingPermissions) > 0 {
		return SupStateWaitingPermission, "pending_permission"
	}
	switch superviseStatus(agent.Status) {
	case StatusRunning:
		return SupStateWorking, "status=running"
	case StatusIdle:
		if report != nil {
			switch strings.ToLower(strings.TrimSpace(report.Status)) {
			case ReportReady:
				return SupStateDone, "report=ready"
			case ReportAttention:
				return SupStateWaitingInput, "report=attention"
			}
			// Any other report status is not a verdict; fall through to the
			// idle-duration facts instead of inventing a third one.
		}
		if !agent.LastActiveAt.IsZero() && !now.Before(agent.LastActiveAt) && now.Sub(agent.LastActiveAt) >= grace {
			return SupStateOrphan, "idle_beyond_grace"
		}
		// No activity timestamp at all is treated as "just went quiet": the
		// grace window is the cheap, reversible answer, and orphan is not.
		return SupStateGrace, "idle_within_grace"
	case "error":
		return SupStateFailed, "status=error"
	}
	raw := strings.TrimSpace(agent.Status)
	if raw == "" {
		raw = "missing"
	}
	return SupStateUnknown, "status=" + raw
}

// permissionSafe reports whether every open permission request names a tool free
// of the high-risk words delete / push / release / deploy (AC-005).
//
// An empty tool name is not safe: an unreadable request cannot be proven
// harmless, and escalating costs a human one glance while allowing costs them
// the repository.
func permissionSafe(tools []string) bool {
	if len(tools) == 0 {
		return false
	}
	for _, tool := range tools {
		lowered := strings.ToLower(tool)
		for _, verb := range highRiskVerbs {
			if strings.Contains(lowered, verb) {
				return false
			}
		}
	}
	return true
}

// waitingFact is how long this row has been waiting for a human, which is what
// `adbd inbox` sorts by (AC-009). Only the two human-facing states have one.
func waitingFact(state string, agent PaseoAgent, report *WorkItemReport, now time.Time) (string, int64) {
	var since time.Time
	switch state {
	case SupStateWaitingPermission:
		for _, perm := range agent.PendingPermissions {
			if perm.At.IsZero() || perm.At.After(now) {
				continue
			}
			if since.IsZero() || perm.At.Before(since) {
				since = perm.At
			}
		}
	case SupStateWaitingInput:
		if report != nil {
			since = parseInstant(report.At)
		}
	default:
		return "", 0
	}
	if since.IsZero() {
		since = agent.LastActiveAt
	}
	if since.IsZero() || since.After(now) {
		return formatDuration(0), 0
	}
	waited := now.Sub(since)
	return formatDuration(waited), int64(waited / time.Second)
}

// ---------------------------------------------------------------------------
// what supervision remembers between passes
// ---------------------------------------------------------------------------

// superviseItemState is what one work item remembers: whether the single probe
// has been sent (AC-006) and whether the agent was already archived (AC-007).
type superviseItemState struct {
	Item       string `json:"item"`
	AgentID    string `json:"agent_id,omitempty"`
	ProbedAt   string `json:"probed_at,omitempty"`
	Archived   bool   `json:"archived,omitempty"`
	ArchivedAt string `json:"archived_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// superviseDoc is the whole memory document, keyed by work item id.
type superviseDoc struct {
	Version int                           `json:"version"`
	Items   map[string]superviseItemState `json:"items"`
}

// loadSuperviseDoc reads the memory document. A missing or empty file is an
// empty document: the first pass of a fresh checkout is not an error.
func loadSuperviseDoc(path string) (*superviseDoc, error) {
	doc := &superviseDoc{Version: stateVersion, Items: map[string]superviseItemState{}}
	if strings.TrimSpace(path) == "" {
		return doc, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil
		}
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return doc, nil
	}
	if err := json.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("adbd supervise: %s is not a supervise state: %w", path, err)
	}
	if doc.Items == nil {
		doc.Items = map[string]superviseItemState{}
	}
	return doc, nil
}

// saveSuperviseDoc writes the memory document atomically (temp file + rename), so
// a killed pass never leaves half a document behind.
func saveSuperviseDoc(path string, doc *superviseDoc) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	doc.Version = stateVersion
	if doc.Items == nil {
		doc.Items = map[string]superviseItemState{}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".adbd-supervise-state-*.tmp")
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
// the audit ledger
// ---------------------------------------------------------------------------

// superviseAudit is one line of .beacon/adbd-supervise.jsonl. AC-005 froze the
// field set at exactly these five keys, in this order.
type superviseAudit struct {
	Item    string `json:"item"`
	AgentID string `json:"agent_id"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
	At      string `json:"at"`
}

// appendSuperviseAudit appends one line per action in a single write, so a killed
// process leaves whole lines behind.
func appendSuperviseAudit(path string, lines []superviseAudit) error {
	if len(lines) == 0 || strings.TrimSpace(path) == "" {
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
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
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

// readSuperviseAudit reads the whole ledger, for tests and for `--outcome json`
// neighbours that want to show history.
func readSuperviseAudit(path string) ([]superviseAudit, error) {
	var out []superviseAudit
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec superviseAudit
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("adbd supervise: %s line %d is not an audit line: %w", path, i+1, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// the paseo CLI seam (the only subprocess in this feature)
// ---------------------------------------------------------------------------

// AgentLister lists the agents paseo knows about.
type AgentLister interface {
	ListAgents() ([]PaseoAgent, error)
}

// AgentLabeler tags an existing agent.
type AgentLabeler interface {
	TagLabel(agentID, key, value string) error
}

// TaggingRunner is the paseo surface `adbd adopt` needs.
type TaggingRunner interface {
	AgentLister
	AgentLabeler
}

// LabelLookup is an optional capability: paseo's `ls --json` does not return
// labels, but `ls --label k=v` filters by one. A runner that implements this
// can report which agents already carry a label, which is what makes a second
// `adopt --apply` report the same numbers as the first (AC-003).
type LabelLookup interface {
	LabeledIDs(key, value string) (map[string]bool, error)
}

// SupervisorRunner is the whole paseo surface supervision needs: list, tag,
// permit, probe, archive. It is an interface so a test can prove each action
// without a real paseo, and because the loop must be unable to invent a sixth
// verb.
type SupervisorRunner interface {
	TaggingRunner
	// PermitAllow answers every open permission request of one agent.
	PermitAllow(agentID string) error
	// SendProbe sends one text to one agent.
	SendProbe(agentID, text string) error
	// Archive archives one agent. There is deliberately no delete.
	Archive(agentID string) error
}

// PaseoCliRunner drives the paseo CLI. paseo is treated as an opaque external
// CLI: argv only, no import, no forking of its code.
type PaseoCliRunner struct {
	// Bin is the paseo executable; empty means "paseo" from PATH.
	Bin string
}

// compile-time proof that the CLI runner satisfies the seams.
var (
	_ SupervisorRunner = PaseoCliRunner{}
	_ TaggingRunner    = PaseoCliRunner{}
	_ LabelLookup      = PaseoCliRunner{}
)

// LabeledIDs lists the ids of agents that already carry key=value.
func (r PaseoCliRunner) LabeledIDs(key, value string) (map[string]bool, error) {
	out, err := r.run("ls", "--label", key+"="+value, "--json")
	if err != nil {
		return nil, err
	}
	agents, err := parseAgentList(out)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(agents))
	for _, a := range agents {
		ids[a.ID] = true
	}
	return ids, nil
}

func (r PaseoCliRunner) bin() string {
	if bin := strings.TrimSpace(r.Bin); bin != "" {
		return bin
	}
	return "paseo"
}

// run executes one paseo verb and returns stdout, folding stderr into the error
// so a failed call explains itself.
func (r PaseoCliRunner) run(args ...string) ([]byte, error) {
	bin := r.bin()
	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("runner %s %s: %w (output: %s)",
			bin, strings.Join(args, " "), err, preview(stderr.Bytes()))
	}
	return out, nil
}

// ListAgents reads `paseo ls --json`.
func (r PaseoCliRunner) ListAgents() ([]PaseoAgent, error) {
	out, err := r.run("ls", "--json")
	if err != nil {
		return nil, err
	}
	return parseAgentList(out)
}

// TagLabel writes one label onto an existing agent (AC-001 second half).
func (r PaseoCliRunner) TagLabel(agentID, key, value string) error {
	_, err := r.run("agent", "update", agentID, "--label", key+"="+value)
	return err
}

// PermitAllow answers every open permission request of one agent (AC-005).
func (r PaseoCliRunner) PermitAllow(agentID string) error {
	_, err := r.run("permit", "allow", agentID, "--all")
	return err
}

// SendProbe sends the single probe message (AC-006).
func (r PaseoCliRunner) SendProbe(agentID, text string) error {
	_, err := r.run("send", agentID, text)
	return err
}

// Archive archives one agent (AC-007). Never a delete.
func (r PaseoCliRunner) Archive(agentID string) error {
	_, err := r.run("archive", agentID)
	return err
}

// ---------------------------------------------------------------------------
// one supervision pass
// ---------------------------------------------------------------------------

// SuperviseRow is one work item's judgement for this pass.
type SuperviseRow struct {
	// Item is the work item id (the thread string).
	Item string `json:"item"`
	// AgentID is the paseo agent the item owns.
	AgentID string `json:"agent_id"`
	// Source is how ownership was matched: label or name.
	Source string `json:"source,omitempty"`
	// State is the frozen state token.
	State string `json:"state"`
	// Because is the fact that decided the state.
	Because string `json:"because"`
	// Action is what this pass does (or would do) about it.
	Action string `json:"action"`
	// Actions lists every action performed, in order. It differs from Action
	// only for the orphan -> failed transition, where the state move and the
	// archive are two separate acts.
	Actions []string `json:"actions,omitempty"`
	// Reason is the machine token recorded in the audit line.
	Reason string `json:"reason,omitempty"`
	// Tools are the tools an agent is waiting on (waiting_permission only).
	Tools []string `json:"tools,omitempty"`
	// Waiting / WaitingSeconds describe how long a human-facing state has held.
	Waiting        string `json:"waiting,omitempty"`
	WaitingSeconds int64  `json:"waiting_seconds,omitempty"`
}

// SuperviseResult is one pass, in the shape the CLI prints.
type SuperviseResult struct {
	Apply   bool           `json:"apply"`
	Owned   int            `json:"owned"`
	Foreign int            `json:"foreign"`
	Actions int            `json:"actions"`
	Rows    []SuperviseRow `json:"rows"`
}

// Supervisor is one supervision pass. Like the watchdog it owns no timer and no
// daemon: one call, one pass, exit.
type Supervisor struct {
	// Runner is the paseo surface; nil is a configuration error, never a no-op.
	Runner SupervisorRunner
	// Store carries the bindings (thread -> agent), read-only here.
	Store *Store
	// ReportsDir is adbd_report_dir: <beacon>/adbd-reports by default.
	ReportsDir string
	// StatePath is the memory document; <beacon>/adbd-supervise-state.json by
	// default.
	StatePath string
	// AuditPath is the audit ledger; <beacon>/adbd-supervise.jsonl by default.
	AuditPath string
	// GraceAfter is adbd_grace_after; DefaultGraceAfter when zero.
	GraceAfter time.Duration
	// Apply=false is a dry run: nothing is called and nothing is written.
	Apply bool
	// Now is injectable so tests drive an exact clock.
	Now func() time.Time
}

func (s *Supervisor) now() time.Time {
	if s == nil || s.Now == nil {
		return time.Now().UTC()
	}
	return s.Now().UTC()
}

func (s *Supervisor) graceAfter() time.Duration {
	if s == nil || s.GraceAfter <= 0 {
		return DefaultGraceAfter
	}
	return s.GraceAfter
}

// beaconDir is the directory the default paths hang off.
func (s *Supervisor) beaconDir() string {
	if s == nil || s.Store == nil {
		return ""
	}
	return filepath.Dir(s.Store.Path)
}

func (s *Supervisor) reportsDir() string {
	if dir := strings.TrimSpace(s.ReportsDir); dir != "" {
		return dir
	}
	return filepath.Join(s.beaconDir(), reportsDirName)
}

func (s *Supervisor) statePath() string {
	if path := strings.TrimSpace(s.StatePath); path != "" {
		return path
	}
	return filepath.Join(s.beaconDir(), superviseStateFile)
}

func (s *Supervisor) auditPath() string {
	if path := strings.TrimSpace(s.AuditPath); path != "" {
		return path
	}
	return filepath.Join(s.beaconDir(), superviseAuditFile)
}

// Pass classifies every owned work item and, when Apply is set, acts on the
// judgement. A pass that cannot read paseo at all returns a
// *PaseoUnavailableError and changes nothing: a silent "no news" would look
// exactly like "nothing to do".
func (s *Supervisor) Pass() (SuperviseResult, error) {
	res := SuperviseResult{Rows: []SuperviseRow{}}
	if s == nil || s.Runner == nil {
		return res, &PaseoUnavailableError{Err: errors.New("adbd supervise: no agent lister configured")}
	}
	agents, err := s.Runner.ListAgents()
	if err != nil {
		return res, &PaseoUnavailableError{Err: err}
	}
	bindings, err := loadBindingRows(s.Store)
	if err != nil {
		return res, err
	}
	owned, foreign := classifyOwnership(agents, bindingThreads(bindings))
	res.Owned, res.Foreign = len(owned), len(foreign)
	res.Apply = s.Apply

	doc, err := loadSuperviseDoc(s.statePath())
	if err != nil {
		return res, err
	}
	now := s.now()
	grace := s.graceAfter()
	audits := make([]superviseAudit, 0, len(owned))

	for _, target := range owned {
		agent := target.Agent
		item := target.Item
		entry := doc.Items[item]
		row := SuperviseRow{Item: item, AgentID: agent.ID, Source: target.Source, Action: SupActionNone}

		if entry.Archived && (entry.AgentID == "" || entry.AgentID == agent.ID) {
			// AC-007: an archived agent is never acted on again, and it is not
			// re-judged either: the work item is closed.
			row.State = SupStateArchived
			row.Because = "archived"
			res.Rows = append(res.Rows, row)
			continue
		}

		report, err := readReport(s.reportsDir(), item)
		if err != nil {
			return res, err
		}
		state, because := classifyItem(agent, report, now, grace)
		action, reason := planAction(state, agent, entry)
		if state == SupStateOrphan && action == SupActionFail {
			// AC-006: the one probe was already sent and the agent is still
			// orphaned, so the item is no longer "undecided" -- it is dead.
			state, because = SupStateFailed, "probe_unanswered"
		}
		row.State, row.Because, row.Action, row.Reason = state, because, action, reason
		if state == SupStateWaitingPermission {
			row.Tools = agent.toolNames()
		}
		row.Waiting, row.WaitingSeconds = waitingFact(state, agent, report, now)

		if s.Apply {
			if err := s.act(&row, &entry, agent, &audits, now); err != nil {
				return res, err
			}
			// Only an item this pass actually touched is remembered: a state
			// document full of empty rows is noise an operator has to read
			// through.
			if entry.Item != "" || entry.ProbedAt != "" || entry.Archived {
				doc.Items[item] = entry
			}
		}
		res.Rows = append(res.Rows, row)
	}

	// Audit first, memory second: a crash in between re-audits one action (loud)
	// rather than forgetting it (silent).
	if err := appendSuperviseAudit(s.auditPath(), audits); err != nil {
		return res, err
	}
	res.Actions = len(audits)
	if s.Apply {
		if err := saveSuperviseDoc(s.statePath(), doc); err != nil {
			return res, err
		}
	}
	return res, nil
}

// planAction is the frozen 观察 -> 动作 mapping (AC-005..AC-007).
func planAction(state string, agent PaseoAgent, entry superviseItemState) (action, reason string) {
	switch state {
	case SupStateWaitingPermission:
		if permissionSafe(agent.toolNames()) {
			return SupActionAllow, "permission_safe"
		}
		// A high-risk tool is never allowed automatically: a human decides.
		return SupActionEscalate, "permission_high_risk"
	case SupStateOrphan:
		if strings.TrimSpace(entry.ProbedAt) == "" {
			return SupActionProbe, "orphan_unprobed"
		}
		// AC-006: the probe was already sent and the agent is still orphaned.
		// No second probe -- the honest answer is that it is dead.
		return SupActionFail, "probe_unanswered"
	case SupStateDone:
		return SupActionArchive, "report_ready"
	case SupStateFailed:
		return SupActionArchive, "status_error"
	}
	return SupActionNone, ""
}

// act performs one row's action and records it. Every branch that calls paseo
// also writes exactly one audit line per action taken.
func (s *Supervisor) act(row *SuperviseRow, entry *superviseItemState, agent PaseoAgent, audits *[]superviseAudit, now time.Time) error {
	record := func(action, reason string) {
		*audits = append(*audits, superviseAudit{
			Item:    row.Item,
			AgentID: agent.ID,
			Action:  action,
			Reason:  reason,
			At:      now.Format(time.RFC3339),
		})
		row.Actions = append(row.Actions, action)
	}
	switch row.Action {
	case SupActionAllow:
		if err := s.Runner.PermitAllow(agent.ID); err != nil {
			return err
		}
		record(SupActionAllow, row.Reason)
	case SupActionEscalate:
		// AC-005: escalate means "do not touch it, tell a human". The audit line
		// is the telling.
		record(SupActionEscalate, row.Reason)
	case SupActionProbe:
		if err := s.Runner.SendProbe(agent.ID, probeMessage); err != nil {
			return err
		}
		entry.Item, entry.AgentID = row.Item, agent.ID
		entry.ProbedAt = now.Format(time.RFC3339)
		entry.UpdatedAt = entry.ProbedAt
		record(SupActionProbe, row.Reason)
	case SupActionFail:
		// AC-006 -> AC-007: the state moves to failed, and a failed item is
		// reaped like any other terminal one.
		record(SupActionFail, row.Reason)
		if err := s.Runner.Archive(agent.ID); err != nil {
			return err
		}
		record(SupActionArchive, "failed")
		entry.Item, entry.AgentID = row.Item, agent.ID
		entry.Archived = true
		entry.ArchivedAt = now.Format(time.RFC3339)
		entry.UpdatedAt = entry.ArchivedAt
	case SupActionArchive:
		if err := s.Runner.Archive(agent.ID); err != nil {
			return err
		}
		record(SupActionArchive, row.Reason)
		entry.Item, entry.AgentID = row.Item, agent.ID
		entry.Archived = true
		entry.ArchivedAt = now.Format(time.RFC3339)
		entry.UpdatedAt = entry.ArchivedAt
	}
	return nil
}

// CountActiveOwned answers AC-008's question: how many owned work items are
// working right now. It is the router's ceiling check, so it never guesses --
// an unreadable agent list returns an error and the caller leaves the ceiling
// unapplied rather than blocking work on a broken probe.
func CountActiveOwned(runner AgentLister, bindings []Binding) (int, error) {
	if runner == nil {
		return 0, errors.New("adbd supervise: no agent lister configured")
	}
	agents, err := runner.ListAgents()
	if err != nil {
		return 0, err
	}
	owned, _ := classifyOwnership(agents, bindingThreads(bindings))
	count := 0
	for _, target := range owned {
		if superviseStatus(target.Agent.Status) == StatusRunning {
			count++
		}
	}
	return count, nil
}

// superviseClock is the CLI clock seam, mirroring reconcileClock: tests inject a
// fake so grace windows and waiting durations are exact instants, not wall-clock
// noise.
var superviseClock = func() time.Time { return time.Now() }

// ---------------------------------------------------------------------------
// CLIs
// ---------------------------------------------------------------------------

func runSupervise(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("supervise", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var apply bool
	var graceRaw, reportsDir, auditPath string
	fs.BoolVar(&apply, "apply", false, "act on the judgement instead of only printing it")
	fs.StringVar(&graceRaw, "grace", "", "idle grace before an unreported agent becomes orphan, e.g. 5m")
	fs.StringVar(&reportsDir, "reports", "", "work item report directory")
	fs.StringVar(&auditPath, "audit", "", "supervision audit ledger")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd supervise: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd supervise: %v\n", err)
		return 1
	}
	grace := cfg.GraceAfter
	if strings.TrimSpace(graceRaw) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(graceRaw))
		if err != nil || parsed <= 0 {
			fmt.Fprintf(stderr, "adbd supervise: --grace %q is not a positive duration (e.g. 5m)\n", graceRaw)
			return 2
		}
		grace = parsed
	}
	bin := cf.runnerBin
	if bin == "" {
		bin = envOr("ADBD_PASEO_BIN", cfg.PaseoBin)
	}
	if bin == "" {
		bin = "paseo"
	}
	if strings.TrimSpace(reportsDir) == "" {
		// adbd_report_dir names the same directory for every reader of the
		// reports, so an operator sets it once.
		reportsDir = strings.TrimSpace(cfg.ReportDir)
	}
	supervisor := &Supervisor{
		Runner:     PaseoCliRunner{Bin: bin},
		Store:      &Store{Path: cf.statePath, SlowPath: cf.slowPath},
		ReportsDir: reportsDir,
		AuditPath:  auditPath,
		GraceAfter: grace,
		Apply:      apply,
		Now:        superviseClock,
	}
	res, err := supervisor.Pass()
	if cf.outcome == "json" {
		if code := writeJSON(stdout, stderr, res); code != 0 {
			return code
		}
	} else {
		fmt.Fprintln(stdout, formatSuperviseHeader(res))
		for _, row := range res.Rows {
			fmt.Fprintln(stdout, formatSuperviseRow(row, res.Apply))
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "adbd supervise: %v\n", err)
		var unavailable *PaseoUnavailableError
		if errors.As(err, &unavailable) {
			return 2
		}
		return 1
	}
	return 0
}

// formatSuperviseHeader is the one-line summary of a pass.
func formatSuperviseHeader(res SuperviseResult) string {
	mode := "dry_run"
	if res.Apply {
		mode = "apply"
	}
	return fmt.Sprintf("mode=%s owned=%d foreign=%d actions=%d", mode, res.Owned, res.Foreign, res.Actions)
}

// formatSuperviseRow renders one judgement. The `item= state= because=` triple is
// the frozen dry-run shape (AC-004) and it is a prefix in every mode; the action
// column says what this pass does about the row, and `performed=` appears only
// when something was actually executed, so "planned" and "done" never blur.
func formatSuperviseRow(row SuperviseRow, apply bool) string {
	line := fmt.Sprintf("item=%s state=%s because=%s action=%s",
		row.Item, row.State, row.Because, row.Action)
	if row.Reason != "" {
		line += " reason=" + row.Reason
	}
	if len(row.Tools) > 0 {
		line += " tools=" + strings.Join(row.Tools, ",")
	}
	if apply && len(row.Actions) > 0 {
		line += " performed=" + strings.Join(row.Actions, ",")
	}
	return line
}
