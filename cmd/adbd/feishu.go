// Feishu inbound mapping for the Agent Delivery Bus (truth.md, feature
// feishu-inbound, v0.1.5).
//
// The lake is one command: `adbd feishu-feed` reads the NDJSON stream
// `lark-cli event consume` prints on stdout, maps every usable event onto a
// threaded event and submits it to Core.Intake -- the very same kernel `adbd
// intake`, `adbd feed` and `adbd poll-github` use. This file owns no socket, no
// reconnection and no rich-text parsing (those stay with lark-cli):
//
//   - the thread is frozen at feishu:<chat_id>, so one group chat is one
//     conversation and a typo cannot invent a new one (AC-001);
//   - the idempotency key is the Feishu message_id the event carries, so a
//     redelivered event is deduped by the kernel instead of being handed to a
//     worker twice (AC-005);
//   - message_type must be text; anything else is skipped with
//     reason=unsupported_message_type and the stream keeps going (AC-002);
//   - sender_type=bot is skipped with reason=from_bot, so the bot never answers
//     itself (AC-003);
//   - a line that is not JSON, or that lacks chat_id/message_id/content, is
//     skipped with reason=invalid_json / reason=missing_field, per line, without
//     stopping the lines that follow (AC-004);
//   - a bad line is data, not a crash: after the (root, outcome) flags are
//     accepted the command exits 0 in every case -- unreadable input included --
//     and the last line it prints is always
//     accepted=<n> skipped=<m> deduped=<k> (AC-001, AC-004).
//
// What it deliberately does not do: no reply, no router decision, no state of
// its own. It is a pure mapping from event lines to IntakeInput.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Frozen shapes for this source (truth.md 映射规则).
const (
	// DefaultFeishuChannel is the channel a Feishu event arrives on. It is a
	// constant, not a config key: the thread already carries the chat identity,
	// and one source refusing to share a channel keeps the allow-list honest.
	DefaultFeishuChannel = "feishu"
	// feishuThreadPrefix is the only accepted thread prefix for this source.
	feishuThreadPrefix = "feishu"
	// feishuReadBuffer is the initial stdin buffer. Lines are read with
	// bufio.Reader, which grows past it, so a big event is never truncated.
	feishuReadBuffer = 64 * 1024
	// feishuReasonDetailLimit is how much of a kernel error a skip line may quote.
	feishuReasonDetailLimit = 200
)

// Feishu skip reasons. Stable machine tokens, never prose: an operator (or a
// test) can branch on them without parsing a sentence (AC-002..AC-004).
const (
	ReasonFeishuInvalidJSON            = "invalid_json"
	ReasonFeishuMissingField           = "missing_field"
	ReasonFeishuFromBot                = "from_bot"
	ReasonFeishuUnsupportedMessageType = "unsupported_message_type"
	// ReasonFeishuIntakeFailed is not a data problem: the kernel itself could not
	// account for the event (unreadable state store, busy lock). It is reported
	// per line and never aborts the stream, because the remaining events are
	// still worth mapping.
	ReasonFeishuIntakeFailed = "intake_failed"
)

