// One-command install / uninstall / status for the four resident adbd jobs
// (feature install, v0.1.5).
//
// The four jobs are fixed data, not configuration: the night watchman, the
// slow-path router, the GitHub poller and the reconciler each run once and exit,
// and launchd is what makes them resident. Writing that out by hand is four
// plists per machine, so this file turns it into one command:
//
//	adbd install --dry-run     say exactly what would be written, write nothing
//	adbd install --apply       write four plists, then bootout+bootstrap each
//	adbd install --uninstall   bootout and delete the four plists
//	adbd install --status      read-only: installed / loaded / last_exit
//
// Three properties are load-bearing and every one of them is a negative one:
//
//   - --dry-run touches nothing: no file, no directory, no launchctl call
//     (AC-001). Planning an install can never install anything.
//   - The LLM key travels from the environment into the plist and nowhere else:
//     no line this command prints ever contains it (AC-004), so a screenshot of
//     a terminal or a pasted report cannot leak it.
//   - A write that fails stops the whole install with exit 2, reason_code
//     install_failed and the first failing path (AC-007). Half an install is
//     never reported as a success.
//
// os/exec appears in exactly one place here, behind the launchctlCall seam, so
// tests inject a recorder and never touch the real launchd or the real
// ~/Library/LaunchAgents (--home points at a temp directory instead).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// InstallLabelPrefix plus a job suffix is the launchd job name. It carries the
// product name, not the machine name, so the plists stay portable.
const InstallLabelPrefix = "com.agent-delivery-bus."

// Reason codes are stable machine tokens, never prose.
const (
	// ReasonInstallFailed means at least one plist could not be written (or
	// removed) and the command stopped there (AC-007).
	ReasonInstallFailed = "install_failed"
	// ReasonInstallLaunchctlUnavailable is reused from the agent surface:
	// launchctl missing is a different operator fact from "not loaded".
	ReasonInstallLaunchctlUnavailable = ReasonLaunchctlUnavailable
)

// The four actions. Exactly one is required per invocation; apply and dry_run in
// the same call is an illegal migration (truth.md 禁止的迁移).
const (
	InstallActionDryRun    = "dry_run"
	InstallActionApply     = "apply"
	InstallActionUninstall = "uninstall"
	InstallActionStatus    = "status"
)

// The PATH a job gets. launchd hands a job a minimal environment, so the two
// directories the paid-for toolchain lives in are spelled out: Homebrew on Apple
// silicon first, Homebrew on Intel next, then the user's own bin (AC-003).
const (
	installPathHead = "/opt/homebrew/bin:/usr/local/bin:"
	installPathTail = ":/usr/bin:/bin"
)

// UnknownLastExit is what --status reports when launchd does not tell us an exit
// code (job never ran, or launchctl unavailable). A guess would be a lie.
const UnknownLastExit = "unknown"

// installJob is one resident job: a label suffix, the adbd subcommand it runs
// and how often launchd should start it.
type installJob struct {
	Suffix   string
	Verb     []string
	Interval int // seconds, written as StartInterval
}

// verb renders the subcommand for humans ("watchdog", "poll-github").
func (j installJob) verb() string { return strings.Join(j.Verb, " ") }

// installJobs is the frozen task list from truth.md 任务清单. Order is the order
// the commands print and write in, so output is stable.
var installJobs = []installJob{
	{Suffix: "adbd", Verb: []string{"watchdog"}, Interval: 600},
	{Suffix: "adbd-route", Verb: []string{"route"}, Interval: 120},
	{Suffix: "adbd-github", Verb: []string{"poll-github"}, Interval: 120},
	{Suffix: "adbd-reconcile", Verb: []string{"reconcile"}, Interval: 120},
}

// installPlan is everything the four plists are built from: the binary, the ADB
// root, the home directory and the launchd domain. It is pure data and never
// holds a handle to launchd.
type installPlan struct {
	Root    string
	Home    string
	Program string
	UID     int
	Jobs    []installJob
}

// newInstallPlan resolves the flags into a plan. An empty home falls back to the
// current user's home; an empty program falls back to this executable, which is
// what makes the written jobs point at the binary the operator just ran.
func newInstallPlan(root, home, program string) installPlan {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if strings.TrimSpace(home) == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	if abs, err := filepath.Abs(home); err == nil {
		home = abs
	}
	if strings.TrimSpace(program) == "" {
		program = "adbd"
		if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
			program = exe
		}
	}
	return installPlan{Root: root, Home: home, Program: program, UID: os.Getuid(), Jobs: installJobs}
}

