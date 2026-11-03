// The attention funnel (truth.md, feature agent-supervisor, v0.1.5).
//
// Supervision judges a hundred agents; a human can only answer a few. `adbd
// inbox` is the difference between the two: it recomputes the same judgement
// read-only and prints only the states that are actually blocked on a person.
// There are three of them (AC-009):
//
//   - `waiting_permission` -- an agent is stopped on a permission question;
//   - `waiting_input` -- a work item reported that it needs an answer;
//   - `escalated` -- the router gave up on a message and asked for a human,
//     read from `.beacon/adbd-decisions.jsonl`.
//
// Everything else is either progress (working), over (done/failed/archived), or
// not yet a decision (grace/orphan/unknown), and none of it belongs in front of
// a human.
//
// It writes nothing, calls nothing but `paseo ls --json`, and sorts by how long
// the wait has lasted, longest first -- the oldest question is the one costing
// the most. When there is nothing to ask, it prints exactly NOTHING and exits 0
// (AC-009): one line a scheduler can grep, and no invented placeholder.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InboxNothing is what an empty inbox prints (AC-009). It is a whole line, not a
// phrase, so `adbd inbox | grep -qx NOTHING` is a valid check.
const InboxNothing = "NOTHING"

// InboxClassEscalated is the third class AC-009 adds: a router verdict of
// decision=escalate that is still inside the escalation window. The other two
// class names are the supervision states themselves (SupStateWaitingPermission,
// SupStateWaitingInput).
const InboxClassEscalated = "escalated"

// DefaultInboxEscalationWindow is adbd_inbox_escalation_window: an escalate
// decision is a live question for this long, and then it stops being one. One
// day is long enough to cover a night and short enough that a stale escalation
// cannot outlive the work it was about.
const DefaultInboxEscalationWindow = 24 * time.Hour

// errNoSupervisor is the honest answer when the caller wired no paseo seam at
// all: an inbox that cannot classify is not an empty inbox.
var errNoSupervisor = errors.New("adbd inbox: no supervisor configured")

// InboxEntry is one work item that needs a human.
type InboxEntry struct {
	// Item is the work item id (the thread string).
	Item string `json:"item"`
	// AgentID is the agent holding the question. Empty for an escalated row:
	// the router's trace names the thread and the reason, not an agent.
	AgentID string `json:"agent_id"`
	// Class is the inbox class: waiting_permission, waiting_input or escalated
	// (AC-009). It is always written so a consumer can tell the three apart
	// without knowing which fields happen to be set.
	Class string `json:"class"`
	// State is the class spelled the way supervision spells it: the two
	// supervision states, or escalated for a router escalation.
	State string `json:"state"`
	// Because is the fact that decided it (supervision rows only).
	Because string `json:"because"`
	// ReasonCode is the router's own reason for an escalated row (AC-009).
	ReasonCode string `json:"reason_code,omitempty"`
	// Waiting / WaitingSeconds is how long the human has been the blocker.
	Waiting        string `json:"waiting"`
	WaitingSeconds int64  `json:"waiting_seconds"`
	// Tools are the permission requests being asked about, when they are known.
	Tools []string `json:"tools,omitempty"`
}

// InboxResult is one inbox listing.
type InboxResult struct {
	Count   int          `json:"count"`
	Entries []InboxEntry `json:"entries"`
}

// Inbox recomputes the supervision judgement read-only and keeps only the rows
// that need a human, longest wait first.
func Inbox(supervisor *Supervisor) (InboxResult, error) {
	res := InboxResult{Entries: []InboxEntry{}}
	if supervisor == nil {
		return res, &PaseoUnavailableError{Err: errNoSupervisor}
	}
	// A dry run by construction: no action, no audit line, no state write.
	dry := *supervisor
	dry.Apply = false
	pass, err := dry.Pass()
	if err != nil {
		return res, err
	}
	for _, row := range pass.Rows {
		if row.State != SupStateWaitingPermission && row.State != SupStateWaitingInput {
			continue
		}
		res.Entries = append(res.Entries, InboxEntry{
			Item:           row.Item,
			AgentID:        row.AgentID,
			Class:          row.State,
			State:          row.State,
			Because:        row.Because,
			Waiting:        row.Waiting,
			WaitingSeconds: row.WaitingSeconds,
			Tools:          row.Tools,
		})
	}
	sortInboxEntries(res.Entries)
	res.Count = len(res.Entries)
	return res, nil
}

// sortInboxEntries orders one inbox listing: longest wait first, with the item
// id as the tie-break so the order is stable. All three classes share this one
// ordering, which is what "merged by waiting time" means in AC-009.
func sortInboxEntries(entries []InboxEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].WaitingSeconds != entries[j].WaitingSeconds {
			return entries[i].WaitingSeconds > entries[j].WaitingSeconds
		}
		return entries[i].Item < entries[j].Item
	})
}

