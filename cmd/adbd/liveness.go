// Dispatch liveness: did the worker really start working?
// (feature dispatch-liveness, v0.1.5)
//
// The failure this file exists for is silent and cheap to miss: `paseo send`
// exits 0, the message is received, and the worker never calls its model even
// once -- 0 tokens, the turn aborted, the agent back to idle. From the router's
// side that looked exactly like a successful dispatch.
//
// The fix is a read-only observation, never an interpretation of provider
// internals: before sending, remember how much timeline paseo already recorded
// for that agent (`paseo logs <id> --tail N`); after sending, poll until the
// timeline grows. Growth means alive. No growth before the deadline means dead,
// which the caller turns into a bounded retry and then into `dispatch_dead`.
//
// Two rules make this safe rather than decorative:
//
//   - A probe that FAILS is never "alive" (AC-005). A broken `paseo logs` says
//     `dispatch_unverified`, which is requeued like a death -- the one thing a
//     liveness check must never do is pass work nobody confirmed.
//   - Every wait has a hard upper bound, so the router can not block on an
//     opaque CLI that stopped answering.
//
// Nothing here changes what `paseo send` means (AC-007): the check only adds
// read-only paseo calls around an unchanged send.
package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Dispatch-check defaults (truth.md 配置项).
const (
	// DefaultDispatchTimeout is adbd_worker_dispatch_timeout_sec's default: how
	// long one liveness check may wait for progress.
	DefaultDispatchTimeout = 10 * time.Second
	// DefaultDispatchRetries is adbd_worker_dispatch_retries's default: how many
	// EXTRA sends a dead worker gets.
	DefaultDispatchRetries = 1
	// DefaultDispatchProbeInterval is how often the timeline is re-read inside one
	// check. It is deliberately small next to the timeout: the probe is a single
	// read-only CLI call, so a tighter loop only costs latency the router does not
	// pay for (drains are seconds apart).
	DefaultDispatchProbeInterval = 100 * time.Millisecond
	// DefaultDispatchTail is how many trailing timeline lines the probe reads. The
	// count only has to be *comparable* between two reads, so a fixed window is
	// enough -- and it keeps the probe cheap on a long-running agent.
	DefaultDispatchTail = 200
	// DefaultProbeCommandTimeout bounds the probe command itself. exec.Command has
	// no timeout of its own: without this, one hung `paseo logs` would defeat the
	// wait's deadline and block the drain.
	DefaultProbeCommandTimeout = 20 * time.Second
)

// timelineMarkerPrefixes are the timeline entries that mean "this agent did
// something". They are paseo's own vocabulary, matched as line prefixes because
// the surrounding detail is not this feature's business.
var timelineMarkerPrefixes = []string{
	"[Thought]",
	"[Shell]",
	"[Message received]",
	"[Assistant]",
	"[Tool]",
}

// DispatchDeadError is `paseo send` accepted the text but the worker never
// started: the retries were spent and the timeline never moved (AC-003/AC-004).
// It is a typed error so the router can requeue the item and trace
// decision=dispatch_dead instead of burning an attempt on an infrastructure
// failure.
type DispatchDeadError struct {
	// AgentID is the worker that stayed silent.
	AgentID string
	// Attempts is how many sends were tried in total (the first one included).
	Attempts int
}

func (e *DispatchDeadError) Error() string {
	return fmt.Sprintf("worker %s did not start after %d send(s): no timeline progress", e.AgentID, e.Attempts)
}

// DispatchUnverifiedError is the probe itself failed -- non-zero exit, no output,
// an unusable agent id -- so nothing was learned either way (AC-005). It is NOT a
// death and NOT a success: the item goes back to the queue and the trace says
// dispatch_unverified, because guessing in either direction is worse than asking
// again later.
type DispatchUnverifiedError struct {
	// AgentID is the worker the probe was asked about.
	AgentID string
	// Attempts is how many sends had already gone out when the probe broke.
	Attempts int
	// Err is the probe failure.
	Err error
}

func (e *DispatchUnverifiedError) Error() string {
	return fmt.Sprintf("worker %s: liveness could not be verified after %d send(s): %v", e.AgentID, e.Attempts, e.Err)
}

func (e *DispatchUnverifiedError) Unwrap() error { return e.Err }

// Send hands one text to one agent and, when the check is on, proves the agent
// actually started (AC-002/AC-003). It is the seam PaseoDispatcher.Dispatch
// wraps; the argv of the send itself is unchanged (AC-007).
//
//	check off        -> one send, no probe (the router traces dispatch_unchecked)
//	progress grew    -> alive, nil
//	no progress      -> send again while retries remain, then DispatchDeadError
//	probe failed     -> DispatchUnverifiedError, immediately (never "alive")
func (d PaseoDispatcher) Send(agentID, text string) error {
	if !d.Check {
		return d.sendOnce(agentID, text)
	}
	bin := d.bin()
	// AC-002: the baseline is the timeline this agent already had. A baseline
	// probe that fails is not fatal -- the wait below is what renders the verdict,
	// and it fails just as loudly -- so it degrades to 0, the strictest bar.
	baseline, err := timelineMarkers(bin, agentID)
	if err != nil {
		baseline = 0
	}
	// AC-003: one send plus d.Retries extra sends. The retry deliberately keeps
	// the ORIGINAL baseline: nothing moved after the first send, so any growth
	// after the second one is still proof of life.
	attempts := 1 + d.retries()
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := d.sendOnce(agentID, text); err != nil {
			// The send itself failed (CLI error, unknown agent): that is a dispatch
			// failure with its own meaning, and the probe has nothing to judge.
			return err
		}
		alive, probeErr := d.waitAlive(agentID, baseline, d.timeout(), DefaultDispatchProbeInterval)
		if probeErr != nil {
			// AC-005: never retry blind on an unreadable probe, and never pass it
			// off as success. The item goes back to the queue as unverified.
			return &DispatchUnverifiedError{AgentID: agentID, Attempts: attempt, Err: probeErr}
		}
		if alive {
			return nil
		}
	}
	return &DispatchDeadError{AgentID: agentID, Attempts: attempts}
}