// Label is the launchd job name for one job.
//
// An alternate --home gets its own label namespace: launchd keys jobs by label,
// not by plist path, so writing to a temp home under the production label would
// replace the real job with one pointing at a temporary file.
func (p installPlan) Label(j installJob) string {
	if p.customHome() {
		return InstallLabelPrefix + "dev-" + j.Suffix
	}
	return InstallLabelPrefix + j.Suffix
}

// customHome reports whether --home moved the plists out of ~/Library.
func (p installPlan) customHome() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return filepath.Clean(p.Home) != filepath.Clean(home)
}

// LaunchAgentsDir is where the plists land. --home can point it anywhere, which
// is how tests keep their hands off the real ~/Library/LaunchAgents.
func (p installPlan) LaunchAgentsDir() string {
	return filepath.Join(p.Home, "Library", "LaunchAgents")
}

// PlistPath is the file one job is written to.
func (p installPlan) PlistPath(j installJob) string {
	return filepath.Join(p.LaunchAgentsDir(), p.Label(j)+".plist")
}

// LogPath is where launchd sends one job's output: <root>/.beacon, next to every
// other adbd ledger.
func (p installPlan) LogPath(j installJob, kind string) string {
	name := strings.TrimPrefix(j.Suffix, "adbd-")
	return filepath.Join(p.Root, ".beacon", "adbd-"+name+"."+kind+".log")
}

// Domain is the launchd domain the jobs live in: gui/<uid>.
func (p installPlan) Domain() string { return "gui/" + strconv.Itoa(p.UID) }

// Target is the domain-qualified job name, what bootout and print take.
func (p installPlan) Target(j installJob) string { return p.Domain() + "/" + p.Label(j) }

// Arguments is the full ProgramArguments array: this binary, the subcommand,
// --once (there is no resident mode), and the one shared --root (AC-003).
func (p installPlan) Arguments(j installJob) []string {
	args := []string{p.Program}
	args = append(args, j.Verb...)
	args = append(args, "--once", "--root", p.Root)
	return args
}

// PathValue is the PATH written into every plist.
func (p installPlan) PathValue() string {
	return installPathHead + filepath.Join(p.Home, ".local", "bin") + installPathTail
}

// environment is the EnvironmentVariables block as ordered pairs so the plist is
// byte-stable between runs (a map would shuffle it).
//
// ADBD_LLM_API_KEY is copied from the environment only when it is set: an unset
// key writes no key at all rather than an empty one.
func (p installPlan) environment() [][2]string {
	env := [][2]string{{"PATH", p.PathValue()}}
	if key := strings.TrimSpace(os.Getenv("ADBD_LLM_API_KEY")); key != "" {
		env = append(env, [2]string{"ADBD_LLM_API_KEY", key})
	}
	return env
}

// Render returns the launchd property list for one job.
func (p installPlan) Render(j installJob) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")
	writePlistString(&b, "Label", p.Label(j))
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, arg := range p.Arguments(j) {
		b.WriteString("\t\t<string>" + xmlEscape(arg) + "</string>\n")
	}
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>StartInterval</key>\n")
	b.WriteString("\t<integer>" + strconv.Itoa(j.Interval) + "</integer>\n")
	// RunAtLoad means a fresh login starts one pass immediately instead of
	// waiting a whole interval in the dark.
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	writePlistString(&b, "WorkingDirectory", p.Root)
	writePlistString(&b, "StandardOutPath", p.LogPath(j, "out"))
	writePlistString(&b, "StandardErrorPath", p.LogPath(j, "err"))
	b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
	for _, kv := range p.environment() {
		writePlistString(&b, kv[0], kv[1])
	}
	b.WriteString("\t</dict>\n")
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// launchctl seam
// ---------------------------------------------------------------------------

// launchctlResult is the outcome of one launchctl invocation.
//
// Available=false means launchctl itself could not be executed (not on this
// machine, no permission to exec). Available=true with a non-nil Err means
// launchctl ran and refused -- a real "the job is not loaded", not a missing
// probe (AC-006).
type launchctlResult struct {
	Stdout    string
	Available bool
	Err       error
}

