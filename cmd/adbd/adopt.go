// Claiming the existing agent population (truth.md, feature agent-supervisor,
// v0.1.5).
//
// The lake starts from a hundred-odd agents nobody labelled. Before supervision
// can tell "done" from "abandoned" it first has to answer a cheaper question per
// agent: is this one mine at all? `adbd adopt` answers it once, in one pass, and
// writes the answer where paseo itself keeps it -- on the agent, as a label.
//
// Two outcomes and no third:
//
//	owned    the agent carries `adbd.item=<thread>`, or its name is a bound
//	         thread. It gets (or already has) the ownership label.
//	foreign  everything else. It gets `adbd.foreign=1` so the next pass knows
//	         the decision was made, and supervision never touches it again.
//
// A dry run prints exactly that classification and writes nothing (AC-002). An
// applying run writes only the missing labels, so a second run issues no update
// call at all (AC-003): the label is the state, and there is no second ledger to
// drift from it.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// Ownership values for one adopt row.
const (
	// AdoptOwnershipOwned marks an agent that matches a bound thread.
	AdoptOwnershipOwned = "owned"
	// AdoptOwnershipForeign marks an agent that matches nothing.
	AdoptOwnershipForeign = "foreign"

	// AdoptActionTag means this pass writes a label.
	AdoptActionTag = "tag"
	// AdoptActionSkip means the label is already right; no call is made.
	AdoptActionSkip = "skip"
)

// AdoptRow is one agent's classification and, when applying, what was written.
type AdoptRow struct {
	// AgentID is the paseo agent id.
	AgentID string `json:"agent_id"`
	// Name is the agent name, which is the second way to match a thread.
	Name string `json:"name,omitempty"`
	// Ownership is owned or foreign.
	Ownership string `json:"ownership"`
	// Item is the work item id; empty for a foreign agent.
	Item string `json:"item,omitempty"`
	// Label is the label this row is about, as key=value.
	Label string `json:"label"`
	// Action is tag (the label is missing) or skip (it is already there).
	Action string `json:"action"`
	// Reason is a stable machine token: tagged, already_tagged, foreign_tagged,
	// already_foreign, or dry_run when the row was only classified.
	Reason string `json:"reason,omitempty"`
}

// AdoptResult is one adopt pass, in the shape the CLI prints.
type AdoptResult struct {
	Apply   bool       `json:"apply"`
	Owned   int        `json:"owned"`
	Foreign int        `json:"foreign"`
	Tagged  int        `json:"tagged"`
	Skipped int        `json:"skipped"`
	Rows    []AdoptRow `json:"rows"`
}

