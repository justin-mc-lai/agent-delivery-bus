// Skill trigger intake for the Agent Delivery Bus (truth.md, feature
// skill-trigger, v0.1.5).
//
// The lake is one command: a session utterance plus a (project, slug) pair
// becomes a threaded event that walks the intake path the daemon already has.
// Everything that needs to be exact is derived here and nowhere else:
//
//   - the thread shape is frozen at skill:<project>:<slug>, with project and
//     slug limited to [a-z0-9._-]{1,64}, so a typo cannot invent a new thread
//     and two projects can never share one (AC-003);
//   - the idempotency key is sha256(channel|thread|text)[:16], so the same
//     sentence repeated in a session is deduped instead of dispatched twice
//     (AC-002);
//   - the message is handed to Core.Intake -- the very same kernel `adbd intake`
//     uses -- so feed owns no routing logic of its own and can never disagree
//     with the daemon (AC-001, AC-004);
//   - every failure carries a stable reason_code and a non-zero exit, because a
//     trigger that silently succeeds is worse than one that fails loudly
//     (AC-005);
//   - the command is one synchronous call: no heartbeat, no launchd, no resident
//     process, so it works the same in a bare shell (AC-006).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Feed defaults and frozen shapes (truth.md AC-002/AC-003, 配置项).
const (
	// DefaultFeedChannel is the channel a skill-triggered event arrives on when
	// neither --channel nor adbd_feed_project_channel names one.
	DefaultFeedChannel = "skill"
	// feedThreadPrefix is the only accepted thread prefix.
	feedThreadPrefix = "skill"
	// feedMessageIDHexLen is how many hex characters of the digest become the
	// idempotency key: 64 bits of a content hash, short enough to read in a
	// trace, wide enough that two utterances colliding is not a practical worry.
	feedMessageIDHexLen = 16
)

// Feed reason codes. They are stable machine tokens, so a caller (or a test) can
// branch on them without parsing prose (AC-005).
const (
	ReasonThreadInvalid  = "thread_invalid"
	ReasonTextEmpty      = "text_empty"
	ReasonDispatchFailed = "dispatch_failed"
)

// feedPartPattern is the frozen project/slug alphabet: lowercase ASCII, digits,
// dot, underscore, dash; 1 to 64 characters. Anything else is rejected instead
// of sanitized, so a caller learns about the typo instead of getting a quietly
// renamed thread.
var feedPartPattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)

// feedCoreFor builds the Core the feed command delegates to. It is a seam: tests
// inject a stub Runner (and so no paseo binary is ever spawned), exactly like
// deciderFor does for the router.
var feedCoreFor = func(cf *commonFlags) (*Core, error) { return cf.core() }

// feedNow is the clock a fed event is stamped with. Tests replace it so lease,
// dedupe and trace timestamps are exact.
var feedNow = func() time.Time { return time.Now() }

// FeedError is a feed failure that has a stable reason code the CLI can map to
// exit code 2. Any other error is an infrastructure failure (exit code 1).
type FeedError struct {
	Code   string
	Detail string
}

func (e *FeedError) Error() string {
	detail := strings.TrimSpace(e.Detail)
	if detail == "" {
		return e.Code
	}
	return e.Code + ": " + detail
}

// IsFeedError reports whether err is a feed failure with this reason code. An
// empty code matches any feed failure.
func IsFeedError(err error, code string) bool {
	var fe *FeedError
	if !errors.As(err, &fe) {
		return false
	}
	return code == "" || fe.Code == code
}

// FeedThread validates the two thread parts and returns skill:<project>:<slug>.
// It rejects rather than repairs: a thread is an identity, and an identity that
// the caller did not ask for is a bug, not a convenience.
func FeedThread(project, slug string) (string, error) {
	if !feedPartPattern.MatchString(project) || !feedPartPattern.MatchString(slug) {
		return "", &FeedError{
			Code: ReasonThreadInvalid,
			Detail: fmt.Sprintf("project %q and slug %q must each match %s",
				project, slug, feedPartPattern.String()),
		}
	}
	return feedThreadPrefix + ":" + project + ":" + slug, nil
}

// FeedMessageID is the content idempotency key: the first 16 hex characters of
// sha256(channel|thread|text). Same channel, thread and words -> same key, so the
// second identical trigger is deduped instead of dispatched again (AC-002).
func FeedMessageID(channel, thread, text string) string {
	sum := sha256.Sum256([]byte(channel + "|" + thread + "|" + text))
	return hex.EncodeToString(sum[:])[:feedMessageIDHexLen]
}

