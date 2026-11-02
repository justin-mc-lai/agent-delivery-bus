package main

// Thread -> repository mapping tests (feature thread-workdir, v0.1.5):
// TC-TW-001 .. TC-TW-006.
//
// These tests drive the production seam for real: the dispatcher builds the argv
// and hands it to an executable on disk (the /bin/sh fake paseo from
// worker_test.go), so what is asserted is the `--cwd` a real `paseo run` would
// receive -- not an in-memory expectation. Nothing calls the real paseo and
// nothing touches the network: the fake is a temp script and every path is a
// temp dir.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// writeProjectRegistry writes <root>/config/projects.json with these rows and
// returns the file path. The shape mirrors the committed registry: a `projects`
// array of slug/repo/aliases objects, plus the optional `github` field this
// feature reads.
func writeProjectRegistry(t *testing.T, root string, projects []map[string]any) string {
	t.Helper()
	path := filepath.Join(root, "config", "projects.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir registry dir: %v", err)
	}
	data, err := json.MarshalIndent(map[string]any{"schema_version": "1.0", "projects": projects}, "", "  ")
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	return path
}

// mustDir creates a directory (and its parents) or fails the test.
func mustDir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	return path
}

// argvCwd extracts the value of `--cwd` from one recorded paseo invocation.
func argvCwd(t *testing.T, line string) string {
	t.Helper()
	const marker = " --cwd "
	i := strings.Index(line, marker)
	if i < 0 {
		t.Fatalf("argv %q carries no --cwd", line)
	}
	rest := line[i+len(marker):]
	if j := strings.Index(rest, " --"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// readFallbackRecords returns the workdir_fallback lines of a reports ledger,
// in order. Worker receipts (which carry no `kind`) are skipped, which is also
// what proves the two record shapes coexist in one file.
func readFallbackRecords(t *testing.T, path string) []workdirFallbackRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read reports ledger: %v", err)
	}
	var out []workdirFallbackRecord
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var record workdirFallbackRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("reports line %d is not JSON: %v (line: %q)", i+1, err, line)
		}
		if record.Kind == WorkdirFallbackKind {
			out = append(out, record)
		}
	}
	return out
}

// appendQueueItems adds router input lines without rewriting the queue, the way
// the fastpath appends them.
func appendQueueItems(t *testing.T, slowPath string, items ...QueueItem) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(slowPath), 0o755); err != nil {
		t.Fatalf("mkdir queue dir: %v", err)
	}
	f, err := os.OpenFile(slowPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open slow path: %v", err)
	}
	defer func() { _ = f.Close() }()
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("marshal queue item: %v", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			t.Fatalf("append queue item: %v", err)
		}
	}
}

// workdirDispatcher is the production dispatcher pointed at the fake paseo, with
// a registry and a fallback directory as the daemon wiring sets them.
func workdirDispatcher(f *fakePaseo, registry ProjectRegistry, fallback, reportsPath string) PaseoDispatcher {
	return PaseoDispatcher{
		Bin:             f.bin,
		Provider:        DefaultWorkerProvider,
		Model:           DefaultWorkerModel,
		FallbackWorkdir: fallback,
		Registry:        registry,
		ReportsPath:     reportsPath,
	}
}

// ---------------------------------------------------------------------------
// TC-TW-001 (AC-001): the thread projection key
// ---------------------------------------------------------------------------

