// Package main implements adbd, the Agent Delivery Bus daemon surface for the
// intake fastpath.
//
// Scope (truth.md, feature intake-fastpath, v0.1.5):
//   - a bound (channel, thread) delivers follow-up messages straight to the
//     worker agent that already owns the conversation, without paying for a
//     Router/LLM decision again;
//   - an unbound thread, an expired lease or an unknown source falls back to the
//     slow path instead of guessing;
//   - a replayed message is deduped so the worker is never fed twice;
//   - the binding lifecycle is a small state machine with illegal moves rejected.
//
// Storage note: this module is stdlib-only, and the stdlib ships no sqlite
// driver, so runtime state lives in one JSON document guarded by an O_EXCL lock
// file and written atomically (temp file + rename). The shape of the data is
// still the SQLite schema from truth.md (binding row + idempotency key with a
// uniqueness guarantee, enforced here by the map key).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Thread binding states (truth.md "状态模型").
const (
	StateUnbound  = "unbound"
	StateBound    = "bound"
	StateLeasing  = "leasing"
	StateExpiring = "expiring"
	StateReleased = "released"
)

// Intake routes. These are the four values the caller is allowed to see.
const (
	RouteFast     = "fast"
	RouteSlow     = "slow"
	RouteDeduped  = "deduped"
	RouteRejected = "rejected"
)

// Reason codes carried next to a non-fast route.
const (
	ReasonSourceNotAllowed = "source_not_allowed"
	ReasonNoBinding        = "no_binding"
	ReasonBindingReleased  = "binding_released"
	ReasonLeaseExpired     = "lease_expired"
	ReasonDuplicateMessage = "duplicate_message"
)

// Lease labels printed as lease=<label>.
const (
	LeaseOK       = "ok"
	LeaseExpiring = "expiring"
	LeaseExpired  = "expired"
	LeaseNone     = "none"
)

// Defaults agreed on in truth.md Open Questions Q1/Q2.
const (
	DefaultTTL            = 24 * time.Hour
	DefaultExpiringWindow = time.Hour

	stateVersion    = 1
	dedupeRetention = 7 * 24 * time.Hour
	lockStaleAfter  = 30 * time.Second
	lockWaitTimeout = 3 * time.Second
	lockPollEvery   = 2 * time.Millisecond
)

// Event is a state-machine event on the thread_binding axis.
type Event string

const (
	EventBind            Event = "bind"
	EventIntake          Event = "intake"
	EventIntakeDuplicate Event = "intake_duplicate"
	EventReport          Event = "report"
	EventExpire          Event = "expire"
	EventExpireRelease   Event = "expire_release"
	EventRelease         Event = "release"
)

// TransitionError reports a rejected move. Code is a stable machine token, so a
// caller can branch on it without parsing prose.
type TransitionError struct {
	Code  string
	From  string
	Event Event
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("%s: illegal transition %s --%s-->", e.Code, e.From, e.Event)
}

// IsTransitionError reports whether err is a rejected state-machine move, and if
// so which reason code it carried.
func IsTransitionError(err error, code string) bool {
	var te *TransitionError
	if !errors.As(err, &te) {
		return false
	}
	return code == "" || te.Code == code
}

func reject(code, from string, ev Event) (string, error) {
	return "", &TransitionError{Code: code, From: from, Event: ev}
}

