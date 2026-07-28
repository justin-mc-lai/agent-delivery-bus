# Architecture

## Layers

1. **Core control plane**
   - `registry.py`
   - `preflight.py`
   - `approvals.py`
   - `storage.py`
   - `service.py`
   - `cli.py`

2. **Adapter SPI**
   - `adapters/spi.py`
   - `adapters/factory.py`

3. **Example adapters**
   - `adapters/hermes.py` (executor)
   - `adapters/beacon.py` (truth gate)

4. **Skills**
   - `skills/agent-delivery-bus` (control-plane skill)
   - `skills/collaboration-rules-template` (policy template; not a scheduler)

## Non-goals permanently owned elsewhere

- Knowledge prose / inspiration / methods: Knowledge OS
- Worker claim/retry internals: Executor backend
- Requirement freeze/QA/release verdict: Truth-gate backend

## Extension point

To support another stack:

1. implement `ExecutorAdapter`
2. implement `TruthGateAdapter`
3. register both in `adapters/factory.py`
4. set `adapters.executor` / `adapters.truth_gate` in the registry JSON
