> 日常入口：`$beacon-goal`（见 goal-one-shot-30s.md）

# Full Product Domain Freeze Example

This is the **recommended complete demonstration** for business products (not CLI meta-features).

## Read first

- `docs/beacon/v1.6.7/fixtures/golden-commerce-checkout/README.md`
- `docs/beacon/v1.6.7/fixtures/golden-commerce-checkout/positive/truth.md`
- Program: `docs/beacon/v1.6.7/programs/beacon-product-goal-domain-delivery-v167/`

## What "done" means here

1. Domain Model + Domain FSM frozen
2. Entity precedence explicit (Product before Order)
3. Tests cover legal walks **and** illegal transitions
4. UI state matrix maps each business state
5. Negative fixture (order without product) fails freeze/QA

## Commands (after implement lands)

```bash
beacon validate-feature-package beacon-domain-fsm-authority-v167 --project . --version v1.6.7 --json
beacon freeze beacon-domain-fsm-authority-v167 --project . --version v1.6.7 --json
# Product goal (future CLI from beacon-product-goal-runtime-v167):
# beacon goal start --feature <slug> --version v1.6.7 --done-when domain_fsm,evidence,qa
```

## Anti-pattern

Do **not** treat `examples/basic-workflow.md` or `examples/go-todo-crud` as complete business-product templates.
