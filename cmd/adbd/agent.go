// launchd hosting for the night watchman (truth.md, feature watchdog, v0.1.5).
//
// Two read-mostly surfaces live here:
//
//   - `adbd agent plist` (AC-006) renders the launchd job to stdout and stops
//     there. It writes no file, creates no directory and calls no external
//     binary: installing a system service stays a human decision, so the daemon
//     can never bootstrap itself onto a machine by accident.
//   - `adbd agent status` (AC-007) answers two questions read-only -- is the job
//     file on disk, and does launchd have it loaded -- and reports
//     launchctl_unavailable when the probe cannot run at all, which is a
//     different fact from "not loaded".
//
// os/exec appears in exactly one place in this feature: the launchctl status
// probe, behind an injectable seam, so tests never shell out and the watchdog
// itself keeps working when adbd is dead.
package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultAgentLabel is the launchd job name. It carries the product name, not the
// machine name, so the plist is portable between machines.
const DefaultAgentLabel = "com.agent-delivery-bus.adbd"

// Agent status slots (AC-007).
const (
	AgentStateLoaded       = "loaded"
	AgentStateNotLoaded    = "not_loaded"
	AgentStateNotInstalled = "not_installed"
	AgentStateUnknown      = "unknown"
)

// Agent is the launchd job this repository knows how to describe. It is pure
// data: nothing here holds a handle to launchd.
type Agent struct {
	// Label is the job name written into the plist.
	Label string
	// Program is the executable launchd runs (absolute when resolvable).
	Program string
	// Root is the ADB root: the job's WorkingDirectory and the --root it passes.
	Root string
	// Interval is the StartInterval.
	Interval time.Duration
	// HomeDir is where ~/Library/LaunchAgents lives.
	HomeDir string
	// UID selects the launchd domain gui/<uid>; -1 means the current user.
	UID int
}

// NewAgent builds the job description from the operator config.
func NewAgent(root, program, homeDir string, interval time.Duration) Agent {
	if interval <= 0 {
		interval = DefaultAgentInterval
	}
	if strings.TrimSpace(program) == "" {
		program = "adbd"
	}
	return Agent{
		Label:    DefaultAgentLabel,
		Program:  program,
		Root:     root,
		Interval: interval,
		HomeDir:  homeDir,
		UID:      -1,
	}
}

// Path is where a human would put the plist (this command never writes it).
func (a Agent) Path() string {
	return filepath.Join(a.HomeDir, "Library", "LaunchAgents", a.Label+".plist")
}

// Domain is the launchd domain the probe asks about: gui/<uid>.
func (a Agent) Domain() string {
	uid := a.UID
	if uid < 0 {
		uid = os.Getuid()
	}
	return "gui/" + strconv.Itoa(uid)
}

