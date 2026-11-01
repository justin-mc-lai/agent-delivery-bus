// GitHub issue polling source for the Agent Delivery Bus (truth.md, feature
// github-issue, v0.1.5).
//
// The lake is one command: `adbd poll-github --once` reads the configured repos,
// asks the gh CLI for the open issues of each, maps every issue onto a threaded
// event and submits it through Core.Intake -- the very same kernel `adbd intake`
// and `adbd feed` use. Everything that has to be exact is derived here:
//
//   - the thread is frozen at gh:<owner>/<repo>#<number>, so one issue is one
//     conversation across polls (AC-002);
//   - the idempotency key is gh:<number>:<updatedAt>, so an unchanged issue is
//     deduped by the kernel instead of being dispatched again, while a new
//     comment (updatedAt moved) counts as a new event (AC-002);
//   - only state=open issues are requested and an optional label whitelist keeps
//     the source quiet; nothing here ever writes to GitHub -- this file contains
//     no create/comment/edit/close call at all (AC-003);
//   - the operator-declared proxy is injected into the gh child environment as
//     HTTPS_PROXY/HTTP_PROXY, replacing whatever the parent had (AC-004);
//   - the three ways gh can fail each carry a stable reason_code and exit 2,
//     because a source that silently polls nothing is worse than one that
//     complains (AC-005);
//   - one pass prints polled/new/deduped, so a scheduled run is verifiable from
//     the logs alone (AC-006).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Frozen defaults (truth.md 配置项).
const (
	// DefaultGitHubLimit is adbd_github_limit: issues fetched per repo per pass.
	DefaultGitHubLimit = 50
	// DefaultGHBin is adbd_gh_bin: the gh executable name.
	DefaultGHBin = "gh"
	// DefaultGitHubChannel is the channel a polled issue arrives on. It is a
	// constant, not a config key: the thread already carries the repo identity,
	// and one source refusing to share a channel keeps the allow-list honest.
	DefaultGitHubChannel = "github"
	// ghThreadPrefix is the only accepted thread prefix for this source.
	ghThreadPrefix = "gh"
	// ghStderrDetailLimit is how much of gh's stderr a failure may quote (AC-005).
	ghStderrDetailLimit = 200
)

// GitHub poll reason codes. Stable machine tokens, never prose (AC-005).
const (
	// ReasonReposNotConfigured is the calm case: nothing to poll, exit 0.
	ReasonReposNotConfigured = "repos_not_configured"
	// ReasonGHNotFound means the gh binary could not be started at all.
	ReasonGHNotFound = "gh_not_found"
	// ReasonGHFailed means gh ran and exited non-zero.
	ReasonGHFailed = "gh_failed"
	// ReasonGHOutputInvalid means gh exited 0 but printed something that is not
	// the JSON array the --json flags promise.
	ReasonGHOutputInvalid = "gh_output_invalid"
)

// GitHubLabel is one label as `gh issue list --json labels` renders it.
type GitHubLabel struct {
	Name string `json:"name"`
}

// GitHubIssue is one open issue, in the exact shape the gh --json flags return.
type GitHubIssue struct {
	Number    int           `json:"number"`
	Title     string        `json:"title"`
	Body      string        `json:"body"`
	UpdatedAt string        `json:"updatedAt"`
	Labels    []GitHubLabel `json:"labels"`
}

// labelNames flattens the label objects into their names.
func (i GitHubIssue) labelNames() []string {
	out := make([]string, 0, len(i.Labels))
	for _, label := range i.Labels {
		out = append(out, label.Name)
	}
	return out
}

// GitHubEvent is one issue as it was submitted, kept for the JSON outcome so an
// operator can see which thread each poll touched and why.
type GitHubEvent struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	Thread     string `json:"thread"`
	MessageID  string `json:"message_id"`
	Route      string `json:"route"`
	ReasonCode string `json:"reason_code,omitempty"`
}

// GitHubPollResult is one pass in the shape the CLI prints.
//
// Invariant: Polled == New + Deduped + Rejected. Polled counts the issues that
// reached intake (after the label filter), because that is the number the kernel
// can account for; Filtered reports what the whitelist dropped so the two are
// never confused.
type GitHubPollResult struct {
	Polled     int           `json:"polled"`
	New        int           `json:"new"`
	Deduped    int           `json:"deduped"`
	Filtered   int           `json:"filtered,omitempty"`
	Rejected   int           `json:"rejected,omitempty"`
	Repos      []string      `json:"repos,omitempty"`
	ReasonCode string        `json:"reason_code,omitempty"`
	Events     []GitHubEvent `json:"events,omitempty"`
}

