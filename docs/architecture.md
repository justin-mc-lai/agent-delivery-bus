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

3. **Adapters**
   - `adapters/null.py` — zero-dependency demo backend
   - `adapters/hermes.py` — example executor
   - `adapters/beacon.py` — example truth gate

4. **Skills**
   - `skills/agent-delivery-bus`
   - `skills/collaboration-rules-template`

## Default demo path

Fresh clones default to:

```json
{
  "adapters": {
    "executor": "null",
    "truth_gate": "null"
  }
}
```

`NullExecutor` writes local evidence under `.adb/evidence/<stage>/<feature>.json`.
`NullTruthGate` reconciles that evidence. No Hermes/Beacon install required.

## Extension point

1. implement `ExecutorAdapter`
2. implement `TruthGateAdapter`
3. register both in `adapters/factory.py`
4. set `adapters.executor` / `adapters.truth_gate` in registry JSON