// launchctlCall runs one launchctl invocation. It is the only process seam of
// this file and it is a var so tests inject a recorder: no test ever runs the
// real launchctl.
var launchctlCall = func(args ...string) launchctlResult {
	cmd := exec.Command("launchctl", args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return launchctlResult{Stdout: stdout.String(), Available: true}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return launchctlResult{Stdout: stdout.String(), Available: true, Err: err}
	}
	return launchctlResult{Available: false, Err: err}
}

// ---------------------------------------------------------------------------
// per-job observations
// ---------------------------------------------------------------------------

// installStatus is the read-only answer for one job (AC-006).
type installStatus struct {
	Label     string
	PlistPath string
	Verb      string
	Interval  int
	Installed bool
	// Known is false when launchctl could not be asked at all, which is why
	// Loaded is then meaningless rather than false.
	Known    bool
	Loaded   bool
	LastExit string
}

// observe reads one job's state. It writes nothing and starts nothing.
func (p installPlan) observe(j installJob) installStatus {
	st := installStatus{
		Label:     p.Label(j),
		PlistPath: p.PlistPath(j),
		Verb:      j.verb(),
		Interval:  j.Interval,
		Installed: fileExists(p.PlistPath(j)),
		LastExit:  UnknownLastExit,
	}
	res := launchctlCall("print", p.Target(j))
	if !res.Available {
		return st
	}
	st.Known = true
	st.Loaded = res.Err == nil
	if st.Loaded {
		st.LastExit = parseLastExit(res.Stdout)
	}
	return st
}

// parseLastExit reads launchd's "last exit code = N" line. Anything else stays
// unknown instead of being invented.
func parseLastExit(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "last exit code") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		if value := strings.TrimSpace(line[idx+1:]); value != "" {
			return value
		}
	}
	return UnknownLastExit
}

// fileExists is the only "is it installed" question: a stat, never a read.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ---------------------------------------------------------------------------
// actions
// ---------------------------------------------------------------------------

// dryRun prints the four jobs and what applying them would touch (AC-001). It
// opens no file for writing, creates no directory and never calls launchctl.
func (p installPlan) dryRun(stdout, _ io.Writer) int {
	for _, j := range p.Jobs {
		fmt.Fprintf(stdout, "label=%s path=%s verb=%s interval=%d installed=%s\n",
			p.Label(j), p.PlistPath(j), j.verb(), j.Interval, boolToken(fileExists(p.PlistPath(j))))
	}
	fmt.Fprintf(stdout, "jobs=%d dry_run=yes wrote=0\n", len(p.Jobs))
	return 0
}

// apply writes all four plists, then (re)loads each one (AC-002).
//
// The write phase finishes before launchctl is touched at all: a failed write
// therefore cannot leave launchd holding a half-written job (AC-007). bootout
// before bootstrap is what makes a second run a no-op instead of a duplicate
// label error.
func (p installPlan) apply(stdout, stderr io.Writer) int {
	for _, j := range p.Jobs {
		if code := p.writePlist(j, stdout, stderr); code != 0 {
			return code
		}
	}
	unavailable := false
	for _, j := range p.Jobs {
		// A bootout of a job that is not loaded fails; that is the expected
		// first-run case, not an error.
		launchctlCall("bootout", p.Target(j))
		res := launchctlCall("bootstrap", p.Domain(), p.PlistPath(j))
		if !res.Available {
			unavailable = true
		}
		loaded := res.Available && res.Err == nil
		fmt.Fprintf(stdout, "label=%s path=%s verb=%s interval=%d installed=yes loaded=%s\n",
			p.Label(j), p.PlistPath(j), j.verb(), j.Interval, boolToken(loaded))
	}
	if unavailable {
		fmt.Fprintf(stdout, "applied=%d reason_code=%s\n", len(p.Jobs), ReasonInstallLaunchctlUnavailable)
		return 2
	}
	fmt.Fprintf(stdout, "applied=%d\n", len(p.Jobs))
	return 0
}

