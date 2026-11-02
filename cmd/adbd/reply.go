// Outbound reply delivery for the router (feature reply-delivery, v0.1.5).
//
// Before this file the router decided "reply" and wrote the sentence into the
// trace, where it stayed forever: the front desk answered, and nobody heard it.
// This file is the mouth. It is deliberately tiny and deliberately off by
// default:
//
//   - Off by default (AC-006). No adbd_reply_bin means no process is spawned and
//     the trace says not_configured, so an unconfigured daemon behaves exactly
//     like the one that had no mouth at all.
//   - One channel only (AC-005). Threads are named "<channel>:..."; only a
//     feishu thread has an outbound path in this lake. A skill or gh thread is
//     recorded as unsupported_channel, never retried, never an error.
//   - Honest failure (AC-003). A send that fails does not undo the decision: the
//     item stays applied, the process exit code is untouched, and the trace
//     carries delivered=false plus the sender's own words in delivery_detail.
//   - Idempotent on the sender side (AC-004). The queue's message_id is handed to
//     lark-cli as --idempotency-key, so a replay after a crash cannot post twice.
//
// The sender is an injected interface, so tests drive a real script and never a
// network or a real lark-cli.
package main

import (
	"bytes"
	"os/exec"
	"strings"
)

// Delivery reasons are stable machine tokens, never prose (AC-002).
const (
	// DeliverySent means the sender accepted the message.
	DeliverySent = "sent"
	// DeliveryNotConfigured means there is no sender at all (AC-006).
	DeliveryNotConfigured = "not_configured"
	// DeliveryUnsupportedChannel means this thread has no outbound path (AC-005).
	DeliveryUnsupportedChannel = "unsupported_channel"
	// DeliverySendFailed means the sender ran and refused (AC-003).
	DeliverySendFailed = "send_failed"
)

const (
	// DefaultReplyChannel is the one channel this lake can reply into.
	DefaultReplyChannel = "feishu"
	// FeishuChannelPrefix is how a queue thread spells its channel: threads are
	// "<channel>:...", so the prefix is the whole routing decision.
	FeishuChannelPrefix = "feishu:"
	// DefaultReplyIdentity is adbd_reply_identity's default: a reply comes from
	// the bot, not from a human's account (AC-001).
	DefaultReplyIdentity = "bot"
	// maxDeliveryDetailRunes caps delivery_detail at 200 characters (AC-003):
	// enough to diagnose a refusal, small enough to keep a trace line readable.
	maxDeliveryDetailRunes = 200
)

// Replier is the router's only way to speak outside this process. It returns the
// delivery_reason the trace records, an optional detail (the sender's stderr, used
// only when delivery failed) and the underlying error.
//
// A Replier that returns an error has still been asked a question it must answer
// in the trace: the router records send_failed rather than guessing.
type Replier interface {
	Reply(thread, messageID, text string) (deliveryReason, detail string, err error)
}

// LarkReplier delivers a reply through the lark-cli binary (AC-001):
//
//	<bin> im +messages-send --chat-id <thread> --text <reply> \
//	      --as <identity> --idempotency-key <message_id>
//
// Bin is the executable (adbd_reply_bin); Identity is the sender identity
// (adbd_reply_identity, default "bot"). Nothing here reads the network: the
// binary does, and in tests the binary is a shell script.
type LarkReplier struct {
	Bin      string
	Identity string
}

// compile-time proof that the lark sender satisfies the injection point.
var _ Replier = LarkReplier{}

// Reply sends one reply. It returns DeliverySendFailed together with whatever the
// sender said on stderr when it exits non-zero; the router turns that into a
// capped trace field, never into a failed item.
func (r LarkReplier) Reply(thread, messageID, text string) (string, string, error) {
	identity := strings.TrimSpace(r.Identity)
	if identity == "" {
		identity = DefaultReplyIdentity
	}
	cmd := exec.Command(
		r.Bin,
		"im", "+messages-send",
		// The thread id carries the channel prefix ("feishu:oc_xxx"); lark wants
		// the bare chat id.
		"--chat-id", strings.TrimPrefix(thread, "feishu:"),
		"--text", text,
		"--as", identity,
		"--idempotency-key", messageID,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return DeliverySendFailed, strings.TrimSpace(stderr.String()), err
	}
	return DeliverySent, "", nil
}

// feishuThread reports whether a thread belongs to the feishu channel. Threads
// carry their channel as a prefix ("feishu:oc_123"), so this is the whole check.
func feishuThread(thread string) bool {
	return strings.HasPrefix(strings.TrimSpace(thread), FeishuChannelPrefix)
}

// replyChannel is the operator-selected outbound channel, defaulting to feishu.
func (r *Router) replyChannel() string {
	if r == nil {
		return DefaultReplyChannel
	}
	if channel := strings.TrimSpace(r.ReplyChannel); channel != "" {
		return channel
	}
	return DefaultReplyChannel
}

// replySupported reports whether this thread can be delivered to at all (AC-005).
// The default feishu check is the frozen one; a configured channel only ever
// narrows it, so a mistyped adbd value cannot invent an outbound path.
func (r *Router) replySupported(thread string) bool {
	channel := r.replyChannel()
	if strings.EqualFold(channel, DefaultReplyChannel) {
		return feishuThread(thread)
	}
	return strings.HasPrefix(strings.TrimSpace(thread), channel+":")
}

// attachDelivery fills a reply trace row with what actually happened outbound
// (AC-002) and sends the reply when the daemon is allowed to.
//
// A send failure never fails the item (AC-003): the verdict is already applied,
// the item has already left the queue, and the trace records the truth instead.
// Nothing in here can change the drain's error or the process exit code.
func (r *Router) attachDelivery(rec *traceRecord, item QueueItem, decision Decision) {
	if r == nil || r.Replier == nil {
		// AC-006: switched off (the default). The verdict is still recorded; the
		// row says plainly that nothing was sent.
		rec.Delivered = deliveryFlag(false)
		rec.DeliveryReason = DeliveryNotConfigured
		return
	}
	if !r.replySupported(item.Thread) {
		// AC-005: a skill or gh thread has no outbound path in this lake. That is
		// a fact about the thread, not a failure, so nothing is retried.
		rec.Delivered = deliveryFlag(false)
		rec.DeliveryReason = DeliveryUnsupportedChannel
		return
	}

	reason, detail, err := r.Replier.Reply(item.Thread, item.MessageID, replyText(item, decision))
	if err != nil || reason == "" {
		// AC-003: an error, or a sender that said nothing at all, is a failed
		// send -- never an assumed success.
		reason = DeliverySendFailed
	}
	rec.Delivered = deliveryFlag(reason == DeliverySent)
	rec.DeliveryReason = reason
	if reason == DeliverySendFailed {
		// This is the only place the cap lives: the sender's words go in whole and
		// the trace keeps the first 200 characters (AC-003).
		rec.DeliveryDetail = truncateRunes(strings.TrimSpace(detail), maxDeliveryDetailRunes)
	}
}

// deliveryFlag is the pointer the trace field needs: nil means "this verdict had
// no outbound step at all", false means "there was one and it did not land".
func deliveryFlag(delivered bool) *bool {
	return &delivered
}

// truncateRunes keeps the first n characters of s (runes, not bytes: a UTF-8
// message must not be cut in half).
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
