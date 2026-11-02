package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// This file pins the worker-crud router contract from the outside: it starts the
// CLI the way an operator does and reads what comes back. Everything else -- the
// argv handed to paseo, the label lookups, the audit ledger -- is asserted
// against the real process the CLI spawned, so the tests cannot pass by agreeing
// with an assumption inside the code under test.

// TestWorkerListFilters is TC-WC-001 (AC-001): one screen of workers with the
// project each cwd maps to, and three filters that compose.
func TestWorkerListFilters(t *testing.T) {
	h := newWorkerCLIHarness(t)
	h.writeWorkspaces(map[string]any{"workspaces": []any{
		map[string]any{"slug": "alpha", "path": h.projectDir("alpha")},
		map[string]any{"slug": "beta", "path": h.projectDir("beta"), "aliases": []any{"b"}},
	}})
	h.addAgent("ag_alpha", "item-alpha", "running", h.projectDir("alpha")+"/src/module", "5 minutes ago", "", "bash")
	h.addAgent("ag_beta", "item-beta", "idle", h.projectDir("beta"), "2 hours ago", "", "")
	h.addAgent("ag_stray", "stray", "idle", h.outsideDir(), "just now", "", "")
	h.addAgent("ag_foreign", "foreign-one", "idle", h.projectDir("beta")+"/nested", "3 days ago", "", "")
	h.addLabel("ag_alpha", OwnershipLabel, "item-alpha")
	h.addLabel("ag_beta", OwnershipLabel, "item-beta")
	h.addLabel("ag_foreign", ForeignLabel, ForeignLabelValue)

	code, out, errOut := h.run("list", "--json")
	if code != 0 {
		t.Fatalf("worker list exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
	}
	workers := decodeWorkerList(t, out)
	if workers.Count != 4 || len(workers.Workers) != 4 {
		t.Fatalf("worker list = %+v, want four workers", workers)
	}
	want := map[string]struct{ project, ownership, state, age string }{
		"ag_alpha":   {"alpha", WorkerOwned, "running", "5m"},
		"ag_beta":    {"beta", WorkerOwned, "idle", "2h"},
		"ag_stray":   {WorkerProjectUnmatched, WorkerUnknown, "idle", "0s"},
		"ag_foreign": {"beta", WorkerForeign, "idle", "3d"},
	}
	for _, row := range workers.Workers {
		expected, ok := want[row.ID]
		if !ok {
			t.Fatalf("unexpected worker %q in %+v", row.ID, workers.Workers)
		}
		if row.Project != expected.project {
			t.Fatalf("%s project = %q, want %q (a subdirectory belongs to its longest prefix)",
				row.ID, row.Project, expected.project)
		}
		if row.Ownership != expected.ownership {
			t.Fatalf("%s ownership = %q, want %q", row.ID, row.Ownership, expected.ownership)
		}
		if row.State != expected.state {
			t.Fatalf("%s state = %q, want %q", row.ID, row.State, expected.state)
		}
		if row.Age != expected.age {
			t.Fatalf("%s age = %q, want %q", row.ID, row.Age, expected.age)
		}
	}

	// The text surface prints one line per worker and "-" when nothing matches.
	code, out, _ = h.run("list")
	if code != 0 {
		t.Fatalf("worker list (text) exit = %d, want 0", code)
	}
	if !strings.Contains(out, "id=ag_stray name=stray project=- ownership=unknown state=idle age=0s") {
		t.Fatalf("text list = %q, want the unmatched worker to print project=-", out)
	}

	// ① --project keeps one project only.
	workers = h.list(t, "--project", "alpha")
	if workers.Count != 1 || workers.Workers[0].ID != "ag_alpha" {
		t.Fatalf("--project alpha = %+v, want only ag_alpha", workers.Workers)
	}
	// The alias resolves to the same project.
	workers = h.list(t, "--project", "b")
	if workers.Count != 2 {
		t.Fatalf("--project b (alias of beta) = %+v, want the two beta workers", workers.Workers)
	}
	// ② --state keeps one state only.
	workers = h.list(t, "--state", "running")
	if workers.Count != 1 || workers.Workers[0].ID != "ag_alpha" {
		t.Fatalf("--state running = %+v, want only ag_alpha", workers.Workers)
	}
	// ③ --owned keeps the agents that carry adbd.item.
	workers = h.list(t, "--owned")
	if workers.Count != 2 {
		t.Fatalf("--owned = %+v, want ag_alpha and ag_beta", workers.Workers)
	}
	for _, row := range workers.Workers {
		if row.Ownership != WorkerOwned {
			t.Fatalf("--owned returned %q with ownership=%q", row.ID, row.Ownership)
		}
	}
	// ④ The three filters compose: one project, one state, owned only.
	workers = h.list(t, "--project", "beta", "--state", "idle", "--owned")
	if workers.Count != 1 || workers.Workers[0].ID != "ag_beta" {
		t.Fatalf("composed filters = %+v, want only ag_beta (ag_foreign is foreign)", workers.Workers)
	}

	// --outcome json is the same payload as --json: one machine surface, two
	// spellings, and neither may drift from the other.
	code, out, _ = h.run("list", "--outcome", "json")
	if code != 0 {
		t.Fatalf("worker list --outcome json exit = %d, want 0", code)
	}
	if got := decodeWorkerList(t, out); got.Count != 4 {
		t.Fatalf("--outcome json returned %d workers, want 4", got.Count)
	}
}

// TestWorkerShowDetail is TC-WC-002 (AC-002): the nine facts, and agent_not_found
// with exit 2 for an id paseo does not know.
func TestWorkerShowDetail(t *testing.T) {
	h := newWorkerCLIHarness(t)
	h.writeWorkspaces(map[string]any{"workspaces": []any{
		map[string]any{"slug": "alpha", "path": h.projectDir("alpha")},
	}})
	h.addAgent("ag_alpha", "item-alpha", "running", h.projectDir("alpha"), "2026-01-01T00:00:00Z", "2026-01-01T04:00:00Z", "bash")
	h.addLabel("ag_alpha", OwnershipLabel, "item-alpha")

	code, out, errOut := h.run("show", "ag_alpha", "--json")
	if code != 0 {
		t.Fatalf("worker show exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
	}
	show := decodeWorkerShow(t, out)
	if show.ID != "ag_alpha" || show.Name != "item-alpha" {
		t.Fatalf("show = %+v, want ag_alpha/item-alpha", show)
	}
	if show.Cwd != h.projectDir("alpha") {
		t.Fatalf("show cwd = %q, want the worker directory", show.Cwd)
	}
	if show.Project != "alpha" {
		t.Fatalf("show project = %q, want alpha", show.Project)
	}
	if show.Status != "running" {
		t.Fatalf("show status = %q, want running", show.Status)
	}
	if len(show.PendingPermissions) != 1 || show.PendingPermissions[0] != "bash" {
		t.Fatalf("show pending_permissions = %v, want [bash]", show.PendingPermissions)
	}
	if show.Labels[OwnershipLabel] != "item-alpha" {
		t.Fatalf("show labels = %v, want %s=item-alpha", show.Labels, OwnershipLabel)
	}
	if show.Created != "2026-01-01T00:00:00Z" || show.Updated != "2026-01-01T04:00:00Z" {
		t.Fatalf("show created/updated = %q/%q, want the paseo stamps", show.Created, show.Updated)
	}

	// The text surface names every field, so a human never has to guess a column.
	code, out, _ = h.run("show", "ag_alpha")
	if code != 0 {
		t.Fatalf("worker show (text) exit = %d, want 0", code)
	}
	for _, field := range []string{
		"id=ag_alpha", "name=item-alpha", "cwd=" + h.projectDir("alpha"), "project=alpha",
		"status=running", "pending_permissions=bash", "labels=" + OwnershipLabel + "=item-alpha",
		"created=2026-01-01T00:00:00Z", "updated=2026-01-01T04:00:00Z",
	} {
		if !strings.Contains(out, field) {
			t.Fatalf("show output %q is missing %q", out, field)
		}
	}

	// An id paseo cannot resolve is a typed refusal, not an empty screen.
	code, out, _ = h.run("show", "ag_missing")
	if code != 2 {
		t.Fatalf("worker show ag_missing exit = %d, want 2", code)
	}
	if !strings.Contains(out, "reason_code="+ReasonAgentNotFound) {
		t.Fatalf("show ag_missing stdout = %q, want reason_code=%s", out, ReasonAgentNotFound)
	}
}

// TestWorkerCreateInProject is TC-WC-003 (AC-003): the worker is started in the
// registered project directory with its ownership label, and an unregistered
// project is refused before paseo is asked to do anything.
func TestWorkerCreateInProject(t *testing.T) {
	t.Run("creates_in_project_directory", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.writeWorkspaces(map[string]any{"workspaces": []any{
			map[string]any{"slug": "alpha", "path": h.projectDir("alpha")},
		}})

		code, out, errOut := h.run("create", "--project", "alpha", "--item", "item-gamma")
		if code != 0 {
			t.Fatalf("worker create exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
		}
		if !strings.Contains(out, "agent_id=ag_new") {
			t.Fatalf("create stdout = %q, want the parsed agent_id", out)
		}
		runs := h.logLines("run")
		if len(runs) != 1 {
			t.Fatalf("paseo run calls = %v, want exactly one", runs)
		}
		for _, want := range []string{
			"run --no-wait --json",
			"--new-workspace local",
			"--cwd " + h.projectDir("alpha"),
			"--title item-gamma",
			"--label " + OwnershipLabel + "=item-gamma",
		} {
			if !strings.Contains(runs[0], want) {
				t.Fatalf("run argv = %q, want it to contain %q", runs[0], want)
			}
		}
		// Provider/model/mode are paseo's defaults unless the operator names one.
		for _, unwanted := range []string{"--provider", "--model", "--mode"} {
			if strings.Contains(runs[0], unwanted) {
				t.Fatalf("run argv = %q, want no %s when none was given", runs[0], unwanted)
			}
		}

		// The optional flags, when given, reach paseo verbatim.
		code, _, errOut = h.run("create", "--project", "alpha", "--item", "item-delta",
			"--prompt", "start here", "--provider", "codex", "--model", "codex/gpt-5.4", "--mode", "build")
		if code != 0 {
			t.Fatalf("worker create (flags) exit = %d, want 0 (stderr=%q)", code, errOut)
		}
		runs = h.logLines("run")
		if len(runs) != 2 {
			t.Fatalf("paseo run calls = %v, want two", runs)
		}
		for _, want := range []string{"--provider codex", "--model codex/gpt-5.4", "--mode build", "--prompt"} {
			if want == "--prompt" {
				continue // the prompt is positional; paseo reads it as an argv, not a flag
			}
			if !strings.Contains(runs[1], want) {
				t.Fatalf("run argv = %q, want it to contain %q", runs[1], want)
			}
		}
		if !strings.HasSuffix(runs[1], "start here") {
			t.Fatalf("run argv = %q, want the prompt as the trailing argv", runs[1])
		}
	})

	t.Run("unregistered_project_makes_no_call", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.writeWorkspaces(map[string]any{"workspaces": []any{
			map[string]any{"slug": "alpha", "path": h.projectDir("alpha")},
		}})
		before := len(h.logLines(""))

		code, out, _ := h.run("create", "--project", "ghost", "--item", "item-ghost")
		if code != 2 {
			t.Fatalf("worker create (unregistered) exit = %d, want 2", code)
		}
		if !strings.Contains(out, "reason_code="+ReasonProjectNotRegistered) {
			t.Fatalf("create stdout = %q, want reason_code=%s", out, ReasonProjectNotRegistered)
		}
		if after := len(h.logLines("")); after != before {
			t.Fatalf("paseo was called %d times for an unregistered project, want 0", after-before)
		}
	})
}

// TestWorkerUpdateSupportedFields is TC-WC-004 (AC-004): name/label/mode map onto
// the two paseo verbs, and the three rebuild-only fields are refused.
func TestWorkerUpdateSupportedFields(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.seedAlpha()
		code, out, errOut := h.run("update", "ag_alpha", "--name", "renamed-worker")
		if code != 0 {
			t.Fatalf("update --name exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
		}
		lines := h.logLines("agent")
		if len(lines) != 1 || lines[0] != "agent update ag_alpha --name renamed-worker" {
			t.Fatalf("paseo calls = %v, want [agent update ag_alpha --name renamed-worker]", lines)
		}
	})

	t.Run("label", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.seedAlpha()
		code, out, errOut := h.run("update", "ag_alpha", "--label", "owner=me", "--label", "stage=review")
		if code != 0 {
			t.Fatalf("update --label exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
		}
		lines := h.logLines("agent")
		if len(lines) != 1 || lines[0] != "agent update ag_alpha --label owner=me --label stage=review" {
			t.Fatalf("paseo calls = %v, want one agent update carrying both labels", lines)
		}
		// A label without "=" is a usage error, not a silent empty value.
		if code, _, _ := h.run("update", "ag_alpha", "--label", "novalue"); code != 2 {
			t.Fatalf("update --label novalue exit = %d, want 2", code)
		}
	})

	t.Run("mode", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.seedAlpha()
		code, out, errOut := h.run("update", "ag_alpha", "--mode", "plan")
		if code != 0 {
			t.Fatalf("update --mode exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
		}
		lines := h.logLines("agent")
		if len(lines) != 1 || lines[0] != "agent mode ag_alpha plan" {
			t.Fatalf("paseo calls = %v, want [agent mode ag_alpha plan]", lines)
		}
	})

	t.Run("rebuild_only_fields_are_refused", func(t *testing.T) {
		for _, blocked := range []string{"cwd", "provider", "model"} {
			h := newWorkerCLIHarness(t)
			h.seedAlpha()
			before := len(h.logLines(""))

			code, out, _ := h.run("update", "ag_alpha", "--"+blocked, "whatever")
			if code != 2 {
				t.Fatalf("update --%s exit = %d, want 2", blocked, code)
			}
			if !strings.Contains(out, "reason_code="+ReasonFieldNotUpdatable) {
				t.Fatalf("update --%s stdout = %q, want reason_code=%s", blocked, out, ReasonFieldNotUpdatable)
			}
			if after := len(h.logLines("")); after != before {
				t.Fatalf("update --%s made %d paseo calls, want 0", blocked, after-before)
			}
		}
	})
}