// GitHubError is a poll failure with a stable reason code the CLI maps to exit 2.
type GitHubError struct {
	Code   string
	Detail string
}

func (e *GitHubError) Error() string {
	detail := strings.TrimSpace(e.Detail)
	if detail == "" {
		return e.Code
	}
	return e.Code + ": " + detail
}

// IsGitHubError reports whether err is a poll failure with this reason code. An
// empty code matches any poll failure.
func IsGitHubError(err error, code string) bool {
	var ge *GitHubError
	if !errors.As(err, &ge) {
		return false
	}
	return code == "" || ge.Code == code
}

// ghResult is what one gh invocation produced, whichever way it ended.
type ghResult struct {
	Stdout string
	Stderr string
	// Exit is the process exit status; 0 means success.
	Exit int
	// NotFound is true only when the binary could not be started (missing path
	// or not executable), which is a different failure from a non-zero exit.
	NotFound bool
}

// ghRunner is the injectable seam: the poller never calls os/exec itself, so a
// test can swap the process boundary for a stub without touching poll logic.
type ghRunner interface {
	Run(bin string, args, env []string) ghResult
}

// execGHRunner is the one and only place this file starts a process.
type execGHRunner struct{}

// compile-time proof that the production runner satisfies the seam.
var _ ghRunner = execGHRunner{}

// Run executes one gh call with an explicit environment. argv is passed as
// separate elements, so no shell ever parses an issue body or a repo name.
func (execGHRunner) Run(bin string, args, env []string) ghResult {
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := ghResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return out
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		out.Exit = exitErr.ExitCode()
		return out
	}
	// Could not start at all: ENOENT from PATH lookup (exec.ErrNotFound) or from
	// an absolute path that does not exist (fs.ErrNotExist).
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		out.NotFound = true
		out.Stderr = err.Error()
		return out
	}
	// Anything else (a directory, a permission problem, a broken interpreter) is
	// still a start failure, but not a missing binary: report it as gh_failed.
	out.Exit = -1
	out.Stderr = strings.TrimSpace(out.Stderr + "\n" + err.Error())
	return out
}

// GitHubPoller polls the configured repos once. It writes nothing to GitHub: the
// only gh subcommand it ever builds is `issue list` (AC-003).
type GitHubPoller struct {
	// Repos is adbd_github_repos, comma separated owner/repo entries.
	Repos []string
	// Labels is adbd_github_labels: keep only issues carrying at least one of
	// these. Empty means no filtering.
	Labels []string
	// Limit is adbd_github_limit, defaulting to DefaultGitHubLimit.
	Limit int
	// Bin is adbd_gh_bin, defaulting to DefaultGHBin.
	Bin string
	// Proxy is adbd_github_proxy: when set it becomes HTTPS_PROXY/HTTP_PROXY in
	// the gh child environment, verbatim and without any string surgery.
	Proxy string
	// Channel is the source channel; empty means DefaultGitHubChannel.
	Channel string
	// Runner is the command seam; nil means the production execGHRunner.
	Runner ghRunner
	// Core is the intake kernel every issue is submitted to.
	Core *Core
}

// GitHubThread is the frozen thread shape for one issue: gh:<owner>/<repo>#<n>.
func GitHubThread(repo string, number int) string {
	return ghThreadPrefix + ":" + repo + "#" + strconv.Itoa(number)
}

// GitHubMessageID is the idempotency key: gh:<number>:<updatedAt>. An issue that
// has not changed keeps producing the same key and is deduped by the kernel; a
// new comment moves updatedAt and therefore produces a new key (AC-002).
func GitHubMessageID(issue GitHubIssue) string {
	return ghThreadPrefix + ":" + strconv.Itoa(issue.Number) + ":" + issue.UpdatedAt
}

// GitHubText is the message body handed to a worker: one title line, a blank
// line, then the issue body untouched. It is untrusted data, never instructions;
// the guard line is added at the intake boundary and by the router.
func GitHubText(issue GitHubIssue) string {
	return "#" + strconv.Itoa(issue.Number) + " " + issue.Title + "\n\n" + issue.Body
}

// ghIssueListArgs is the only argv this source ever builds (AC-001).
func ghIssueListArgs(repo string, limit int) []string {
	return []string{
		"issue", "list",
		"-R", repo,
		"--state", "open",
		"--limit", strconv.Itoa(limit),
		"--json", "number,title,body,updatedAt,labels",
	}
}

