package main

// launchd hosting tests (feature watchdog, v0.1.5): TC-WD-006 and TC-WD-007.
//
// Two hard properties are asserted here and they are both negative ones:
//
//   - `agent plist` writes nothing and runs nothing, so generating a job can never
//     install one behind the operator's back (AC-006).
//   - `agent status` only reads, and it never shells out to the real launchctl in
//     a test: the probe is injected through launchctlProbe (AC-007).

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// agentHarness is a throwaway HOME plus a throwaway ADB root.
type agentHarness struct {
	t     *testing.T
	root  string
	home  string
	plist string
}

func newAgentHarness(t *testing.T) *agentHarness {
	t.Helper()
	root := filepath.Join(t.TempDir(), "adb-root")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	// os.UserHomeDir reads $HOME on darwin, which is what `adbd agent status` uses.
	t.Setenv("HOME", home)
	return &agentHarness{
		t:     t,
		root:  root,
		home:  home,
		plist: filepath.Join(home, "Library", "LaunchAgents", DefaultAgentLabel+".plist"),
	}
}

// install writes the plist where a human would have put it.
func (h *agentHarness) install(content string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.plist), 0o755); err != nil {
		h.t.Fatalf("mkdir LaunchAgents: %v", err)
	}
	if err := os.WriteFile(h.plist, []byte(content), 0o644); err != nil {
		h.t.Fatalf("write plist: %v", err)
	}
}

func (h *agentHarness) config(lines ...string) string {
	h.t.Helper()
	beacon := filepath.Join(h.root, ".beacon")
	if err := os.MkdirAll(beacon, 0o755); err != nil {
		h.t.Fatalf("mkdir beacon: %v", err)
	}
	path := filepath.Join(beacon, "config")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
	return path
}

// launchctlSpy replaces the probe and records every call.
type launchctlSpy struct {
	calls []string
}

func withLaunchctlProbe(t *testing.T, loaded, available bool) *launchctlSpy {
	t.Helper()
	spy := &launchctlSpy{}
	old := launchctlProbe
	launchctlProbe = func(domain, label string) (bool, bool) {
		spy.calls = append(spy.calls, domain+"/"+label)
		return loaded, available
	}
	t.Cleanup(func() { launchctlProbe = old })
	return spy
}

// ---------------------------------------------------------------------------
// TC-WD-006 (AC-006): the plist is printed, never installed
// ---------------------------------------------------------------------------

func TestAgentPlistGeneration(t *testing.T) {
	h := newAgentHarness(t)
	cfg := h.config("adbd_interval=10m")
	spy := withLaunchctlProbe(t, true, true)

	code, stdout, stderr := runCLI([]string{"agent", "plist", "--root", h.root, "--config", cfg})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	// It is a plist, not just text that mentions one: the whole document has to
	// parse as XML.
	var doc struct {
		XMLName xml.Name `xml:"plist"`
		Inner   string   `xml:",innerxml"`
	}
	if err := xml.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not a well-formed plist: %v\n%s", err, stdout)
	}

	absRoot, err := filepath.Abs(h.root)
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	for _, want := range []string{
		"<key>Label</key>",
		"<string>" + DefaultAgentLabel + "</string>",
		"<key>ProgramArguments</key>",
		"<string>watchdog</string>",
		"<string>--once</string>",
		"<key>StartInterval</key>",
		"<integer>600</integer>",
		"<key>WorkingDirectory</key>",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("plist is missing %q:\n%s", want, stdout)
		}
	}
	if !strings.Contains(stdout, "<string>"+absRoot+"</string>") {
		t.Fatalf("plist is missing the absolute root %q:\n%s", absRoot, stdout)
	}
	if strings.Contains(stdout, "--apply") {
		t.Fatalf("the hosted job must observe, not act, by default:\n%s", stdout)
	}

	// Nothing was written: not the plist, not a directory, not a log.
	if _, err := os.Stat(h.plist); !os.IsNotExist(err) {
		t.Fatalf("agent plist wrote %s (%v); it must only print", h.plist, err)
	}
	if _, err := os.Stat(filepath.Dir(h.plist)); !os.IsNotExist(err) {
		t.Fatalf("agent plist created %s; it must only print", filepath.Dir(h.plist))
	}
	entries, err := os.ReadDir(h.root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != ".beacon" {
		t.Fatalf("root changed beyond the config file it was given: %v", entries)
	}
	// And it never asked launchd anything.
	if len(spy.calls) != 0 {
		t.Fatalf("agent plist called launchctl: %v", spy.calls)
	}

	// The interval is configurable and lands in StartInterval as seconds.
	cfg2 := h.config("adbd_interval=90s")
	code, stdout, stderr = runCLI([]string{"agent", "plist", "--root", h.root, "--config", cfg2})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "<integer>90</integer>") {
		t.Fatalf("plist ignored adbd_interval=90s:\n%s", stdout)
	}

	// --outcome json carries the same document, still without writing anything.
	code, stdout, stderr = runCLI([]string{"agent", "plist", "--root", h.root, "--config", cfg2, "--outcome", "json"})
	if code != 0 {
		t.Fatalf("json exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, `"plist_path"`) || !strings.Contains(stdout, `StartInterval`) {
		t.Fatalf("json outcome = %s", stdout)
	}
	if _, err := os.Stat(h.plist); !os.IsNotExist(err) {
		t.Fatalf("json plist wrote %s", h.plist)
	}
}

// ---------------------------------------------------------------------------
// TC-WD-007 (AC-007): status is a read-only report
// ---------------------------------------------------------------------------

