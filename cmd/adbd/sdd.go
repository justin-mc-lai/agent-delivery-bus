// Structural SDD gate for the router (feature worker-sdd-binding, v0.1.5).
//
// Why this file exists: before it, a dispatched worker was governed by words
// only -- the prompt asked it to look at the requirement, and nothing stopped it
// from editing code no frozen acceptance criterion covers. Code moves, the truth
// does not: that is drift. This file adds the structure instead of another
// sentence.
//
// The rule is deliberately narrow, and it is asked per item, after the source
// trust grade and long before any worker is created:
//
//	no project directory resolves            -> guard_only
//	project is not Beacon-managed            -> not_applicable
//	Beacon-managed + skill:<project>:<slug>  -> gated, subject to the truth check
//	Beacon-managed + gh:<owner>/<repo>#<n>   -> gated, subject to the bug binding
//	Beacon-managed + any other thread        -> guard_only
//	adbd_sdd_gate=off                        -> gated_off (never refuse)
//
// "Beacon-managed" means the project carries <repo>/<docsRoot>/<version>/
// features/ for at least one version directory; the highest version wins. The
// judgment is recorded on every trace row as `sdd`, so an operator can audit the
// gate from the audit trail alone instead of re-running it.
//
// Two refusals sit inside the gated lane, and they are the point of this file:
// feature_not_registered (that slug has no truth.md) and truth_not_frozen
// (truth.md exists but its frontmatter status is not `frozen`). A refusal writes
// one trace row, keeps the queue item and touches no worker binding, so the work
// is retried after the truth is frozen instead of being lost -- and retrying
// alone never bypasses the gate, because every drain re-reads the truth.
//
// A GitHub issue is the second gated shape (feature gh-bug-binding, v0.1.5): a
// bug report has to declare the frozen acceptance criterion it violates with a
// `beacon:ac=<slug>#<AC-ID>` label, or carry `beacon:no-ac` when it is a chore.
// No declaration (or an ambiguous one) is issue_unbound; a declaration whose
// package is unfrozen is truth_not_frozen; one whose package does not contain
// that AC is ac_not_found. A bound, frozen, existing AC dispatches with the AC
// reference in the text and the trace. adbd_bug_binding=off falls that lane back
// to guard_only and the row says binding_off.
package main

import (
	"os"
	"path/filepath"
	"strings"
)

// SDD lanes: the value the trace carries in its `sdd` field (AC-001, AC-007).
const (
	// SDDLaneGated: a Beacon-managed project whose feature truth is frozen, so
	// the worker may be dispatched to.
	SDDLaneGated = "gated"
	// SDDLaneGuardOnly: no feature can be resolved from the thread (a gh: thread,
	// a thread naming no known project), so the work is dispatched with the SDD
	// guard paragraph and nothing is refused.
	SDDLaneGuardOnly = "guard_only"
	// SDDLaneNotApplicable: the project is not Beacon-managed, so the bus keeps
	// its old behaviour (AC-006).
	SDDLaneNotApplicable = "not_applicable"
	// SDDLaneGatedOff: adbd_sdd_gate=off. The gate judges nothing and refuses
	// nothing (AC-007).
	SDDLaneGatedOff = "gated_off"
)

// SDD refusal reason codes. They are stable machine tokens, never prose.
const (
	// ReasonFeatureNotRegistered: the thread names a slug Beacon knows nothing
	// about -- no <slug>/truth.md anywhere in the project's Beacon tree.
	ReasonFeatureNotRegistered = "feature_not_registered"
	// ReasonTruthNotFrozen: the feature is registered but its truth.md status is
	// not `frozen`, so the intent is not settled yet.
	ReasonTruthNotFrozen = "truth_not_frozen"
)

// Beacon tree layout. The docs root itself is configurable
// (adbd_beacon_docs_root); these two names are the frozen part of the shape.
const (
	// DefaultBeaconDocsRoot is adbd_beacon_docs_root's default.
	DefaultBeaconDocsRoot = "docs/beacon"
	// beaconFeaturesDir is the directory that makes a version directory a Beacon
	// tree rather than an unrelated folder that happens to live under docs/.
	beaconFeaturesDir = "features"
	// beaconTruthFile is the requirement package's authoritative file. Its
	// frontmatter `status` is the only thing read here (truth Open Question Q1:
	// the package is the authority, so no beacon-go CLI is needed).
	beaconTruthFile = "truth.md"
	// frozenStatus is the one frontmatter status that may be dispatched to.
	frozenStatus = "frozen"
)

