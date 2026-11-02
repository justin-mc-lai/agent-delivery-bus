// The attention funnel (truth.md, feature agent-supervisor, v0.1.5).
//
// Supervision judges a hundred agents; a human can only answer a few. `adbd
// inbox` is the difference between the two: it recomputes the same judgement
// read-only and prints only the two states that are actually blocked on a person
// -- `waiting_permission` (an agent is stopped on a permission question) and
// `waiting_input` (a work item reported that it needs an answer). Everything else
// is either progress (working), over (done/failed/archived), or not yet a
// decision (grace/orphan/unknown), and none of it belongs in front of a human.
//
// It writes nothing, calls nothing but `paseo ls --json`, and sorts by how long
// the wait has lasted, longest first -- the oldest question is the one costing
// the most. When there is nothing to ask, it prints exactly NOTHING and exits 0
// (AC-009): one line a scheduler can grep, and no invented placeholder.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// InboxNothing is what an empty inbox prints (AC-009). It is a whole line, not a
// phrase, so `adbd inbox | grep -qx NOTHING` is a valid check.
const InboxNothing = "NOTHING"

// errNoSupervisor is the honest answer when the caller wired no paseo seam at
// all: an inbox that cannot classify is not an empty inbox.
var errNoSupervisor = errors.New("adbd inbox: no supervisor configured")

// InboxEntry is one work item that needs a human.
type InboxEntry struct {
	// Item is the work item id (the thread string).
	Item string `json:"item"`
	// AgentID is the agent holding the question.
	AgentID string `json:"agent_id"`
	// State is waiting_permission or waiting_input.
	State string `json:"state"`
	// Because is the fact that decided it.
	Because string `json:"because"`
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
			State:          row.State,
			Because:        row.Because,
			Waiting:        row.Waiting,
			WaitingSeconds: row.WaitingSeconds,
			Tools:          row.Tools,
		})
	}
	// Longest wait first; the item id breaks ties so the order is stable.
	sort.Slice(res.Entries, func(i, j int) bool {
		if res.Entries[i].WaitingSeconds != res.Entries[j].WaitingSeconds {
			return res.Entries[i].WaitingSeconds > res.Entries[j].WaitingSeconds
		}
		return res.Entries[i].Item < res.Entries[j].Item
	})
	res.Count = len(res.Entries)
	return res, nil
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
	supervisor := &Supervisor{
		Runner:     PaseoCliRunner{Bin: bin},
		Store:      &Store{Path: cf.statePath, SlowPath: cf.slowPath},
		ReportsDir: strings.TrimSpace(cfg.ReportDir),
		GraceAfter: grace,
		Apply:      false,
		Now:        superviseClock,
	}
	res, err := Inbox(supervisor)
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

// formatInboxEntry renders one waiting item: what it is, how long it has waited,
// and which agent is holding the question.
func formatInboxEntry(entry InboxEntry) string {
	line := fmt.Sprintf("item=%s state=%s waiting=%s agent=%s because=%s",
		entry.Item, entry.State, entry.Waiting, entry.AgentID, entry.Because)
	if len(entry.Tools) > 0 {
		line += " tools=" + strings.Join(entry.Tools, ",")
	}
	return line
}