// Adopt classifies every agent paseo knows and, when apply is set, writes the
// one missing label per agent.
//
// Nothing here archives, interrupts or deletes anything, and a dry run performs
// no call other than the read of `paseo ls --json` (AC-002).
func Adopt(runner TaggingRunner, store *Store, apply bool) (AdoptResult, error) {
	res := AdoptResult{Apply: apply, Rows: []AdoptRow{}}
	if runner == nil {
		return res, &PaseoUnavailableError{Err: errors.New("adbd adopt: no agent lister configured")}
	}
	agents, err := runner.ListAgents()
	if err != nil {
		return res, &PaseoUnavailableError{Err: err}
	}
	bindings, err := loadBindingRows(store)
	if err != nil {
		return res, err
	}
	owned, foreign := classifyOwnership(agents, bindingThreads(bindings))
	res.Owned, res.Foreign = len(owned), len(foreign)

	// AC-003: `paseo ls --json` carries no labels, so ask the filter once per
	// label value. Without this a second --apply would re-report every tag even
	// though paseo stored it already.
	lookup, _ := runner.(LabelLookup)
	foreignTagged := map[string]bool{}
	ownedTagged := map[string]map[string]bool{}
	if lookup != nil {
		if ids, err := lookup.LabeledIDs(ForeignLabel, ForeignLabelValue); err == nil {
			foreignTagged = ids
		}
		for _, target := range owned {
			if _, done := ownedTagged[target.Item]; done {
				continue
			}
			ids, err := lookup.LabeledIDs(OwnershipLabel, target.Item)
			if err != nil {
				continue
			}
			ownedTagged[target.Item] = ids
		}
	}

	for _, target := range owned {
		row := AdoptRow{
			AgentID:   target.Agent.ID,
			Name:      target.Agent.Name,
			Ownership: AdoptOwnershipOwned,
			Item:      target.Item,
			Label:     OwnershipLabel + "=" + target.Item,
		}
		if target.Agent.Label(OwnershipLabel) == target.Item || ownedTagged[target.Item][target.Agent.ID] {
			// AC-003: the label is already right, so the second run makes no
			// call and writes no duplicate.
			row.Action, row.Reason = AdoptActionSkip, "already_tagged"
			res.Skipped++
		} else {
			row.Action, row.Reason = AdoptActionTag, "tagged"
			res.Tagged++
			if apply {
				if err := runner.TagLabel(target.Agent.ID, OwnershipLabel, target.Item); err != nil {
					return res, err
				}
			}
		}
		res.Rows = append(res.Rows, row)
	}

	for _, agent := range foreign {
		row := AdoptRow{
			AgentID:   agent.ID,
			Name:      agent.Name,
			Ownership: AdoptOwnershipForeign,
			Label:     ForeignLabel + "=" + ForeignLabelValue,
		}
		if agent.Label(ForeignLabel) == ForeignLabelValue || foreignTagged[agent.ID] {
			row.Action, row.Reason = AdoptActionSkip, "already_foreign"
			res.Skipped++
		} else {
			row.Action, row.Reason = AdoptActionTag, "foreign_tagged"
			res.Tagged++
			if apply {
				if err := runner.TagLabel(agent.ID, ForeignLabel, ForeignLabelValue); err != nil {
					return res, err
				}
			}
		}
		res.Rows = append(res.Rows, row)
	}

	if !apply {
		// A dry run reports what it would write without claiming to have written
		// it; "tagged" counts stay at zero so nobody reads the summary as a fact
		// that already happened.
		res.Tagged = 0
		res.Skipped = 0
		for i := range res.Rows {
			if res.Rows[i].Reason == "" {
				continue
			}
			res.Rows[i].Reason = "dry_run:" + res.Rows[i].Reason
		}
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// CLI: adbd adopt [--apply] [--root] [--outcome json|text]
// ---------------------------------------------------------------------------

func runAdopt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("adopt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var apply bool
	fs.BoolVar(&apply, "apply", false, "write the labels instead of only classifying")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd adopt: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd adopt: %v\n", err)
		return 1
	}
	bin := cf.runnerBin
	if bin == "" {
		bin = envOr("ADBD_PASEO_BIN", cfg.PaseoBin)
	}
	if bin == "" {
		bin = "paseo"
	}
	res, err := Adopt(PaseoCliRunner{Bin: bin}, &Store{Path: cf.statePath, SlowPath: cf.slowPath}, apply)
	if cf.outcome == "json" {
		if code := writeJSON(stdout, stderr, res); code != 0 {
			return code
		}
	} else {
		fmt.Fprintln(stdout, formatAdoptHeader(res))
		for _, row := range res.Rows {
			fmt.Fprintln(stdout, formatAdoptRow(row))
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "adbd adopt: %v\n", err)
		var unavailable *PaseoUnavailableError
		if errors.As(err, &unavailable) {
			return 2
		}
		return 1
	}
	return 0
}

// formatAdoptHeader is the first line of an adopt pass: the two counts AC-002
// names, plus what the pass did about them.
func formatAdoptHeader(res AdoptResult) string {
	mode := "dry_run"
	if res.Apply {
		mode = "apply"
	}
	return fmt.Sprintf("mode=%s owned=%d foreign=%d tagged=%d skipped=%d",
		mode, res.Owned, res.Foreign, res.Tagged, res.Skipped)
}

// formatAdoptRow renders one agent. The ownership and item columns are identical
// on every run (AC-003); only the last column says whether this run had work to
// do.
func formatAdoptRow(row AdoptRow) string {
	line := fmt.Sprintf("agent=%s ownership=%s label=%s action=%s",
		row.AgentID, row.Ownership, row.Label, row.Action)
	if row.Item != "" {
		line += " item=" + row.Item
	}
	if row.Name != "" && row.Name != row.Item {
		line = strings.TrimSpace(line + " name=" + row.Name)
	}
	if row.Reason != "" {
		line += " reason=" + row.Reason
	}
	return line
}