// sddGuard is the fixed guard paragraph every dispatched text starts with when
// the SDD lane is gated or guard_only (AC-005). It names the command to run and
// the one rule that matters, in two sentences, so a worker that reads only the
// first lines still knows both.
const sddGuard = "SDD: run `beacon-go spine show --project . --feature <slug>` first. " +
	"Do not modify code that no frozen acceptance criterion covers."

// sddVerdict is one item's grade. Lane is always set; Refuse is true only for
// the refusals, which also name their ReasonCode.
type sddVerdict struct {
	Lane   string
	Refuse bool
	// ReasonCode is the stable token the trace carries: a refusal's reason, or
	// the explanatory code of a lane that never refuses (issue_exempt_no_ac).
	ReasonCode string
	// ACRef is `<slug>#<AC-ID>` for a gh issue that declared its binding. It is
	// set even on a refusal, so the audit row keeps both the claim and the
	// verdict (gh-bug-binding AC-007).
	ACRef string
	// Binding is the bug-binding gate's own state for a gh thread: BindingOn, or
	// BindingOff when adbd_bug_binding=off. Empty for non-gh threads.
	Binding string
}

// sddGateItem grades one queue item. It reads the filesystem (the project
// directory, the Beacon tree, one truth.md and the files next to it) and nothing
// else: no network, no subprocess, so a drain holding the queue lock cannot block
// on an external CLI.
func (r *Router) sddGateItem(item QueueItem) sddVerdict {
	// AC-007: an operator who turned the gate off gets no refusal at all. The
	// lane still says so, so the trace shows the gate was off rather than
	// guessing that every project happened to be fine.
	if !r.SDDGate {
		return sddVerdict{Lane: SDDLaneGatedOff}
	}
	thread := strings.TrimSpace(item.Thread)
	repo, ok := r.sddProjectRoot(thread)
	if !ok {
		// No project directory resolves: there is nothing to grade against, so
		// the work goes out with the guard instead of being refused for a reason
		// the caller cannot fix.
		return sddVerdict{Lane: SDDLaneGuardOnly}
	}
	version, managed := beaconManaged(repo, r.beaconDocsRoot())
	if !managed {
		// AC-006: not a Beacon project (content, research, a plain checkout), so
		// nothing about this dispatch changes. For a gh issue that also means the
		// bug-binding gate has no baseline to hang on: the dispatch is untouched.
		return sddVerdict{Lane: SDDLaneNotApplicable}
	}
	if _, isGitHub := ghThreadRepo(thread); isGitHub {
		// gh-bug-binding: an issue must declare the frozen AC it violates. The
		// old guard_only answer was the loophole this lake closes.
		return r.bugBindingGate(item, repo, version)
	}
	slug, ok := featureSlugFromThread(thread)
	if !ok {
		// A Beacon project, but the thread cannot be mapped onto a requirement
		// (a feishu thread, a malformed skill thread): dispatch, guarded, no
		// refusal.
		return sddVerdict{Lane: SDDLaneGuardOnly}
	}
	status, err := readTruthStatus(featureTruthPath(repo, r.beaconDocsRoot(), version, slug))
	if err != nil {
		// AC-003: the feature is not registered in this tree. A read failure
		// (permissions, a directory in place of the file) is indistinguishable
		// from a missing file at this level, and both mean "no frozen truth to
		// dispatch against".
		return sddVerdict{Lane: SDDLaneGated, Refuse: true, ReasonCode: ReasonFeatureNotRegistered}
	}
	if !isFrozenStatus(status) {
		// AC-002: registered but not frozen. Every other status -- draft,
		// superseded, empty -- refuses; only `frozen` opens the gate.
		return sddVerdict{Lane: SDDLaneGated, Refuse: true, ReasonCode: ReasonTruthNotFrozen}
	}
	return sddVerdict{Lane: SDDLaneGated}
}