// TestWorkerArchiveOwnershipRules is TC-WC-005 (AC-005): archive is the only
// removal verb, ownership is a precondition, a repeat is a no-op success, and a
// successful archive leaves one audit line behind.
func TestWorkerArchiveOwnershipRules(t *testing.T) {
	t.Run("archives_owned_worker_and_writes_audit", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.seedAlpha()

		code, out, errOut := h.run("archive", "ag_alpha")
		if code != 0 {
			t.Fatalf("archive exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
		}
		if !strings.Contains(out, "archived="+boolToken(true)) {
			t.Fatalf("archive stdout = %q, want archived=%s", out, boolToken(true))
		}
		lines := h.logLines("archive")
		if len(lines) != 1 || lines[0] != "archive ag_alpha" {
			t.Fatalf("paseo archive calls = %v, want [archive ag_alpha]", lines)
		}
		audit := h.auditLines()
		if len(audit) != 1 {
			t.Fatalf("audit lines = %v, want exactly one", audit)
		}
		for _, want := range []string{
			`"action":"archive"`, `"agent_id":"ag_alpha"`, `"ownership":"owned"`, `"item":"item-alpha"`,
		} {
			if !strings.Contains(audit[0], want) {
				t.Fatalf("audit line %q is missing %q", audit[0], want)
			}
		}

		// ② The same call again: exit 0, no second archive, no second audit line.
		code, out, errOut = h.run("archive", "ag_alpha")
		if code != 0 {
			t.Fatalf("repeat archive exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
		}
		if !strings.Contains(out, "already_archived="+boolToken(true)) {
			t.Fatalf("repeat archive stdout = %q, want already_archived=%s", out, boolToken(true))
		}
		if got := h.logLines("archive"); len(got) != 1 {
			t.Fatalf("paseo archive calls = %v, want the repeat to call nothing", got)
		}
		if got := h.auditLines(); len(got) != 1 {
			t.Fatalf("audit lines = %v, want the repeat to append nothing", got)
		}

		// ③ The default screen hides the archived worker; asking for the archived
		// state finds it, still owned -- an archive is history, not a deletion.
		if got := h.list(t); got.Count != 0 {
			t.Fatalf("default list = %+v, want the archived worker hidden", got.Workers)
		}
		archivedList := h.list(t, "--state", "archived")
		if archivedList.Count != 1 || archivedList.Workers[0].ID != "ag_alpha" {
			t.Fatalf("--state archived = %+v, want the archived worker", archivedList.Workers)
		}
		if archivedList.Workers[0].Ownership != WorkerOwned || archivedList.Workers[0].State != WorkerStateArchived {
			t.Fatalf("--state archived row = %+v, want owned/archived", archivedList.Workers[0])
		}
	})

	t.Run("refuses_an_agent_nobody_owns", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.seedAlpha()
		h.addAgent("ag_stray", "stray", "idle", h.outsideDir(), "just now", "", "")

		code, out, _ := h.run("archive", "ag_stray")
		if code != 2 {
			t.Fatalf("archive (unknown ownership) exit = %d, want 2", code)
		}
		if !strings.Contains(out, "reason_code="+ReasonOwnershipUnknown) {
			t.Fatalf("archive stdout = %q, want reason_code=%s", out, ReasonOwnershipUnknown)
		}
		if got := h.logLines("archive"); len(got) != 0 {
			t.Fatalf("paseo archive calls = %v, want none for an unowned worker", got)
		}
		if got := h.auditLines(); len(got) != 0 {
			t.Fatalf("audit lines = %v, want none for a refused archive", got)
		}
	})

	t.Run("force_reaches_paseo", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.seedAlpha()
		code, out, errOut := h.run("archive", "ag_alpha", "--force")
		if code != 0 {
			t.Fatalf("archive --force exit = %d, want 0 (stdout=%q stderr=%q)", code, out, errOut)
		}
		lines := h.logLines("archive")
		if len(lines) != 1 || lines[0] != "archive ag_alpha --force" {
			t.Fatalf("paseo archive calls = %v, want [archive ag_alpha --force]", lines)
		}
		audit := h.auditLines()
		if len(audit) != 1 || !strings.Contains(audit[0], `"forced":true`) {
			t.Fatalf("audit lines = %v, want one line marked forced", audit)
		}
	})
}