func TestThreadWorkdirKey(t *testing.T) {
	cases := []struct {
		name   string
		thread string
		want   string
	}{
		{"skill thread", "skill:alpha:add-retry", "alpha"},
		{"skill thread with dots and digits", "skill:acme.app_2:fix-1", "acme.app_2"},
		{"github thread", "gh:cli/cli#1234", "cli/cli"},
		{"github thread with a dotted repo", "gh:acme/my.repo-2#7", "acme/my.repo-2"},

		// Everything else carries no project, and "" is the honest answer: the
		// caller then uses its fallback directory instead of guessing.
		{"feishu thread", "feishu:om_abc", ""},
		{"plain thread", "t-plain", ""},
		{"empty thread", "", ""},
		{"whitespace thread", "   ", ""},
		{"skill thread with two parts", "skill:alpha", ""},
		{"skill thread with four parts", "skill:alpha:slug:extra", ""},
		{"skill thread with uppercase parts", "skill:Alpha:slug", ""},
		{"unknown prefix with three parts", "thread:alpha:slug", ""},
		{"github thread without a number", "gh:cli/cli", ""},
		{"github thread without a repo", "gh:cli#12", ""},
		{"github thread with a nested path", "gh:cli/cli/extra#12", ""},
		{"github thread with a non-numeric number", "gh:cli/cli#abc", ""},
		{"github thread with an empty number", "gh:cli/cli#", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := threadWorkdirKey(tc.thread); got != tc.want {
				t.Fatalf("threadWorkdirKey(%q) = %q, want %q", tc.thread, got, tc.want)
			}
		})
	}

	// Surrounding spaces are trimmed, so a thread copied out of a log still
	// resolves to the same key.
	if got := threadWorkdirKey("  skill:alpha:add-retry  "); got != "alpha" {
		t.Fatalf("padded skill thread = %q, want alpha", got)
	}
}

// ---------------------------------------------------------------------------
// TC-TW-002 (AC-002): an unusable registry is empty, and never fatal
// ---------------------------------------------------------------------------

func TestProjectsRegistryTolerant(t *testing.T) {
	dir := t.TempDir()

	// ① A missing file is an empty registry, not an error.
	missing := loadProjectRegistry(filepath.Join(dir, "absent.json"), dir)
	if missing.Len() != 0 {
		t.Fatalf("missing registry has %d keys, want 0", missing.Len())
	}
	if _, ok := missing.Resolve("alpha"); ok {
		t.Fatal("a missing registry resolved a key")
	}

	// ② Invalid JSON, ③ `projects` not an array, ④ entries that are not objects:
	// all three degrade to the same empty registry, because a broken registry may
	// never stop dispatch.
	broken := map[string]string{
		"invalid-json":     "{ this is not json",
		"projects-object":  `{"projects": {"slug": "alpha", "repo": "."}}`,
		"projects-string":  `{"projects": "alpha"}`,
		"projects-scalars": `{"projects": [1, 2, 3]}`,
		"projects-empty":   `{"projects": []}`,
		"empty-document":   `{}`,
	}
	for name, body := range broken {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".json")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			registry := loadProjectRegistry(path, dir)
			if registry.Len() != 0 {
				t.Fatalf("registry %s has %d keys, want 0", name, registry.Len())
			}
			if _, ok := registry.Resolve("alpha"); ok {
				t.Fatalf("registry %s resolved a key", name)
			}
		})
	}

	// ⑤ A valid registry with a relative repo and one alias: the repo is resolved
	// against the root, and existence is only checked when resolving.
	root := t.TempDir()
	alphaDir := mustDir(t, filepath.Join(root, "vendor", "alpha"))
	writeProjectRegistry(t, root, []map[string]any{
		{"slug": "alpha", "repo": "vendor/alpha", "aliases": []string{"alpha-app"}, "github": "acme/alpha"},
		{"slug": "gone", "repo": "vendor/gone"},
		{"slug": "norepo", "repo": ""},
	})
	registry := loadProjectRegistry("", root)

	if registry.Source() != filepath.Join(root, "config", "projects.json") {
		t.Fatalf("registry source = %q, want <root>/config/projects.json", registry.Source())
	}
	for _, key := range []string{"alpha", "alpha-app", "acme/alpha", "Alpha", " acme/alpha "} {
		dir, ok := registry.Resolve(key)
		if !ok || dir != alphaDir {
			t.Fatalf("Resolve(%q) = %q, %v; want %q, true", key, dir, ok, alphaDir)
		}
	}

	// A registered project whose directory is absent resolves to nothing, while
	// Lookup still knows it *was* registered -- that difference is what names the
	// fallback reason (repo_missing vs project_not_registered).
	if _, ok := registry.Resolve("gone"); ok {
		t.Fatal("a repo directory that does not exist resolved")
	}
	if dir, ok := registry.Lookup("gone"); !ok || dir != filepath.Join(root, "vendor", "gone") {
		t.Fatalf("Lookup(gone) = %q, %v; want the registered path, true", dir, ok)
	}
	if _, ok := registry.Lookup("norepo"); ok {
		t.Fatal("a project without a repo was indexed")
	}
	if _, ok := registry.Resolve("ghost"); ok {
		t.Fatal("an unregistered project resolved")
	}

	// ⑥ adbd_projects_file overrides the default, and a relative override is read
	// against the root.
	custom := filepath.Join(root, "alt", "registry.json")
	if err := os.MkdirAll(filepath.Dir(custom), 0o755); err != nil {
		t.Fatalf("mkdir alt: %v", err)
	}
	body := `{"projects":[{"slug":"beta","repo":"` + alphaDir + `"}]}`
	if err := os.WriteFile(custom, []byte(body), 0o644); err != nil {
		t.Fatalf("write custom registry: %v", err)
	}
	for _, path := range []string{custom, filepath.Join("alt", "registry.json")} {
		overridden := loadProjectRegistry(path, root)
		if _, ok := overridden.Resolve("beta"); !ok {
			t.Fatalf("adbd_projects_file %q was not read (keys: %d)", path, overridden.Len())
		}
	}

	// ⑦ The zero value is usable: no panic, no resolution.
	var zero ProjectRegistry
	if _, ok := zero.Resolve("alpha"); ok {
		t.Fatal("the zero registry resolved a key")
	}
	if zero.Len() != 0 || zero.Source() != "" {
		t.Fatalf("zero registry = %d keys from %q, want 0 keys and no source", zero.Len(), zero.Source())
	}

	// ⑧ adbd_projects_file is what configures the registry file, and an unset key
	// keeps the <root>/config/projects.json default.
	configPath := filepath.Join(root, "daemon-config")
	if err := os.WriteFile(configPath, []byte("adbd_projects_file=alt/registry.json\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := projectsFilePath(cfg.ProjectsFile, root); got != custom {
		t.Fatalf("projectsFilePath(%q) = %q, want %q", cfg.ProjectsFile, got, custom)
	}
	emptyCfg, err := LoadConfig(filepath.Join(root, "absent-config"))
	if err != nil {
		t.Fatalf("LoadConfig(absent): %v", err)
	}
	if got := projectsFilePath(emptyCfg.ProjectsFile, root); got != filepath.Join(root, "config", "projects.json") {
		t.Fatalf("default projects file = %q, want <root>/config/projects.json", got)
	}
}