// ---------------------------------------------------------------------------
// gh bug binding (feature gh-bug-binding, v0.1.5)
// ---------------------------------------------------------------------------

// Bug-binding label grammar. The issue declares its own ownership with a label;
// the router never infers it from the issue body (frozen non-goal: 不猜归属).
const (
	// bugACLabelPrefix is `beacon:ac=<slug>#<AC-ID>`.
	bugACLabelPrefix = "beacon:ac="
	// bugNoACLabel is the explicit exemption for chores that are not a contract
	// violation: `beacon:no-ac`.
	bugNoACLabel = "beacon:no-ac"
	// bugBindingClause is the sentence every bound dispatch must carry (AC-002):
	// fixing one criterion may not silently break another.
	bugBindingClause = "回归必须让其它 AC 保持绿"
	// bugBindingTokenLen caps one side of a binding (slug, AC id), matching the
	// frozen thread alphabet's 64-character ceiling.
	bugBindingTokenLen = 64
)

// Bug-binding reason codes. Stable machine tokens, never prose.
const (
	// ReasonIssueUnbound: a Beacon-managed gh issue that declared no frozen AC
	// (no label, a malformed label, or more than one ac label).
	ReasonIssueUnbound = "issue_unbound"
	// ReasonIssueExemptNoAC: the issue carried beacon:no-ac, so it is a chore and
	// is dispatched with the SDD guard instead of being refused.
	ReasonIssueExemptNoAC = "issue_exempt_no_ac"
	// ReasonACNotFound: the issue declared a frozen package that does not contain
	// the AC id it claims to violate.
	ReasonACNotFound = "ac_not_found"
)

// Bug-binding trace tokens, written to the `binding` field of every gh row so the
// audit trail states whether the gate judged it at all.
const (
	// BindingOn: the gate was on and graded the issue.
	BindingOn = "on"
	// BindingOff: adbd_bug_binding=off, so the gh lane fell back to guard_only.
	// The token is deliberately literal `binding_off`, which is what the frozen
	// truth promises an operator will see.
	BindingOff = "binding_off"
)

// bugBinding is one issue's declaration, as parsed from its labels.
type bugBinding struct {
	// Bound is true only for exactly one well-formed ac label.
	Bound bool
	// Exempt is true only for beacon:no-ac with no ac label beside it.
	Exempt bool
	Slug   string
	ACID   string
}

// ref is the trace's ac_ref value: `<slug>#<AC-ID>`, empty when nothing was
// declared.
func (b bugBinding) ref() string {
	if !b.Bound {
		return ""
	}
	return b.Slug + "#" + b.ACID
}

// parseBugBindingLabels reads the declaration out of an issue's labels (AC-001).
//
//	beacon:ac=<slug>#<AC-ID>  -> Bound (exactly one, well-formed)
//	beacon:no-ac             -> Exempt
//	nothing / malformed      -> neither (unbound)
//	two or more ac labels    -> neither (ambiguous is not bound)
//
// Every ambiguity resolves to "unbound", because guessing which criterion an
// issue violates is exactly what the frozen non-goal forbids. Matching is
// case-insensitive and trims spaces: a label is typed by a human.
func parseBugBindingLabels(labels []string) bugBinding {
	var (
		bound     bugBinding
		acCount   int
		exempt    bool
		malformed bool
	)
	for _, raw := range labels {
		label := strings.TrimSpace(raw)
		if label == "" {
			continue
		}
		lowered := strings.ToLower(label)
		if lowered == bugNoACLabel {
			exempt = true
			continue
		}
		if !strings.HasPrefix(lowered, bugACLabelPrefix) {
			// Another `beacon:ac*` spelling (no `=`, a dash) is a typo the caller
			// must fix, not a criterion to guess at.
			if strings.HasPrefix(lowered, "beacon:ac") {
				malformed = true
			}
			continue
		}
		slug, acID, ok := parseBugACValue(label[len(bugACLabelPrefix):])
		if !ok {
			malformed = true
			continue
		}
		acCount++
		bound.Slug, bound.ACID = slug, acID
	}
	switch {
	case malformed, acCount > 1, acCount == 1 && exempt:
		// Ambiguous in every direction: refuse to guess.
		return bugBinding{}
	case acCount == 1:
		bound.Bound = true
		return bound
	case exempt:
		return bugBinding{Exempt: true}
	default:
		return bugBinding{}
	}
}

