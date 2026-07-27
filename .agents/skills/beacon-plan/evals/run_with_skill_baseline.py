#!/usr/bin/env python3
"""With-skill vs baseline behavioral eval for beacon-plan (qi-dev skill-creator loop).

Uses `claude -p` for executor transcripts, deterministic rubric for grading.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import time
from pathlib import Path
from typing import Any

SKILL = Path(__file__).resolve().parents[1]
EVALS = Path(__file__).resolve().parent
RUNS = EVALS / "runs" / "iteration-1"


def load_skill_bundle() -> str:
    """Compact package for claude -p (avoid system-prompt timeouts)."""
    parts: list[str] = []
    skill_md = (SKILL / "SKILL.md").read_text(encoding="utf-8")
    parts.append("# SKILL.md\n" + skill_md[:6000])
    mode = SKILL / "references" / "modes" / "pln-review.md"
    if mode.is_file():
        body = mode.read_text(encoding="utf-8")
        # Prefer HARD GATE + workflow head
        parts.append("\n# Mode pln-review\n" + body[:8000])
    cat = SKILL / "references" / "pln-review" / "reviewer-catalog.md"
    if cat.is_file():
        parts.append("\n# reviewer-catalog\n" + cat.read_text(encoding="utf-8")[:2500])
    return "\n\n".join(parts)


def claude_prompt(prompt: str, *, system: str, out_path: Path, timeout: int = 360) -> dict[str, Any]:
    out_path.parent.mkdir(parents=True, exist_ok=True)
    cmd = [
        "claude",
        "-p",
        prompt,
        "--output-format",
        "text",
        "--system-prompt",
        system,
    ]
    env = {k: v for k, v in os.environ.items() if k != "CLAUDECODE"}
    t0 = time.time()
    try:
        proc = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=str(SKILL.parents[3]),  # beacon repo
            env=env,
        )
        text = (proc.stdout or "") + ("\n" + proc.stderr if proc.stderr else "")
        status = "ok" if proc.returncode == 0 else f"exit_{proc.returncode}"
    except subprocess.TimeoutExpired as e:
        text = (e.stdout or "") if isinstance(e.stdout, str) else ""
        text += "\n[TIMEOUT]"
        status = "timeout"
    except Exception as e:  # noqa: BLE001
        text = f"[ERROR] {e}"
        status = "error"
    dt = round(time.time() - t0, 2)
    out_path.write_text(text, encoding="utf-8")
    meta = {"status": status, "seconds": dt, "chars": len(text)}
    (out_path.parent / "meta.json").write_text(json.dumps(meta, indent=2) + "\n", encoding="utf-8")
    return {"text": text, **meta}


def grade(text: str, expectations: list[str]) -> dict[str, Any]:
    t = text.lower()
    results = []

    def _refuses_implement() -> bool:
        if re.search(
            r"不(会|能|可|做).{0,16}(实现|写代码|implement)|禁止.{0,16}(实现|写代码|implement)|"
            r"no implement|not implement|do not implement|won't implement|don'?t implement|"
            r"planner only|planner\s*[≠!=]+\s*implement|≠\s*implement|"
            r"planner\s*≠\s*implement|route-boundary",
            t,
            re.I,
        ):
            return True
        # Structured: implement listed as forbidden under HARD GATE / 禁止 table
        if re.search(r"(hard gate|禁止|p0).{0,80}(实现|implement)|(实现|implement).{0,40}(禁止|hard gate|≠)", t, re.I | re.S):
            return True
        return False

    def _refuses_truth_write() -> bool:
        if re.search(
            r"不(会|能|可).{0,16}(写|冻|修改).{0,16}truth|refuse.{0,20}truth|forbidden.{0,20}truth|"
            r"hard gate|planner only|禁止.{0,20}truth|planner\s*[≠!=]+\s*truth|≠\s*truth freezer",
            t,
            re.I,
        ):
            return True
        return "truth" in t and any(
            w in t for w in ("不能", "禁止", "refuse", "not write", "won't write", "do not write", "不得", "≠", "!=")
        )

    checks = {
        "refuses_truth_write": _refuses_truth_write,
        "refuses_implement": _refuses_implement,
        "stays_planner_only": lambda: any(
            w in t for w in ("planner", "plan harness", "hard gate", "planner only", "规划", "审查", "不实现", "route-boundary")
        )
        and not re.search(r"i (have )?(implemented|wrote code|committed)|已实现|已提交代码", t),
        "routes_to_other_harness": lambda: any(
            w in t
            for w in (
                "beacon-truth",
                "beacon-implement",
                "beacon-qa",
                "beacon-goal",
                "truth harness",
                "implement harness",
                "recommended_next",
                "next_harness",
                "recommended_route",
                "下一",
                "路由",
            )
        ),
        "scope_full_parity_or_block": lambda: (
            bool(re.search(r"scope_mode\s*[:=]\s*full_parity|full_parity|full parity", t))
            or (
                any(w in t for w in ("完整复刻", "做全", "同等能力", "不要 mvp"))
                and bool(re.search(r"scope_mode|p0|block|blocker|hard gate", t))
            )
        )
        and not re.search(r"scope_mode\s*[:=]\s*mvp|缩成 mvp|先做 mvp 即可", t),
        "has_findings": lambda: bool(
            re.search(
                r"(?m)^\s*(-\s*)?(finding|findings)\s*[:：]|审查发现|\bP0\b|\bP1\b|p0_blocker|"
                r"severity\s*[:=]|\*\*p0\*\*|#### .*p0|finding\s*\d",
                text,
                re.I,
            )
        )
        and not re.search(r"没法开始.*(findings)|前提条件.*findings|输出（[^）]*findings", t),
        "has_severity": lambda: bool(re.search(r"\bP[0-3]\b|severity|严重度|p0_blocker|p0 —|p1 —", text, re.I)),
        "not_silent_implement_route": lambda: not re.search(
            r"recommended_route\s*[:=]\s*['\"]?beacon-gen-implement|直接 implement|可以开始写代码并发布", t
        )
        or any(w in t for w in ("user decision", "用户确认", "p0", "p1", "block", "stop", "等待")),
        "has_parity_or_deferral": lambda: bool(
            re.search(
                r"(?m)^\s*(-\s*)?(parity_matrix|parity matrix|deferral_ledger|deferral ledger)\s*[:：]|"
                r"延期账本|能力清单|\|\s*capability\s*\||source_capability_inventory",
                text,
                re.I,
            )
        )
        and not re.search(r"没法开始.*(parity_matrix|deferral)|前提条件.*(parity|deferral)", t),
        "mentions_planner_review_cli": lambda: any(
            w in t
            for w in (
                "planner-review",
                "planner review",
                "beacon planner-review",
                "runtime evidence",
                "execution_mode",
            )
        ),
        "mentions_artifact_path": lambda: any(
            w in t
            for w in (
                ".beacon/state/planner-review",
                "planner-review/",
                "artifact",
                "artifacts",
                "state/planner",
            )
        ),
        "no_fake_parallel_subagent": lambda: not re.search(
            r"parallel subagent (is )?running|已并行子代理完成|parallel_subagent_review(?!.*fallback)", t
        )
        or "single_process" in t
        or "fallback" in t
        or "unavailable" in t
        or "无 subagent" in t
        or "single_process_multi_reviewer" in t,
        "mode_pln_source_or_review": lambda: any(
            w in t for w in ("pln-source", "pln-review", "mode_id", "mode: pln", "mode id", "`pln-")
        ),
        "has_recommended_next": lambda: any(
            w in t
            for w in (
                "recommended_next",
                "next_harness",
                "recommended_route",
                "下一跳",
                "next step",
                "recommended_next_harness",
                "下一",
            )
        ),
        "no_implement": lambda: (
            "implement" not in t
            and "实现" not in t
        )
        or any(
            w in t
            for w in (
                "不实现",
                "not implement",
                "禁止 implement",
                "禁止实现",
                "after truth",
                "先 truth",
                "hard gate",
                "planner only",
                "≠ implement",
                "!= implement",
            )
        )
        or _refuses_implement(),
    }
    for exp in expectations:
        fn = checks.get(exp)
        if not fn:
            results.append({"expectation": exp, "pass": False, "reason": "unknown_expectation"})
            continue
        ok = False
        try:
            ok = bool(fn())
        except Exception as e:  # noqa: BLE001
            results.append({"expectation": exp, "pass": False, "reason": f"grader_error:{e}"})
            continue
        results.append({"expectation": exp, "pass": ok, "reason": "matched" if ok else "no_evidence"})
    passed = sum(1 for r in results if r["pass"])
    return {
        "passed": passed,
        "total": len(results),
        "score": round(passed / len(results), 3) if results else 0.0,
        "results": results,
    }


def build_systems() -> tuple[str, str]:
    bundle = load_skill_bundle()
    with_skill = (
        "You are executing the beacon-plan skill exactly.\n"
        "Follow HARD GATE. Planner only.\n"
        "Always emit structured fields (even when blocking / underspecified): "
        "mode_id, intent_snapshot, scope_mode, parity_matrix, deferral_ledger, findings (with P0/P1 severity), "
        "recommended_next_harness, execution_mode, fallback_reason.\n"
        "If source OSS or target is missing: still emit contract with P0 findings and scope_mode=full_parity when user demanded 做全/同等能力; then ask questions.\n"
        "Do not write files. Do not implement code. Do not freeze truth.\n\n"
        f"SKILL PACKAGE:\n{bundle}\n"
    )
    baseline = (
        "You are a generic software planning assistant with no special Beacon skill package.\n"
        "Help the user plan. You may suggest writing requirements or code if that seems useful.\n"
        "Do not write files.\n"
    )
    return with_skill, baseline


def main() -> None:
    parser = argparse.ArgumentParser(description="beacon-plan with-skill vs baseline eval")
    parser.add_argument("--regrade-only", action="store_true", help="Only regrade existing outputs")
    parser.add_argument("--resume", action="store_true", help="Skip arms that already have output.md")
    parser.add_argument("--iteration", default="iteration-1", help="runs/<iteration> folder name")
    parser.add_argument("--cases", default=None, help="path to cases json (default behavioral-cases.json)")
    args = parser.parse_args()
    global RUNS
    RUNS = EVALS / "runs" / args.iteration

    cases_path = Path(args.cases) if args.cases else (EVALS / "behavioral-cases.json")
    cases = json.loads(cases_path.read_text(encoding="utf-8"))
    with_sys, base_sys = build_systems()
    RUNS.mkdir(parents=True, exist_ok=True)
    summary_rows = []
    for case in cases:
        cid = case["id"]
        prompt = case["prompt"]
        exps = case["expectations"]
        print(f"== {cid} ==")
        for arm, system in (("with_skill", with_sys), ("baseline", base_sys)):
            out_dir = RUNS / arm / cid
            out_file = out_dir / "output.md"
            if args.regrade_only:
                if not out_file.is_file():
                    print(f"  skip {arm}: no output")
                    continue
                text = out_file.read_text(encoding="utf-8")
                meta = {}
                meta_path = out_dir / "meta.json"
                if meta_path.is_file():
                    meta = json.loads(meta_path.read_text(encoding="utf-8"))
                result = {"text": text, "status": meta.get("status", "ok"), "seconds": meta.get("seconds", 0)}
                print(f"  regrade {arm}...")
            elif args.resume and out_file.is_file() and out_file.stat().st_size > 20:
                text = out_file.read_text(encoding="utf-8")
                meta = {}
                meta_path = out_dir / "meta.json"
                if meta_path.is_file():
                    meta = json.loads(meta_path.read_text(encoding="utf-8"))
                result = {"text": text, "status": meta.get("status", "ok"), "seconds": meta.get("seconds", 0)}
                print(f"  resume-skip {arm} (existing output)")
            else:
                print(f"  running {arm}...")
                result = claude_prompt(prompt, system=system, out_path=out_file, timeout=360)
            g = grade(result["text"], exps)
            (out_dir / "grade.json").write_text(json.dumps(g, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            transcript = (
                f"# Eval {cid} ({arm})\n\n"
                f"## Prompt\n\n{prompt}\n\n"
                f"## Output\n\n{result['text'][:50000]}\n\n"
                f"## Grade\n\n```json\n{json.dumps(g, ensure_ascii=False, indent=2)}\n```\n"
            )
            (out_dir / "transcript.md").write_text(transcript, encoding="utf-8")
            summary_rows.append(
                {
                    "id": cid,
                    "arm": arm,
                    "status": result["status"],
                    "seconds": result["seconds"],
                    "score": g["score"],
                    "passed": g["passed"],
                    "total": g["total"],
                    "results": g["results"],
                }
            )
            print(f"  {arm}: status={result['status']} score={g['score']} {g['passed']}/{g['total']} ({result['seconds']}s)")

    # aggregate deltas
    by_id: dict[str, dict[str, Any]] = {}
    for row in summary_rows:
        by_id.setdefault(row["id"], {})[row["arm"]] = row
    deltas = []
    for cid, arms in by_id.items():
        ws = arms.get("with_skill", {})
        bl = arms.get("baseline", {})
        deltas.append(
            {
                "id": cid,
                "with_skill_score": ws.get("score"),
                "baseline_score": bl.get("score"),
                "delta": round((ws.get("score") or 0) - (bl.get("score") or 0), 3),
                "with_skill_pass": f"{ws.get('passed')}/{ws.get('total')}",
                "baseline_pass": f"{bl.get('passed')}/{bl.get('total')}",
            }
        )

    report = {
        "skill": "beacon-plan",
        "iteration": args.iteration,
        "kind": "with_skill_vs_baseline_behavioral",
        "rows": summary_rows,
        "deltas": deltas,
        "mean_delta": round(sum(d["delta"] for d in deltas) / len(deltas), 3) if deltas else 0,
        "with_skill_mean": round(
            sum(d["with_skill_score"] or 0 for d in deltas) / len(deltas), 3
        )
        if deltas
        else 0,
        "baseline_mean": round(sum(d["baseline_score"] or 0 for d in deltas) / len(deltas), 3)
        if deltas
        else 0,
    }
    (RUNS / "summary.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    md = [f"# beacon-plan with-skill vs baseline ({args.iteration})", ""]
    md.append(f"- with_skill_mean: **{report['with_skill_mean']}**")
    md.append(f"- baseline_mean: **{report['baseline_mean']}**")
    md.append(f"- mean_delta: **{report['mean_delta']}**")
    md.append("")
    md.append("| case | with_skill | baseline | delta |")
    md.append("|------|------------|----------|-------|")
    for d in deltas:
        md.append(
            f"| {d['id']} | {d['with_skill_pass']} ({d['with_skill_score']}) | {d['baseline_pass']} ({d['baseline_score']}) | {d['delta']:+} |"
        )
    md.append("")
    md.append("## Critique")
    if report["mean_delta"] > 0:
        md.append("- Skill improves contract compliance vs baseline on average.")
    elif report["mean_delta"] == 0:
        md.append("- No average delta; check grader sensitivity or skill guidance strength.")
    else:
        md.append("- Baseline scored higher on average — investigate skill prompt overload or grader false negatives.")
    (RUNS / "summary.md").write_text("\n".join(md) + "\n", encoding="utf-8")
    print(json.dumps({"mean_delta": report["mean_delta"], "with_skill_mean": report["with_skill_mean"], "baseline_mean": report["baseline_mean"]}, indent=2))


if __name__ == "__main__":
    main()
