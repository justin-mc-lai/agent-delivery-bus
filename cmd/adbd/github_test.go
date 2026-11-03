package main

// GitHub issue source tests (feature github-issue, v0.1.5): TC-GH-001 .. TC-GH-006.
//
// Every test runs against a temp root, a REAL executable shell script standing in
// for gh (so the process boundary -- argv, exit status, environment -- is what is
// actually under test) and a stub Dispatcher. The real gh binary and the network
// are never touched: adbd_gh_bin always points at the script.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// harness: temp root + fake gh executable + stub runner + injected clock
// ---------------------------------------------------------------------------

type ghHarness struct {
	t          *testing.T
	root       string
	configPath string
	statePath  string
	slowPath   string
	argvLog    string
	envLog     string
	jsonDir    string
	ghBin      string
	clock      *fakeClock
	runner     *stubRunner
}

func newGHHarness(t *testing.T) *ghHarness {
	t.Helper()
	root := t.TempDir()
	beacon := filepath.Join(root, ".beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		t.Fatalf("mkdir .beacon: %v", err)
	}
	h := &ghHarness{
		t:          t,
		root:       root,
		configPath: filepath.Join(beacon, "config"),
		statePath:  filepath.Join(beacon, "adbd-state.json"),
		slowPath:   filepath.Join(beacon, "adbd-slowpath.jsonl"),
		argvLog:    filepath.Join(root, "gh-argv.log"),
		envLog:     filepath.Join(root, "gh-env.log"),
		jsonDir:    filepath.Join(root, "gh-json"),
		runner:     &stubRunner{},
	}
	if err := os.MkdirAll(h.jsonDir, 0o755); err != nil {
		t.Fatalf("mkdir json dir: %v", err)
	}
	h.ghBin = writeFakeGH(t, h.argvLog, h.envLog, h.jsonDir)

	start, err := time.Parse(time.RFC3339, baseTime)
	if err != nil {
		t.Fatalf("parse base time: %v", err)
	}
	h.clock = &fakeClock{t: start}

	// Stub Dispatcher + injected clock: intake hands text to the stub, never to
	// paseo, and dedupe timestamps are exact.
	prevCoreFor := githubCoreFor
	githubCoreFor = func(cf *commonFlags) (*Core, error) {
		core, err := cf.core()
		if err != nil {
			return nil, err
		}
		core.Runner = h.runner
		core.Now = h.clock.now
		return core, nil
	}
	t.Cleanup(func() { githubCoreFor = prevCoreFor })
	return h
}

// writeFakeGH writes a real #!/bin/sh gh stand-in. It records every argv element
// and its own environment per call, then prints the JSON fixture belonging to the
// repo named after -R (one file per repo, so multi-repo polls are distinguishable).
func writeFakeGH(t *testing.T, argvLog, envLog, jsonDir string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "fake-gh.sh")
	body := "#!/bin/sh\n" +
		"repo=''\n" +
		"prev=''\n" +
		"for a in \"$@\"; do\n" +
		"  printf 'ARG %s\\n' \"$a\" >> " + shellQuote(argvLog) + "\n" +
		"  if [ \"$prev\" = '-R' ]; then repo=\"$a\"; fi\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		"printf '%s\\n' '--- call ---' >> " + shellQuote(argvLog) + "\n" +
		"env >> " + shellQuote(envLog) + "\n" +
		"printf '%s\\n' '--- call ---' >> " + shellQuote(envLog) + "\n" +
		"fixture=\"" + strings.TrimSuffix(jsonDir, "/") + "/$(printf '%s' \"$repo\" | tr '/' '_').json\"\n" +
		"if [ -f \"$fixture\" ]; then cat \"$fixture\"; else printf '[]\\n'; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	return script
}

// writeScript installs a custom gh stand-in (used by the failure-mode tests).
func writeScript(t *testing.T, name, body string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return script
}