// parseBugACValue splits `<slug>#<AC-ID>`. Both sides must be a binding token and
// there must be exactly one `#`, so `a#AC-1#x` is a malformed label rather than
// two answers.
func parseBugACValue(value string) (slug, acID string, ok bool) {
	slug, acID, found := strings.Cut(value, "#")
	if !found || strings.Contains(acID, "#") {
		return "", "", false
	}
	if !isBindingToken(slug) || !isBindingToken(acID) {
		return "", "", false
	}
	return slug, acID, true
}

// isBindingToken reports whether s is a non-empty, bounded token of letters,
// digits, dot, underscore and dash. AC ids are upper case (AC-001), so this is
// deliberately not the lowercase thread alphabet.
func isBindingToken(s string) bool {
	if s == "" || len(s) > bugBindingTokenLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// bugBindingGate grades one gh issue against the frozen requirement package
// (AC-002 .. AC-006). repo and version are already resolved by sddGateItem, so
// this function only reads the package itself.
func (r *Router) bugBindingGate(item QueueItem, repo, version string) sddVerdict {
	// AC-007 (config): the switch. Off means the gh lane keeps its pre-feature
	// behaviour -- the SDD guard, no refusal -- and the row says binding_off, so a
	// disabled gate is never mistaken for a passing one.
	if !r.BugBinding {
		return sddVerdict{Lane: SDDLaneGuardOnly, Binding: BindingOff}
	}
	binding := parseBugBindingLabels(item.Labels)
	if !binding.Bound {
		if binding.Exempt {
			// AC-004: an explicit exemption is a chore, not a contract
			// violation. It dispatches, guarded, and the row says why.
			return sddVerdict{
				Lane:       SDDLaneGuardOnly,
				ReasonCode: ReasonIssueExemptNoAC,
				Binding:    BindingOn,
			}
		}
		// AC-003: no declaration (or an ambiguous one) means the issue cannot say
		// which frozen criterion it breaks. Refuse and keep it in the queue.
		return sddVerdict{Lane: SDDLaneGated, Refuse: true, ReasonCode: ReasonIssueUnbound, Binding: BindingOn}
	}
	ref := binding.ref()
	dir := featurePackageDir(repo, r.beaconDocsRoot(), version, binding.Slug)
	status, err := readTruthStatus(filepath.Join(dir, beaconTruthFile))
	if err != nil {
		// The slug names a package that does not exist in this tree: there is no
		// baseline at all, which is the same verdict as a missing feature.
		return sddVerdict{
			Lane:       SDDLaneGated,
			Refuse:     true,
			ReasonCode: ReasonFeatureNotRegistered,
			ACRef:      ref,
			Binding:    BindingOn,
		}
	}
	if !isFrozenStatus(status) {
		// AC-005: the same judgment a skill thread gets. An unfrozen baseline is
		// not a baseline, so the bug waits for a refreeze instead of being fixed
		// against a moving target.
		return sddVerdict{
			Lane:       SDDLaneGated,
			Refuse:     true,
			ReasonCode: ReasonTruthNotFrozen,
			ACRef:      ref,
			Binding:    BindingOn,
		}
	}
	if !packageHasAC(dir, binding.ACID) {
		// AC-006: the package is frozen but does not contain the AC the issue
		// claims to violate -- the label is stale or invented.
		return sddVerdict{
			Lane:       SDDLaneGated,
			Refuse:     true,
			ReasonCode: ReasonACNotFound,
			ACRef:      ref,
			Binding:    BindingOn,
		}
	}
	// AC-002: bound, frozen, and the criterion exists. The lane is gated and the
	// AC reference travels with the dispatch text and the trace.
	return sddVerdict{Lane: SDDLaneGated, ACRef: ref, Binding: BindingOn}
}

// packageHasAC answers "does this requirement package contain that AC id?".
//
// The search is token-matched over the package's own markdown files: the frozen
// rule is "包内出现该 AC ID", and a substring hit inside `AC-0010` must not count
// as `AC-001`. truth.md is the authority; the sibling files are the same package
// and are read only so a criterion spelled out in tests.md still counts.
func packageHasAC(dir, acID string) bool {
	if !isBindingToken(acID) {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		if containsToken(string(data), acID) {
			return true
		}
	}
	return false
}

// containsToken reports whether doc contains token delimited by characters that
// are not part of a binding token, so `AC-001` is found in `| AC-001 |` and not
// in `AC-0010`.
func containsToken(doc, token string) bool {
	for start := 0; start+len(token) <= len(doc); {
		index := strings.Index(doc[start:], token)
		if index < 0 {
			return false
		}
		index += start
		end := index + len(token)
		leftOK := index == 0 || !isBindingByte(doc[index-1])
		rightOK := end == len(doc) || !isBindingByte(doc[end])
		if leftOK && rightOK {
			return true
		}
		start = index + 1
	}
	return false
}

// isBindingByte is one byte of a binding token (see isBindingToken).
func isBindingByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '.', c == '_', c == '-':
		return true
	}
	return false
}