// InboxEscalations reads .beacon/adbd-decisions.jsonl and returns one entry per
// decision=escalate row whose decided_at falls inside the escalation window
// (AC-009). Rows older than the window are dropped -- an escalation nobody acted
// on for a day is history, not a question -- and a future timestamp is dropped
// too, because a row that has not happened yet has no honest waiting duration.
//
// A missing file is not an error: before the router has written its first
// verdict there is simply no escalated class.
func InboxEscalations(path string, window time.Duration, now time.Time) ([]InboxEntry, error) {
	entries := []InboxEntry{}
	if window <= 0 {
		window = DefaultInboxEscalationWindow
	}
	now = now.UTC()
	cutoff := now.Add(-window)
	err := forEachJSONLine(path, func(line []byte, lineNo int) error {
		var rec traceRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("adbd inbox: %s line %d is not a decision trace: %w", path, lineNo, err)
		}
		if strings.TrimSpace(rec.Decision) != DecisionEscalate {
			return nil
		}
		item := strings.TrimSpace(rec.Thread)
		if item == "" {
			return nil // no thread means no work item to put in front of a human
		}
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(rec.DecidedAt))
		if err != nil {
			return nil // an unreadable clock cannot be placed in a window
		}
		at = at.UTC()
		if at.Before(cutoff) || at.After(now) {
			return nil
		}
		waited := now.Sub(at)
		entries = append(entries, InboxEntry{
			Item:           item,
			Class:          InboxClassEscalated,
			State:          InboxClassEscalated,
			ReasonCode:     strings.TrimSpace(rec.ReasonCode),
			Waiting:        formatDuration(waited),
			WaitingSeconds: int64(waited / time.Second),
		})
		return nil
	})
	return entries, err
}

// ---------------------------------------------------------------------------
// CLI: adbd inbox [--root] [--outcome json|text]
// ---------------------------------------------------------------------------

func runInbox(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inbox", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var graceRaw string
	fs.StringVar(&graceRaw, "grace", "", "idle grace used for the read-only classification, e.g. 5m")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd inbox: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd inbox: %v\n", err)
		return 1
	}
	grace := cfg.GraceAfter
	if strings.TrimSpace(graceRaw) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(graceRaw))
		if err != nil || parsed <= 0 {
			fmt.Fprintf(stderr, "adbd inbox: --grace %q is not a positive duration (e.g. 5m)\n", graceRaw)
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
	now := superviseClock()
	supervisor := &Supervisor{
		Runner:     PaseoCliRunner{Bin: bin},
		Store:      &Store{Path: cf.statePath, SlowPath: cf.slowPath},
		ReportsDir: strings.TrimSpace(cfg.ReportDir),
		GraceAfter: grace,
		Apply:      false,
		Now:        func() time.Time { return now },
	}
	res, err := Inbox(supervisor)
	if err == nil {
		// AC-009: the third class comes from the router's own audit trail, which
		// lives next to the state document. A missing file is an empty class.
		decisionsPath := envOr("ADBD_TRACE", filepath.Join(cf.root, ".beacon", decisionsFile))
		escalated, escErr := InboxEscalations(decisionsPath, cfg.InboxEscalationWindow, now)
		if escErr != nil {
			err = escErr
		}
		if len(escalated) > 0 {
			res.Entries = append(res.Entries, escalated...)
			sortInboxEntries(res.Entries)
			res.Count = len(res.Entries)
		}
	}
	if cf.outcome == "json" {
		if code := writeJSON(stdout, stderr, res); code != 0 {
			return code
		}
	} else if res.Count == 0 {
		// AC-009: an empty inbox is exactly this word on exactly one line.
		fmt.Fprintln(stdout, InboxNothing)
	} else {
		for _, entry := range res.Entries {
			fmt.Fprintln(stdout, formatInboxEntry(entry))
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "adbd inbox: %v\n", err)
		var unavailable *PaseoUnavailableError
		if errors.As(err, &unavailable) {
			return 2
		}
		return 1
	}
	return 0
}

// formatInboxEntry renders one waiting item: which class it belongs to, what it
// is, how long it has waited, and what the human needs to know about it -- the
// router's reason_code for an escalation, the deciding fact and the holding
// agent for the two supervision classes.
func formatInboxEntry(entry InboxEntry) string {
	class := entry.Class
	if class == "" {
		class = entry.State
	}
	line := fmt.Sprintf("class=%s item=%s state=%s waiting=%s",
		class, entry.Item, entry.State, entry.Waiting)
	if class == InboxClassEscalated {
		if entry.ReasonCode != "" {
			line += " reason_code=" + entry.ReasonCode
		}
		if entry.AgentID != "" {
			line += " agent=" + entry.AgentID
		}
		return line
	}
	line += " agent=" + entry.AgentID + " because=" + entry.Because
	if len(entry.Tools) > 0 {
		line += " tools=" + strings.Join(entry.Tools, ",")
	}
	return line
}