// TestNoHardDelete is TC-WC-006 (AC-006): the verb does not exist, the answer
// says so, and no source file in this package contains the call to paseo that
// would do it.
func TestNoHardDelete(t *testing.T) {
	t.Run("verb_is_refused", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		h.seedAlpha()
		before := len(h.logLines(""))

		code, out, _ := h.run("delete", "ag_alpha")
		if code != 2 {
			t.Fatalf("worker delete exit = %d, want 2", code)
		}
		if !strings.Contains(out, "reason_code="+ReasonDeleteNotSupported) {
			t.Fatalf("worker delete stdout = %q, want reason_code=%s", out, ReasonDeleteNotSupported)
		}
		if after := len(h.logLines("")); after != before {
			t.Fatalf("worker delete made %d paseo calls, want 0", after-before)
		}

		// --json gets the same refusal in machine form.
		code, out, _ = h.run("delete", "ag_alpha", "--json")
		if code != 2 || !strings.Contains(out, `"reason_code":"`+ReasonDeleteNotSupported+`"`) {
			t.Fatalf("worker delete --json = %d/%q, want exit 2 with the reason code", code, out)
		}
	})

	t.Run("no_source_file_calls_it", func(t *testing.T) {
		// The needles are assembled at run time so this test file, which has to
		// name them, cannot trip its own scan.
		needles := []string{
			"agent" + " " + "delete",
			"delete" + " " + "<id>",
		}
		files, err := packageGoFiles()
		if err != nil {
			t.Fatalf("list package sources: %v", err)
		}
		if len(files) < 10 {
			t.Fatalf("scanned %d files, want the whole package", len(files))
		}
		for _, file := range files {
			source, err := readFileString(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			for _, needle := range needles {
				if strings.Contains(source, needle) {
					t.Fatalf("%s contains %q; this surface must not be able to remove an agent for good", file, needle)
				}
			}
		}
	})

	t.Run("runner_has_no_remove_method", func(t *testing.T) {
		// The structural half of the same promise: the paseo seam has an archive
		// verb and nothing spelled like the hard one. (reflect only reports
		// exported methods, which is exactly the surface a caller can reach.)
		names := runnerMethodNames(t)
		if !names["Archive"] {
			t.Fatalf("runner methods = %v, want the archive verb to exist", names)
		}
		for name := range names {
			if strings.Contains(strings.ToLower(name), "delete") || strings.Contains(strings.ToLower(name), "remove") {
				t.Fatalf("runner method %q looks like a hard removal", name)
			}
		}
	})
}

