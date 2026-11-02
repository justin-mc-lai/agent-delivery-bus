// Command adbd is the Agent Delivery Bus daemon surface.
//
// Usage:
//
//	adbd --version
//	adbd bind   --channel C --thread T --agent-id A [--ttl 24h]
//	adbd feed   --project P --slug S --text T [--channel C] [--message-id M]
//	adbd intake --channel C --thread T --message-id M --text S [--outcome json|text]
//	adbd status --channel C --thread T [--outcome json|text]
//	adbd route  --once [--limit N] [--outcome json|text]
//	adbd route  --list-tools
//	adbd reconcile --once [--root <dir>] [--outcome json|text]
//	adbd watchdog --once [--apply] [--outcome json|text]
//	adbd poll-github --once [--root <dir>] [--outcome json|text]
//	adbd agent  plist | status
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// version is the adbd build version, tracked with the Beacon docs version.
const version = "0.1.5"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "--version", "-version", "version":
		fmt.Fprintf(stdout, "adbd %s\n", version)
		return 0
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	case "bind":
		return runBind(args[1:], stdout, stderr)
	case "feed":
		return runFeed(args[1:], stdout, stderr)
	case "intake":
		return runIntake(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "route":
		return runRoute(args[1:], stdout, stderr)
	case "reconcile":
		return runReconcile(args[1:], stdout, stderr)
	case "watchdog":
		return runWatchdog(args[1:], stdout, stderr)
	case "poll-github":
		return runPollGitHub(args[1:], stdout, stderr)
	case "agent":
		return runAgent(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "adbd: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `adbd - Agent Delivery Bus intake fastpath

usage:
  adbd --version
  adbd bind   --channel C --thread T --agent-id A [--ttl 24h]
  adbd feed   --project P --slug S --text T [--channel C] [--message-id M]
  adbd intake --channel C --thread T --message-id M --text S [--outcome json|text]
  adbd status --channel C --thread T [--outcome json|text]
  adbd route  --once [--limit N] [--outcome json|text]
  adbd route  --list-tools
  adbd reconcile --once [--outcome json|text]
  adbd watchdog --once [--apply] [--outcome json|text]
  adbd poll-github --once [--root <dir>] [--outcome json|text]
  adbd agent  plist [--outcome json|text]
  adbd agent  status [--outcome json|text]

common flags:
  --root PATH        ADB root, defaults to "." (env ADBD_ROOT)
  --state PATH       state document, defaults to <root>/.beacon/adbd-state.json
  --slow-path PATH   slow-path queue, defaults to <root>/.beacon/adbd-slowpath.jsonl
  --config PATH      config file, defaults to <root>/.beacon/config
  --runner-bin PATH  runner binary, defaults to config adbd_paseo_bin or "paseo"
  --outcome FORMAT   text (default) or json

feed flags:
  --project P        project slug; must match [a-z0-9._-]{1,64}
  --slug S           feature slug; same alphabet
  --text T           the utterance to feed (untrusted input)
  --channel C        source channel, defaults to adbd_feed_project_channel or skill
  --message-id M     idempotency key, defaults to sha256(channel|thread|text)[:16]
  --trace PATH       decision trace, defaults to <root>/.beacon/adbd-decisions.jsonl

route flags:
  --once             drain the slow-path backlog once and exit
  --list-tools       print the router execution toolset (always [])
  --limit N          drain at most N items; 0 (default) drains the whole queue
  --agent-id A       standing worker id (verified with paseo inspect); empty
                     means a worker is created per thread
  --trace PATH       decision trace, defaults to <root>/.beacon/adbd-decisions.jsonl

reconcile flags:
  --once             run one reconcile pass and exit (reads "paseo inspect
                     <id> --json" for every leased binding; writes
                     .beacon/adbd-reports.jsonl and .beacon/adbd-exec-state.json)
  --reports PATH     worker receipt ledger, defaults to <root>/.beacon/adbd-reports.jsonl
  --exec-state PATH  exec state document, defaults to <root>/.beacon/adbd-exec-state.json

watchdog flags:
  --once             run one watch pass and exit (the launchd rhythm)
  --apply            act on a stall instead of only observing it
  --decisions PATH   decision trace, defaults to <root>/.beacon/adbd-decisions.jsonl
  --watch PATH       watchdog trace, defaults to <root>/.beacon/adbd-watchdog.jsonl
  --heartbeat PATH   router heartbeat, defaults to <root>/.beacon/adbd-heartbeat.json
  --watch-state PATH scan ledger, defaults to <root>/.beacon/adbd-watch-state.json

agent subcommands:
  plist              print the launchd job to stdout; writes no file, runs nothing
  status             read-only report: plist on disk? loaded in launchd?

reconcile behaviour:
  One pass asks paseo for the status of every agent that still holds a lease and
  treats "running -> idle" as completion: the binding becomes "done", one receipt
  line lands in adbd-reports.jsonl (once per agent) and the requirement's row in
  adbd-exec-state.json becomes exec_state=done. A paseo that cannot be asked
  (missing binary, non-zero exit, unparsable output) prints
  reason_code=paseo_unavailable, exits 2 and changes nothing.

poll-github flags:
  --once             run one poll pass and exit (the scheduled rhythm); the
                     poller has no resident mode, so a bare run is also one pass

poll-github behaviour:
  Polls the open issues of every adbd_github_repos entry through the gh CLI and
  submits them to the same intake kernel as "adbd intake" / "adbd feed":
  thread gh:<owner>/<repo>#<n>, message-id gh:<n>:<updatedAt>. An unchanged
  issue is deduped; a new comment (new updatedAt) counts as a new event. Nothing
  is ever written back to GitHub.

config keys (.beacon/config, key=value):
  adbd_allow_channels=feishu,slack   allow-list; absent or empty means allow all
  adbd_feed_project_channel=skill    default channel for adbd feed
  adbd_paseo_bin=/usr/local/bin/paseo
  adbd_router_agent=...              legacy alias of adbd_worker_agent_id
  adbd_worker_agent_id=<id>          paseo agent id to reuse; empty creates one
  adbd_worker_provider=opencode      provider for a newly created worker
  adbd_worker_model=deepseek/deepseek-flash  model for a newly created worker
  adbd_decider=rule                  rule (default) or llm
  adbd_llm_base_url=...              OpenAI-compatible endpoint root
  adbd_llm_model=deepseek-chat       model name
  adbd_llm_api_key=...               key; falls back to env ADBD_LLM_API_KEY
  adbd_llm_timeout=60s               single request ceiling
  adbd_stall_after=20m               a bound thread idle this long is stalled
  adbd_heartbeat_after=10m           a router heartbeat older than this is stale
  adbd_repeat_window=6h              silence window for repeated alerts per thread
  adbd_interval=10m                  StartInterval written into the launchd plist
  adbd_github_repos=owner/repo,...   open issues to poll; absent means nothing to poll
  adbd_github_labels=bug,...         keep only issues carrying one of these labels
  adbd_github_limit=50               issues fetched per repo per pass
  adbd_github_proxy=http://host:port injected as HTTPS_PROXY/HTTP_PROXY for gh
  adbd_gh_bin=gh                     gh executable path
  note: polled issues arrive on channel "github", so add github to
  adbd_allow_channels (or set adbd_allow_all=true) for them to reach intake
`)
}

// commonFlags mirrors the shared CLI surface.
type commonFlags struct {
	root       string
	statePath  string
	slowPath   string
	configPath string
	runnerBin  string
	outcome    string
}

func addCommonFlags(fs *flag.FlagSet) *commonFlags {
	cf := &commonFlags{}
	fs.StringVar(&cf.root, "root", envOr("ADBD_ROOT", "."), "ADB root directory")
	fs.StringVar(&cf.statePath, "state", "", "state document path")
	fs.StringVar(&cf.slowPath, "slow-path", "", "slow-path queue path")
	fs.StringVar(&cf.configPath, "config", "", "config file path")
	fs.StringVar(&cf.runnerBin, "runner-bin", "", "runner binary path")
	fs.StringVar(&cf.outcome, "outcome", "text", "text or json")
	return cf
}

// resolve fills the path defaults, keeping every flag optional.
func (cf *commonFlags) resolve() error {
	beacon := filepath.Join(cf.root, ".beacon")
	if cf.statePath == "" {
		cf.statePath = envOr("ADBD_STATE", filepath.Join(beacon, "adbd-state.json"))
	}
	if cf.slowPath == "" {
		cf.slowPath = envOr("ADBD_SLOWPATH", filepath.Join(beacon, "adbd-slowpath.jsonl"))
	}
	if cf.configPath == "" {
		cf.configPath = envOr("ADBD_CONFIG", filepath.Join(beacon, "config"))
	}
	switch cf.outcome {
	case "", "text":
		cf.outcome = "text"
	case "json":
	default:
		return fmt.Errorf("--outcome must be text or json, got %q", cf.outcome)
	}
	return nil
}

func (cf *commonFlags) core() (*Core, error) {
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		return nil, err
	}
	bin := cf.runnerBin
	if bin == "" {
		bin = envOr("ADBD_PASEO_BIN", cfg.PaseoBin)
	}
	if bin == "" {
		bin = "paseo"
	}
	core := NewCore(&Store{Path: cf.statePath, SlowPath: cf.slowPath}, PaseoRunner{Bin: bin})
	core.AllowAll = cfg.AllowAll
	core.Allowed = map[string]bool{}
	for _, channel := range cfg.AllowedChannels {
		core.Allowed[channel] = true
	}
	return core, nil
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return flag.ErrHelp
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return nil
}

func runBind(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bind", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var channel, thread, agentID, ttlRaw string
	fs.StringVar(&channel, "channel", "", "source channel")
	fs.StringVar(&thread, "thread", "", "thread id")
	fs.StringVar(&agentID, "agent-id", "", "paseo agent id")
	fs.StringVar(&ttlRaw, "ttl", "24h", "lease TTL, e.g. 24h or 86400")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd bind: %v\n", err)
		return 2
	}
	ttl, err := parseTTL(ttlRaw)
	if err != nil {
		fmt.Fprintf(stderr, "adbd bind: %v\n", err)
		return 2
	}
	core, err := cf.core()
	if err != nil {
		fmt.Fprintf(stderr, "adbd bind: %v\n", err)
		return 1
	}
	binding, err := core.Bind(channel, thread, agentID, ttl)
	if err != nil {
		fmt.Fprintf(stderr, "adbd bind: %v\n", err)
		return 1
	}
	if cf.outcome == "json" {
		return writeJSON(stdout, stderr, binding)
	}
	fmt.Fprintf(stdout, "channel=%s thread=%s agent_id=%s state=%s expires_at=%s ttl=%s\n",
		binding.Channel, binding.Thread, binding.AgentID, binding.State,
		binding.ExpiresAt.Format(time.RFC3339), formatDuration(ttl))
	return 0
}

func runIntake(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("intake", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var channel, thread, messageID, text string
	fs.StringVar(&channel, "channel", "", "source channel")
	fs.StringVar(&thread, "thread", "", "thread id")
	fs.StringVar(&messageID, "message-id", "", "idempotency key")
	fs.StringVar(&text, "text", "", "message text to forward")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd intake: %v\n", err)
		return 2
	}
	core, err := cf.core()
	if err != nil {
		fmt.Fprintf(stderr, "adbd intake: %v\n", err)
		return 1
	}
	res, err := core.Intake(IntakeInput{Channel: channel, Thread: thread, MessageID: messageID, Text: text})
	if err != nil {
		fmt.Fprintf(stderr, "adbd intake: %v\n", err)
		return 1
	}
	if cf.outcome == "json" {
		return writeJSON(stdout, stderr, res)
	}
	fmt.Fprintln(stdout, formatIntake(res))
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var channel, thread string
	fs.StringVar(&channel, "channel", "", "source channel")
	fs.StringVar(&thread, "thread", "", "thread id")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd status: %v\n", err)
		return 2
	}
	core, err := cf.core()
	if err != nil {
		fmt.Fprintf(stderr, "adbd status: %v\n", err)
		return 1
	}
	st, err := core.Status(channel, thread)
	if err != nil {
		fmt.Fprintf(stderr, "adbd status: %v\n", err)
		return 1
	}
	if cf.outcome == "json" {
		return writeJSON(stdout, stderr, st)
	}
	line := fmt.Sprintf("channel=%s thread=%s state=%s lease=%s",
		st.Channel, st.Thread, st.State, st.Lease)
	if st.AgentID != "" {
		line += " agent_id=" + st.AgentID
	}
	if st.ExpiresAt != "" {
		line += " expires_at=" + st.ExpiresAt
		line += " remaining=" + formatDuration(time.Duration(st.RemainingSeconds)*time.Second)
	}
	fmt.Fprintln(stdout, line)
	return 0
}

// formatIntake renders the text outcome; route= is always the first field.
func formatIntake(r IntakeResult) string {
	parts := []string{"route=" + r.Route}
	if r.ReasonCode != "" {
		parts = append(parts, "reason_code="+r.ReasonCode)
	}
	if r.AgentID != "" {
		parts = append(parts, "agent_id="+r.AgentID)
	}
	if r.Lease != "" {
		parts = append(parts, "lease="+r.Lease)
	}
	if r.State != "" {
		parts = append(parts, "state="+r.State)
	}
	return strings.Join(parts, " ")
}

func writeJSON(stdout, stderr io.Writer, payload any) int {
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(stderr, "adbd: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return 0
}

func flagExit(err error, stderr io.Writer) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintf(stderr, "adbd: %v\n", err)
	return 2
}

// parseTTL accepts a Go duration ("24h", "30m") or plain seconds ("86400").
func parseTTL(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultTTL, nil
	}
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if seconds <= 0 {
			return 0, fmt.Errorf("--ttl must be positive, got %q", raw)
		}
		return time.Duration(seconds) * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("--ttl %q is neither a duration nor seconds", raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("--ttl must be positive, got %q", raw)
	}
	return d, nil
}

func formatDuration(d time.Duration) string {
	if d >= time.Hour && d%time.Hour == 0 {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}
	return d.String()
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
