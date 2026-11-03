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
// the two refusals, which also name their ReasonCode.
type sddVerdict struct {
	Lane       string
	Refuse     bool
	ReasonCode string
}

// sddGate grades one thread. It reads the filesystem (the project directory, the
// Beacon tree, one truth.md) and nothing else: no network, no subprocess, so a
// drain holding the queue lock cannot block on an external CLI.
func (r *Router) sddGate(thread string) sddVerdict {
	// AC-007: an operator who turned the gate off gets no refusal at all. The
	// lane still says so, so the trace shows the gate was off rather than
	// guessing that every project happened to be fine.
	if !r.SDDGate {
		return sddVerdict{Lane: SDDLaneGatedOff}
	}
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
		// nothing about this dispatch changes.
		return sddVerdict{Lane: SDDLaneNotApplicable}
	}
	slug, ok := featureSlugFromThread(thread)
	if !ok {
		// A Beacon project, but the thread cannot be mapped onto a requirement
		// (a gh: issue, say): dispatch, guarded, no refusal.
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
	return filepath.Join(beaconDocsBase(repo, docsRoot), version, beaconFeaturesDir, slug, beaconTruthFile)
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