// Transition is the single source of truth for thread_binding moves
// (truth.md state table). Every move not listed there is rejected, including
// the three explicitly illegal ones: released->bound, unbound->leasing and
// expiring->released.
func Transition(from string, ev Event) (string, error) {
	switch ev {
	case EventBind:
		switch from {
		case StateUnbound:
			return StateBound, nil
		case StateReleased:
			return reject(ReasonBindingReleased, from, ev)
		case StateBound, StateLeasing, StateExpiring:
			return reject("binding_exists", from, ev)
		}
	case EventIntake:
		// bound -> leasing, and a live lease stays leasing while it forwards.
		switch from {
		case StateBound, StateLeasing, StateExpiring:
			return StateLeasing, nil
		case StateUnbound:
			// illegal: unbound -> leasing (a slow-path thread may not jump to fastpath)
			return reject("unbound_cannot_lease", from, ev)
		case StateReleased:
			return reject(ReasonBindingReleased, from, ev)
		}
	case EventIntakeDuplicate:
		// A replayed message changes no state; it only gets accounted for.
		switch from {
		case StateBound, StateLeasing, StateExpiring:
			return from, nil
		case StateUnbound:
			return reject("unbound_cannot_lease", from, ev)
		case StateReleased:
			return reject(ReasonBindingReleased, from, ev)
		}
	case EventReport:
		// Any worker report renews the lease, including from expiring.
		switch from {
		case StateBound:
			return StateBound, nil
		case StateLeasing, StateExpiring:
			return StateLeasing, nil
		case StateUnbound:
			return reject("unbound_cannot_lease", from, ev)
		case StateReleased:
			return reject(ReasonBindingReleased, from, ev)
		}
	case EventExpire:
		switch from {
		case StateBound, StateLeasing, StateExpiring:
			return StateExpiring, nil
		case StateUnbound:
			return reject(ReasonNoBinding, from, ev)
		case StateReleased:
			return reject(ReasonBindingReleased, from, ev)
		}
	case EventExpireRelease:
		switch from {
		case StateBound, StateLeasing, StateExpiring:
			return StateUnbound, nil
		case StateUnbound:
			return reject(ReasonNoBinding, from, ev)
		case StateReleased:
			return reject(ReasonBindingReleased, from, ev)
		}
	case EventRelease:
		// illegal: expiring -> released (an expiring lease must expire into unbound)
		switch from {
		case StateBound, StateLeasing:
			return StateReleased, nil
		case StateExpiring:
			return reject("illegal_transition", from, ev)
		case StateUnbound:
			return reject(ReasonNoBinding, from, ev)
		case StateReleased:
			return reject(ReasonBindingReleased, from, ev)
		}
	}
	return reject("unknown_transition", from, ev)
}

func isLiveState(state string) bool {
	return state == StateBound || state == StateLeasing || state == StateExpiring
}

// Runner forwards one message to one worker agent.
type Runner interface {
	Send(agentID, text string) error
}

// PaseoRunner shells out to `paseo send <agent-id> <text>`. paseo is treated as
// an opaque external CLI: no import, no forking, argv only.
type PaseoRunner struct {
	Bin string
}

