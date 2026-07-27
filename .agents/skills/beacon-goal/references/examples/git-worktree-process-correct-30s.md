# 30s — Process-correct git worktree (no cherry-pick alignment)

## Goal

Develop a frozen lake on a **delivery/feature worktree**, with **main as truth authority**.

## Steps

```bash
# 0) Primary tree clean (stash unrelated noise)
git status -sb
git stash push -u -m "noise-before-worktree"   # if dirty

# 1) Ensure on main for truth-only work (freeze already done on main)
git checkout main
git pull --ff-only   # optional

# 2) Create or enter delivery worktree (real git worktree)
WT=".beacon/worktrees/v2.0.0/dev-kernel-goal-delivery"
mkdir -p .beacon/worktrees/v2.0.0
if ! git worktree list | grep -q "$WT"; then
  git worktree add "$WT" beacon/v2.0.0/dev-kernel-goal-delivery
fi

# 3) Align delivery with main by MERGE (not cherry-pick)
git -C "$WT" merge main -m "chore: process-correct merge main into delivery"

# 4) Admit (when version+feature known)
beacon workspace admit --project-root . --version v2.0.0 --feature <slug> --json
# Use worktree_path from JSON as cwd for implement

# 5) Implement only inside admitted worktree
cd "$WT"   # or the admit payload path
# $beacon-gen-implement / beacon goal tick ...

# 6) Verify clean
git -C "$WT" status -sb
```

## Illegal

- Implement on primary `main` while delivery branch exists and should own the work
- `git cherry-pick` from main to “catch up” delivery as routine
- Write `truth.md` revision inside feature worktree

## Related

- Norm: `skills/beacon/references/git-worktree-execution-flow.md`
- Entry: `$beacon-goal` / `$loom-goal`
