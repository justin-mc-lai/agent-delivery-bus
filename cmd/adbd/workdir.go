// Thread -> repository mapping (feature thread-workdir, v0.1.5).
//
// Why this file exists: a worker that is always started in one fixed directory
// has to reach across repositories for every other thread, which turns each
// dispatch into a permission request. The daemon therefore derives the target
// repository from the thread itself, reusing the project registry the rest of
// the bus already reads (config/projects.json).
//
// The rule is deliberately narrow:
//
//	skill:<project>:<slug>  -> the project registered under that slug/alias
//	gh:<owner>/<repo>#<n>   -> the project registered under that github repo
//	anything else           -> no key, so the caller's fallback directory
//
// A registry that cannot be read, a project that is not registered, or a
// registered directory that no longer exists is never an error: dispatch must
// not stop because a mapping is missing. It falls back to the configured worker
// directory and leaves a `workdir_fallback` line in the reports ledger, so the
// fallback is visible instead of pretending the resolution succeeded.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// WorkdirFallbackKind is the `kind` marker of a fallback record in
// .beacon/adbd-reports.jsonl. Receipts written by reconcile.go carry no `kind`,
// so the two record shapes cannot be confused for one another.
const WorkdirFallbackKind = "workdir_fallback"

// Fallback reason codes: why a thread did not get a project directory.
const (
	// ReasonNoProjectKey: the thread carries no project at all (a feishu thread,
	// a malformed skill thread, an empty thread).
	ReasonNoProjectKey = "no_project_key"
	// ReasonProjectNotRegistered: the thread names a project the registry does
	// not know.
	ReasonProjectNotRegistered = "project_not_registered"
	// ReasonRepoMissing: the project is registered but its repo directory is not
	// a directory on this machine any more.
	ReasonRepoMissing = "repo_missing"
)

// workdirFallbackRecord is one line of the reports ledger recording a fallback.
// Exactly these three keys: a reader can tell the record apart from a receipt by
// `kind` alone, and `thread` + `reason` say which thread fell back and why.
type workdirFallbackRecord struct {
	Kind   string `json:"kind"`
	Thread string `json:"thread"`
	Reason string `json:"reason"`
}

// threadWorkdirKey projects a thread onto the registry key it addresses (AC-001).
//
//	"skill:<project>:<slug>"  -> "<project>"
//	"gh:<owner>/<repo>#<n>"   -> "<owner>/<repo>"
//	anything else             -> ""
//
// The skill shape reuses the alphabet feed.go already froze, so a thread that
// intake would have rejected can never resolve to a repository here either.
func threadWorkdirKey(thread string) string {
	trimmed := strings.TrimSpace(thread)
	if trimmed == "" {
		return ""
	}
	if project, ok := skillThreadProject(trimmed); ok {
		return project
	}
	if repo, ok := ghThreadRepo(trimmed); ok {
		return repo
	}
	return ""
}

// skillThreadProject extracts <project> from "skill:<project>:<slug>".
func skillThreadProject(thread string) (string, bool) {
	parts := strings.Split(thread, ":")
	if len(parts) != 3 || parts[0] != feedThreadPrefix {
		return "", false
	}
	if !feedPartPattern.MatchString(parts[1]) || !feedPartPattern.MatchString(parts[2]) {
		return "", false
	}
	return parts[1], true
}

// ghThreadRepo extracts <owner>/<repo> from "gh:<owner>/<repo>#<number>".
func ghThreadRepo(thread string) (string, bool) {
	prefix := ghThreadPrefix + ":"
	if !strings.HasPrefix(thread, prefix) {
		return "", false
	}
	repo, number, ok := strings.Cut(strings.TrimPrefix(thread, prefix), "#")
	if !ok || number == "" || strings.Trim(number, "0123456789") != "" {
		return "", false
	}
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", false
	}
	return repo, true
}

// ProjectRegistry maps a lookup key -- a project slug, one of its aliases, or its
// github "owner/repo" -- onto the project's repository directory on this machine.
//
// The zero value is a usable "nothing is registered" registry: every Resolve
// misses and no dispatch is blocked.
type ProjectRegistry struct {
	// byKey holds lower-cased keys only, so `Demo-App`, `demo-app` and
	// `Demo/App` all find the same entry. The first registration of a key wins.
	byKey map[string]string
	// source is the file the keys were read from; empty means nothing was read
	// (missing file, unreadable file, unparsable document).
	source string
}