func (r PaseoRunner) Send(agentID, text string) error {
	bin := strings.TrimSpace(r.Bin)
	if bin == "" {
		bin = "paseo"
	}
	out, err := exec.Command(bin, "send", agentID, text).CombinedOutput()
	if err != nil {
		return fmt.Errorf("runner %s send %s: %w (output: %s)", bin, agentID, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Binding is one thread_binding row.
type Binding struct {
	Channel string `json:"channel"`
	Thread  string `json:"thread"`
	AgentID string `json:"agent_id"`
	State   string `json:"state"`
	// TTL is the lease length granted by bind, so a renewal keeps the same
	// lease length instead of silently jumping to the daemon default.
	TTL       time.Duration `json:"ttl_ns"`
	ExpiresAt time.Time     `json:"expires_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// leaseTTL is the lease length this binding renews to.
func (c *Core) leaseTTL(b *Binding) time.Duration {
	if b != nil && b.TTL > 0 {
		return b.TTL
	}
	return c.ttl()
}

// snapshot is the whole persisted state document.
type snapshot struct {
	Version    int                  `json:"version"`
	Bindings   map[string]*Binding  `json:"bindings"`
	DedupeKeys map[string]time.Time `json:"dedupe_keys"`
}

func newSnapshot() *snapshot {
	return &snapshot{
		Version:    stateVersion,
		Bindings:   map[string]*Binding{},
		DedupeKeys: map[string]time.Time{},
	}
}

// bindingKey and dedupeKey use a NUL separator so no channel/thread/message-id
// pair can collide with another one by construction.
func bindingKey(channel, thread string) string {
	return channel + "\x00" + thread
}

func dedupeKey(channel, thread, messageID string) string {
	return channel + "\x00" + thread + "\x00" + messageID
}

// slowPathRecord is one line of the slow-path queue (JSONL, append only).
type slowPathRecord struct {
	Channel    string `json:"channel"`
	Thread     string `json:"thread"`
	MessageID  string `json:"message_id"`
	Text       string `json:"text"`
	ReasonCode string `json:"reason_code"`
	RecordedAt string `json:"recorded_at"`
}

// Store keeps runtime state in one JSON document plus an append-only slow-path
// queue. Writes are serialized by an O_EXCL lock file and land atomically.
type Store struct {
	// Path is the state document, e.g. .beacon/adbd-state.json.
	Path string
	// SlowPath is the slow-path queue, e.g. .beacon/adbd-slowpath.jsonl.
	SlowPath string
	// LockPath defaults to Path + ".lock".
	LockPath string
}

func (s *Store) lockPath() string {
	if s.LockPath != "" {
		return s.LockPath
	}
	return s.Path + ".lock"
}

func (s *Store) acquireLock() (func(), error) {
	lock := s.lockPath()
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(lockWaitTimeout)
	for {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
			_ = f.Close()
			return func() { _ = os.Remove(lock) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		// A crashed holder must not wedge the daemon forever.
		if info, statErr := os.Stat(lock); statErr == nil && time.Since(info.ModTime()) > lockStaleAfter {
			_ = os.Remove(lock)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("state lock busy: %s", lock)
		}
		time.Sleep(lockPollEvery)
	}
}

func (s *Store) load() (*snapshot, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newSnapshot(), nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return newSnapshot(), nil
	}
	snap := newSnapshot()
	if err := json.Unmarshal(data, snap); err != nil {
		return nil, fmt.Errorf("corrupt state document %s: %w", s.Path, err)
	}
	if snap.Bindings == nil {
		snap.Bindings = map[string]*Binding{}
	}
	if snap.DedupeKeys == nil {
		snap.DedupeKeys = map[string]time.Time{}
	}
	return snap, nil
}

func (s *Store) save(snap *snapshot) error {
	if snap.Bindings == nil {
		snap.Bindings = map[string]*Binding{}
	}
	if snap.DedupeKeys == nil {
		snap.DedupeKeys = map[string]time.Time{}
	}
	snap.Version = stateVersion
	// Keep the idempotency table bounded; replay windows are hours, not months.
	// The cutoff is derived from the newest recorded key, not the wall clock, so
	// pruning stays deterministic for an injected clock in tests.
	var newest time.Time
	for _, seenAt := range snap.DedupeKeys {
		if seenAt.After(newest) {
			newest = seenAt
		}
	}
	cutoff := newest.Add(-dedupeRetention)
	for key, seenAt := range snap.DedupeKeys {
		if seenAt.Before(cutoff) {
			delete(snap.DedupeKeys, key)
		}
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".adbd-state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// Update runs fn against a locked, freshly loaded snapshot and persists the
// result atomically. fn returning an error rolls the whole update back.
func (s *Store) Update(fn func(*snapshot) error) error {
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return errors.New("adbd: state path is empty")
	}
	release, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer release()
	snap, err := s.load()
	if err != nil {
		return err
	}
	if err := fn(snap); err != nil {
		return err
	}
	return s.save(snap)
}

// View runs fn against a locked snapshot without persisting it.
func (s *Store) View(fn func(*snapshot) error) error {
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return errors.New("adbd: state path is empty")
	}
	release, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer release()
	snap, err := s.load()
	if err != nil {
		return err
	}
	return fn(snap)
}

func (s *Store) appendSlowPath(rec slowPathRecord) error {
	if strings.TrimSpace(s.SlowPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.SlowPath), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.SlowPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// Config is the operator-facing adbd configuration from .beacon/config.
type Config struct {
	// AllowAll is true only when adbd_allow_all=true is set explicitly.
	// Absent allow-list is fail-closed (AC-005: unknown source is rejected).
	AllowAll bool
	// AllowedChannels is adbd_allow_channels, comma separated.
	AllowedChannels []string
	// PaseoBin is adbd_paseo_bin, the runner binary path.
	PaseoBin string
	// RouterAgent is adbd_router_agent, the worker the router loop dispatches
	// new slow-path work to. Empty means the daemon default ("main").
	RouterAgent string
}

// LoadConfig reads a key=value config file. A missing file, or a file without
// adbd_allow_channels, is fail-closed: every channel is rejected until the
// operator names the allowed ones (AC-005). adbd_allow_all=true is the
// explicit, opt-in escape for local development.
func LoadConfig(path string) (Config, error) {
	cfg := Config{}
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue // section header, not a key
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if strings.EqualFold(values["adbd_allow_all"], "true") {
		cfg.AllowAll = true
	}
	if raw, ok := values["adbd_allow_channels"]; ok {
		var listed []string
		for _, channel := range strings.Split(raw, ",") {
			if channel = strings.TrimSpace(channel); channel != "" {
				listed = append(listed, channel)
			}
		}
		if len(listed) > 0 {
			cfg.AllowAll = false
			cfg.AllowedChannels = listed
		}
	}
	cfg.PaseoBin = values["adbd_paseo_bin"]
	cfg.RouterAgent = values["adbd_router_agent"]
	return cfg, nil
}

// Core is the intake fastpath kernel:
// allow-list -> dedupe -> lease expiry -> forward through the runner.
type Core struct {
	Store          *Store
	Runner         Runner
	TTL            time.Duration
	ExpiringWindow time.Duration
	Now            func() time.Time
	AllowAll       bool
	Allowed        map[string]bool
}

// NewCore builds a Core with the frozen defaults (TTL 24h, expiring window 1h).
func NewCore(store *Store, runner Runner) *Core {
	return &Core{
		Store:          store,
		Runner:         runner,
		TTL:            DefaultTTL,
		ExpiringWindow: DefaultExpiringWindow,
		Now:            time.Now,
		AllowAll:       false,
		Allowed:        map[string]bool{},
	}
}

func (c *Core) now() time.Time {
	if c.Now == nil {
		return time.Now().UTC()
	}
	return c.Now().UTC()
}

func (c *Core) ttl() time.Duration {
	if c.TTL <= 0 {
		return DefaultTTL
	}
	return c.TTL
}

func (c *Core) expiringWindow() time.Duration {
	if c.ExpiringWindow <= 0 {
		return DefaultExpiringWindow
	}
	return c.ExpiringWindow
}

func (c *Core) channelAllowed(channel string) bool {
	if c.AllowAll || len(c.Allowed) == 0 {
		return true
	}
	return c.Allowed[strings.TrimSpace(channel)]
}

// leaseLabel turns "time left on the lease" into the operator-facing label.
func leaseLabel(b *Binding, now time.Time, window time.Duration) string {
	if b == nil || !isLiveState(b.State) {
		return LeaseNone
	}
	if !now.Before(b.ExpiresAt) {
		return LeaseExpired
	}
	if b.ExpiresAt.Sub(now) < window {
		return LeaseExpiring
	}
	return LeaseOK
}

// IntakeInput is one caller-fed intake event.
type IntakeInput struct {
	Channel   string
	Thread    string
	MessageID string
	Text      string
}

// IntakeResult is the decision plus the accounting the caller needs.
type IntakeResult struct {
	Route      string `json:"route"`
	ReasonCode string `json:"reason_code,omitempty"`
	Channel    string `json:"channel"`
	Thread     string `json:"thread"`
	MessageID  string `json:"message_id"`
	AgentID    string `json:"agent_id,omitempty"`
	State      string `json:"state,omitempty"`
	Lease      string `json:"lease,omitempty"`
	Forwarded  bool   `json:"forwarded"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

// Intake routes one message. It never guesses: no binding, an expired lease or a
// replayed message all resolve without calling the runner.
func (c *Core) Intake(in IntakeInput) (IntakeResult, error) {
	res := IntakeResult{
		Channel:   in.Channel,
		Thread:    in.Thread,
		MessageID: in.MessageID,
	}
	if strings.TrimSpace(in.Channel) == "" || strings.TrimSpace(in.Thread) == "" || strings.TrimSpace(in.MessageID) == "" {
		return res, errors.New("adbd: --channel, --thread and --message-id are required")
	}
	// AC-005: unknown source is rejected before any state or runner work.
	if !c.channelAllowed(in.Channel) {
		res.Route = RouteRejected
		res.ReasonCode = ReasonSourceNotAllowed
		res.State = StateUnbound
		res.Lease = LeaseNone
		return res, nil
	}
	if c.Runner == nil {
		return res, errors.New("adbd: no runner configured")
	}

	now := c.now()
	dkey := bindingKey(in.Channel, in.Thread)
	idemKey := dedupeKey(in.Channel, in.Thread, in.MessageID)

	err := c.Store.Update(func(snap *snapshot) error {
		binding := snap.Bindings[dkey]

		// AC-003: webhook replay. Seen before -> account for it, forward nothing.
		if _, seen := snap.DedupeKeys[idemKey]; seen {
			res.Route = RouteDeduped
			res.ReasonCode = ReasonDuplicateMessage
			res.State = StateUnbound
			res.Lease = LeaseNone
			if binding != nil {
				res.AgentID = binding.AgentID
				res.State = binding.State
				res.Lease = leaseLabel(binding, now, c.expiringWindow())
			}
			return nil
		}

		// AC-004: the lease ran out -> unbind, and this message takes the slow path.
		if binding != nil && isLiveState(binding.State) && !now.Before(binding.ExpiresAt) {
			next, err := Transition(binding.State, EventExpireRelease)
			if err != nil {
				return err
			}
			applyState(binding, next, now)
			snap.DedupeKeys[idemKey] = now
			return c.toSlowPath(in, ReasonLeaseExpired, StateUnbound, LeaseExpired, now, &res)
		}

		// AC-005: nothing bound here (or the binding was explicitly released).
		if binding == nil || binding.State == StateUnbound || binding.State == StateReleased {
			reason := ReasonNoBinding
			state := StateUnbound
			if binding != nil && binding.State == StateReleased {
				reason = ReasonBindingReleased
				state = StateReleased
			}
			snap.DedupeKeys[idemKey] = now
			return c.toSlowPath(in, reason, state, LeaseNone, now, &res)
		}

		// AC-004: the lease warning is decided on the time left BEFORE the renewal,
		// otherwise renewing to now+TTL would always look healthy and a quiet
		// worker would never be flagged.
		remainingBefore := binding.ExpiresAt.Sub(now)

		// AC-002: live binding -> exactly one forward, then renew the lease.
		if err := c.Runner.Send(binding.AgentID, in.Text); err != nil {
			return err
		}
		next, err := Transition(binding.State, EventIntake)
		if err != nil {
			return err
		}
		binding.State = next
		binding.ExpiresAt = now.Add(c.leaseTTL(binding))
		binding.UpdatedAt = now

		lease := LeaseOK
		if remainingBefore < c.expiringWindow() {
			// AC-004: remaining < 1h -> expiring, still delivered.
			expiring, err := Transition(binding.State, EventExpire)
			if err != nil {
				return err
			}
			binding.State = expiring
			lease = LeaseExpiring
		}

		snap.DedupeKeys[idemKey] = now
		res.Route = RouteFast
		res.ReasonCode = ""
		res.AgentID = binding.AgentID
		res.State = binding.State
		res.Lease = lease
		res.Forwarded = true
		res.ExpiresAt = binding.ExpiresAt.Format(time.RFC3339)
		return nil
	})
	if err != nil {
		return res, err
	}
	return res, nil
}

func (c *Core) toSlowPath(in IntakeInput, reason, state, lease string, now time.Time, res *IntakeResult) error {
	rec := slowPathRecord{
		Channel:    in.Channel,
		Thread:     in.Thread,
		MessageID:  in.MessageID,
		Text:       in.Text,
		ReasonCode: reason,
		RecordedAt: now.Format(time.RFC3339),
	}
	if err := c.Store.appendSlowPath(rec); err != nil {
		return err
	}
	res.Route = RouteSlow
	res.ReasonCode = reason
	res.State = state
	res.Lease = lease
	res.Forwarded = false
	return nil
}

func applyState(b *Binding, state string, now time.Time) {
	b.State = state
	b.UpdatedAt = now
	if state == StateUnbound || state == StateReleased {
		b.AgentID = ""
		b.ExpiresAt = time.Time{}
	}
}

// Bind creates the (channel, thread) -> agent-id binding, valid for ttl.
// AC-006: one worker per thread; a released thread may not be revived.
func (c *Core) Bind(channel, thread, agentID string, ttl time.Duration) (Binding, error) {
	var out Binding
	if strings.TrimSpace(channel) == "" || strings.TrimSpace(thread) == "" {
		return out, errors.New("adbd: --channel and --thread are required")
	}
	if strings.TrimSpace(agentID) == "" {
		return out, errors.New("adbd: --agent-id is required")
	}
	if ttl <= 0 {
		return out, errors.New("adbd: --ttl must be positive")
	}
	now := c.now()
	err := c.Store.Update(func(snap *snapshot) error {
		key := bindingKey(channel, thread)
		if existing := snap.Bindings[key]; existing != nil && existing.State != StateUnbound {
			if _, err := Transition(existing.State, EventBind); err != nil {
				return err
			}
		}
		binding := &Binding{
			Channel:   channel,
			Thread:    thread,
			AgentID:   agentID,
			State:     StateBound,
			TTL:       ttl,
			ExpiresAt: now.Add(ttl),
			UpdatedAt: now,
		}
		snap.Bindings[key] = binding
		out = *binding
		return nil
	})
	return out, err
}

// Release explicitly unbinds a live thread. expiring -> released is illegal.
func (c *Core) Release(channel, thread string) (Binding, error) {
	var out Binding
	now := c.now()
	err := c.Store.Update(func(snap *snapshot) error {
		binding := snap.Bindings[bindingKey(channel, thread)]
		if binding == nil {
			return &TransitionError{Code: ReasonNoBinding, From: StateUnbound, Event: EventRelease}
		}
		next, err := Transition(binding.State, EventRelease)
		if err != nil {
			return err
		}
		applyState(binding, next, now)
		out = *binding
		return nil
	})
	return out, err
}

// Report renews the lease after a successful worker report.
func (c *Core) Report(channel, thread string) (Binding, error) {
	var out Binding
	now := c.now()
	err := c.Store.Update(func(snap *snapshot) error {
		binding := snap.Bindings[bindingKey(channel, thread)]
		if binding == nil {
			return &TransitionError{Code: ReasonNoBinding, From: StateUnbound, Event: EventReport}
		}
		next, err := Transition(binding.State, EventReport)
		if err != nil {
			return err
		}
		binding.State = next
		binding.ExpiresAt = now.Add(c.leaseTTL(binding))
		binding.UpdatedAt = now
		out = *binding
		return nil
	})
	return out, err
}

// StatusResult is the operator view of one thread.
type StatusResult struct {
	Channel          string `json:"channel"`
	Thread           string `json:"thread"`
	State            string `json:"state"`
	AgentID          string `json:"agent_id,omitempty"`
	Lease            string `json:"lease"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	RemainingSeconds int64  `json:"remaining_seconds,omitempty"`
}

// Status reads one thread and lazily applies the time-driven moves, so
// bound -> leasing -> expiring -> unbound is observable without a background sweep.
func (c *Core) Status(channel, thread string) (StatusResult, error) {
	out := StatusResult{Channel: channel, Thread: thread, State: StateUnbound, Lease: LeaseNone}
	now := c.now()
	err := c.Store.Update(func(snap *snapshot) error {
		binding := snap.Bindings[bindingKey(channel, thread)]
		if binding == nil {
			return nil
		}
		if !isLiveState(binding.State) {
			out.State = binding.State
			out.Lease = LeaseNone
			return nil
		}
		if !now.Before(binding.ExpiresAt) {
			next, err := Transition(binding.State, EventExpireRelease)
			if err != nil {
				return err
			}
			applyState(binding, next, now)
			out.State = StateUnbound
			out.Lease = LeaseExpired
			return nil
		}
		remaining := binding.ExpiresAt.Sub(now)
		if remaining < c.expiringWindow() {
			if binding.State != StateExpiring {
				next, err := Transition(binding.State, EventExpire)
				if err != nil {
					return err
				}
				binding.State = next
				binding.UpdatedAt = now
			}
		} else if binding.State == StateExpiring {
			// The lease was renewed since the reminder: it is healthy again.
			next, err := Transition(binding.State, EventReport)
			if err != nil {
				return err
			}
			binding.State = next
			binding.UpdatedAt = now
		}
		out.State = binding.State
		out.AgentID = binding.AgentID
		out.Lease = leaseLabel(binding, now, c.expiringWindow())
		out.ExpiresAt = binding.ExpiresAt.Format(time.RFC3339)
		out.RemainingSeconds = int64(remaining.Seconds())
		return nil
	})
	return out, err
}