// configure writes .beacon/config: the fake gh binary plus whatever the test needs.
func (h *ghHarness) configure(extra ...string) {
	h.t.Helper()
	lines := append([]string{"adbd_gh_bin=" + h.ghBin}, extra...)
	if err := os.WriteFile(h.configPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

// setIssues writes the JSON fixture gh will print for one repo.
func (h *ghHarness) setIssues(repo string, issues ...string) {
	h.t.Helper()
	path := filepath.Join(h.jsonDir, strings.ReplaceAll(repo, "/", "_")+".json")
	if err := os.WriteFile(path, []byte("["+strings.Join(issues, ",")+"]\n"), 0o644); err != nil {
		h.t.Fatalf("write issues for %s: %v", repo, err)
	}
}

// poll runs `adbd poll-github --once --root <root>` and returns exit, stdout, stderr.
func (h *ghHarness) poll(args ...string) (int, string, string) {
	h.t.Helper()
	var out, errOut bytes.Buffer
	argv := append([]string{"poll-github", "--once", "--root", h.root}, args...)
	code := run(argv, &out, &errOut)
	return code, out.String(), errOut.String()
}

// ghCalls returns the argv of every fake-gh invocation, in order.
func (h *ghHarness) ghCalls() [][]string {
	h.t.Helper()
	data, err := os.ReadFile(h.argvLog)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read argv log: %v", err)
	}
	var calls [][]string
	var current []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "--- call ---" {
			calls = append(calls, current)
			current = nil
			continue
		}
		if line == "" {
			continue
		}
		current = append(current, strings.TrimPrefix(line, "ARG "))
	}
	if len(current) > 0 {
		calls = append(calls, current)
	}
	return calls
}

// envDumps returns the environment the fake gh saw, one entry per call.
func (h *ghHarness) envDumps() []string {
	h.t.Helper()
	data, err := os.ReadFile(h.envLog)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read env log: %v", err)
	}
	var out []string
	for _, chunk := range strings.Split(string(data), "--- call ---\n") {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		out = append(out, chunk)
	}
	return out
}

// envValue pulls one variable out of a dumped environment; count tells how many
// times the name appeared (a duplicate would mean the proxy was appended, not
// replaced).
func envValue(dump, key string) (string, int) {
	value, count := "", 0
	for _, line := range strings.Split(dump, "\n") {
		name, rest, ok := strings.Cut(line, "=")
		if !ok || name != key {
			continue
		}
		value = rest
		count++
	}
	return value, count
}

// slowPaths reads the slow-path queue this poll filled.
func (h *ghHarness) slowPaths() []slowPathRecord {
	h.t.Helper()
	data, err := os.ReadFile(h.slowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read slow path: %v", err)
	}
	var out []slowPathRecord
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec slowPathRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			h.t.Fatalf("parse slow-path line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// ghIssueJSON builds one issue in the shape `gh issue list --json` prints.
func ghIssueJSON(number int, title, body, updatedAt string, labels ...string) string {
	rendered := make([]string, 0, len(labels))
	for _, label := range labels {
		rendered = append(rendered, fmt.Sprintf(`{"name":%s}`, strconv.Quote(label)))
	}
	return fmt.Sprintf(`{"number":%d,"title":%s,"body":%s,"updatedAt":%s,"labels":[%s]}`,
		number, strconv.Quote(title), strconv.Quote(body), strconv.Quote(updatedAt),
		strings.Join(rendered, ","))
}

// ghPollFields splits the one-line text outcome into its key=value pairs.
func ghPollFields(t *testing.T, out string) map[string]string {
	t.Helper()
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(out), "\n", 2)[0])
	fields := map[string]string{}
	for _, part := range strings.Fields(line) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		fields[key] = value
	}
	return fields
}

// ---------------------------------------------------------------------------
// TC-GH-001: configured repos are polled with the frozen argv; no config is calm
// ---------------------------------------------------------------------------