// ---------------------------------------------------------------------------
// TC-TW-003 (AC-003): --cwd comes from the registry
// ---------------------------------------------------------------------------

func TestCwdFromRegistry(t *testing.T) {
	root := t.TempDir()
	alphaDir := mustDir(t, filepath.Join(root, "vendor", "alpha"))
	writeProjectRegistry(t, root, []map[string]any{
		{"slug": "alpha", "title": "Alpha", "repo": "vendor/alpha", "aliases": []string{"alpha-app"}, "github": "acme/alpha"},
	})
	registry := loadProjectRegistry("", root)
	f := newFakePaseo(t)
	fallback := mustDir(t, filepath.Join(root, "fallback"))
	reports := filepath.Join(root, ".beacon", reportsFile)
	d := workdirDispatcher(f, registry, fallback, reports)

	// A skill thread resolves through the slug.
	if _, err := d.EnsureWorker("skill:alpha:add-retry", ""); err != nil {
		t.Fatalf("EnsureWorker(skill:alpha): %v", err)
	}
	// The alias resolves to the same checkout.
	t.Setenv("FAKE_PASEO_RUN_AGENT_ID", "ag_alias")
	if _, err := d.EnsureWorker("skill:alpha-app:add-retry", ""); err != nil {
		t.Fatalf("EnsureWorker(alias): %v", err)
	}
	// A gh thread resolves through the github field.
	t.Setenv("FAKE_PASEO_RUN_AGENT_ID", "ag_gh")
	if _, err := d.EnsureWorker("gh:acme/alpha#42", ""); err != nil {
		t.Fatalf("EnsureWorker(gh): %v", err)
	}

	runs := f.verbs("run")
	if len(runs) != 3 {
		t.Fatalf("run calls = %d, want 3 (%v)", len(runs), runs)
	}
	for i, run := range runs {
		if cwd := argvCwd(t, run); cwd != alphaDir {
			t.Fatalf("run[%d] --cwd = %q, want the registered project directory %q", i, cwd, alphaDir)
		}
		if strings.Contains(run, fallback) {
			t.Fatalf("run[%d] used the fallback directory although the project resolved: %q", i, run)
		}
	}

	// Nothing fell back, so the ledger must hold no fallback record at all.
	if records := readFallbackRecords(t, reports); len(records) != 0 {
		t.Fatalf("fallback records = %+v, want none when every thread resolved", records)
	}
}

