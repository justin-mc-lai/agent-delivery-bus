/**
 * pi-beacon: native ADB/Beacon/Prism bridge for the pi agent.
 *
 * - registerTool("adb_dispatch"): intent -> confirm -> dispatch -> reconcile
 * - registerCommand("/prism"): run the prism self-media skill surface
 * - session_start: surface ADB/knowledge readiness hints
 */
import { execFileSync } from "node:child_process";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";

function adb(args: string[]): string {
  return execFileSync("adb", args, { encoding: "utf-8", maxBuffer: 8 * 1024 * 1024 });
}

export default function (pi: ExtensionAPI) {
  pi.registerTool({
    name: "adb_dispatch",
    label: "ADB Dispatch",
    description:
      "Parse a natural-language dispatch intent via adb, show the envelope, and (after explicit human confirmation) dispatch and reconcile.",
    parameters: Type.Object({
      utterance: Type.String({ description: "Natural-language dispatch intent" }),
      project: Type.Optional(Type.String({ description: "Registered project slug/index" })),
      confirm: Type.Boolean({ description: "Must be true only after the human confirmed the envelope" }),
    }),
    async execute(toolCallId, params: { utterance: string; project?: string; confirm?: boolean }, _signal, _onUpdate, ctx) {
      const parsed = JSON.parse(
        adb([
          "intent",
          "parse",
          "--utterance",
          params.utterance,
          ...(params.project ? ["--project", params.project] : []),
          "--json",
        ]),
      );
      if (parsed.blocked) {
        return { content: [{ type: "text", text: `BLOCKED: ${parsed.reason_code}` }], details: parsed };
      }
      if (!params.confirm) {
        return {
          content: [{ type: "text", text: `Envelope ready — human confirmation required:\n${JSON.stringify(parsed.data.envelope, null, 2)}` }],
          details: parsed,
        };
      }
      const env = parsed.data.envelope;
      if (env.requires_approval) {
        const issued = JSON.parse(adb(["approve", "--actor", "pi", "--project", env.project_slug, "--stage", env.stage, "--feature", env.feature, "--json"]));
        ctx.ui.notify(`Approval issued: ${issued.data?.approval_id ?? "n/a"}`, "info");
      }
      const dry = JSON.parse(adb(["dispatch", "--project", env.project_slug, "--stage", env.stage, "--feature", env.feature, "--dry-run", "--json"]));
      if (dry.blocked) {
        return { content: [{ type: "text", text: `Preflight blocked: ${dry.reason_code}` }], details: dry };
      }
      const dispatched = JSON.parse(adb(["dispatch", "--project", env.project_slug, "--stage", env.stage, "--feature", env.feature, "--json"]));
      return {
        content: [{ type: "text", text: `Dispatched: ${dispatched.data?.dispatch?.dispatch_id ?? "?"}` }],
        details: dispatched,
      };
    },
  });

  pi.registerCommand("prism", {
    description: "Run prism self-media production skills (goal/intel/master/director/produce/qa/release)",
    handler: async (args, ctx) => {
      const phase = (args || "goal").trim();
      ctx.ui.notify(`prism ${phase}: invoke skills/prism-${phase} (SKILL.md) for this production.`, "info");
    },
  });

  pi.on("session_start", async (_event, ctx) => {
    ctx.ui.notify("ADB/Beacon bridge ready. /prism for self-media; adb_dispatch for scheduling.", "info");
  });
}