func TestGitHubPollMapsConfiguredRepos(t *testing.T) {
	h := newGHHarness(t)
	h.configure("adbd_github_repos=o/one, o/two")
	h.setIssues("o/one",
		ghIssueJSON(1, "first", "body one", "2026-02-06T08:00:00Z"),
		ghIssueJSON(2, "second", "body two", "2026-02-06T08:10:00Z"),
	)
	h.setIssues("o/two",
		ghIssueJSON(9, "third", "body three", "2026-02-06T08:20:00Z"),
	)

	code, out, errOut := h.poll()
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	fields := ghPollFields(t, out)
	if fields["polled"] != "3" || fields["new"] != "3" || fields["deduped"] != "0" {
		t.Fatalf("outcome = %q, want polled=3 new=3 deduped=0", strings.TrimSpace(out))
	}

	calls := h.ghCalls()
	if len(calls) != 2 {
		t.Fatalf("gh called %d time(s), want 2: %v", len(calls), calls)
	}
	want := [][]string{
		{"issue", "list", "-R", "o/one", "--state", "open", "--limit", "50", "--json", "number,title,body,updatedAt,labels"},
		{"issue", "list", "-R", "o/two", "--state", "open", "--limit", "50", "--json", "number,title,body,updatedAt,labels"},
	}
	for i := range want {
		if strings.Join(calls[i], " ") != strings.Join(want[i], " ") {
			t.Fatalf("gh call %d = %v, want %v", i, calls[i], want[i])
		}
	}

	// AC-001: no repo configured is calm, but never silent, and gh is not spawned.
	empty := newGHHarness(t)
	empty.configure()
	code, out, errOut = empty.poll()
	if code != 0 {
		t.Fatalf("exit = %d, want 0 for an unconfigured poll (stderr: %s)", code, errOut)
	}
	if got, want := strings.TrimSpace(out), "polled=0 reason_code=repos_not_configured"; got != want {
		t.Fatalf("outcome = %q, want %q", got, want)
	}
	if calls := empty.ghCalls(); len(calls) != 0 {
		t.Fatalf("gh was called with no repos configured: %v", calls)
	}
}

// ---------------------------------------------------------------------------
// TC-GH-002: thread / message-id shapes, and updatedAt idempotency
// ---------------------------------------------------------------------------