// TestUnknownOwnershipBlocked is TC-WC-007 (AC-007): an agent with neither label
// is shown but never written to.
func TestUnknownOwnershipBlocked(t *testing.T) {
	h := newWorkerCLIHarness(t)
	h.seedAlpha()
	h.addAgent("ag_stray", "stray", "idle", h.outsideDir(), "just now", "", "")

	code, out, _ := h.run("update", "ag_stray", "--name", "whoever")
	if code != 2 {
		t.Fatalf("update (unknown ownership) exit = %d, want 2", code)
	}
	if !strings.Contains(out, "reason_code="+ReasonOwnershipUnknown) {
		t.Fatalf("update stdout = %q, want reason_code=%s", out, ReasonOwnershipUnknown)
	}
	code, out, _ = h.run("archive", "ag_stray")
	if code != 2 {
		t.Fatalf("archive (unknown ownership) exit = %d, want 2", code)
	}
	if !strings.Contains(out, "reason_code="+ReasonOwnershipUnknown) {
		t.Fatalf("archive stdout = %q, want reason_code=%s", out, ReasonOwnershipUnknown)
	}
	// Neither refusal was allowed to write anything to paseo.
	for _, verb := range []string{"agent", "archive", "run"} {
		if got := h.logLines(verb); len(got) != 0 {
			t.Fatalf("paseo %s calls = %v, want none for an unowned worker", verb, got)
		}
	}

	// list still reports it -- hiding it would make the machine look smaller than
	// it is.
	workers := h.list(t)
	found := false
	for _, row := range workers.Workers {
		if row.ID == "ag_stray" {
			found = true
			if row.Ownership != WorkerUnknown {
				t.Fatalf("ag_stray ownership = %q, want %q", row.Ownership, WorkerUnknown)
			}
		}
	}
	if !found {
		t.Fatalf("worker list hid the unowned agent: %+v", workers.Workers)
	}
}