// ---------------------------------------------------------------------------
// TC-TW-004 (AC-004): falling back is recorded, never silent
// ---------------------------------------------------------------------------

func TestWorkdirFallbackRecorded(t *testing.T) {
	root := t.TempDir()
	mustDir(t, filepath.Join(root, "vendor", "alpha"))
	// "gone" is registered but its directory does not exist.
	writeProjectRegistry(t, root, []map[string]any{
		{"slug": "alpha", "repo": "vendor/alpha"},
		{"slug": "gone", "repo": "vendor/gone"},
	})
	registry := loadProjectRegistry("", root)
	f := newFakePaseo(t)
	fallback := mustDir(t, filepath.Join(root, "fallback"))
	reports := filepath.Join(root, ".beacon", reportsFile)
	d := workdirDispatcher(f, registry, fallback, reports)

	if _, err := d.EnsureWorker("skill:ghost:add-retry", ""); err != nil {
		t.Fatalf("EnsureWorker(unregistered project): %v", err)
	}
	t.Setenv("FAKE_PASEO_RUN_AGENT_ID", "ag_unkeyed")
	if _, err := d.EnsureWorker("feishu:om_123", ""); err != nil {
		t.Fatalf("EnsureWorker(no project key): %v", err)
	}
	t.Setenv("FAKE_PASEO_RUN_AGENT_ID", "ag_gone")
	if _, err := d.EnsureWorker("skill:gone:add-retry", ""); err != nil {
		t.Fatalf("EnsureWorker(missing repo dir): %v", err)
	}

	// Every fallback still created a worker -- with the configured default -- so
	// a missing mapping never blocks the dispatch.
	runs := f.verbs("run")
	if len(runs) != 3 {
		t.Fatalf("run calls = %d, want 3 (%v)", len(runs), runs)
	}
	for i, run := range runs {
		if cwd := argvCwd(t, run); cwd != fallback {
			t.Fatalf("run[%d] --cwd = %q, want the fallback directory %q", i, cwd, fallback)
		}
	}

	// ... and each fallback left a record naming the thread and the reason.
	want := []struct {
		thread string
		reason string
	}{
		{"skill:ghost:add-retry", ReasonProjectNotRegistered},
		{"feishu:om_123", ReasonNoProjectKey},
		{"skill:gone:add-retry", ReasonRepoMissing},
	}
	records := readFallbackRecords(t, reports)
	if len(records) != len(want) {
		t.Fatalf("fallback records = %+v, want %d", records, len(want))
	}
	for i, expected := range want {
		if records[i].Thread != expected.thread || records[i].Reason != expected.reason {
			t.Fatalf("record[%d] = %+v, want thread=%q reason=%q", i, records[i], expected.thread, expected.reason)
		}
	}

	// A thread that DOES resolve writes no record, so the ledger stays a list of
	// real fallbacks instead of a log of every dispatch.
	if _, err := d.EnsureWorker("skill:alpha:add-retry", ""); err != nil {
		t.Fatalf("EnsureWorker(resolved project): %v", err)
	}
	if got := len(readFallbackRecords(t, reports)); got != len(want) {
		t.Fatalf("fallback records = %d after a resolved dispatch, want %d", got, len(want))
	}
}