// FeishuEvent is one line of the lark-cli event stream, in the shape the
// consumer prints it. Unknown fields are ignored on purpose: lark-cli may add
// more later and that is not a reason to drop a message.
type FeishuEvent struct {
	ChatID      string `json:"chat_id"`
	MessageID   string `json:"message_id"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
	SenderType  string `json:"sender_type"`
}

// FeishuOutcome is the verdict for one input line, kept for --outcome json and
// for the per-line text lines. Line is 1-based over the non-blank lines read.
type FeishuOutcome struct {
	Line       int    `json:"line"`
	Skip       string `json:"reason,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Route      string `json:"route,omitempty"`
	ReasonCode string `json:"reason_code,omitempty"`
	Thread     string `json:"thread,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
}

// FeishuFeedResult is one stream in the shape the CLI prints.
//
// Invariant for a stream with no kernel failure: Lines ==
// Accepted + Skipped + Deduped. Reasons counts every skipped line by reason, so
// "skipped=3" can always be explained.
type FeishuFeedResult struct {
	Accepted int `json:"accepted"`
	Skipped  int `json:"skipped"`
	Deduped  int `json:"deduped"`
	// Lines counts the non-blank lines that were looked at.
	Lines   int             `json:"lines"`
	Reasons map[string]int  `json:"reasons,omitempty"`
	Events  []FeishuOutcome `json:"events,omitempty"`
}

// FeishuThread is the frozen thread shape for one group chat: feishu:<chat_id>.
func FeishuThread(chatID string) string {
	return feishuThreadPrefix + ":" + strings.TrimSpace(chatID)
}

// FeishuEventKeyFields is the ordered list of fields an event cannot do without.
// It exists so the missing_field skip names what was actually absent.
func FeishuEventKeyFields(ev FeishuEvent) []string {
	var missing []string
	if strings.TrimSpace(ev.ChatID) == "" {
		missing = append(missing, "chat_id")
	}
	if strings.TrimSpace(ev.MessageID) == "" {
		missing = append(missing, "message_id")
	}
	if strings.TrimSpace(ev.Content) == "" {
		missing = append(missing, "content")
	}
	return missing
}

// ClassifyFeishuLine turns one raw line into either an IntakeInput or a skip
// reason. The order of the checks is the contract (AC-002..AC-004):
//
//	① not JSON                                        -> invalid_json
//	② chat_id/message_id/content missing or empty      -> missing_field
//	③ sender_type=bot                                 -> from_bot
//	④ message_type != text                            -> unsupported_message_type
//	⑤ otherwise                                       -> submit to intake
//
// Nothing is repaired and nothing is guessed: a line that reaches intake is a
// line that carried every field the mapping rule names.
func ClassifyFeishuLine(raw string) (IntakeInput, string, error) {
	var ev FeishuEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return IntakeInput{}, ReasonFeishuInvalidJSON, nil
	}
	if missing := FeishuEventKeyFields(ev); len(missing) > 0 {
		return IntakeInput{}, ReasonFeishuMissingField, fmt.Errorf("%s missing or empty", strings.Join(missing, ", "))
	}
	if strings.EqualFold(strings.TrimSpace(ev.SenderType), "bot") {
		return IntakeInput{}, ReasonFeishuFromBot, nil
	}
	if !strings.EqualFold(strings.TrimSpace(ev.MessageType), "text") {
		return IntakeInput{}, ReasonFeishuUnsupportedMessageType, nil
	}
	return IntakeInput{
		Channel:   DefaultFeishuChannel,
		Thread:    FeishuThread(ev.ChatID),
		MessageID: strings.TrimSpace(ev.MessageID),
		// The content is a chat message written by whoever is in the group, so it
		// enters the bus as untrusted data. guardedText is idempotent, so the
		// router's own guard does not stack a second tag on it.
		Text: guardedText(ev.Content),
	}, "", nil
}

// FeishuFeed reads NDJSON from r, submits every usable event to core and
// returns the counts. It returns an error only when the stream itself cannot be
// read (or no core was handed in); a line that fails is counted, never fatal.
func FeishuFeed(core *Core, r io.Reader) (FeishuFeedResult, error) {
	res := FeishuFeedResult{Reasons: map[string]int{}}
	if core == nil {
		return res, errors.New("adbd feishu-feed: no core configured")
	}
	if r == nil {
		return res, errors.New("adbd feishu-feed: no input stream")
	}
	reader := bufio.NewReaderSize(r, feishuReadBuffer)
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r")
		// A blank line is formatting, not an event: it is neither counted nor
		// reported, so a trailing newline never shows up as invalid_json.
		if strings.TrimSpace(line) != "" {
			res.submit(core, line)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return res, nil
			}
			return res, readErr
		}
	}
}

// submit accounts for one non-blank line: skip it with a reason, or let the
// kernel route it and count what the kernel decided.
func (res *FeishuFeedResult) submit(core *Core, line string) {
	res.Lines++
	n := res.Lines

	in, skip, detail := ClassifyFeishuLine(line)
	if skip != "" {
		res.skip(FeishuOutcome{Line: n, Skip: skip, Detail: truncateFeishu(detail)})
		return
	}

	result, err := core.Intake(in)
	if err != nil {
		res.skip(FeishuOutcome{Line: n, Skip: ReasonFeishuIntakeFailed, Detail: truncateFeishu(err)})
		return
	}

	out := FeishuOutcome{
		Line:       n,
		Route:      result.Route,
		ReasonCode: result.ReasonCode,
		Thread:     result.Thread,
		MessageID:  result.MessageID,
	}
	switch result.Route {
	case RouteDeduped:
		// AC-005: the same message_id arrived twice. The kernel saw it before, so
		// nothing was handed to a worker and the count says so.
		res.Deduped++
	case RouteFast, RouteSlow:
		// Fast and slow are both "taken in": the message is either with a worker
		// or on the queue for one. The route tells the operator which.
		res.Accepted++
	default:
		// Rejected (e.g. the channel is not on the allow-list). Accounted for
		// under the kernel's own reason code, never counted as accepted work.
		out.Skip = result.ReasonCode
		if out.Skip == "" {
			out.Skip = result.Route
		}
		res.skip(out)
		return
	}
	res.Events = append(res.Events, out)
}

// skip records one skipped line and its reason.
func (res *FeishuFeedResult) skip(out FeishuOutcome) {
	res.Skipped++
	res.Reasons[out.Skip]++
	res.Events = append(res.Events, out)
}

// truncateFeishu keeps a detail string short, in runes so a UTF-8 message is
// never cut mid-character.
func truncateFeishu(err error) string {
	if err == nil {
		return ""
	}
	runes := []rune(strings.TrimSpace(err.Error()))
	if len(runes) <= feishuReasonDetailLimit {
		return string(runes)
	}
	return string(runes[:feishuReasonDetailLimit])
}

// formatFeishuReasons renders the reason tally deterministically (alphabetical),
// so two runs over the same stream print byte-identical output.
func formatFeishuReasons(reasons map[string]int) string {
	if len(reasons) == 0 {
		return ""
	}
	keys := make([]string, 0, len(reasons))
	for key := range reasons {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+strconv.Itoa(reasons[key]))
	}
	return strings.Join(parts, ",")
}

// formatFeishuFeed renders the text outcome: one line per event that needs
// explaining, then the summary line.
//
// The summary line is always last and always starts with
// accepted=<n> skipped=<m> deduped=<k> (AC-001, AC-005). The reason tally is
// appended, never prefixed, so a caller can read the three counts without
// parsing the rest.
func formatFeishuFeed(res FeishuFeedResult) []string {
	lines := make([]string, 0, len(res.Events)+1)
	for _, ev := range res.Events {
		if ev.Skip != "" {
			parts := []string{fmt.Sprintf("line=%d", ev.Line), "skipped", "reason=" + ev.Skip}
			if ev.Detail != "" {
				parts = append(parts, fmt.Sprintf("detail=%q", ev.Detail))
			}
			lines = append(lines, strings.Join(parts, " "))
			continue
		}
		parts := []string{
			fmt.Sprintf("line=%d", ev.Line),
			"accepted",
			"route=" + ev.Route,
			"thread=" + ev.Thread,
			"message_id=" + ev.MessageID,
		}
		if ev.ReasonCode != "" {
			parts = append(parts, "reason_code="+ev.ReasonCode)
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	summary := fmt.Sprintf("accepted=%d skipped=%d deduped=%d", res.Accepted, res.Skipped, res.Deduped)
	if reasons := formatFeishuReasons(res.Reasons); reasons != "" {
		summary += " reasons=" + reasons
	}
	return append(lines, summary)
}

// feishuCoreFor builds the Core the mapper submits to. It is a seam, like
// feedCoreFor and githubCoreFor: tests inject a stub Dispatcher, so no paseo
// binary is ever started and no network is touched.
var feishuCoreFor = func(cf *commonFlags) (*Core, error) { return cf.core() }

// feishuStdin is the stream feishu-feed maps. It is a seam so a test can hand the
// command a fixture instead of a terminal.
var feishuStdin io.Reader = os.Stdin

// ---------------------------------------------------------------------------
// CLI: adbd feishu-feed [--root <dir>] [--outcome json|text]
// ---------------------------------------------------------------------------

func runFeishuFeed(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feishu-feed", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cf := addCommonFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return flagExit(err, stderr)
	}
	if err := cf.resolve(); err != nil {
		fmt.Fprintf(stderr, "adbd feishu-feed: %v\n", err)
		return 2
	}
	core, err := feishuCoreFor(cf)
	if err != nil {
		// Nothing was read yet, so this is an environment failure, not a bad
		// line: it is the one case that may still exit non-zero.
		fmt.Fprintf(stderr, "adbd feishu-feed: %v\n", err)
		return 1
	}
	in := feishuStdin
	if in == nil {
		in = os.Stdin
	}
	res, readErr := FeishuFeed(core, in)

	// The outcome is printed even when the stream broke: a run that only says
	// "exit 1" tells an operator nothing about what was already taken in.
	if cf.outcome == "json" {
		if code := writeJSON(stdout, stderr, res); code != 0 {
			return code
		}
	} else {
		for _, line := range formatFeishuFeed(res) {
			fmt.Fprintln(stdout, line)
		}
	}
	if readErr != nil {
		fmt.Fprintf(stderr, "adbd feishu-feed: %v\n", readErr)
	}
	// A bad line is data, not a crash: whatever came in, this command ends 0.
	return 0
}
