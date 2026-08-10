"""Canonical channel-agnostic keyword map (single source of truth).

Feishu / WeChat / Line and any other channel resolve the same table, so the
same sentence yields the same intent envelope everywhere. The map is
versioned and exposed machine-readably via ``adb intent keywords --json``.
"""

from __future__ import annotations

from typing import Any


KEYWORD_SCHEMA = "adb-keyword-map.v1"
KEYWORD_VERSION = "1.0"

# stage -> aliases (zh / en / channel-neutral). Channel aliases are identical
# on purpose: channels must not fork semantics.
STAGE_KEYWORDS: dict[str, dict[str, list[str]]] = {
    "plan": {
        "zh": ["规划", "计划", "立项规划"],
        "en": ["plan", "planning"],
        "channel": ["plan", "规划"],
    },
    "truth": {
        "zh": ["需求", "冻结需求", "写需求"],
        "en": ["truth", "requirement", "freeze-requirement"],
        "channel": ["truth", "需求"],
    },
    "implement": {
        "zh": ["实现", "开发", "编码", "写代码"],
        "en": ["implement", "implementation", "develop", "code"],
        "channel": ["implement", "实现"],
    },
    "qa": {
        "zh": ["验收", "测试", "质检", "检查"],
        "en": ["qa", "test", "verify", "acceptance"],
        "channel": ["qa", "验收"],
    },
    "freeze": {
        "zh": ["冻结", "收口", "定稿"],
        "en": ["freeze", "lock"],
        "channel": ["freeze", "冻结"],
    },
    "goal": {
        "zh": ["长程", "全流程", "一条龙", "整链"],
        "en": ["goal", "pipeline", "full-run"],
        "channel": ["goal", "长程"],
    },
}

# Dispatch/workflow verbs -> canonical actions (channel-neutral).
ACTION_KEYWORDS: dict[str, dict[str, list[str]]] = {
    "dispatch": {"zh": ["派发", "派活", "派单", "分配"], "en": ["dispatch", "assign"], "channel": ["派发"]},
    "approve": {"zh": ["审批", "同意", "放行", "拍板"], "en": ["approve"], "channel": ["审批"]},
    "reconcile": {"zh": ["验收对账", "对账", "收账"], "en": ["reconcile"], "channel": ["对账"]},
    "workflow_install": {"zh": ["安装工作流", "登记工作流", "接入工作流"], "en": ["install workflow"], "channel": ["安装工作流"]},
    "workflow_ingest": {"zh": ["适配工作流", "分析这个库", "接入开源库"], "en": ["ingest workflow", "adapt repo"], "channel": ["适配工作流"]},
    "workflow_list": {"zh": ["列出工作流", "查看工作流"], "en": ["list workflows"], "channel": ["列出工作流"]},
    "workflow_remove": {"zh": ["删除工作流", "移除工作流"], "en": ["remove workflow"], "channel": ["删除工作流"]},
    "workflow_verify": {"zh": ["验证工作流", "验收探针"], "en": ["verify workflow"], "channel": ["验证工作流"]},
    "workflow_trace": {"zh": ["查看工作流轨迹", "排查工作流"], "en": ["workflow trace", "debug workflow"], "channel": ["工作流轨迹"]},
}


def canonical_keywords() -> dict[str, Any]:
    """Machine-readable canonical map for any channel agent to query."""
    return {
        "schema": KEYWORD_SCHEMA,
        "schema_version": KEYWORD_VERSION,
        "channels": ["feishu", "weixin", "line", "any"],
        "stages": STAGE_KEYWORDS,
        "actions": ACTION_KEYWORDS,
    }


def all_stage_aliases() -> dict[str, str]:
    """stage -> canonical stage (lowercased alias lookup)."""
    mapping: dict[str, str] = {}
    for stage, groups in STAGE_KEYWORDS.items():
        for aliases in groups.values():
            for alias in aliases:
                mapping[alias.casefold()] = stage
    return mapping


def stage_from_keyword(text: str) -> str:
    lowered = (text or "").casefold()
    mapping = all_stage_aliases()
    for alias, stage in mapping.items():
        if alias in lowered:
            return stage
    return ""