// bugBindingParagraph is the block prepended to a bound issue's dispatch text
// (AC-002): the AC reference the fix must align with, and the regression rule.
// Two lines, so a worker reading only the head of the message still knows both.
func bugBindingParagraph(acRef string) string {
	return "绑定 AC: " + acRef + "\n" +
		"本次修复必须对齐 " + acRef + "；" + bugBindingClause + "。"
}

// beaconDocsRoot is adbd_beacon_docs_root with its default applied. The default
// lives here, not in Router, so a Router built by hand and a Router built by the
// CLI agree about where the tree is.
func (r *Router) beaconDocsRoot() string {
	if root := strings.TrimSpace(r.BeaconDocsRoot); root != "" {
		return root
	}
	return DefaultBeaconDocsRoot
}

// sddProjectRoot resolves the repository a thread names, through the same
// registry the dispatcher uses for `--cwd` (thread-workdir). Reusing that
// resolution is deliberate: the directory the gate inspects is exactly the
// directory the worker would be started in, so the two can never disagree.
func (r *Router) sddProjectRoot(thread string) (string, bool) {
	key := threadWorkdirKey(thread)
	if key == "" {
		return "", false
	}
	return r.Registry.Resolve(key)
}

// featureSlugFromThread extracts <slug> from "skill:<project>:<slug>" (AC-002).
// Any other thread shape answers ok=false: it cannot be mapped onto a
// requirement, so it takes the guard_only lane instead of being refused.
func featureSlugFromThread(thread string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(thread), ":")
	if len(parts) != 3 || parts[0] != feedThreadPrefix {
		return "", false
	}
	if !feedPartPattern.MatchString(parts[1]) || !feedPartPattern.MatchString(parts[2]) {
		return "", false
	}
	return parts[2], true
}

// featureTruthPath is <repo>/<docsRoot>/<version>/features/<slug>/truth.md -- the
// one file the gate judges.
func featureTruthPath(repo, docsRoot, version, slug string) string {
	return filepath.Join(featurePackageDir(repo, docsRoot, version, slug), beaconTruthFile)
}

// featurePackageDir is <repo>/<docsRoot>/<version>/features/<slug> -- the
// requirement package a bound gh issue points at.
func featurePackageDir(repo, docsRoot, version, slug string) string {
	return filepath.Join(beaconDocsBase(repo, docsRoot), version, beaconFeaturesDir, slug)
}

// beaconDocsBase resolves the Beacon documentation root against a project. An
// absolute adbd_beacon_docs_root is used as-is (a machine may keep one shared
// tree); a relative one is a path inside the project, which is the committed
// layout.
func beaconDocsBase(projectRoot, docsRoot string) string {
	root := strings.TrimSpace(docsRoot)
	if root == "" {
		root = DefaultBeaconDocsRoot
	}
	root = expandHome(root)
	if filepath.IsAbs(root) {
		return filepath.Clean(root)
	}
	return filepath.Join(projectRoot, root)
}