func TestGitHubThreadAndDedup(t *testing.T) {
	h := newGHHarness(t)
	h.configure("adbd_github_repos=o/r")
	h.setIssues("o/r", ghIssueJSON(12, "Fix the thing", "steps to reproduce", "2026-02-06T09:00:00Z"))

	code, out, errOut := h.poll()
	if code != 0 {
		t.Fatalf("first poll exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if fields := ghPollFields(t, out); fields["new"] != "1" || fields["deduped"] != "0" {
		t.Fatalf("first poll outcome = %q, want new=1 deduped=0", strings.TrimSpace(out))
	}

	queued := h.slowPaths()
	if len(queued) != 1 {
		t.Fatalf("slow path has %d record(s), want 1", len(queued))
	}
	if got, want := queued[0].Thread, "gh:o/r#12"; got != want {
		t.Fatalf("thread = %q, want %q", got, want)
	}
	if got, want := queued[0].MessageID, "gh:12:2026-02-06T09:00:00Z"; got != want {
		t.Fatalf("message-id = %q, want %q", got, want)
	}
	if got, want := queued[0].Channel, DefaultGitHubChannel; got != want {
		t.Fatalf("channel = %q, want %q", got, want)
	}

	// Unchanged updatedAt: the very same event, so the kernel dedupes it.
	code, out, errOut = h.poll()
	if code != 0 {
		t.Fatalf("second poll exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if fields := ghPollFields(t, out); fields["new"] != "0" || fields["deduped"] != "1" {
		t.Fatalf("second poll outcome = %q, want new=0 deduped=1", strings.TrimSpace(out))
	}
	if queued := h.slowPaths(); len(queued) != 1 {
		t.Fatalf("an unchanged issue produced a second slow-path record: %v", queued)
	}

	// A new comment moves updatedAt, so the same thread becomes a new event.
	h.setIssues("o/r", ghIssueJSON(12, "Fix the thing", "steps to reproduce", "2026-02-06T10:30:00Z"))
	code, out, errOut = h.poll()
	if code != 0 {
		t.Fatalf("third poll exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if fields := ghPollFields(t, out); fields["new"] != "1" || fields["deduped"] != "0" {
		t.Fatalf("third poll outcome = %q, want new=1 deduped=0", strings.TrimSpace(out))
	}
	queued = h.slowPaths()
	if len(queued) != 2 {
		t.Fatalf("slow path has %d record(s), want 2", len(queued))
	}
	if got, want := queued[1].MessageID, "gh:12:2026-02-06T10:30:00Z"; got != want {
		t.Fatalf("updated message-id = %q, want %q", got, want)
	}
	if queued[0].Thread != queued[1].Thread {
		t.Fatalf("threads differ across polls: %q vs %q", queued[0].Thread, queued[1].Thread)
	}
}

// ---------------------------------------------------------------------------
// TC-GH-003: open-only query, label whitelist, and no GitHub write path
// ---------------------------------------------------------------------------

func TestGitHubOpenOnlyAndLabelFilter(t *testing.T) {
	h := newGHHarness(t)
	h.configure("adbd_github_repos=o/r", "adbd_github_labels=bug")
	h.setIssues("o/r",
		ghIssueJSON(1, "keep me", "", "2026-02-06T09:00:00Z", "bug"),
		ghIssueJSON(2, "drop me", "", "2026-02-06T09:00:00Z", "docs"),
		ghIssueJSON(3, "drop me too", "", "2026-02-06T09:00:00Z"),
	)

	code, out, errOut := h.poll()
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if fields := ghPollFields(t, out); fields["polled"] != "1" || fields["new"] != "1" {
		t.Fatalf("outcome = %q, want polled=1 new=1 (label filter)", strings.TrimSpace(out))
	}
	queued := h.slowPaths()
	if len(queued) != 1 || queued[0].Thread != "gh:o/r#1" {
		t.Fatalf("slow path = %v, want exactly gh:o/r#1", queued)
	}

	// AC-003: the query itself is open-only, and no argv ever writes to GitHub.
	calls := h.ghCalls()
	if len(calls) != 1 {
		t.Fatalf("gh called %d time(s), want 1", len(calls))
	}
	argv := strings.Join(calls[0], " ")
	if !strings.Contains(argv, "--state open") {
		t.Fatalf("gh argv %q does not ask for open issues only", argv)
	}
	for _, call := range calls {
		if len(call) < 2 || call[0] != "issue" || call[1] != "list" {
			t.Fatalf("gh argv %v is not a read-only `issue list` call", call)
		}
	}

	// The source itself builds no write subcommand: no create/comment/edit/close,
	// no `gh api`, no HTTP method. Scanner over the Go source, not over prose.
	source, err := os.ReadFile("github.go")
	if err != nil {
		t.Fatalf("read github.go: %v", err)
	}
	for _, forbidden := range []string{
		`"issue", "create"`, `"issue", "comment"`, `"issue", "edit"`, `"issue", "close"`,
		`"issue", "reopen"`, `"pr", "create"`, `"api"`, `"-X", "POST"`, `"--method"`,
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("github.go builds a GitHub write call: found %s", forbidden)
		}
	}
	if !strings.Contains(string(source), `"issue", "list"`) {
		t.Fatal("github.go no longer builds the read-only `issue list` call")
	}
}

// ---------------------------------------------------------------------------
// TC-GH-004: proxy injection into the gh child environment
// ---------------------------------------------------------------------------

func TestGitHubProxyInjectedIntoGhEnv(t *testing.T) {
	t.Run("configured proxy replaces inherited value", func(t *testing.T) {
		h := newGHHarness(t)
		h.configure("adbd_github_repos=o/r", "adbd_github_proxy=http://127.0.0.1:7890")
		h.setIssues("o/r", ghIssueJSON(1, "t", "", "2026-02-06T09:00:00Z"))

		// The parent carries a different proxy and a marker the child must inherit.
		t.Setenv("HTTPS_PROXY", "http://inherited.example:1111")
		t.Setenv("HTTP_PROXY", "http://inherited.example:1111")
		t.Setenv("ADBD_GITHUB_TEST_MARKER", "inherit-me")

		if code, _, errOut := h.poll(); code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
		}
		dumps := h.envDumps()
		if len(dumps) != 1 {
			t.Fatalf("env dump count = %d, want 1", len(dumps))
		}
		for _, key := range []string{"HTTPS_PROXY", "HTTP_PROXY"} {
			value, count := envValue(dumps[0], key)
			if value != "http://127.0.0.1:7890" {
				t.Fatalf("%s = %q, want %q", key, value, "http://127.0.0.1:7890")
			}
			if count != 1 {
				t.Fatalf("%s appears %d time(s), want exactly 1 (replace, not append)", key, count)
			}
		}
		if strings.Contains(dumps[0], "inherited.example") {
			t.Fatalf("the inherited proxy survived alongside the configured one")
		}
		if value, _ := envValue(dumps[0], "ADBD_GITHUB_TEST_MARKER"); value != "inherit-me" {
			t.Fatalf("marker = %q, want the parent environment to be inherited", value)
		}
	})

	t.Run("no proxy configured inherits the parent environment", func(t *testing.T) {
		h := newGHHarness(t)
		h.configure("adbd_github_repos=o/r")
		h.setIssues("o/r", ghIssueJSON(1, "t", "", "2026-02-06T09:00:00Z"))
		t.Setenv("HTTPS_PROXY", "http://inherited.example:1111")
		t.Setenv("ADBD_GITHUB_TEST_MARKER", "inherit-me")

		if code, _, errOut := h.poll(); code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
		}
		dumps := h.envDumps()
		if len(dumps) != 1 {
			t.Fatalf("env dump count = %d, want 1", len(dumps))
		}
		if value, count := envValue(dumps[0], "HTTPS_PROXY"); value != "http://inherited.example:1111" || count != 1 {
			t.Fatalf("HTTPS_PROXY = %q (x%d), want the inherited value once", value, count)
		}
		if value, _ := envValue(dumps[0], "ADBD_GITHUB_TEST_MARKER"); value != "inherit-me" {
			t.Fatalf("marker = %q, want the parent environment to be inherited", value)
		}
	})
}