// loadProjectRegistry reads the project registry and indexes it for lookup
// (AC-002).
//
// Tolerances are the point of this function: a missing file, invalid JSON, or a
// `projects` value that is not an array all return an empty registry and no
// error, because a broken registry must never stop dispatch. Repository paths
// are resolved here -- relative ones against root, `~` against the home
// directory -- but their existence is deliberately NOT checked here: AC-004
// checks it at resolution time, so a directory that appears after the daemon
// started is picked up without a restart.
func loadProjectRegistry(path, root string) ProjectRegistry {
	registry := ProjectRegistry{byKey: map[string]string{}}
	source := projectsFilePath(path, root)
	if source == "" {
		return registry
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return registry
	}
	var doc struct {
		Projects []struct {
			Slug    string   `json:"slug"`
			Repo    string   `json:"repo"`
			Aliases []string `json:"aliases"`
			// GitHub is the optional owner/repo field a registry entry may carry
			// so a gh:<owner>/<repo>#<n> thread lands in the right checkout.
			GitHub string `json:"github"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		// Includes `"projects": {...}` / `"projects": 3`: not an array means the
		// document is unusable, and an unusable document is an empty registry.
		return registry
	}
	registry.source = source
	base := absoluteBase(root)
	for _, project := range doc.Projects {
		dir := resolveRepoPath(project.Repo, base)
		if dir == "" {
			continue
		}
		registry.index(project.Slug, dir)
		for _, alias := range project.Aliases {
			registry.index(alias, dir)
		}
		registry.index(project.GitHub, dir)
	}
	return registry
}

// index records one key -> dir mapping, keeping the first registration.
func (r *ProjectRegistry) index(key, dir string) {
	if r.byKey == nil {
		r.byKey = map[string]string{}
	}
	normalized := normalizeProjectKey(key)
	if normalized == "" || dir == "" {
		return
	}
	if _, taken := r.byKey[normalized]; taken {
		return
	}
	r.byKey[normalized] = dir
}

// Lookup answers "where is this key registered?", without touching the
// filesystem. It is what lets the caller explain a miss: a key that is absent
// was never registered, a key that is present but Resolve refuses has a
// directory that is gone.
func (r ProjectRegistry) Lookup(key string) (string, bool) {
	normalized := normalizeProjectKey(key)
	if normalized == "" {
		return "", false
	}
	dir, ok := r.byKey[normalized]
	if !ok || dir == "" {
		return "", false
	}
	return dir, true
}

// Resolve returns the repository directory for a key, but only when that
// directory exists right now (AC-003/AC-004). ok=false covers every miss: no
// key, no registration, and a registration pointing at a path that is not a
// directory.
func (r ProjectRegistry) Resolve(key string) (string, bool) {
	dir, ok := r.Lookup(key)
	if !ok {
		return "", false
	}
	if !isDirectory(dir) {
		return "", false
	}
	return dir, true
}

// Source is the registry file that was read; empty means nothing was read.
func (r ProjectRegistry) Source() string { return r.source }

// Len is the number of distinct lookup keys, for diagnostics and tests.
func (r ProjectRegistry) Len() int { return len(r.byKey) }

// normalizeProjectKey folds the case and trims the spaces off a lookup key, so a
// thread written `Skill:Demo-App:x`-style casing still finds its project while
// the registry itself stays a plain map.
func normalizeProjectKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

// projectsFilePath resolves adbd_projects_file (AC-002): an unset value means
// <root>/config/projects.json, and a relative override is read relative to the
// ADB root, the same way every other daemon path is.
func projectsFilePath(configured, root string) string {
	path := strings.TrimSpace(configured)
	base := absoluteBase(root)
	if path == "" {
		return filepath.Join(base, "config", "projects.json")
	}
	expanded := expandHome(path)
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	return filepath.Join(base, expanded)
}

// resolveRepoPath turns a registry `repo` value into a directory path. Empty
// stays empty (a project without a repo is not a target); a relative value is
// interpreted against the ADB root, which is what makes a committed
// config/projects.json usable on any machine.
func resolveRepoPath(repo, base string) string {
	value := strings.TrimSpace(repo)
	if value == "" {
		return ""
	}
	expanded := expandHome(value)
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	return filepath.Join(base, expanded)
}

// expandHome expands a leading "~" -- the registry is hand-written, and a
// checkout under the operator's home is the common case.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

// absoluteBase makes the root absolute so a registry read from a relative root
// still yields absolute --cwd values.
func absoluteBase(root string) string {
	base := strings.TrimSpace(root)
	if base == "" {
		base = "."
	}
	expanded := expandHome(base)
	if abs, err := filepath.Abs(expanded); err == nil {
		return abs
	}
	return expanded
}

// isDirectory reports whether path exists and is a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