// beaconManaged answers "is this project Beacon-managed, and under which
// version?" (AC-001). A version directory only counts when it carries a
// features/ directory, so an unrelated docs/beacon/v1/README.md tree is not
// mistaken for one. The highest version directory wins, compared numerically, so
// v0.1.10 beats v0.1.9 instead of losing to it as a plain string would.
func beaconManaged(projectRoot, docsRoot string) (version string, ok bool) {
	base := beaconDocsBase(projectRoot, docsRoot)
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", false
	}
	best := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isDirectory(filepath.Join(base, name, beaconFeaturesDir)) {
			continue
		}
		if best == "" || compareVersion(name, best) > 0 {
			best = name
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// readTruthStatus reads the `status` value out of a truth.md frontmatter block.
// It returns the read error unchanged, so the caller can tell "no such file"
// (feature_not_registered) from "a file whose status is something else"
// (truth_not_frozen).
func readTruthStatus(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return frontmatterValue(string(data), "status"), nil
}

// frontmatterValue returns one top-level key's value from a leading `---` block.
// An empty string means "no such key", which is the same answer as "no
// frontmatter at all": neither can be frozen.
func frontmatterValue(doc, key string) string {
	lines := strings.Split(doc, "\n")
	if len(lines) == 0 {
		return ""
	}
	// Tolerate a UTF-8 BOM: a hand-edited file may carry one.
	first := strings.TrimPrefix(strings.TrimSpace(lines[0]), "\ufeff")
	if first != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			return "" // the block ended without carrying the key
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(name), key) {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}

// isFrozenStatus reports whether a truth.md status opens the gate. Comparison is
// case- and space-insensitive, because the value is hand-written YAML.
func isFrozenStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), frozenStatus)
}

// sddGuardedText prepends the SDD guard paragraph for the two lanes that carry
// it (AC-005). It is idempotent: a text already starting with the guard is left
// alone, so no path can stack a second copy.
func sddGuardedText(lane, text string) string {
	if lane != SDDLaneGated && lane != SDDLaneGuardOnly {
		return text
	}
	if strings.HasPrefix(text, sddGuard) {
		return text
	}
	return sddGuard + "\n" + text
}

// ---------------------------------------------------------------------------
// numeric-aware version comparison (AC-001)
// ---------------------------------------------------------------------------

// versionChunk is one run of digits or one run of everything else, which is what
// makes "0.1.10" > "0.1.9" while "v0.1.5" still equals itself.
type versionChunk struct {
	num   bool
	value string
}

// versionChunks splits a version directory name into digit / non-digit runs.
func versionChunks(version string) []versionChunk {
	isDigit := func(b byte) bool { return b >= '0' && b <= '9' }
	var out []versionChunk
	start := 0
	for i := 1; i < len(version); i++ {
		if isDigit(version[i]) != isDigit(version[start]) {
			out = append(out, versionChunk{num: isDigit(version[start]), value: version[start:i]})
			start = i
		}
	}
	if start < len(version) {
		out = append(out, versionChunk{num: isDigit(version[start]), value: version[start:]})
	}
	return out
}

// compareVersion answers -1 / 0 / 1 the way strings.Compare does, but with digit
// runs compared as numbers. A digit run sorts above a text run, so "v1" outranks
// "vDraft"; a shorter version whose chunks are all equal sorts lower, so
// "v0.1.5" beats "v0.1.5-rc1".
func compareVersion(a, b string) int {
	achunks, bchunks := versionChunks(a), versionChunks(b)
	for i := 0; i < len(achunks) && i < len(bchunks); i++ {
		if diff := compareVersionChunk(achunks[i], bchunks[i]); diff != 0 {
			return diff
		}
	}
	switch {
	case len(achunks) < len(bchunks):
		return -1
	case len(achunks) > len(bchunks):
		return 1
	}
	return 0
}

// compareVersionChunk compares two runs of the same kind where possible.
func compareVersionChunk(a, b versionChunk) int {
	if a.num && b.num {
		trimmedA := strings.TrimLeft(a.value, "0")
		trimmedB := strings.TrimLeft(b.value, "0")
		if len(trimmedA) != len(trimmedB) {
			if len(trimmedA) < len(trimmedB) {
				return -1
			}
			return 1
		}
		return strings.Compare(trimmedA, trimmedB)
	}
	if a.num != b.num {
		if a.num {
			return 1
		}
		return -1
	}
	return strings.Compare(a.value, b.value)
}