// ---------------------------------------------------------------------------
// TC-GH-005: the three gh failure modes, each with its own reason code and exit 2
// ---------------------------------------------------------------------------

func TestGitHubGhErrorCodes(t *testing.T) {
	t.Run("gh_not_found", func(t *testing.T) {
		h := newGHHarness(t)
		h.ghBin = filepath.Join(h.root, "no-such-gh-binary")
		h.configure("adbd_github_repos=o/r")

		code, out, errOut := h.poll()
		if code != 2 {
			t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, out, errOut)
		}
		if got, want := strings.TrimSpace(out), "polled=0 reason_code=gh_not_found"; got != want {
			t.Fatalf("outcome = %q, want %q", got, want)
		}
		if !strings.Contains(errOut, "gh_not_found") {
			t.Fatalf("stderr = %q, want the reason code spelled out", errOut)
		}
	})

	t.Run("gh_failed quotes the first 200 characters of stderr", func(t *testing.T) {
		h := newGHHarness(t)
		long := strings.Repeat("x", 260)
		h.ghBin = writeScript(t, "failing-gh.sh", "printf '%s\\n' '"+long+"' >&2\nexit 3\n")
		h.configure("adbd_github_repos=o/r")

		code, out, errOut := h.poll()
		if code != 2 {
			t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, out, errOut)
		}
		if got, want := strings.TrimSpace(out), "polled=0 reason_code=gh_failed"; got != want {
			t.Fatalf("outcome = %q, want %q", got, want)
		}
		if !strings.Contains(errOut, long[:200]) {
			t.Fatalf("stderr = %q, want the first 200 characters of gh's stderr", errOut)
		}
		if strings.Contains(errOut, long[:201]) {
			t.Fatalf("stderr carries more than 200 characters of gh's stderr")
		}
	})

	t.Run("gh_output_invalid", func(t *testing.T) {
		h := newGHHarness(t)
		h.ghBin = writeScript(t, "garbage-gh.sh", "printf '%s\\n' 'not a json array'\nexit 0\n")
		h.configure("adbd_github_repos=o/r")

		code, out, errOut := h.poll()
		if code != 2 {
			t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, out, errOut)
		}
		if got, want := strings.TrimSpace(out), "polled=0 reason_code=gh_output_invalid"; got != want {
			t.Fatalf("outcome = %q, want %q", got, want)
		}
		if !strings.Contains(errOut, "gh_output_invalid") {
			t.Fatalf("stderr = %q, want the reason code spelled out", errOut)
		}
	})
}

