# 真实项目 30 秒用法：一句话跑到验收

> **你只需要一个入口。** 不要先调 loop 再调 goal。

## 唯一入口

在 Codex / Claude 里：

```text
$beacon-goal
目标：<用一句话说清要交付的产品能力>
项目：.
版本：v1.x   # 或你项目 docs/beacon 下的当前版本
要求：自动拆需求 → 实现 → 验收；业务顺序要合法；没验过不要说做完。
```

或 CLI（实现后）：

```bash
beacon goal run "做一个可下单的迷你店：先有商品再有订单，状态合法，验收通过" \
  --project . --version v1.x
```

可选别名（Should）：

```bash
beacon run "同上..." --project . --version v1.x
```

## 系统内部会做什么（你不用点）

1. 拆意图 / 湖（需要时）
2. 写/冻需求（含业务状态机，若适用）
3. 实现（可 Ralph；可主星/卫星并行多湖）
4. 持续 QA（合法路径 + 非法路径）
5. 停在 **release 人工确认**（默认）

## 不要这样用

```text
❌ $beacon-loop-goal
❌ 再 $beacon-goal
❌ 再手搓 implement / qa
```

`beacon-loop-goal` 是 **support 发现器**（排队/心跳），不是日常交付按钮。

## 业务产品请对照

完整示范（含「没商品不能下单」）：

- `docs/beacon/v1.6.7/fixtures/golden-commerce-checkout/`
- `skills/beacon/examples/full-product-domain-freeze.md`

## 中断后续跑

```text
$beacon-goal
继续   # 或：beacon goal resume --run-id <id>
```

## 完成标准（对你）

- 验收/测试按 done_when 通过  
- 证据链可查  
- 发布前你点头  

不是：agent 口头说「做完了」。


## UI design (unattended)

When no human design decision exists, establish a complete visual OS first:

```bash
beacon design baseline --project . --version auto --write --json
```

If `DESIGN.md` already exists, the same command writes a fine-tune proposal instead of overwriting.
Secondary polish still routes through `beacon design route polish|review|system` and binds via prototype adapt / change.
See `skills/beacon/examples/design-md-complete.md`.
