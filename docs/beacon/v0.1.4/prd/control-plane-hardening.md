# PRD: control-plane-hardening (v0.1.4)

## 问题

agent-delivery-bus 已进入真实运营，但控制面自身出现三类治理裂缝：

1. **版本真值分散**：package（0.1.0）、docs 交付树（v0.1.3）、AGENTS/CLAUDE
   （v0.0.5）、onboarding state（v0.0.5）、git tag（仅 v0.1.0）五处各说各话。
2. **存储无演进纪律**：SQLite 变更靠 ad-hoc ALTER；账本与本地配置无备份策略。
3. **适配器契约靠异常回退**：service 用 `except TypeError` 猜适配器/resolver 能力；
   无 CI，回归只靠本地 pytest。

## 目标

1. `docs/beacon/governance/version-truth-catalog.md` 成为唯一真值，四套表面单向投影并机检。
2. SQLite 引入 schema_version 迁移框架；`data/` + `projects.local.json` 纳入可重复备份策略。
3. SPI 引入显式 capabilities / 版本化签名，移除 TypeError 回退链。
4. 补 CI：只跑测试与版本校验，不碰 release。

## 非目标

- 不自动发布/推送；release 永远人工门。
- 不做数据库内容迁移（如业务行改写），只做 schema 演进框架。
- 不做多租户/网络化备份，备份策略面向本地单机控制面。
- 不改适配器外部行为（hermes/pi/beacon 调用方式不变）。
