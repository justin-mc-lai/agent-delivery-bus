---
slug: github-issue
version: v0.1.5
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v0.1.5/features/github-issue/truth.md
  tests: docs/beacon/v0.1.5/features/github-issue/tests.md
  tasks: docs/beacon/v0.1.5/features/github-issue/tasks.md
  evidence: docs/beacon/v0.1.5/features/github-issue/evidence.md
---

# Requirement Truth: github-issue

## 人话

真正的活，一半写在 issue 里。
这个湖让系统自己去看：每两分钟扫一遍配置好的仓库，把新开的、刚被改过的 issue
变成一条带线程的事件送进 router。人不用复制粘贴，也不用记得哪个 issue 有新进展。

同一个 issue 没变化就不重复触发；有新评论（updatedAt 变了）就重新触发一次。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "主动，可以是飞书，也可以是我使用 skill 稳定的触发router沟通，也可以是监控github issue 跟自媒体关键词分析"
goal: "轮询配置的 GitHub 仓库，把 open issue 映射为 gh:<repo>#<n> 线程的事件交给 intake；未变化不重复触发。"
non_goals: "不做 webhook；不接飞书/自媒体；不改 router 决策；不写 GitHub。"
constraints: "Go 放 cmd/adbd/；复用 gh CLI；代理通过配置注入子进程；真相只读。"
scope_mode: "lake"
```

## 状态模型

```text
poll: idle -> querying -> mapped -> submitted | failed
thread: gh:<owner>/<repo>#<number>
```

- `querying`：调用 gh 拉一个仓库的 open issue。
- `mapped`：字段映射完成（thread / message-id / text）。
- `submitted`：交给 intake。
- `failed`：gh 不可用、非零退出、输出不可解析。

## 用户旅程

1. 运维在 `.beacon/config` 里写上要看的仓库：`adbd_github_repos=justin-mc-lai/agent-delivery-bus`。
2. 定时任务每 2 分钟跑一次 `adbd poll-github --once`。
3. 某个仓库新开了一个 open issue → 变成 `gh:justin-mc-lai/agent-delivery-bus#12` 的事件。
4. 没有绑定时落慢路径，route 在 2 分钟内决定派不派活。
5. 同一个 issue 再扫一遍，updatedAt 没变 → deduped，不重复派活。
6. issue 上来了新评论，updatedAt 变了 → 重新触发，worker 收到新消息。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话：监控 github issue | truth.md#Acceptance-Criteria |
| AC-002 | goal：不重复触发 | truth.md#Acceptance-Criteria |
| AC-003 | non_goals：不自动建 issue | truth.md#Acceptance-Criteria |
| AC-004 | constraints：代理注入 | truth.md#Acceptance-Criteria |
| AC-005 | goal：失败不静默 | truth.md#Acceptance-Criteria |
| AC-006 | goal：端到端可验证 | truth.md#Acceptance-Criteria |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd poll-github --once` 从 `adbd_github_repos`（逗号分隔的 `owner/repo`）读取仓库列表，对每个仓库执行 `gh issue list -R <repo> --state open --limit <adbd_github_limit 默认50> --json number,title,body,updatedAt,labels`；无配置时输出 `polled=0 reason_code=repos_not_configured` 且退出码 0 |
| AC-002 | thread 固定 `gh:<owner>/<repo>#<number>`；message-id 固定 `gh:<number>:<updatedAt>`；同一 issue 的 updatedAt 未变化时重复轮询只产生 `deduped`，不重复派活；updatedAt 变化时视为新事件 |
| AC-003 | 只提交 `state=open` 的 issue；`adbd_github_labels`（逗号分隔）非空时只提交带其中任一 label 的 issue；本湖**不**创建、不评论、不关闭任何 issue（代码中不存在写 GitHub 的调用） |
| AC-004 | `adbd_github_proxy` 非空时，把 `HTTPS_PROXY` / `HTTP_PROXY` 注入到 gh 子进程环境；为空时继承当前进程环境；代理地址作为字符串原样传递，不做拼接 |
| AC-005 | 三种失败各有 reason_code 且不静默：`gh` 不在 PATH → `gh_not_found`；gh 非零退出 → `gh_failed`（携带 stderr 前 200 字符）；输出不是合法 JSON → `gh_output_invalid`。三种情况退出码均为 2，其余为 0 |
| AC-006 | 端到端：一次轮询输出 `polled=<n> new=<m> deduped=<k>`；新 issue 经 route 后 worker 收到以 `UNTRUSTED_INPUT` 开头的文本；重复轮询 `new=0` |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_github_repos` | 空 | 逗号分隔的 `owner/repo` |
| `adbd_github_labels` | 空 | label 白名单，空表示不过滤 |
| `adbd_github_limit` | `50` | 每个仓库最多取多少条 |
| `adbd_github_proxy` | 空 | 注入给 gh 子进程的代理地址 |
| `adbd_gh_bin` | `gh` | gh 可执行文件路径 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: failed -> submitted
- illegal: submitted -> querying（同一轮内）

## Non-goals（冻结边界）

- 不做 GitHub webhook、不监听 push/PR/review
- 不接飞书、自媒体
- 不写 GitHub（不建 issue、不评论、不改 label）
- 不改 router 的四选一决策

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 走 gh CLI 还是直接 HTTP 调 GitHub API？ | gh CLI：认证与分页现成，代理注入子进程即可；本机已验证代理下可用 | answered | gh CLI；adbd_github_proxy 注入 HTTPS_PROXY/HTTP_PROXY。 | human |
| Q2 | issue 有变化才算新事件，用什么判定？ | updatedAt 变化即视为新事件（新评论会重新触发） | answered | updatedAt 变化即新事件；message-id = gh:<n>:<updatedAt>。 | human |