// TestWorkdirFallbackLedgerCoexistsWithReceipts guards the ledger invariant
// AC-004 depends on: fallback records and reconcile receipts share one file, so
// reconcile's receipt reader must skip a fallback line instead of calling the
// ledger damaged (which would make every later reconcile pass fail).
func TestWorkdirFallbackLedgerCoexistsWithReceipts(t *testing.T) {
	dir := t.TempDir()
	f := newFakePaseo(t)
	reports := filepath.Join(dir, reportsFile)
	d := workdirDispatcher(f, ProjectRegistry{}, mustDir(t, filepath.Join(dir, "fallback")), reports)

	if _, err := d.EnsureWorker("feishu:om_1", ""); err != nil {
		t.Fatalf("EnsureWorker: %v", err)
	}
	receipt := `{"thread":"skill:alpha:x","agent_id":"ag_done","dispatched_at":"2026-02-06T08:00:00Z","finished_at":"2026-02-06T08:30:00Z","status":"done"}` + "\n"
	file, err := os.OpenFile(reports, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	if _, err := file.WriteString(receipt); err != nil {
		_ = file.Close()
		t.Fatalf("append receipt: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close ledger: %v", err)
	}

	// The receipt reader sees the one real receipt and does not choke on the
	// fallback line; the fallback reader sees the one fallback and ignores the
	// receipt.
	ids, err := readReportAgentIDs(reports)
	if err != nil {
		t.Fatalf("readReportAgentIDs: %v", err)
	}
	if len(ids) != 1 || !ids["ag_done"] {
		t.Fatalf("receipts = %v, want exactly [ag_done]", ids)
	}
	records := readFallbackRecords(t, reports)
	if len(records) != 1 || records[0].Thread != "feishu:om_1" {
		t.Fatalf("fallback records = %+v, want exactly the om_1 record", records)
	}
}

// ---------------------------------------------------------------------------
// TC-TW-005 (AC-005): a bound thread is reused, not re-resolved
// ---------------------------------------------------------------------------

func TestBoundThreadSkipsResolution(t *testing.T) {
	root := t.TempDir()
	mustDir(t, filepath.Join(root, "vendor", "alpha"))
	writeProjectRegistry(t, root, []map[string]any{
		{"slug": "alpha", "repo": "vendor/alpha"},
	})
	registry := loadProjectRegistry("", root)

	f := newFakePaseo(t)
	f.exists("ag_bound")
	fallback := mustDir(t, filepath.Join(root, "fallback"))
	reports := filepath.Join(root, ".beacon", reportsFile)

	h := newRouteHarness(t, RuleDecider{})
	h.router.AgentID = ""
	h.router.Dispatcher = workdirDispatcher(f, registry, fallback, reports)

	// Both threads already own a live worker: one that WOULD resolve, one that
	// would not. Neither may reach the registry, and neither may leave a record.
	seedBinding(t, h, "feishu", "skill:alpha:add-retry", "ag_bound")
	seedBinding(t, h, "feishu", "feishu:om_unmapped", "ag_bound")

	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "skill:alpha:add-retry", MessageID: "m-1", Text: "add a retry to the webhook"},
		QueueItem{Channel: "feishu", Thread: "feishu:om_unmapped", MessageID: "m-2", Text: "rename the loader"},
	)
	res := h.drain()
	if res.Applied != 2 || res.Failed != 0 {
		t.Fatalf("drain = %+v, want applied=2 failed=0", res)
	}
	if got := f.verbs("run"); len(got) != 0 {
		t.Fatalf("a bound thread created a worker: %v", got)
	}
	// The second message on the same threads changes nothing either.
	h.enqueue(
		QueueItem{Channel: "feishu", Thread: "skill:alpha:add-retry", MessageID: "m-3", Text: "also add sorting"},
		QueueItem{Channel: "feishu", Thread: "feishu:om_unmapped", MessageID: "m-4", Text: "also rename the writer"},
	)
	res = h.drain()
	if res.Applied != 2 {
		t.Fatalf("second drain = %+v, want applied=2", res)
	}
	if got := f.verbs("run"); len(got) != 0 {
		t.Fatalf("a reused binding created a worker: %v", got)
	}
	if records := readFallbackRecords(t, reports); len(records) != 0 {
		t.Fatalf("reused bindings wrote %+v, want no fallback record at all", records)
	}

	// A configured worker is reused the same way, without resolving the unkeyed
	// thread it is used for.
	f2 := newFakePaseo(t)
	f2.exists("ag_config")
	h2 := newRouteHarness(t, RuleDecider{})
	h2.router.AgentID = "ag_config"
	h2.router.Dispatcher = workdirDispatcher(f2, registry, fallback, reports)
	h2.enqueue(QueueItem{Channel: "feishu", Thread: "feishu:om_other", MessageID: "m-5", Text: "write the migration"})
	if res := h2.drain(); res.Applied != 1 {
		t.Fatalf("configured-worker drain = %+v, want applied=1", res)
	}
	if got := f2.verbs("run"); len(got) != 0 {
		t.Fatalf("a verified configured worker was replaced: %v", got)
	}
	if records := readFallbackRecords(t, reports); len(records) != 0 {
		t.Fatalf("configured-worker dispatch wrote %+v, want no fallback record", records)
	}
}

