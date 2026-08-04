# UI State Matrix — vision-flywheel

schema: ui-state-matrix.v1
bound_to: Domain FSM — ScheduleHeartbeat

CLI/status surfaces only (no product marketing UI). Badges map operator-visible heartbeat states.

| State | Page | Badge | Primary action |
|-------|------|-------|----------------|
| idle | adb schedule list / should-run | idle | tick / should-run |
| checking | adb schedule should-run | checking | quota + health gate |
| running | adb schedule show | running | execute registered command |
| done | adb schedule show / ledger | done | record evidence |
| blocked | adb schedule should-run | blocked | fix quota / health / illegal |

## Do
- Bind badges only to ScheduleHeartbeat FSM states above
- Show blocked distinctly from done
- Never auto-approve or auto-dispatch from heartbeat

## Don't
- Skip should-run into running
- Treat Hermes cron as ADB-owned daemon
