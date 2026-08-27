<!-- BEACON:START -->
<!-- BEACON:VERSION:v1.6.12 -->
<!-- BEACON:DOCS_VERSION:v0.1.4 -->
# AGENTS.md

This file is Beacon-managed agent operation guidance.

## Agent behavior
- Keep scope tight and implement the smallest correct change.
- Verify diagnostics/tests before claiming completion.
- Preserve user-authored sections outside Beacon managed blocks.

## Beacon requirement-material usage
- Runtime target version: `v1.6.12`
- Docs target version: `v0.1.4`
- Use `docs/beacon/global-boundaries.md` as the canonical Beacon-wide global constraint source.
- Use `docs/beacon/<version>/` as the canonical delivery tree.
- Read progressively in this order:
  1. `docs/beacon/global-boundaries.md`
  2. `docs/beacon/<version>/SUMMARY.md`
  3. `docs/beacon/<version>/execution/index.md`
  4. `docs/beacon/<version>/execution/architecture-blueprint.md`
  5. `docs/beacon/<version>/prd/`, `user-story/`, `qa/test-cases/`
  6. `docs/beacon/<version>/.machine/`
- For greenfield projects: start with architecture blueprint, then enter think → user-story → prd.
- For takeover projects: document current-state architecture, service map, constraints, and version timeline before new implementation.
- If context blocks are missing/corrupted, run:
  - `beacon doctor setup-context --project-root .`
  - `beacon doctor verify-context --project-root . --strict`
- After runtime/version upgrades on a real project, sync machine requirement materials before QA:
  - `beacon doctor sync-materials --project-root . --version <version> --all-features`
<!-- BEACON:END -->


<!-- USER:CUSTOM-START -->
## 用户规则（大白话汇报）

- 每次交付的「这次做了什么」必须让不懂本项目的人看懂：说人话、少术语、必要术语一句话解释。
- 复杂批次按 `/eli5`（本仓 `skills/eli5` + 全局已装，Apache-2.0）的大图少字思路输出。
- 参考实现：`selfmedia/prism` 仓 `contracts/human-card.md` 大白话规则 + `vendor/skills/eli5`。

## 安全准则（强制，开源仓库）

本项目是**开源框架**，任何提交都可能被公开。以下为硬性规则：

### 密钥与凭据红线（禁止提交）
- **绝对禁止**把以下任何值写入被 git 跟踪的文件：
  - API key（如 `sk-...`、`sk-z8...`、`cfut_...`、`ghp_...`、`AKIA...`）
  - token、secret、password、私钥（`-----BEGIN PRIVATE KEY-----`）
  - 数据库连接串含密码、`CLOUDFLARE_API_TOKEN=` 等环境变量赋值
- 凭据只允许放在**本地被忽略**的文件：
  - `config/projects.local.json`（已 gitignore）
  - `data/*.db`（已 gitignore）
  - `~/.config/adb-d1/token.env`（本机，0600 权限，绝不同步/提交）
  - `f0-brain-runtime/brain/secrets.env`（已 gitignore）
- `*.example` / `*.template` 模板文件只放**占位符**（如 `<YOUR_API_KEY>`），不放真实值。

### 提交前检查清单（每次 commit 前必过）
```bash
# 1. 检查工作树是否有真实密钥
git grep -lE "sk-[a-zA-Z0-9]{16,}|cfut_[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{20,}|CLOUDFLARE_API_TOKEN\s*=\s*[a-zA-Z0-9]{16,}" --include="*.py" --include="*.json" --include="*.toml" --include="*.sh" --include="*.md"
# 2. 确认敏感文件被忽略
git check-ignore config/projects.local.json data/adb.db
# 3. 看即将提交的内容
git add -A && git status --short
# 4. 若误提交过密钥，必须 git rm --cached + 重写历史（git filter-repo）或轮换该密钥
```
- `git add -A` 后**必须** `git status --short` 核对无 `.env`、`*token*`、`*.db`、`projects.local.json`、`backups/`、`volume/` 混入。
- 用 `git add <明确路径>` 代替 `git add -A` 更安全。

### 权限与边界
- `data/`（含 adb.db 数据库）与 `config/projects.local.json` 是本地状态，**永不提交**。
- `f0-brain-runtime/brain/volume/`（大脑记忆卷）与 `backups/` 含运行数据，**永不提交**。
- Cloudflare D1 只读 token 只配置在 mini 的 `~/.config/adb-d1/token.env`（0600），代码只读该文件，不写明文。
- 若怀疑某密钥已泄露（进过 git 历史或公开渠道），**立即轮换该密钥**而非仅删除引用。

### 新增敏感文件时
- 新增任何 `.env`/`token`/`secret` 相关文件，先加进 `.gitignore` 再使用。
- 不要用 `echo KEY=value >> file` 直接写入仓库目录；写到 `~/.config/` 或 `data/`（均已忽略）。

<!-- USER:CUSTOM-END -->