// ghChildEnv is the environment handed to gh: the parent environment, plus the
// configured proxy as HTTPS_PROXY and HTTP_PROXY (AC-004).
//
// A configured proxy replaces any inherited value instead of appending a second
// one, because two proxy variables of the same name and different values make
// curl's choice an accident. With no proxy configured the environment is passed
// through untouched.
func ghChildEnv(proxy string) []string {
	base := os.Environ()
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		return base
	}
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(key) {
		case "HTTPS_PROXY", "HTTP_PROXY":
			continue // replaced below
		}
		env = append(env, kv)
	}
	return append(env, "HTTPS_PROXY="+proxy, "HTTP_PROXY="+proxy)
}

// truncateDetail returns at most limit runes of s, trimmed at the edges. Runes,
// not bytes, so a UTF-8 stderr line is never cut mid-character.
func truncateDetail(s string, limit int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

func (p *GitHubPoller) repos() []string {
	out := make([]string, 0, len(p.Repos))
	for _, repo := range p.Repos {
		if repo = strings.TrimSpace(repo); repo != "" {
			out = append(out, repo)
		}
	}
	return out
}

func (p *GitHubPoller) labels() []string {
	out := make([]string, 0, len(p.Labels))
	for _, label := range p.Labels {
		if label = strings.TrimSpace(label); label != "" {
			out = append(out, label)
		}
	}
	return out
}

func (p *GitHubPoller) limit() int {
	if p.Limit <= 0 {
		return DefaultGitHubLimit
	}
	return p.Limit
}

func (p *GitHubPoller) bin() string {
	if bin := strings.TrimSpace(p.Bin); bin != "" {
		return bin
	}
	return DefaultGHBin
}

func (p *GitHubPoller) channel() string {
	if channel := strings.TrimSpace(p.Channel); channel != "" {
		return channel
	}
	return DefaultGitHubChannel
}

func (p *GitHubPoller) runner() ghRunner {
	if p.Runner != nil {
		return p.Runner
	}
	return execGHRunner{}
}

// keep applies the label whitelist: an issue survives when it carries at least
// one listed label. An empty whitelist keeps everything (AC-003).
func (p *GitHubPoller) keep(issue GitHubIssue) bool {
	wanted := p.labels()
	if len(wanted) == 0 {
		return true
	}
	have := issue.labelNames()
	for _, want := range wanted {
		for _, name := range have {
			if name == want {
				return true
			}
		}
	}
	return false
}

// fetch asks gh for the open issues of one repo, mapping the three failure modes
// onto reason codes (AC-001, AC-005).
func (p *GitHubPoller) fetch(repo string) ([]GitHubIssue, error) {
	out := p.runner().Run(p.bin(), ghIssueListArgs(repo, p.limit()), ghChildEnv(p.Proxy))
	if out.NotFound {
		return nil, &GitHubError{
			Code:   ReasonGHNotFound,
			Detail: fmt.Sprintf("%s could not be started (%s)", p.bin(), truncateDetail(out.Stderr, ghStderrDetailLimit)),
		}
	}
	if out.Exit != 0 {
		detail := truncateDetail(out.Stderr, ghStderrDetailLimit)
		if detail == "" {
			detail = fmt.Sprintf("%s exited %d with no stderr", p.bin(), out.Exit)
		}
		return nil, &GitHubError{Code: ReasonGHFailed, Detail: detail}
	}
	var issues []GitHubIssue
	if err := json.Unmarshal([]byte(out.Stdout), &issues); err != nil {
		return nil, &GitHubError{
			Code:   ReasonGHOutputInvalid,
			Detail: fmt.Sprintf("gh output is not a JSON issue array: %v (first %d chars: %s)", err, ghStderrDetailLimit, truncateDetail(out.Stdout, ghStderrDetailLimit)),
		}
	}
	return issues, nil
}

// submit hands one issue to the intake kernel, exactly like `adbd intake` and
// `adbd feed` do: no routing logic is duplicated here (AC-001, AC-006).
func (p *GitHubPoller) submit(repo string, issue GitHubIssue) (IntakeResult, error) {
	// The guard line goes on before intake sees the text: an issue body is
	// written by anyone on the internet, so it is untrusted data. guardedText is
	// idempotent, so the router will not stack a second tag.
	in := IntakeInput{
		Channel:   p.channel(),
		Thread:    GitHubThread(repo, issue.Number),
		MessageID: GitHubMessageID(issue),
		Text:      guardedText(GitHubText(issue)),
	}
	return p.Core.Intake(in)
}

// Poll runs exactly one pass over every configured repo.
//
// It stops at the first gh failure instead of pretending the rest went fine: a
// partial pass that still exits 0 is the silent failure AC-005 forbids. The
// returned result carries the counts gathered so far, so the caller can still
// print what was done before the error.
func (p *GitHubPoller) Poll() (GitHubPollResult, error) {
	res := GitHubPollResult{Repos: p.repos()}
	if len(res.Repos) == 0 {
		// AC-001: nothing configured is not an error, but it is never silent.
		res.ReasonCode = ReasonReposNotConfigured
		return res, nil
	}
	if p.Core == nil {
		return res, errors.New("adbd poll-github: no core configured")
	}
	for _, repo := range res.Repos {
		issues, err := p.fetch(repo)
		if err != nil {
			var ge *GitHubError
			if errors.As(err, &ge) {
				res.ReasonCode = ge.Code
			}
			return res, err
		}
		for _, issue := range issues {
			if !p.keep(issue) {
				res.Filtered++
				continue
			}
			intake, err := p.submit(repo, issue)
			if err != nil {
				return res, err
			}
			res.Polled++
			switch intake.Route {
			case RouteDeduped:
				res.Deduped++
			case RouteRejected:
				// Intake refused it (e.g. the channel is not on the allow-list):
				// accounted for, never counted as new work.
				res.Rejected++
			default:
				res.New++
			}
			res.Events = append(res.Events, GitHubEvent{
				Repo:       repo,
				Number:     issue.Number,
				Thread:     intake.Thread,
				MessageID:  intake.MessageID,
				Route:      intake.Route,
				ReasonCode: intake.ReasonCode,
			})
		}
	}
	return res, nil
}

// formatGitHubPoll renders the one-line text outcome.
//
// A calm pass prints polled/new/deduped (AC-006). A pass that carries a reason
// code prints polled=<n> reason_code=<code> instead, so the no-op case reads
// exactly like the contract says: `polled=0 reason_code=repos_not_configured`.
// The full counts always survive in --outcome json.
func formatGitHubPoll(r GitHubPollResult) string {
	if r.ReasonCode != "" {
		return fmt.Sprintf("polled=%d reason_code=%s", r.Polled, r.ReasonCode)
	}
	return fmt.Sprintf("polled=%d new=%d deduped=%d", r.Polled, r.New, r.Deduped)
}

// githubCoreFor builds the Core the poller submits to. It is a seam, like
// feedCoreFor: tests inject a stub Dispatcher so no paseo binary is ever spawned.
var githubCoreFor = func(cf *commonFlags) (*Core, error) { return cf.core() }

// ---------------------------------------------------------------------------
// CLI: adbd poll-github --once [--root <dir>] [--outcome json|text]
// ---------------------------------------------------------------------------

func runPollGitHub(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("poll-github", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var once bool
	fs.BoolVar(&once, "once", false, "run one poll pass and exit (the launchd rhythm)")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	// The poller has no resident mode: every invocation is exactly one pass, so
	// --once is the scheduled rhythm rather than a switch. It is accepted (and
	// documented) so a launchd job never has to change shape.
	_ = once
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd poll-github: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd poll-github: %v\n", err)
		return 1
	}
	core, err := githubCoreFor(cf)
	if err != nil {
		fmt.Fprintf(stderr, "adbd poll-github: %v\n", err)
		return 1
	}
	poller := &GitHubPoller{
		Repos:   cfg.GitHubRepos,
		Labels:  cfg.GitHubLabels,
		Limit:   cfg.GitHubLimit,
		Bin:     cfg.GHBin,
		Proxy:   cfg.GitHubProxy,
		Channel: DefaultGitHubChannel,
		Runner:  execGHRunner{},
		Core:    core,
	}
	res, pollErr := poller.Poll()

	// The outcome is printed even when the pass failed: "exit 2 and nothing else"
	// is the silent failure AC-005 forbids.
	if cf.outcome == "json" {
		if code := writeJSON(stdout, stderr, res); code != 0 {
			return code
		}
	} else {
		fmt.Fprintln(stdout, formatGitHubPoll(res))
	}

	if pollErr != nil {
		var ge *GitHubError
		if errors.As(pollErr, &ge) {
			fmt.Fprintf(stderr, "adbd poll-github: %s\n", ge.Error())
			return 2
		}
		fmt.Fprintf(stderr, "adbd poll-github: %v\n", pollErr)
		return 1
	}
	return 0
}