// Render returns the launchd property list for `adbd watchdog --once`.
//
// StartInterval plus WorkingDirectory are the two keys AC-006 names: the first is
// the whole hosting model (run once, exit, run again), the second is what makes
// --root "." mean the project instead of "/".
func (a Agent) Render() string {
	interval := a.Interval
	if interval <= 0 {
		interval = DefaultAgentInterval
	}
	program := a.Program
	if strings.TrimSpace(program) == "" {
		program = "adbd"
	}
	root := a.Root
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	beacon := filepath.Join(root, ".beacon")

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")
	writePlistString(&b, "Label", a.Label)
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, arg := range []string{program, "watchdog", "--once", "--root", root} {
		b.WriteString("\t\t<string>" + xmlEscape(arg) + "</string>\n")
	}
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>StartInterval</key>\n")
	b.WriteString("\t<integer>" + strconv.FormatInt(int64(interval/time.Second), 10) + "</integer>\n")
	writePlistString(&b, "WorkingDirectory", root)
	// A watchdog whose stdout goes nowhere is a watchdog nobody reads.
	writePlistString(&b, "StandardOutPath", filepath.Join(beacon, "adbd-watchdog.out.log"))
	writePlistString(&b, "StandardErrorPath", filepath.Join(beacon, "adbd-watchdog.err.log"))
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

func writePlistString(b *strings.Builder, key, value string) {
	b.WriteString("\t<key>" + xmlEscape(key) + "</key>\n")
	b.WriteString("\t<string>" + xmlEscape(value) + "</string>\n")
}

// xmlEscape quotes a plist string. The stdlib escaper is used rather than a
// hand-rolled one so a path with &, < or a quote stays a valid plist.
func xmlEscape(value string) string {
	var buf strings.Builder
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

// AgentStatus is the read-only answer of `adbd agent status` (AC-007).
type AgentStatus struct {
	Label      string `json:"label"`
	Domain     string `json:"domain"`
	PlistPath  string `json:"plist_path"`
	Installed  bool   `json:"installed"`
	Loaded     bool   `json:"loaded"`
	State      string `json:"state"`
	ReasonCode string `json:"reason_code,omitempty"`
}

// Status reports whether the plist exists and whether launchd has the job loaded.
// It never writes, never creates a directory and never starts anything.
func (a Agent) Status() AgentStatus {
	st := AgentStatus{Label: a.Label, Domain: a.Domain(), PlistPath: a.Path()}
	if _, err := os.Stat(a.Path()); err != nil {
		// Nothing on disk: the job was never installed, and there is no point
		// asking launchd about it.
		st.State = AgentStateNotInstalled
		return st
	}
	st.Installed = true
	loaded, available := launchctlProbe(st.Domain, a.Label)
	if !available {
		// launchctl missing or refused to run is not the same operator fact as
		// "not loaded", so it gets its own reason code.
		st.State = AgentStateUnknown
		st.ReasonCode = ReasonLaunchctlUnavailable
		return st
	}
	st.Loaded = loaded
	if loaded {
		st.State = AgentStateLoaded
		return st
	}
	st.State = AgentStateNotLoaded
	return st
}

// launchctlProbe asks launchd whether a job is loaded. It is the only process
// seam of this feature and it is a var so tests inject a fake: no test ever runs
// the real launchctl (AC-007).
var launchctlProbe = probeLaunchctl

// probeLaunchctl reports (loaded, available).
//
// available=false means the probe itself could not run (no launchctl on this
// machine, no permission to exec). available=true with loaded=false means
// launchctl ran and said the job is not there, which is a real "not loaded".
func probeLaunchctl(domain, label string) (bool, bool) {
	cmd := exec.Command("launchctl", "print", domain+"/"+label)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, true
		}
		return false, false
	}
	return true, true
}

// ---------------------------------------------------------------------------
// CLI: adbd agent plist | status
// ---------------------------------------------------------------------------

func runAgent(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "adbd agent: plist or status is required")
		return 2
	}
	switch args[0] {
	case "plist":
		return runAgentPlist(args[1:], stdout, stderr)
	case "status":
		return runAgentStatus(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "adbd agent: unknown subcommand %q (want plist or status)\n", args[0])
		return 2
	}
}

// agentFor resolves the common flags into a job description.
func agentFor(cf *commonFlags) (Agent, error) {
	if err := cf.resolve(); err != nil {
		return Agent{}, err
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		return Agent{}, err
	}
	root := cf.root
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Agent{}, fmt.Errorf("cannot locate ~/Library/LaunchAgents: %w", err)
	}
	program := "adbd"
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		program = exe
	}
	agent := NewAgent(root, program, home, cfg.Interval)
	return agent, nil
}

func runAgentPlist(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agent plist", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	agent, err := agentFor(cf)
	if err != nil {
		fmt.Fprintf(stderr, "adbd agent plist: %v\n", err)
		return 1
	}
	plist := agent.Render()
	if cf.outcome == "json" {
		// Same bytes, wrapped: stdout stays a machine payload either way.
		return writeJSON(stdout, stderr, struct {
			Label     string `json:"label"`
			PlistPath string `json:"plist_path"`
			Plist     string `json:"plist"`
		}{Label: agent.Label, PlistPath: agent.Path(), Plist: plist})
	}
	fmt.Fprint(stdout, plist)
	return 0
}

func runAgentStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agent status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	agent, err := agentFor(cf)
	if err != nil {
		fmt.Fprintf(stderr, "adbd agent status: %v\n", err)
		return 1
	}
	st := agent.Status()
	if cf.outcome == "json" {
		return writeJSON(stdout, stderr, st)
	}
	line := fmt.Sprintf("label=%s domain=%s plist=%s installed=%s loaded=%s state=%s",
		st.Label, st.Domain, st.PlistPath, boolToken(st.Installed), boolToken(st.Loaded), st.State)
	if st.ReasonCode != "" {
		line += " reason_code=" + st.ReasonCode
	}
	fmt.Fprintln(stdout, line)
	return 0
}
