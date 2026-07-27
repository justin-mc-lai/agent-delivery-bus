# Example: truth with plain-language layer (English)

```markdown
---
slug: demo-order-pay
language: en
---

# Order payment

## Plain language

A seller collects payment for an order that already exists. No order, no pay.
After pay, the order is paid and can ship.

- Can: pay a `created` order; retry on failure
- Cannot: pay without create; pretend unpaid after success
- Done when: legal path tests pass; illegal jumps rejected

## User Intent

Pay after create.

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-PAY-001 | created → paying → paid |
| AC-PAY-002 | reject paid without payment record |
```
