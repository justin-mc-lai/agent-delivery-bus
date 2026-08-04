# UI State Matrix — search-boundary-curation

schema: ui-state-matrix.v1
bound_to: Domain FSM — BoundaryProposal

CLI/status surfaces only (no product marketing UI). Badges map operator-visible proposal states.
Wire may still serialize `awaiting_review` as `pending` for CLI compat.

| State | Page | Badge | Primary action |
|-------|------|-------|----------------|
| idle | adb boundary ingest | idle | ingest with profile refs |
| validating | adb boundary ingest | validating | VerticalGate + field check |
| awaiting_review | adb boundary pending / show | pending | decide approve\|reject |
| blocked | adb boundary ingest (error) | blocked | fix profile / vertical / fields |
| approved | adb boundary decide | approved | activates → active |
| rejected | adb boundary decide | rejected | record reason; stop |
| active | adb boundary list --status active | active | use as curated boundary |

## Do
- Bind badges only to BoundaryProposal FSM states above
- Show awaiting_review distinctly from active
- Never show active without decide approve

## Don't
- Treat wire `pending` as a separate domain state
- Auto-approve from cron/tick
- Map emoji/sticker banks into awaiting_review