// TestWorkspacesLongestPrefix is TC-WC-008 (AC-008): the longest path prefix
// wins, and a table that cannot answer is workspaces_invalid with exit 2.
func TestWorkspacesLongestPrefix(t *testing.T) {
	t.Run("longest_prefix_wins", func(t *testing.T) {
		h := newWorkerCLIHarness(t)
		outer := h.projectDir("outer")
		inner := outer + "/inner"
		h.mkdir(inner + "/deep")
		h.writeWorkspaces(map[string]any{"workspaces": []any{
			map[string]any{"slug": "outer", "path": outer},
			map[string]any{"slug": "inner", "path": inner},
		}})
		h.addAgent("ag_inner", "item-inner", "idle", inner+"/deep", "5 minutes ago", "", "")
		h.addAgent("ag_outer", "item-outer", "idle", outer+"/elsewhere", "5 minutes ago", "", "")
		h.addLabel("ag_inner", OwnershipLabel, "item-inner")
		h.addLabel("ag_outer", OwnershipLabel, "item-outer")

		workers := h.list(t)
		want := map[string]string{"ag_inner": "inner", "ag_outer": "outer"}
		for _, row := range workers.Workers {
			if expected := want[row.ID]; row.Project != expected {
				t.Fatalf("%s project = %q, want %q (longest prefix wins)", row.ID, row.Project, expected)
			}
		}

		// A slug that is not registered is not silently folded into its parent.
		if got := h.list(t, "--project", "outer").Count; got != 1 {
			t.Fatalf("--project outer matched %d workers, want 1", got)
		}
	})

	t.Run("shipped_table_is_valid", func(t *testing.T) {
		// The table committed with the repository must load: a broken default is
		// a broken first run for everyone who clones it. The configured value is
		// relative to the root, which is how the shipped config is written.
		root := filepath.Join("..", "..")
		table, err := LoadWorkspaces("", root)
		if err != nil {
			t.Fatalf("the committed mapping table does not load: %v", err)
		}
		if len(table.Entries) == 0 {
			t.Fatal("the committed mapping table maps no project")
		}
	})

	for _, tc := range []struct {
		name  string
		table string
	}{
		{
			name:  "duplicate_slug",
			table: `{"workspaces":[{"slug":"alpha","path":"PATH"},{"slug":"alpha","path":"PATH"}]}`,
		},
		{
			name:  "missing_path",
			table: `{"workspaces":[{"slug":"alpha","path":"PATH/gone"}]}`,
		},
		{
			name:  "invalid_json",
			table: `{"workspaces": [ this is not json `,
		},
	} {
		t.Run(tc.name+"_is_invalid", func(t *testing.T) {
			h := newWorkerCLIHarness(t)
			alpha := h.projectDir("alpha")
			h.writeWorkspacesRaw(strings.ReplaceAll(tc.table, "PATH", alpha))
			h.addAgent("ag_alpha", "item-alpha", "idle", alpha, "5 minutes ago", "", "")
			h.addLabel("ag_alpha", OwnershipLabel, "item-alpha")

			code, out, _ := h.run("list")
			if code != 2 {
				t.Fatalf("worker list with an unusable table exit = %d, want 2", code)
			}
			if !strings.Contains(out, "reason_code="+ReasonWorkspacesInvalid) {
				t.Fatalf("worker list stdout = %q, want reason_code=%s", out, ReasonWorkspacesInvalid)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// the fake paseo, on disk
// ---------------------------------------------------------------------------

// fakeWorkerPaseo is a real executable standing in for paseo. State lives in
// $FAKE_PASEO_HOME as two tab-separated files:
//
//	agents.txt  id  name  status  cwd  created  updated  pending-tool
//	labels.txt  id  key  value
//
// It speaks the verbs this feature uses and reproduces the two behaviours the
// real CLI has that matter here: `ls` drops labels (so ownership has to be asked
// for separately) and `ls` hides archived agents by default. Every invocation is
// appended to $FAKE_PASEO_LOG, which is what the assertions read.
const fakeWorkerPaseo = `#!/bin/sh
H="${FAKE_PASEO_HOME:?FAKE_PASEO_HOME must point at the state directory}"
AG="$H/agents.txt"
LB="$H/labels.txt"
if [ -n "${FAKE_PASEO_LOG:-}" ]; then printf '%s\n' "$*" >> "$FAKE_PASEO_LOG"; fi
[ -f "$AG" ] || : > "$AG"
[ -f "$LB" ] || : > "$LB"

verb="${1:-}"
if [ $# -gt 0 ]; then shift; fi

case "$verb" in
ls)
  want=""
  include_archived=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --label) want="$2"; shift 2 ;;
      -a|--all) include_archived=1; shift ;;
      *) shift ;;
    esac
  done
  awk -F'\t' -v want="$want" -v lb="$LB" -v with_archived="$include_archived" '
    BEGIN {
      if (want != "") {
        while ((getline line < lb) > 0) {
          if (line == "") continue
          if ((split(line, p, "\t")) >= 3 && (p[2] "=" p[3]) == want) keep[p[1]] = 1
        }
      }
      printf "["
      first = 1
    }
    {
      if ($1 == "") next
      if (want != "" && !($1 in keep)) next
      if ($3 == "archived" && with_archived == 0) next
      if (!first) printf ","
      printf "{\"id\":\"%s\",\"shortId\":\"%s\",\"name\":\"%s\",\"status\":\"%s\",\"cwd\":\"%s\",\"created\":\"%s\"}", $1, substr($1, 1, 7), $2, $3, $4, $5
      first = 0
    }
    END { printf "]\n" }
  ' "$AG"
  exit 0
  ;;
inspect)
  id="${1:-}"
  awk -F'\t' -v id="$id" -v lb="$LB" '
    BEGIN {
      n = 0
      while ((getline line < lb) > 0) {
        if (line == "") continue
        if ((split(line, p, "\t")) >= 3 && p[1] == id) { n++; labs[n] = p[2] "=" p[3] }
      }
    }
    $1 == id {
      archived = "false"; stamp = "null"
      if ($3 == "archived") { archived = "true"; stamp = "\"" $6 "\"" }
      printf "{\"Id\":\"%s\",\"Name\":\"%s\",\"Status\":\"%s\",\"Archived\":%s,\"ArchivedAt\":%s,\"Cwd\":\"%s\",\"CreatedAt\":\"%s\",\"UpdatedAt\":\"%s\",\"PendingPermissions\":[", $1, $2, $3, archived, stamp, $4, $5, $6
      if ($7 != "") printf "{\"id\":\"perm-1\",\"tool\":\"%s\"}", $7
      printf "],\"labels\":["
      for (i = 1; i <= n; i++) { if (i > 1) printf ","; printf "\"%s\"", labs[i] }
      printf "]}\n"
      hit = 1
    }
    END { if (!hit) exit 1 }
  ' "$AG"
  exit $?
  ;;
agent)
  sub="${1:-}"
  if [ $# -gt 0 ]; then shift; fi
  case "$sub" in
  update)
    id="${1:-}"
    if [ $# -gt 0 ]; then shift; fi
    newname=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --name) newname="$2"; shift 2 ;;
        --label)
          kv="$2"; shift 2
          printf '%s\t%s\t%s\n' "$id" "${kv%%=*}" "${kv#*=}" >> "$LB"
          ;;
        *) shift ;;
      esac
    done
    if [ -n "$newname" ]; then
      awk -F'\t' -v OFS='\t' -v id="$id" -v nd="$newname" '{ if ($1 == id) $2 = nd; print }' "$AG" > "$AG.new"
      mv "$AG.new" "$AG"
    fi
    exit 0
    ;;
  mode) exit 0 ;;
  esac
  exit 0
  ;;
archive)
  id="${1:-}"
  shift || true
  stamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  awk -F'\t' -v OFS='\t' -v id="$id" -v stamp="$stamp" '
    { if ($1 == id) { if ($3 != "archived") $3 = "archived"; $6 = stamp } print }
  ' "$AG" > "$AG.new"
  mv "$AG.new" "$AG"
  exit 0
  ;;
run)
  title=""; cwd=""; newid="${FAKE_PASEO_NEW_ID:-ag_new}"
  while [ $# -gt 0 ]; do
    case "$1" in
      --title) title="$2"; shift 2 ;;
      --cwd) cwd="$2"; shift 2 ;;
      --label)
        kv="$2"; shift 2
        printf '%s\t%s\t%s\n' "$newid" "${kv%%=*}" "${kv#*=}" >> "$LB"
        ;;
      --provider|--model|--mode|--new-workspace|--prompt) shift 2 ;;
      *) shift ;;
    esac
  done
  printf '%s\t%s\trunning\t%s\t%s\t%s\t\n' "$newid" "$title" "$cwd" "2026-01-01T00:00:00Z" "2026-01-01T00:00:00Z" >> "$AG"
  printf '{\n  "agentId": "%s",\n  "status": "running"\n}\n' "$newid"
  exit 0
  ;;
esac
exit 0
`

// ---------------------------------------------------------------------------
// the harness
// ---------------------------------------------------------------------------

// workerCLIHarness wires one test run: a temp ADB root, a config file pointing at
// the fake paseo and at a mapping table, and the fake's state directory.
type workerCLIHarness struct {
	t        *testing.T
	root     string
	bin      string
	logPath  string
	stateDir string
	wsPath   string
}

func newWorkerCLIHarness(t *testing.T) *workerCLIHarness {
	t.Helper()
	dir := t.TempDir()
	h := &workerCLIHarness{
		t:        t,
		root:     filepath.Join(dir, "adb"),
		bin:      filepath.Join(dir, "fake-paseo.sh"),
		logPath:  filepath.Join(dir, "argv.log"),
		stateDir: filepath.Join(dir, "paseo-state"),
	}
	h.wsPath = filepath.Join(h.root, "config", "workspaces.json")
	if err := os.MkdirAll(filepath.Join(h.root, ".beacon"), 0o755); err != nil {
		t.Fatalf("create .beacon: %v", err)
	}
	if err := os.MkdirAll(h.stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(h.bin, []byte(fakeWorkerPaseo), 0o755); err != nil {
		t.Fatalf("write fake paseo: %v", err)
	}
	config := "adbd_paseo_bin=" + h.bin + "\nadbd_workspaces_file=" + h.wsPath + "\n"
	if err := os.WriteFile(filepath.Join(h.root, ".beacon", "config"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("FAKE_PASEO_HOME", h.stateDir)
	t.Setenv("FAKE_PASEO_LOG", h.logPath)
	// A pinned clock keeps the age column exact instead of "whatever the wall
	// clock said between two assertions".
	h.freezeClock(time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC))
	return h
}

// freezeClock pins workerClock for the lifetime of the test.
func (h *workerCLIHarness) freezeClock(at time.Time) {
	previous := workerClock
	workerClock = func() time.Time { return at }
	h.t.Cleanup(func() { workerClock = previous })
}

// run invokes the CLI the way an operator would: `adbd worker <args> --root R`.
func (h *workerCLIHarness) run(args ...string) (int, string, string) {
	h.t.Helper()
	full := append([]string{"worker"}, args...)
	full = append(full, "--root", h.root)
	var out, errOut bytes.Buffer
	code := run(full, &out, &errOut)
	return code, out.String(), errOut.String()
}

// seedAlpha is the standard one-project, one-owned-worker fixture.
func (h *workerCLIHarness) seedAlpha() {
	h.t.Helper()
	h.writeWorkspaces(map[string]any{"workspaces": []any{
		map[string]any{"slug": "alpha", "path": h.projectDir("alpha")},
	}})
	h.addAgent("ag_alpha", "item-alpha", "running", h.projectDir("alpha"), "2026-01-01T00:00:00Z", "2026-01-01T04:00:00Z", "bash")
	h.addLabel("ag_alpha", OwnershipLabel, "item-alpha")
}

// projectDir creates (once) and returns a directory under the ADB root.
func (h *workerCLIHarness) projectDir(name string) string {
	h.t.Helper()
	dir := filepath.Join(h.root, "projects", name)
	h.mkdir(dir)
	return dir
}

// outsideDir is a directory that belongs to no registered project.
func (h *workerCLIHarness) outsideDir() string {
	h.t.Helper()
	dir := filepath.Join(filepath.Dir(h.root), "elsewhere")
	h.mkdir(dir)
	return dir
}

func (h *workerCLIHarness) mkdir(dir string) {
	h.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatalf("create %s: %v", dir, err)
	}
}

// writeWorkspaces writes the mapping table.
func (h *workerCLIHarness) writeWorkspaces(doc map[string]any) {
	h.t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		h.t.Fatalf("marshal workspaces: %v", err)
	}
	h.writeWorkspacesRaw(string(data))
}

// writeWorkspacesRaw writes an exact table, including the broken ones.
func (h *workerCLIHarness) writeWorkspacesRaw(raw string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.wsPath), 0o755); err != nil {
		h.t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(h.wsPath, []byte(raw), 0o644); err != nil {
		h.t.Fatalf("write workspaces: %v", err)
	}
}

// addAgent appends one agent row.
func (h *workerCLIHarness) addAgent(id, name, status, cwd, created, updated, tool string) {
	h.t.Helper()
	line := strings.Join([]string{id, name, status, cwd, created, updated, tool}, "\t") + "\n"
	h.appendFile(filepath.Join(h.stateDir, "agents.txt"), line)
}

// addLabel appends one label.
func (h *workerCLIHarness) addLabel(id, key, value string) {
	h.t.Helper()
	h.appendFile(filepath.Join(h.stateDir, "labels.txt"), strings.Join([]string{id, key, value}, "\t")+"\n")
}

func (h *workerCLIHarness) appendFile(path, line string) {
	h.t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		h.t.Fatalf("open %s: %v", path, err)
	}
	if _, err := f.WriteString(line); err != nil {
		_ = f.Close()
		h.t.Fatalf("write %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		h.t.Fatalf("close %s: %v", path, err)
	}
}

// logLines returns recorded paseo invocations, optionally only one verb's.
func (h *workerCLIHarness) logLines(verb string) []string {
	h.t.Helper()
	data, err := os.ReadFile(h.logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read argv log: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if verb == "" || line == verb || strings.HasPrefix(line, verb+" ") {
			out = append(out, line)
		}
	}
	return out
}

// auditLines returns the archive ledger's lines.
func (h *workerCLIHarness) auditLines() []string {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.root, ".beacon", WorkerAuditFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read audit ledger: %v", err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// list runs `worker list --json` and decodes it.
func (h *workerCLIHarness) list(t *testing.T, extra ...string) workerListDoc {
	t.Helper()
	args := append([]string{"list", "--json"}, extra...)
	code, out, errOut := h.run(args...)
	if code != 0 {
		t.Fatalf("worker list %v exit = %d, want 0 (stdout=%q stderr=%q)", extra, code, out, errOut)
	}
	return decodeWorkerList(t, out)
}

// ---------------------------------------------------------------------------
// decoding helpers
// ---------------------------------------------------------------------------

type workerListDoc struct {
	Workspaces string `json:"workspaces"`
	Count      int    `json:"count"`
	Workers    []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Project   string `json:"project"`
		Ownership string `json:"ownership"`
		State     string `json:"state"`
		Age       string `json:"age"`
		Item      string `json:"item"`
	} `json:"workers"`
}

type workerShowDoc struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Cwd                string            `json:"cwd"`
	Project            string            `json:"project"`
	Status             string            `json:"status"`
	PendingPermissions []string          `json:"pending_permissions"`
	Labels             map[string]string `json:"labels"`
	Created            string            `json:"created"`
	Updated            string            `json:"updated"`
	Ownership          string            `json:"ownership"`
}

func decodeWorkerList(t *testing.T, raw string) workerListDoc {
	t.Helper()
	var doc workerListDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("worker list is not JSON: %v (output=%q)", err, raw)
	}
	if doc.Workspaces == "" {
		t.Fatalf("worker list did not name the mapping table it read: %q", raw)
	}
	return doc
}

func decodeWorkerShow(t *testing.T, raw string) workerShowDoc {
	t.Helper()
	var doc workerShowDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("worker show is not JSON: %v (output=%q)", err, raw)
	}
	return doc
}

// ---------------------------------------------------------------------------
// source scanning
// ---------------------------------------------------------------------------

// packageGoFiles lists this package's production sources (tests excluded: a test
// that has to name a forbidden string must not be scanned for it).
func packageGoFiles() ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

// runnerMethodNames is the method set of the paseo seam.
func runnerMethodNames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	typ := reflect.TypeOf(PaseoCliRunner{})
	for i := 0; i < typ.NumMethod(); i++ {
		names[typ.Method(i).Name] = true
	}
	return names
}

