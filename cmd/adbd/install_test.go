package main

// install tests (feature install, v0.1.5): TC-IN-001 .. TC-IN-007.
//
// Two harness rules keep these tests honest:
//
//   - --home always points at a throwaway directory, so no test can ever write
//     into the real ~/Library/LaunchAgents.
//   - launchctl goes through the injected recorder, so no test runs launchctl.
//     The recorder both records and answers, which is how "loaded", "not loaded"
//     and "launchctl unavailable" become three exact, deterministic cases.
//
// The other hard property asserted throughout is a negative one: the LLM key
// lands in the plist and never in a single line of stdout or stderr (AC-004).

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// installHarness is a throwaway ADB root, a throwaway home and a launchctl
// recorder.
type installHarness struct {
	t    *testing.T
	root string
	home string
	// calls is every launchctl invocation, args joined with spaces.
	calls []string
	// respond lets a test take over the answer; nil means "launchctl works and
	// the job is loaded".
	respond func(args []string) launchctlResult
}

func newInstallHarness(t *testing.T) *installHarness {
	t.Helper()
	h := &installHarness{
		t:    t,
		root: filepath.Join(t.TempDir(), "adb-root"),
		home: filepath.Join(t.TempDir(), "home"),
	}
	if err := os.MkdirAll(h.root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.MkdirAll(h.home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", h.home)
	previous := launchctlCall
	launchctlCall = func(args ...string) launchctlResult {
		h.calls = append(h.calls, strings.Join(args, " "))
		if h.respond != nil {
			return h.respond(args)
		}
		if len(args) > 0 && args[0] == "print" {
			return launchctlResult{Available: true, Stdout: "gui/501/com.agent-delivery-bus.adbd = {\n\tlast exit code = 0\n}\n"}
		}
		return launchctlResult{Available: true}
	}
	t.Cleanup(func() { launchctlCall = previous })
	return h
}

// agentsDir is where the four plists are expected. It is never the real one.
func (h *installHarness) agentsDir() string {
	return filepath.Join(h.home, "Library", "LaunchAgents")
}

// run drives the real CLI entry point (so dispatch is covered too) with --home
// and --root pinned to the harness.
func (h *installHarness) run(flags ...string) (int, string, string) {
	h.t.Helper()
	args := append([]string{"install"}, flags...)
	args = append(args, "--root", h.root, "--home", h.home)
	var stdout, stderr strings.Builder
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// plists lists the *.plist files actually present, sorted.
func (h *installHarness) plists() []string {
	h.t.Helper()
	entries, err := os.ReadDir(h.agentsDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		h.t.Fatalf("read LaunchAgents: %v", err)
	}
	var paths []string
	for _, entry := range entries {
		paths = append(paths, filepath.Join(h.agentsDir(), entry.Name()))
	}
	sort.Strings(paths)
	return paths
}

// plistContents snapshots every plist, for byte-for-byte idempotence checks.
func (h *installHarness) plistContents() map[string]string {
	h.t.Helper()
	snapshot := map[string]string{}
	for _, path := range h.plists() {
		data, err := os.ReadFile(path)
		if err != nil {
			h.t.Fatalf("read %s: %v", path, err)
		}
		snapshot[path] = string(data)
	}
	return snapshot
}

func (h *installHarness) plistPath(suffix string) string {
	return filepath.Join(h.agentsDir(), InstallLabelPrefix+suffix+".plist")
}

func (h *installHarness) readPlist(suffix string) string {
	h.t.Helper()
	data, err := os.ReadFile(h.plistPath(suffix))
	if err != nil {
		h.t.Fatalf("read plist %s: %v", suffix, err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// plist inspection helpers (no third-party dependency: the files are ours)
// ---------------------------------------------------------------------------

var plistStringRE = regexp.MustCompile(`<string>([^<]*)</string>`)

// plistArguments returns the ProgramArguments array of one rendered plist.
func plistArguments(t *testing.T, body string) []string {
	t.Helper()
	const key = "<key>ProgramArguments</key>"
	start := strings.Index(body, key)
	if start < 0 {
		t.Fatalf("plist has no ProgramArguments:\n%s", body)
	}
	rest := body[start+len(key):]
	end := strings.Index(rest, "</array>")
	if end < 0 {
		t.Fatalf("plist has an unterminated ProgramArguments array:\n%s", body)
	}
	var args []string
	for _, match := range plistStringRE.FindAllStringSubmatch(rest[:end], -1) {
		args = append(args, match[1])
	}
	return args
}

// plistValue returns the <string>/<integer> value that follows one <key>.
func plistValue(t *testing.T, body, key string) string {
	t.Helper()
	marker := "<key>" + key + "</key>"
	index := strings.Index(body, marker)
	if index < 0 {
		t.Fatalf("plist has no %s key:\n%s", key, body)
	}
	rest := body[index+len(marker):]
	for _, open := range []string{"<string>", "<integer>"} {
		if at := strings.Index(rest, open); at >= 0 {
			tail := rest[at+len(open):]
			closeTag := strings.Replace(open, "<", "</", 1)
			if end := strings.Index(tail, closeTag); end >= 0 && !strings.Contains(rest[:at], "<key>") {
				return tail[:end]
			}
		}
	}
	t.Fatalf("plist key %s has no value:\n%s", key, body)
	return ""
}

// ---------------------------------------------------------------------------
// TC-IN-001 (AC-001): dry-run writes nothing and calls no launchctl
// ---------------------------------------------------------------------------

func TestInstallDryRunWritesNothing(t *testing.T) {
	h := newInstallHarness(t)

	code, stdout, stderr := h.run("--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	for _, job := range installJobs {
		label := InstallLabelPrefix + job.Suffix
		for _, want := range []string{
			"label=" + label,
			"path=" + h.plistPath(job.Suffix),
			"verb=" + job.verb(),
			"interval=" + itoa(job.Interval),
			"installed=no",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("dry-run output missing %q:\n%s", want, stdout)
			}
		}
	}
	if !strings.Contains(stdout, "wrote=0") {
		t.Fatalf("dry-run output = %q, want wrote=0", stdout)
	}

	// Nothing on disk, not even the directory.
	if paths := h.plists(); len(paths) != 0 {
		t.Fatalf("dry-run wrote %v, want no files", paths)
	}
	if _, err := os.Stat(h.agentsDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created %s (err=%v)", h.agentsDir(), err)
	}
	// And no launchctl at all.
	if len(h.calls) != 0 {
		t.Fatalf("dry-run called launchctl %v, want none", h.calls)
	}
}

// ---------------------------------------------------------------------------
// TC-IN-002 (AC-002): apply is idempotent
// ---------------------------------------------------------------------------

func TestInstallApplyIdempotent(t *testing.T) {
	h := newInstallHarness(t)

	code, stdout, stderr := h.run("--apply")
	if code != 0 {
		t.Fatalf("first apply exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("first apply stderr = %q, want empty", stderr)
	}
	if got := len(h.plists()); got != len(installJobs) {
		t.Fatalf("after first apply %d plists, want %d", got, len(installJobs))
	}
	first := h.plistContents()
	if !strings.Contains(stdout, "applied=4") {
		t.Fatalf("first apply stdout = %q, want applied=4", stdout)
	}
	firstCalls := append([]string(nil), h.calls...)

	// Second apply: same bytes, still exit 0, no error.
	code, stdout2, stderr2 := h.run("--apply")
	if code != 0 {
		t.Fatalf("second apply exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout2, stderr2)
	}
	if stderr2 != "" {
		t.Fatalf("second apply stderr = %q, want empty", stderr2)
	}
	second := h.plistContents()
	if len(first) != len(second) {
		t.Fatalf("plist count changed between applies: %d then %d", len(first), len(second))
	}
	for path, body := range first {
		if second[path] != body {
			t.Fatalf("plist %s changed between applies:\n--- first\n%s\n--- second\n%s", path, body, second[path])
		}
	}

	// Each apply bootouts then bootstraps each of the four jobs.
	secondCalls := h.calls[len(firstCalls):]
	if len(secondCalls) != 2*len(installJobs) {
		t.Fatalf("second apply made %d launchctl calls (%v), want %d", len(secondCalls), secondCalls, 2*len(installJobs))
	}
	for i, job := range installJobs {
		wantBootout := "bootout " + "gui/" + itoa(os.Getuid()) + "/" + InstallLabelPrefix + job.Suffix
		wantBootstrap := "bootstrap gui/" + itoa(os.Getuid()) + " " + h.plistPath(job.Suffix)
		if secondCalls[2*i] != wantBootout {
			t.Fatalf("call %d = %q, want %q", 2*i, secondCalls[2*i], wantBootout)
		}
		if secondCalls[2*i+1] != wantBootstrap {
			t.Fatalf("call %d = %q, want %q", 2*i+1, secondCalls[2*i+1], wantBootstrap)
		}
	}
	if !strings.Contains(stdout2, "loaded=yes") {
		t.Fatalf("second apply stdout = %q, want loaded=yes", stdout2)
	}
}

// ---------------------------------------------------------------------------
// TC-IN-003 (AC-003): all four plists describe the same binary, root and PATH
// ---------------------------------------------------------------------------

func TestInstallPlistShape(t *testing.T) {
	h := newInstallHarness(t)
	if code, stdout, stderr := h.run("--apply"); code != 0 {
		t.Fatalf("apply exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	wantPath := "/opt/homebrew/bin:/usr/local/bin:" + filepath.Join(h.home, ".local", "bin") + ":/usr/bin:/bin"

	for _, job := range installJobs {
		body := h.readPlist(job.Suffix)
		wantLabel := InstallLabelPrefix + job.Suffix
		if got := plistValue(t, body, "Label"); got != wantLabel {
			t.Fatalf("%s Label = %q, want %q", job.Suffix, got, wantLabel)
		}

		args := plistArguments(t, body)
		wantArgs := append(append([]string{exe}, job.Verb...), "--once", "--root", h.root)
		if strings.Join(args, "\x00") != strings.Join(wantArgs, "\x00") {
			t.Fatalf("%s ProgramArguments = %v, want %v", job.Suffix, args, wantArgs)
		}

		if got := plistValue(t, body, "StartInterval"); got != itoa(job.Interval) {
			t.Fatalf("%s StartInterval = %q, want %d", job.Suffix, got, job.Interval)
		}
		if got := plistValue(t, body, "WorkingDirectory"); got != h.root {
			t.Fatalf("%s WorkingDirectory = %q, want %q", job.Suffix, got, h.root)
		}
		if !strings.Contains(body, "<key>RunAtLoad</key>\n\t<true/>") {
			t.Fatalf("%s plist has no RunAtLoad true:\n%s", job.Suffix, body)
		}
		// The log name is the short job name (watchdog/route/github/reconcile):
		// the suffix already carries the "adbd-" prefix.
		short := strings.TrimPrefix(job.Suffix, "adbd-")
		wantOut := filepath.Join(h.root, ".beacon", "adbd-"+short+".out.log")
		wantErr := filepath.Join(h.root, ".beacon", "adbd-"+short+".err.log")
		if got := plistValue(t, body, "StandardOutPath"); got != wantOut {
			t.Fatalf("%s StandardOutPath = %q, want %q", job.Suffix, got, wantOut)
		}
		if got := plistValue(t, body, "StandardErrorPath"); got != wantErr {
			t.Fatalf("%s StandardErrorPath = %q, want %q", job.Suffix, got, wantErr)
		}
		if got := plistValue(t, body, "PATH"); got != wantPath {
			t.Fatalf("%s PATH = %q, want %q", job.Suffix, got, wantPath)
		}
		if !strings.Contains(wantPath, "/opt/homebrew/bin") || !strings.Contains(wantPath, filepath.Join(h.home, ".local", "bin")) {
			t.Fatalf("PATH %q is missing a required directory", wantPath)
		}
	}
}

// ---------------------------------------------------------------------------
// TC-IN-004 (AC-004): the key reaches the plist and never the output
// ---------------------------------------------------------------------------

func TestInstallSecretNotPrinted(t *testing.T) {
	const secret = "sk-install-test-4d61c0ffee9b8f2700aa"
	h := newInstallHarness(t)
	t.Setenv("ADBD_LLM_API_KEY", secret)

	outputs := []string{}
	for _, flags := range [][]string{{"--dry-run"}, {"--apply"}, {"--status"}, {"--uninstall"}} {
		code, stdout, stderr := h.run(flags...)
		if code != 0 {
			t.Fatalf("%v exit = %d, want 0 (stdout=%q stderr=%q)", flags, code, stdout, stderr)
		}
		outputs = append(outputs, stdout, stderr)
	}
	// Re-apply so the plist is on disk when the secret is checked.
	if code, stdout, stderr := h.run("--apply"); code != 0 {
		t.Fatalf("re-apply exit = %d (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	outputs = append(outputs, h.calls...)

	for _, out := range outputs {
		if strings.Contains(out, secret) {
			t.Fatalf("output leaked the API key:\n%s", out)
		}
	}

	body := h.readPlist("adbd")
	if !strings.Contains(body, "<key>ADBD_LLM_API_KEY</key>") {
		t.Fatalf("plist has no ADBD_LLM_API_KEY:\n%s", body)
	}
	if got := plistValue(t, body, "ADBD_LLM_API_KEY"); got != secret {
		t.Fatalf("plist ADBD_LLM_API_KEY = %q, want the environment value", got)
	}

	// With no key in the environment the key is absent, not empty.
	os.Unsetenv("ADBD_LLM_API_KEY")
	if code, stdout, stderr := h.run("--apply"); code != 0 {
		t.Fatalf("apply without key exit = %d (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if strings.Contains(h.readPlist("adbd"), "ADBD_LLM_API_KEY") {
		t.Fatalf("plist carries ADBD_LLM_API_KEY with no key in the environment")
	}
}

// ---------------------------------------------------------------------------
// TC-IN-005 (AC-005): uninstall is idempotent
// ---------------------------------------------------------------------------

func TestInstallUninstallIdempotent(t *testing.T) {
	h := newInstallHarness(t)
	if code, stdout, stderr := h.run("--apply"); code != 0 {
		t.Fatalf("apply exit = %d (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if got := len(h.plists()); got != len(installJobs) {
		t.Fatalf("after apply %d plists, want %d", got, len(installJobs))
	}

	code, stdout, stderr := h.run("--uninstall")
	if code != 0 {
		t.Fatalf("first uninstall exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("first uninstall stderr = %q, want empty", stderr)
	}
	if got := h.plists(); len(got) != 0 {
		t.Fatalf("after uninstall %v, want no plists", got)
	}
	if !strings.Contains(stdout, "uninstalled=4") {
		t.Fatalf("first uninstall stdout = %q, want uninstalled=4", stdout)
	}

	// Second uninstall: nothing to do, still a clean 0.
	code, stdout, stderr = h.run("--uninstall")
	if code != 0 {
		t.Fatalf("second uninstall exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("second uninstall stderr = %q, want empty", stderr)
	}
	if got := h.plists(); len(got) != 0 {
		t.Fatalf("second uninstall left %v", got)
	}
	if !strings.Contains(stdout, "uninstalled=0") {
		t.Fatalf("second uninstall stdout = %q, want uninstalled=0", stdout)
	}
	if !strings.Contains(stdout, "removed=no") {
		t.Fatalf("second uninstall stdout = %q, want removed=no", stdout)
	}
}

// ---------------------------------------------------------------------------
// TC-IN-006 (AC-006): status is read-only and reports the three facts
// ---------------------------------------------------------------------------

func TestInstallStatusReadOnly(t *testing.T) {
	h := newInstallHarness(t)

	// 1. Nothing installed: four "installed=no" lines, nothing created.
	code, stdout, stderr := h.run("--status")
	if code != 0 {
		t.Fatalf("status exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	if strings.Count(stdout, "installed=no") != len(installJobs) {
		t.Fatalf("status stdout = %q, want %d installed=no lines", stdout, len(installJobs))
	}
	if strings.Contains(stdout, "reason_code=") {
		t.Fatalf("status stdout = %q, want no reason_code when launchctl works", stdout)
	}
	if _, err := os.Stat(h.agentsDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("status created %s (err=%v)", h.agentsDir(), err)
	}

	// 2. Installed and loaded: installed/loaded/last_exit per job, files intact.
	if code, stdout, stderr := h.run("--apply"); code != 0 {
		t.Fatalf("apply exit = %d (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	before := h.plistContents()
	code, stdout, stderr = h.run("--status")
	if code != 0 {
		t.Fatalf("status exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	for _, job := range installJobs {
		want := "label=" + InstallLabelPrefix + job.Suffix
		if !strings.Contains(stdout, want) {
			t.Fatalf("status output missing %q:\n%s", want, stdout)
		}
	}
	if strings.Count(stdout, "installed=yes") != len(installJobs) {
		t.Fatalf("status stdout = %q, want %d installed=yes", stdout, len(installJobs))
	}
	if strings.Count(stdout, "loaded=yes") != len(installJobs) {
		t.Fatalf("status stdout = %q, want %d loaded=yes", stdout, len(installJobs))
	}
	if strings.Count(stdout, "last_exit=0") != len(installJobs) {
		t.Fatalf("status stdout = %q, want %d last_exit=0", stdout, len(installJobs))
	}
	after := h.plistContents()
	if len(before) != len(after) {
		t.Fatalf("status changed the plist count: %d -> %d", len(before), len(after))
	}
	for path, body := range before {
		if after[path] != body {
			t.Fatalf("status rewrote %s", path)
		}
	}

	// 3. Installed but launchd refuses: loaded=no is a real answer, not an error.
	h.respond = func(args []string) launchctlResult {
		return launchctlResult{Available: true, Err: errors.New("exit status 113")}
	}
	code, stdout, stderr = h.run("--status")
	if code != 0 {
		t.Fatalf("status (not loaded) exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	if strings.Count(stdout, "loaded=no") != len(installJobs) {
		t.Fatalf("status stdout = %q, want %d loaded=no", stdout, len(installJobs))
	}
	if strings.Contains(stdout, "reason_code=") {
		t.Fatalf("status stdout = %q, want no reason_code when launchctl answers", stdout)
	}
	if got := h.plistContents(); len(got) != len(before) {
		t.Fatalf("status changed the files: %d -> %d", len(before), len(got))
	}

	// 4. launchctl unavailable: its own reason code, still a read-only 0.
	h.respond = func(args []string) launchctlResult {
		return launchctlResult{Available: false, Err: errors.New("exec: launchctl: not found")}
	}
	code, stdout, stderr = h.run("--status")
	if code != 0 {
		t.Fatalf("status (unavailable) exit = %d, want 0 (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, "reason_code="+ReasonInstallLaunchctlUnavailable) {
		t.Fatalf("status stdout = %q, want reason_code=%s", stdout, ReasonInstallLaunchctlUnavailable)
	}
	if strings.Count(stdout, "loaded=unknown") != len(installJobs) {
		t.Fatalf("status stdout = %q, want %d loaded=unknown", stdout, len(installJobs))
	}
	if got := h.plistContents(); len(got) != len(before) {
		t.Fatalf("status changed the files: %d -> %d", len(before), len(got))
	}
}

// ---------------------------------------------------------------------------
// TC-IN-007 (AC-007): a failed write is loud and is never a success
// ---------------------------------------------------------------------------

func TestInstallWriteFailureIsLoud(t *testing.T) {
	h := newInstallHarness(t)
	if err := os.MkdirAll(h.agentsDir(), 0o755); err != nil {
		t.Fatalf("mkdir LaunchAgents: %v", err)
	}
	// Occupy the first plist path with a directory: the rename onto it fails for
	// every user, root included, so the test is deterministic.
	first := h.plistPath(installJobs[0].Suffix)
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatalf("occupy first plist path: %v", err)
	}

	code, stdout, stderr := h.run("--apply")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "reason_code="+ReasonInstallFailed) {
		t.Fatalf("output = %q, want reason_code=%s", combined, ReasonInstallFailed)
	}
	if !strings.Contains(combined, first) {
		t.Fatalf("output = %q, want the first failing path %q", combined, first)
	}
	if strings.Contains(stdout, "applied=") || strings.Contains(stdout, "loaded=") {
		t.Fatalf("failed apply reported success: %q", stdout)
	}
	// No launchctl: the write phase failed before launchd was touched.
	if len(h.calls) != 0 {
		t.Fatalf("failed apply called launchctl %v, want none", h.calls)
	}
	// And no partial install left behind.
	for _, job := range installJobs[1:] {
		if _, err := os.Stat(h.plistPath(job.Suffix)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("partial install left %s (err=%v)", h.plistPath(job.Suffix), err)
		}
	}
}

// itoa keeps the assertions free of strconv noise.
func itoa(value int) string { return strconv.Itoa(value) }
