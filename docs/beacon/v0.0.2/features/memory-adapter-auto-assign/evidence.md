# Evidence: memory-adapter-auto-assign

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.0.2/features/memory-adapter-auto-assign/truth.md | frozen R2 | truth |
| tests | test_truth | docs/beacon/v0.0.2/features/memory-adapter-auto-assign/tests.md | current | qa |
| memory | support_advisory | src/agent_delivery_bus/adapters/memory.py (+ SPI) | implemented | implement |
| approvals | support_advisory | ApprovalService + `adb approvals awaiting` | implemented | implement |
| feishu | support_advisory | pending.render_pending_channel(channel=feishu) | implemented | implement |
| assign | support_advisory | src/agent_delivery_bus/assign.py + `adb assign candidates` | implemented | implement |
| implement | support_advisory | .beacon/evidence/implement/memory-adapter-auto-assign/ | present | implement |
| qa | qa_verdict | .beacon/evidence/qa-feature/memory-adapter-auto-assign/ | planned | qa |

## Implement evidence index

- `.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-001.json` … `AC-MEM-007.json`
- `.beacon/evidence/implement/memory-adapter-auto-assign/GATES.json`
- `.beacon/evidence/implement/memory-adapter-auto-assign/QA-MATRIX.json` (pytest matrix smoke; formal QA harness still next)