// readFileString reads a source file for the scan.
func readFileString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// AC-004/AC-005: a foreign agent is deliberately "not ours"; editing or
// archiving it needs --force, and without it nothing is written to paseo.
func TestForeignWorkerNeedsForce(t *testing.T) {
	h := newWorkerCLIHarness(t)
	h.addAgent("ag_foreign", "foreign-one", "idle", h.projectDir("beta"), "3 days ago", "", "")
	h.addLabel("ag_foreign", ForeignLabel, ForeignLabelValue)

	code, out, _ := h.run("update", "ag_foreign", "--name", "renamed")
	if code != 2 || !strings.Contains(out, ReasonOwnershipUnknown) {
		t.Fatalf("update foreign = code %d stdout %q, want 2 + %s", code, out, ReasonOwnershipUnknown)
	}
	if calls := h.logLines("agent"); len(calls) != 0 {
		t.Fatalf("update foreign wrote to paseo: %v", calls)
	}

	code, out, _ = h.run("archive", "ag_foreign")
	if code != 2 || !strings.Contains(out, ReasonOwnershipUnknown) {
		t.Fatalf("archive foreign = code %d stdout %q, want 2 + %s", code, out, ReasonOwnershipUnknown)
	}
	if calls := h.logLines("archive"); len(calls) != 0 {
		t.Fatalf("archive foreign called paseo: %v", calls)
	}

	code, _, _ = h.run("update", "ag_foreign", "--name", "renamed", "--force")
	if code != 0 {
		t.Fatalf("update foreign --force = %d, want 0", code)
	}
}
