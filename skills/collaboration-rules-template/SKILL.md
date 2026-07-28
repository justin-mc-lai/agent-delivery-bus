---
name: collaboration-rules-template
description: Define how an AI collaborator should work with a human using four working folders and strict delivery boundaries. Use when setting up a personal or team collaboration policy, separating knowledge from scheduling, or writing AGENTS/CLAUDE rules that keep agents from drifting.
---

# Collaboration Rules Template

This skill is a policy template, not a scheduler and not a wiki.

Copy it into your Knowledge OS or agent rules surface. Keep Delivery Bus focused on
dispatch governance. Keep knowledge able to do work without becoming an encyclopedia.

## Four working folders

1. **Projects**
   Active work only: goals, progress, decisions, open tasks.
2. **Knowledge assets**
   Processed experience: methods, cases, templates, reusable decisions.
3. **Inspiration inbox**
   Raw material only: articles, screenshots, half-formed ideas.
4. **Collaboration rules**
   How AI must cooperate: identity, output format, quality bar, automation habits.

## Decision boundaries

- Inspiration is raw input, never software truth.
- Knowledge assets may inform work, but only Projects plus explicit approval can authorize mutation.
- Collaboration rules constrain every write path; without them, agents rewrite freely and drift.
- Delivery Bus / scheduler may route approved work, but must not own knowledge text.
- Truth gates decide completion. Worker self-report is never enough.

## Minimal operating loop

```text
Inspiration inbox
  -> refine into Knowledge assets or Project decision
  -> if software delivery is needed, create a governed dispatch
  -> executor works
  -> truth gate reconciles evidence
```

## Required reporting style

When acting under these rules, always state:

1. which folder the source came from;
2. whether the action is capture, refine, decide, or dispatch;
3. what is explicitly out of scope;
4. the next safe human action if blocked.

## Anti-patterns

- Turning the knowledge base into a browse-only encyclopedia.
- Letting inbox notes directly mutate production code.
- Encoding long procedural scripts instead of decision boundaries.
- Hiding approval, preflight, or evidence requirements inside chat prose.