func TestAgentStatusReadOnly(t *testing.T) {
	uid := os.Getuid()
	wantDomain := "gui/" + strconv.Itoa(uid) + "/" + DefaultAgentLabel

	t.Run("not_installed", func(t *testing.T) {
		h := newAgentHarness(t)
		spy := withLaunchctlProbe(t, true, true)
		code, stdout, stderr := runCLI([]string{"agent", "status", "--root", h.root})
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr)
		}
		if !strings.Contains(stdout, "state="+AgentStateNotInstalled) {
			t.Fatalf("stdout = %q, want state=not_installed", stdout)
		}
		if !strings.Contains(stdout, "installed=no") || !strings.Contains(stdout, "loaded=no") {
			t.Fatalf("stdout = %q, want installed=no loaded=no", stdout)
		}
		// No plist means no question for launchd.
		if len(spy.calls) != 0 {
			t.Fatalf("status probed launchd for a job that was never installed: %v", spy.calls)
		}
		if _, err := os.Stat(h.plist); !os.IsNotExist(err) {
			t.Fatalf("status created the plist it was asked about")
		}
		if _, err := os.Stat(filepath.Dir(h.plist)); !os.IsNotExist(err) {
			t.Fatalf("status created %s", filepath.Dir(h.plist))
		}
	})

	t.Run("loaded", func(t *testing.T) {
		h := newAgentHarness(t)
		h.install("plist-content")
		spy := withLaunchctlProbe(t, true, true)
		code, stdout, stderr := runCLI([]string{"agent", "status", "--root", h.root})
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr)
		}
		if !strings.Contains(stdout, "state="+AgentStateLoaded) || !strings.Contains(stdout, "loaded=yes") {
			t.Fatalf("stdout = %q, want state=loaded loaded=yes", stdout)
		}
		if !strings.Contains(stdout, "plist="+h.plist) {
			t.Fatalf("stdout = %q, want plist=%s", stdout, h.plist)
		}
		if len(spy.calls) != 1 || spy.calls[0] != wantDomain {
			t.Fatalf("probe calls = %v, want exactly [%s] (launchctl print gui/<uid>/<label>)", spy.calls, wantDomain)
		}
	})

	t.Run("not_loaded", func(t *testing.T) {
		h := newAgentHarness(t)
		h.install("plist-content")
		withLaunchctlProbe(t, false, true)
		code, stdout, _ := runCLI([]string{"agent", "status", "--root", h.root})
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if !strings.Contains(stdout, "state="+AgentStateNotLoaded) {
			t.Fatalf("stdout = %q, want state=not_loaded", stdout)
		}
		if strings.Contains(stdout, "reason_code=") {
			t.Fatalf("stdout = %q, want no reason_code: launchctl answered", stdout)
		}
	})

	t.Run("launchctl_unavailable", func(t *testing.T) {
		h := newAgentHarness(t)
		h.install("plist-content")
		withLaunchctlProbe(t, false, false)
		code, stdout, stderr := runCLI([]string{"agent", "status", "--root", h.root})
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr)
		}
		if !strings.Contains(stdout, "reason_code="+ReasonLaunchctlUnavailable) {
			t.Fatalf("stdout = %q, want reason_code=%s", stdout, ReasonLaunchctlUnavailable)
		}
		if !strings.Contains(stdout, "state="+AgentStateUnknown) {
			t.Fatalf("stdout = %q, want state=unknown: an unanswered probe is not 'not loaded'", stdout)
		}
	})

	t.Run("never_writes", func(t *testing.T) {
		h := newAgentHarness(t)
		h.install("plist-content")
		withLaunchctlProbe(t, true, true)
		before, err := os.Stat(h.plist)
		if err != nil {
			t.Fatalf("stat plist: %v", err)
		}
		// The plist content must survive a status call untouched.
		if _, stdout, _ := runCLI([]string{"agent", "status", "--root", h.root}); stdout == "" {
			t.Fatal("status printed nothing")
		}
		after, err := os.Stat(h.plist)
		if err != nil {
			t.Fatalf("stat plist after: %v", err)
		}
		if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
			t.Fatalf("status modified the plist")
		}
		data, err := os.ReadFile(h.plist)
		if err != nil {
			t.Fatalf("read plist: %v", err)
		}
		if string(data) != "plist-content" {
			t.Fatalf("plist content = %q, want it untouched", data)
		}
		// Nor may it leave anything else behind in the root.
		entries, err := os.ReadDir(h.root)
		if err != nil {
			t.Fatalf("read root: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("status wrote into the ADB root: %v", entries)
		}
	})
}

// ---------------------------------------------------------------------------
// supporting: the job description itself
// ---------------------------------------------------------------------------

func TestAgentJobShape(t *testing.T) {
	agent := NewAgent("/tmp/adb", "/usr/local/bin/adbd", "/Users/op", DefaultAgentInterval)
	if got, want := agent.Path(), filepath.Join("/Users/op", "Library", "LaunchAgents", DefaultAgentLabel+".plist"); got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
	if agent.Domain() != "gui/"+strconv.Itoa(os.Getuid()) {
		t.Fatalf("Domain = %q, want gui/<uid>", agent.Domain())
	}
	// A zero interval must fall back to the frozen 10m instead of writing 0 and
	// making launchd spin.
	plist := NewAgent("/tmp/adb", "/usr/local/bin/adbd", "/Users/op", 0).Render()
	if !strings.Contains(plist, "<integer>600</integer>") {
		t.Fatalf("zero interval did not fall back to 600:\n%s", plist)
	}
	if !strings.Contains(plist, "--root") {
		t.Fatalf("the job must pin --root so WorkingDirectory is not the only anchor:\n%s", plist)
	}
	if strings.Contains(plist, "\x00") {
		t.Fatalf("plist contains a NUL byte")
	}
	if DefaultAgentInterval != 10*time.Minute {
		t.Fatalf("DefaultAgentInterval = %s, want 10m", DefaultAgentInterval)
	}
}
