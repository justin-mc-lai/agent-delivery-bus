---
name: pi-beacon
description: Native ADB/Beacon/Prism bridge for the pi agent. Use to let pi dispatch via adb (intent→confirm→dispatch→reconcile), run prism self-media skills, and auto-load beacon/adb/prism skill directories. Install with install.sh (idempotent, --dry-run supported).
---

# pi-beacon

Pi agent 原生接入 ADB/Beacon/Prism 的扩展包。

## 安装

```bash
./install.sh --dry-run   # 预览
./install.sh             # 幂等安装：合并 settings.skills + 拷贝 adb-bridge.ts
```

## 能力

- `adb_dispatch` 工具：自然语言 intent → envelope → 人工确认 → dispatch → reconcile。
- `/prism <phase>` 命令：prism-goal/intel/master/director/produce/qa/release。
- 会话启动时加载 ADB/知识库提示。

## 边界

- 未确认 envelope 绝不派发；approval 只发给受限阶段；release 永远人工门。