// DispatchChecked reports whether this dispatcher's send path performs the
// liveness check (AC-006). The router reads it through DispatchCheckReporter to
// stamp reason_code=dispatch_unchecked on a row whose send was never verified;
// an unchecked send is not a failure, but the trace must not imply a check
// happened.
func (d PaseoDispatcher) DispatchChecked() bool { return d.Check }

// sendOnce is the unchanged `paseo send <agent-id> <text>` call. The text is a
// single argv element, so no shell ever parses it (AC-007).
func (d PaseoDispatcher) sendOnce(agentID, text string) error {
	bin := d.bin()
	out, err := exec.Command(bin, "send", agentID, text).CombinedOutput()
	if err != nil {
		return fmt.Errorf("paseo send %s: %w (output: %s)", agentID, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// timelineMarkers counts the timeline entries paseo has recorded for agentID
// (AC-002): `paseo logs <agent-id> --tail 200`, counting the lines that start
// with one of timelineMarkerPrefixes.
//
// A missing id, a non-zero exit, a timeout or output that is not text at all are
// errors: "I could not read the timeline" must never be indistinguishable from
// "nothing happened". An EMPTY timeline, on the other hand, is readable and is
// exactly what a freshly created agent looks like -- it counts as zero markers,
// which is what makes "no progress" a verdict (dispatch_dead) rather than a
// probe failure (dispatch_unverified).
func timelineMarkers(bin, agentID string) (int, error) {
	return timelineMarkersWithin(bin, agentID, DefaultProbeCommandTimeout)
}

// timelineMarkersWithin is timelineMarkers with an explicit command ceiling, so
// the probe itself can never outlive the wait that asked for it.
func timelineMarkersWithin(bin, agentID string, timeout time.Duration) (int, error) {
	if bin = strings.TrimSpace(bin); bin == "" {
		bin = "paseo"
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return 0, errors.New("adbd liveness: no agent id to probe")
	}
	if timeout <= 0 {
		timeout = DefaultProbeCommandTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "logs", agentID, "--tail", strconv.Itoa(DefaultDispatchTail))
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail = strings.TrimSpace(string(exitErr.Stderr))
		}
		if ctx.Err() != nil {
			detail = "probe timed out"
		}
		return 0, fmt.Errorf("paseo logs %s --tail %d: %w (output: %s)",
			agentID, DefaultDispatchTail, err, detail)
	}
	return countTimelineMarkers(out)
}

// countTimelineMarkers is the pure half of the probe, so the counting rule can be
// tested without a process. An empty answer is zero markers (a brand-new agent
// has no timeline yet), while output that is not valid text is unusable and says
// so instead of counting as idle.
func countTimelineMarkers(out []byte) (int, error) {
	if !utf8.Valid(out) {
		return 0, errors.New("adbd liveness: paseo logs printed output that is not text")
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		for _, prefix := range timelineMarkerPrefixes {
			if strings.HasPrefix(line, prefix) {
				count++
				break
			}
		}
	}
	return count, nil
}

// waitAlive polls the timeline until it grows past baseline (alive) or the
// deadline passes (dead). It is AC-002's loop and AC-005's bound:
//
//	(true, nil)  progress observed -- the worker started
//	(false, nil) the deadline passed with no progress -- the worker is dead
//	(false, err) the probe failed -- neither alive nor dead (never "alive")
//
// The only blocking call is time.Sleep, and every branch is reached through a
// bounded probe, so the whole call is bounded by timeout plus one probe.
func (d PaseoDispatcher) waitAlive(agentID string, baseline int, timeout, interval time.Duration) (bool, error) {
	if timeout <= 0 {
		timeout = DefaultDispatchTimeout
	}
	if interval <= 0 {
		interval = DefaultDispatchProbeInterval
	}
	bin := d.bin()
	deadline := time.Now().Add(timeout)
	// The first probe runs immediately (an agent that already made progress is
	// alive at once); every later one has to fit inside the deadline.
	first := true
	for first || time.Now().Before(deadline) {
		first = false
		count, err := timelineMarkers(bin, agentID)
		if err != nil {
			return false, err
		}
		if count > baseline {
			return true, nil
		}
		now := time.Now()
		if !now.Before(deadline) {
			break
		}
		wait := interval
		if remaining := deadline.Sub(now); remaining < wait {
			wait = remaining
		}
		time.Sleep(wait)
	}
	return false, nil
}

// timeout is adbd_worker_dispatch_timeout_sec, defaulted for a hand-built
// dispatcher (tests, embedders) as well as the daemon.
func (d PaseoDispatcher) timeout() time.Duration {
	if d.Timeout > 0 {
		return d.Timeout
	}
	return DefaultDispatchTimeout
}

// retries is adbd_worker_dispatch_retries. A negative value is read as zero: the
// retry count can only ever add sends, never remove the first one.
func (d PaseoDispatcher) retries() int {
	if d.Retries < 0 {
		return 0
	}
	return d.Retries
}
