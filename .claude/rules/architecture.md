# Architecture — package layout, adapter families, LLM reasoner design

## Architecture

Hexagonal / Ports & Adapters, same shape as the five sibling bounded-context
repos:

```
cmd/agent/                           composition root (main.go, reasoner.go)
internal/
  domain/policy/                     pure decision-policy layer:
                                        arbitrate.go   — combines deterministic
                                                          Decision + optional LLM
                                                          Plan (ADR 0004)
                                        dailybrief.go  — E3 daily brief correlation
                                        flow_balance.go — E1 flow-balance correlation
                                        stranded_reservation.go — E2 (not yet wired
                                                          to an inbound adapter)
  application/usecases/              orchestrates policy over ports:
                                        dailybrief.go, flow_balance_advisory.go,
                                        stranded_reservation.go, order_lifecycle.go
                                        (console-bff), console_reports_wms.go,
                                        console_reports_wes.go (console-bff)
  ports/                             OUT: one client interface per upstream
                                      context (WesWorkPlanningClient,
                                      FulfillmentExecutionClient,
                                      InventoryStorageClient,
                                      WorkforceManagementClient,
                                      FacilityLayoutClient, TelemetryReader,
                                      Reasoner) + console-bff's separate
                                      OrderManagementClient/REST port shapes
  adapters/
    inbound/
      http/          chi router: GET /healthz, /daily-brief,
                      /flow-balance/{pathId}, /console/orders/{id}/lifecycle,
                      /console/reports/wms, /console/reports/wes
      mcp/            this agent's OWN MCP server: get_daily_brief,
                      list_open_exceptions, get_flow_balance_exception
    outbound/
      mcpclient/      one thin, schema-typed MCP client per upstream
                      context, Streamable HTTP (facility_layout.go,
                      fulfillment_execution.go, inventory_storage.go,
                      wes_work_planning.go, workforce_management.go),
                      plus tool_invoker.go / session.go used by the LLM
                      reasoner's tool-use loop
      restclient/     console-bff's REST clients — a SEPARATE family from
                      mcpclient, pointed at each context's OLTP API
                      (clients.go) and separately at each context's
                      *-reports analytics reader binary
                      (reports_clients.go) — different process, different
                      Postgres, different base URL from the OLTP one
      llm/anthropic/  ADR-0004 Reasoner implementation: Anthropic Messages
                      API with tool use, restricted to the mcpclient
                      sessions/tools on LLM_TOOL_ALLOWLIST
      telemetry/      Prometheus/OTel reader port (stub — not wired to a
                      real backend yet)
  config/             env-var configuration loader (internal/config/config.go)
  architecture/       arch-go hexagonal + no-cross-context-import fitness
                      tests (internal/architecture/architecture_test.go)
  observability/      OTel setup + slog bridge
```

### The two outbound adapter families are deliberately not unified

`mcpclient` (synchronous MCP tool calls, at request time, backing the
decision-support use cases and the LLM reasoner's tool use) and
`restclient` (plain HTTP calls backing the `console-bff` fan-out) answer
different questions for different callers — an LLM host asking "what needs
attention right now" versus a browser asking "what happened to order X".
See [context-map.md](docs/docs/ecosystem/context-map.md) for the full
Mermaid diagram and the per-context MCP-tool / REST-endpoint table.

### Model-backed reasoner (ADR 0004)

`GET /flow-balance/{pathId}` can consult a real Anthropic model behind the
deterministic policy layer. `policy.Decide` (deterministic) always runs
first; `LLM_MODE` controls what the model's plan may do with the result:

| `LLM_MODE` | behaviour |
|---|---|
| `off` (default) | model never called; byte-for-byte pre-ADR-0004 behaviour |
| `shadow` | model called, plan logged + counted (`ops_agent_llm_agreement_total{agree}`), deterministic decision still returned |
| `on` | a schema-valid plan replaces action/heads/rationale; deterministic decision is the fallback on error, timeout, or out-of-vocabulary output (logged `source=fallback`) |

The model's **only actuators are MCP read tools** (`LLM_TOOL_ALLOWLIST`,
comma-separated `<upstream>/<tool>`), invoked through the exact same
`mcpclient` sessions the deterministic path uses, schema-validated on every
call, and logged as `llm.tool_call`. It answers only through a
`submit_plan` tool whose schema is `policy`'s closed action vocabulary;
`policy.ValidatePlan`/`policy.Arbitrate` reject anything outside it. An
unrecognized `LLM_MODE` is a **startup error**, never a silent fallback to
`off`; a non-`off` mode with no `ANTHROPIC_API_KEY` is also a startup
error. See
[ADR 0004](docs/docs/adr/0004-llm-reasoner-behind-the-policy-layer.md) for
the full design and rationale.