// ---------------------------------------------------------------------------
// TC-GH-006: one pass end to end -- counts out, guarded text reaches a worker
// ---------------------------------------------------------------------------

func TestGitHubPollEndToEnd(t *testing.T) {
	h := newGHHarness(t)
	// AC-010 trusts github by default; this test pins the strict setting
	// (adbd_trusted_channels=) because it is about the untrusted path: an issue
	// body from a stranger must reach the worker behind the guard line.
	h.configure("adbd_github_repos=o/r", "adbd_trusted_channels=")
	h.setIssues("o/r", ghIssueJSON(7, "Fix the thing", "steps to reproduce", "2026-02-06T09:00:00Z"))

	code, out, errOut := h.poll()
	if code != 0 {
		t.Fatalf("poll exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if fields := ghPollFields(t, out); fields["polled"] != "1" || fields["new"] != "1" || fields["deduped"] != "0" {
		t.Fatalf("outcome = %q, want polled=1 new=1 deduped=0", strings.TrimSpace(out))
	}

	// The router drains the queue and hands the text to a worker: it must arrive
	// as untrusted data, not as instructions.
	textLog := filepath.Join(h.root, "worker-text.log")
	runnerBin := writeTextLogRunner(t, textLog)
	var routeOut, routeErr bytes.Buffer
	if code := run([]string{"route", "--once", "--root", h.root, "--runner-bin", runnerBin}, &routeOut, &routeErr); code != 0 {
		t.Fatalf("route exit = %d, want 0 (stdout: %s, stderr: %s)", code, routeOut.String(), routeErr.String())
	}
	delivered := readTextLog(t, textLog)
	if len(delivered) != 1 {
		t.Fatalf("worker received %d message(s), want 1: %v", len(delivered), delivered)
	}
	if !strings.HasPrefix(delivered[0], UntrustedGuardLine) {
		t.Fatalf("worker text = %q, want it to start with %s", delivered[0], UntrustedGuardLine)
	}
	if !strings.Contains(delivered[0], "#7 Fix the thing") || !strings.Contains(delivered[0], "steps to reproduce") {
		t.Fatalf("worker text = %q, want the issue number, title and body", delivered[0])
	}

	// Second pass, JSON outcome: nothing new, everything deduped.
	code, out, errOut = h.poll("--outcome", "json")
	if code != 0 {
		t.Fatalf("second poll exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	var res GitHubPollResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &res); err != nil {
		t.Fatalf("parse JSON outcome %q: %v", strings.TrimSpace(out), err)
	}
	if res.Polled != 1 || res.New != 0 || res.Deduped != 1 {
		t.Fatalf("JSON outcome = %+v, want polled=1 new=0 deduped=1", res)
	}
	if len(res.Events) != 1 || res.Events[0].Route != RouteDeduped || res.Events[0].Thread != "gh:o/r#7" {
		t.Fatalf("JSON events = %+v, want one deduped gh:o/r#7 event", res.Events)
	}
}