// FeedRequest is one trigger request, in the shape a session produces it.
type FeedRequest struct {
	// Project is the registered project slug.
	Project string
	// Slug is the feature (or task) slug inside that project.
	Slug string
	// Text is the utterance to feed. It is untrusted data, never instructions.
	Text string
	// Channel is the source channel; empty falls back to defaultChannel, then to
	// DefaultFeedChannel.
	Channel string
	// MessageID is the idempotency key; empty derives it from the content.
	MessageID string
	// TracePath is where a fast-path delivery is recorded; empty means
	// <slow-path dir>/adbd-decisions.jsonl.
	TracePath string
}

// FeedResult is the trigger outcome in the shape the CLI prints: route and
// reason_code first, then enough identity to rerun or audit the call.
type FeedResult struct {
	Route      string `json:"route"`
	ReasonCode string `json:"reason_code,omitempty"`
	Channel    string `json:"channel"`
	Thread     string `json:"thread"`
	MessageID  string `json:"message_id"`
	Project    string `json:"project,omitempty"`
	Slug       string `json:"slug,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	State      string `json:"state,omitempty"`
	Lease      string `json:"lease,omitempty"`
	Forwarded  bool   `json:"forwarded"`
	// Queued is true only when feed itself put the item back on the slow path
	// because the hand-off to a worker failed (AC-005).
	Queued bool `json:"queued"`
}

// feedRunner flags a hand-off that never reached the worker, so feed can report
// dispatch_failed instead of blaming the state store. It forwards untouched.
type feedRunner struct {
	inner  Runner
	failed bool
}

func (r *feedRunner) Send(agentID, text string) error {
	if r.inner == nil {
		r.failed = true
		return errors.New("adbd feed: no runner configured")
	}
	if err := r.inner.Send(agentID, text); err != nil {
		r.failed = true
		return err
	}
	return nil
}

// Feed turns one trigger request into one threaded event and submits it to the
// intake kernel (AC-001). It validates first, submits second, and never repairs
// a caller mistake (AC-003, AC-005).
func Feed(core *Core, req FeedRequest, defaultChannel string) (FeedResult, error) {
	res := FeedResult{Project: req.Project, Slug: req.Slug}

	thread, err := FeedThread(req.Project, req.Slug)
	if err != nil {
		res.Route = RouteRejected
		res.ReasonCode = ReasonThreadInvalid
		return res, err
	}
	res.Thread = thread

	// AC-005: empty and all-whitespace text are the same mistake, and neither may
	// reach the queue.
	if strings.TrimSpace(req.Text) == "" {
		res.Route = RouteRejected
		res.ReasonCode = ReasonTextEmpty
		return res, &FeedError{Code: ReasonTextEmpty, Detail: "text is empty or all whitespace"}
	}

	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = strings.TrimSpace(defaultChannel)
	}
	if channel == "" {
		channel = DefaultFeedChannel
	}
	res.Channel = channel

	messageID := strings.TrimSpace(req.MessageID)
	if messageID == "" {
		messageID = FeedMessageID(channel, thread, req.Text)
	}
	res.MessageID = messageID

	if core == nil || core.Store == nil {
		return res, errors.New("adbd feed: no core configured")
	}

	// The guard line goes on at this boundary: a session utterance is untrusted
	// data (router AC-007), and the fastpath hands text to a worker unchanged, so
	// the tag has to exist before intake sees it. guardedText is idempotent, so
	// the slow path does not end up with a second copy of the tag.
	guarded := guardedText(req.Text)

	// The Core is copied, not mutated: the caller's value stays exactly as it was
	// and the runner that keeps the error flag is only visible to this call.
	tracked := &feedRunner{inner: core.Runner}
	routed := *core
	routed.Runner = tracked

	in := IntakeInput{Channel: channel, Thread: thread, MessageID: messageID, Text: guarded}
	intake, err := routed.Intake(in)
	if err != nil {
		if !tracked.failed {
			// Not the worker's fault: an unreadable state store or a busy lock.
			// Report it as infrastructure, never as a dispatch failure.
			return res, err
		}
		// AC-005: a failed hand-off keeps the message. It goes back on the slow
		// path with its own reason code, where the router retries it under its
		// attempt budget instead of the message evaporating.
		requeueErr := core.Store.appendSlowPath(slowPathRecord{
			Channel:    channel,
			Thread:     thread,
			MessageID:  messageID,
			Text:       guarded,
			ReasonCode: ReasonDispatchFailed,
			RecordedAt: core.now().Format(time.RFC3339),
		})
		if requeueErr != nil {
			return res, fmt.Errorf("adbd feed: dispatch failed (%v) and the requeue failed too: %w", err, requeueErr)
		}
		res.Route = RouteSlow
		res.ReasonCode = ReasonDispatchFailed
		res.Queued = true
		return res, &FeedError{Code: ReasonDispatchFailed, Detail: err.Error()}
	}

	res.Route = intake.Route
	res.ReasonCode = intake.ReasonCode
	res.AgentID = intake.AgentID
	res.State = intake.State
	res.Lease = intake.Lease
	res.Forwarded = intake.Forwarded

	// AC-004: exactly one audit line per delivered message. The router traces the
	// slow path when it applies an item; a fast-path delivery never reaches the
	// router, so feed records it here in the same frozen shape.
	if intake.Route == RouteFast {
		if err := recordFeedDelivery(core, in, req.TracePath); err != nil {
			return res, err
		}
	}
	return res, nil
}

// recordFeedDelivery appends the one decision-trace line a fast-path delivery
// earns. It reuses the router's trace record and appender, so the audit trail has
// one format and one reader.
func recordFeedDelivery(core *Core, in IntakeInput, tracePath string) error {
	path := strings.TrimSpace(tracePath)
	if path == "" {
		path = filepath.Join(filepath.Dir(core.Store.SlowPath), decisionsFile)
	}
	rec := newTrace(
		QueueItem{Channel: in.Channel, Thread: in.Thread, MessageID: in.MessageID},
		DecisionRoute, ReasonBoundThread, 1, core.now(),
	)
	return appendTrace(path, []traceRecord{rec})
}

// formatFeed renders the text outcome; route= is always the first field and
// reason_code= second, matching the intake output the rest of the CLI prints.
func formatFeed(r FeedResult) string {
	parts := []string{"route=" + r.Route}
	if r.ReasonCode != "" {
		parts = append(parts, "reason_code="+r.ReasonCode)
	}
	parts = append(parts, "channel="+r.Channel, "thread="+r.Thread, "message_id="+r.MessageID)
	if r.AgentID != "" {
		parts = append(parts, "agent_id="+r.AgentID)
	}
	if r.State != "" {
		parts = append(parts, "state="+r.State)
	}
	if r.Lease != "" {
		parts = append(parts, "lease="+r.Lease)
	}
	if r.Queued {
		parts = append(parts, "queued=true")
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// CLI: adbd feed --project P --slug S --text T [--channel C] [--message-id M]
// ---------------------------------------------------------------------------

func runFeed(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feed", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var project, slug, text, channel, messageID, tracePath string
	fs.StringVar(&project, "project", "", "project slug")
	fs.StringVar(&slug, "slug", "", "feature slug inside the project")
	fs.StringVar(&text, "text", "", "utterance to feed")
	fs.StringVar(&channel, "channel", "", "source channel, defaults to config adbd_feed_project_channel or "+DefaultFeedChannel)
	fs.StringVar(&messageID, "message-id", "", "idempotency key, defaults to sha256(channel|thread|text)[:16]")
	fs.StringVar(&tracePath, "trace", "", "decision trace path")
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd feed: %v\n", err)
		return 2
	}
	cfg, err := LoadConfig(cf.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "adbd feed: %v\n", err)
		return 1
	}
	core, err := feedCoreFor(cf)
	if err != nil {
		fmt.Fprintf(stderr, "adbd feed: %v\n", err)
		return 1
	}
	core.Now = feedNow
	if tracePath == "" {
		tracePath = envOr("ADBD_TRACE", filepath.Join(cf.root, ".beacon", decisionsFile))
	}

	res, err := Feed(core, FeedRequest{
		Project:   project,
		Slug:      slug,
		Text:      text,
		Channel:   channel,
		MessageID: messageID,
		TracePath: tracePath,
	}, cfg.FeedChannel)

	// The outcome is printed even when the call failed: a trigger that says only
	// "exit 2" is exactly the silent failure AC-005 forbids.
	if cf.outcome == "json" {
		if code := writeJSON(stdout, stderr, res); code != 0 {
			return code
		}
	} else {
		fmt.Fprintln(stdout, formatFeed(res))
	}

	if err != nil {
		var fe *FeedError
		if errors.As(err, &fe) {
			fmt.Fprintf(stderr, "adbd feed: %s\n", fe.Error())
			return 2
		}
		fmt.Fprintf(stderr, "adbd feed: %v\n", err)
		return 1
	}
	// An intake rejection (e.g. the channel is not on the allow-list) is not a
	// success: nothing was delivered and nothing was queued.
	if res.Route == RouteRejected {
		return 2
	}
	return 0
}
