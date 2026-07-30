# UI State Matrix (version-wide)

Status: ready

| UI state | Page expectation | Primary action |
|----------|------------------|----------------|
| empty | honest empty + why | create/import/connect |
| loading | progress/skeleton | cancel if long |
| error | cause + recovery | retry/fix |
| success | short confirmation | next/dismiss |
| blocked | gate reason | unblock |
| stale | age / freshness | refresh |
| recovery | post-failure path | resume/reset |

Feature packages with Domain FSM must add a **business-state** matrix (`ui-state-matrix.md`) binding badges and primary actions to domain states.