// ---------------------------------------------------------------------------
// TC-TW-006 (AC-006): both thread shapes end to end through the router
// ---------------------------------------------------------------------------

func TestWorkdirEndToEnd(t *testing.T) {
	root := t.TempDir()
	alphaDir := mustDir(t, filepath.Join(root, "vendor", "alpha"))
	betaDir := mustDir(t, filepath.Join(root, "vendor", "beta"))
	writeProjectRegistry(t, root, []map[string]any{
		{"slug": "alpha", "repo": "vendor/alpha", "aliases": []string{"alpha-app"}},
		{"slug": "beta", "repo": "vendor/beta", "github": "acme/beta"},
	})
	registry := loadProjectRegistry("", root)

	f := newFakePaseo(t)
	fallback := mustDir(t, filepath.Join(root, "fallback"))
	reports := filepath.Join(root, ".beacon", reportsFile)

	h := newRouteHarness(t, RuleDecider{})
	h.router.AgentID = ""
	h.router.Dispatcher = workdirDispatcher(f, registry, fallback, reports)

	// One skill thread -> alpha, one gh thread (with its own id) -> beta, one
	// unkeyed thread -> the fallback directory and a record.
	steps := []struct {
		agentID string
		item    QueueItem
	}{
		{"ag_skill", QueueItem{Channel: "skill", Thread: "skill:alpha:add-retry", MessageID: "m-skill", Text: "add a retry to the webhook"}},
		{"ag_gh", QueueItem{Channel: "github", Thread: "gh:acme/beta#7", MessageID: "m-gh", Text: "the export hangs on big files"}},
		{"ag_plain", QueueItem{Channel: "feishu", Thread: "feishu:om_9", MessageID: "m-plain", Text: "rename the loader module"}},
	}
	for _, step := range steps {
		t.Setenv("FAKE_PASEO_RUN_AGENT_ID", step.agentID)
		h.enqueue(step.item)
		if res := h.drain(); res.Applied != 1 || res.Failed != 0 {
			t.Fatalf("drain(%s) = %+v, want applied=1 failed=0", step.item.Thread, res)
		}
	}

	runs := f.verbs("run")
	if len(runs) != len(steps) {
		t.Fatalf("run calls = %d, want %d (%v)", len(runs), len(steps), runs)
	}
	wantCwds := []string{alphaDir, betaDir, fallback}
	for i, want := range wantCwds {
		if cwd := argvCwd(t, runs[i]); cwd != want {
			t.Fatalf("run[%d] (%s) --cwd = %q, want %q", i, steps[i].item.Thread, cwd, want)
		}
	}
	// The two resolved workers landed in different repositories: the whole point
	// of the feature is that one worker no longer has to reach across repos.
	if alphaDir == betaDir {
		t.Fatal("the two projects resolved to the same directory; the fixture is wrong")
	}

	records := readFallbackRecords(t, reports)
	if len(records) != 1 {
		t.Fatalf("fallback records = %+v, want exactly one (only the unkeyed thread)", records)
	}
	if records[0].Thread != "feishu:om_9" || records[0].Reason != ReasonNoProjectKey {
		t.Fatalf("fallback record = %+v, want thread=feishu:om_9 reason=%s", records[0], ReasonNoProjectKey)
	}

	// Each thread kept its own worker, so a second message on a resolved thread
	// does not create another worker in an unresolved directory.
	t.Setenv("FAKE_PASEO_RUN_AGENT_ID", "ag_unused")
	h.enqueue(QueueItem{Channel: "skill", Thread: "skill:alpha:add-retry", MessageID: "m-skill-2", Text: "also add sorting"})
	if res := h.drain(); res.Applied != 1 {
		t.Fatalf("follow-up drain = %+v, want applied=1", res)
	}
	if got := len(f.verbs("run")); got != len(steps) {
		t.Fatalf("run calls = %d after a follow-up message, want %d", got, len(steps))
	}
	if got := len(readFallbackRecords(t, reports)); got != 1 {
		t.Fatalf("fallback records = %d after a follow-up message, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// The daemon wiring: `adbd route` resolves through adbd_projects_file and writes
// the fallback ledger under its own root
// ---------------------------------------------------------------------------

// TestWorkdirRouteCLIWiring pins the wiring itself: the CLI builds the dispatcher
// with the registry read from adbd_projects_file (default <root>/config/
// projects.json) and with <root>/.beacon/adbd-reports.jsonl as the ledger. The
// unit tests above would all pass even if route.go forgot one of the two, so this
// test drives the real command line.
func TestWorkdirRouteCLIWiring(t *testing.T) {
	root := t.TempDir()
	alphaDir := mustDir(t, filepath.Join(root, "vendor", "alpha"))
	writeProjectRegistry(t, root, []map[string]any{
		{"slug": "alpha", "repo": "vendor/alpha"},
	})
	fallback := mustDir(t, filepath.Join(root, "fallback"))
	f := newFakePaseo(t)

	beacon := filepath.Join(root, ".beacon")
	statePath := filepath.Join(beacon, "adbd-state.json")
	slowPath := filepath.Join(beacon, "adbd-slowpath.jsonl")
	configPath := filepath.Join(root, "daemon-config")
	configBody := strings.Join([]string{
		"adbd_allow_channels=skill,feishu",
		"adbd_paseo_bin=" + f.bin,
		"adbd_worker_workdir=" + fallback,
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	appendQueueItems(t, slowPath,
		QueueItem{Channel: "skill", Thread: "skill:alpha:add-retry", MessageID: "m-cli-1", Text: "add a retry to the webhook"},
		QueueItem{Channel: "feishu", Thread: "feishu:om_cli", MessageID: "m-cli-2", Text: "rename the loader module"},
	)

	t.Setenv("FAKE_PASEO_RUN_AGENT_ID", "ag_cli")
	var stdout, stderr strings.Builder
	code := run([]string{
		"route", "--once",
		"--root", root,
		"--state", statePath,
		"--slow-path", slowPath,
		"--config", configPath,
		"--trace", filepath.Join(beacon, decisionsFile),
		"--outcome", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("adbd route exited %d: %s %s", code, stdout.String(), stderr.String())
	}

	runs := f.verbs("run")
	if len(runs) != 2 {
		t.Fatalf("run calls = %d, want 2 (%v)", len(runs), runs)
	}
	if cwd := argvCwd(t, runs[0]); cwd != alphaDir {
		t.Fatalf("skill thread --cwd = %q, want %q", cwd, alphaDir)
	}
	if cwd := argvCwd(t, runs[1]); cwd != fallback {
		t.Fatalf("feishu thread --cwd = %q, want the fallback %q", cwd, fallback)
	}

	// The ledger the CLI wrote is the daemon one, and it holds exactly the one
	// fallback the unkeyed thread caused.
	records := readFallbackRecords(t, filepath.Join(beacon, reportsFile))
	if len(records) != 1 || records[0].Thread != "feishu:om_cli" || records[0].Reason != ReasonNoProjectKey {
		t.Fatalf("fallback records = %+v, want one no_project_key record for feishu:om_cli", records)
	}
}