// writePlist writes one plist atomically: a temp file in the same directory then
// a rename, so launchd can never read a truncated job.
func (p installPlan) writePlist(j installJob, stdout, stderr io.Writer) int {
	path := p.PlistPath(j)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return installFailure(path, err, stdout, stderr)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(p.Render(j)), 0o644); err != nil {
		return installFailure(path, err, stdout, stderr)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return installFailure(path, err, stdout, stderr)
	}
	return 0
}

// installFailure is the loud, machine-readable failure of AC-007: exit 2, one
// stable reason code and the first path that could not be written. It prints no
// plist content, so a failure can never leak the key either.
func installFailure(path string, err error, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "adbd install: %v\n", err)
	fmt.Fprintf(stdout, "path=%s reason_code=%s\n", path, ReasonInstallFailed)
	return 2
}

// uninstall bootouts and deletes the four plists (AC-005). A job that is not
// there is not an error: the second run says removed=no and still exits 0.
func (p installPlan) uninstall(stdout, stderr io.Writer) int {
	removed := 0
	for _, j := range p.Jobs {
		path := p.PlistPath(j)
		// Best effort: bootout of a job that was never loaded fails, and that
		// is exactly the case an idempotent uninstall has to survive.
		launchctlCall("bootout", p.Target(j))
		existed := fileExists(path)
		removeErr := os.Remove(path)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return installFailure(path, removeErr, stdout, stderr)
		}
		if existed && removeErr == nil {
			removed++
		}
		fmt.Fprintf(stdout, "label=%s path=%s installed=%s removed=%s\n",
			p.Label(j), path, boolToken(existed), boolToken(existed && removeErr == nil))
	}
	fmt.Fprintf(stdout, "uninstalled=%d\n", removed)
	return 0
}

// report is the read-only status surface (AC-006): installed, loaded and
// last_exit per job, plus launchctl_unavailable when the probe could not run.
func (p installPlan) report(stdout, _ io.Writer) int {
	unavailable := false
	for _, j := range p.Jobs {
		st := p.observe(j)
		loaded := UnknownLastExit
		if st.Known {
			loaded = boolToken(st.Loaded)
		} else {
			unavailable = true
		}
		fmt.Fprintf(stdout, "label=%s path=%s verb=%s interval=%d installed=%s loaded=%s last_exit=%s\n",
			st.Label, st.PlistPath, st.Verb, st.Interval, boolToken(st.Installed), loaded, st.LastExit)
	}
	if unavailable {
		fmt.Fprintf(stdout, "reason_code=%s\n", ReasonInstallLaunchctlUnavailable)
	}
	return 0
}

// ---------------------------------------------------------------------------
// CLI: adbd install --dry-run | --apply | --uninstall | --status
// ---------------------------------------------------------------------------

func runInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var dryRun, apply, uninstall, status bool
	fs.BoolVar(&dryRun, "dry-run", false, "print the four jobs and their plist paths; write nothing")
	fs.BoolVar(&apply, "apply", false, "write the four plists and (re)bootstrap each")
	fs.BoolVar(&uninstall, "uninstall", false, "bootout and delete the four plists")
	fs.BoolVar(&status, "status", false, "read-only report: installed, loaded, last_exit")
	root := fs.String("root", envOr("ADBD_ROOT", "."), "ADB root directory")
	home := fs.String("home", "", "home directory holding Library/LaunchAgents")
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}

	actions := make([]string, 0, 4)
	if dryRun {
		actions = append(actions, InstallActionDryRun)
	}
	if apply {
		actions = append(actions, InstallActionApply)
	}
	if uninstall {
		actions = append(actions, InstallActionUninstall)
	}
	if status {
		actions = append(actions, InstallActionStatus)
	}
	if len(actions) != 1 {
		// Also the guard for the illegal apply -> dry_run in one call: two
		// actions would make "did it write?" unanswerable.
		fmt.Fprintf(stderr, "adbd install: exactly one of --dry-run, --apply, --uninstall, --status is required (got %d)\n", len(actions))
		return 2
	}

	plan := newInstallPlan(*root, *home, "")
	switch actions[0] {
	case InstallActionDryRun:
		return plan.dryRun(stdout, stderr)
	case InstallActionApply:
		return plan.apply(stdout, stderr)
	case InstallActionUninstall:
		return plan.uninstall(stdout, stderr)
	default:
		return plan.report(stdout, stderr)
	}
}
