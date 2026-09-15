# warehouse-ops-agent

**⚠️ Study project.** This repository, and the five bounded-context services
it reads from, are a personal DDD / hexagonal-architecture learning exercise
modeling a simplified warehouse fleet. Nothing here is production warehouse
software; treat all business behavior, quality claims, and "production-grade"
language as illustrative of the pattern being practiced, not operational
fact.

## What this is

A thin, independently-deployable **read-side / decision-support** mechanism
over the warehouse-systems fleet's five bounded contexts
(`fulfillment-execution`, `wes-work-planning`, `inventory-storage`,
`workforce-management`, `facility-layout`). It is a **Customer** of those
contexts' published Model Context Protocol (MCP) Open Host Services — never
a new domain context. It owns no aggregate, enforces no new invariant, and
persists no domain state; its "domain" layer is decision **policy**
(correlation rules over facts read from the five contexts plus live
telemetry).

See `ADR-warehouse-ops-agent-DRAFT.md` and
`PROPOSAL-agentic-warehouse-ops.md` (in the originating kanban task's
workspace) for the full design rationale, and
[ADR 0001](./docs/docs/adr/0001-warehouse-ops-agent-placement.md) in this
repo for the placement decision this repo embodies. The full documentation
site (business context, DDD placement, API surface, governance note, and
every ADR) is published from `docs/` — see
[claudioed.github.io/warehouse-ops-agent](https://claudioed.github.io/warehouse-ops-agent/)
once the `Docs` GitHub Actions workflow has deployed it, or run it locally
with `cd docs && npm install && npm start`.

## Guardrails (non-negotiable)

- **No new domain aggregate; no direct DB writes.** This agent writes to any
  bounded context ONLY via that context's existing published write MCP
  tools (a later slice — none is wired yet).
- **Published contracts only.** No cross-repo Go imports of any of the five
  contexts' packages, ever — only their published MCP tool schemas. Enforced
  by `internal/architecture/architecture_test.go`'s
  `TestNoDirectDependencyOnBoundedContexts`.
- **GitFlow.** `feature/*` branches → PR into `develop`; CI verified green
  before merge. Never commit directly to `main`.
- **Untrusted model input.** Every tool argument this agent ever accepts
  (once it grows its own inbound adapter) is validated; unknown enum values
  are rejected, never silently defaulted.

## Architecture

Hexagonal / Ports & Adapters, same shape as the five sibling services:

```
cmd/agent/                          composition root
internal/
  domain/policy/                    decision-policy layer (empty in T1 —
                                     no correlation rules land until a later
                                     slice)
  application/usecases/             orchestrates policy over ports (empty
                                     in T1)
  ports/                            OUT: one client interface per upstream
                                     context (WesWorkPlanningClient,
                                     FulfillmentExecutionClient,
                                     InventoryStorageClient,
                                     WorkforceManagementClient,
                                     FacilityLayoutClient) + TelemetryReader
  adapters/
    inbound/                         driving adapters:
      http/                            GET /daily-brief REST endpoint
      mcp/                             this agent's own MCP server
                                        (get_daily_brief,
                                        list_open_exceptions)
    outbound/mcpclient/             one thin, schema-typed MCP client per
                                     upstream context, implementing the
                                     ports above over Streamable HTTP
    outbound/telemetry/             Prometheus/OTel reader port
                                     implementation (stub in T1)
  config/                           env-var configuration loader
  architecture/                     arch-go fitness tests
```

## Configuration

One Streamable-HTTP endpoint per upstream context, read from the
environment:

| Context | Endpoint env var |
|---|---|
| wes-work-planning | `WES_WORK_PLANNING_MCP_ENDPOINT` |
| fulfillment-execution | `FULFILLMENT_EXECUTION_MCP_ENDPOINT` |
| inventory-storage | `INVENTORY_STORAGE_MCP_ENDPOINT` |
| workforce-management | `WORKFORCE_MANAGEMENT_MCP_ENDPOINT` |
| facility-layout | `FACILITY_LAYOUT_MCP_ENDPOINT` |

Plus `PROMETHEUS_URL` (unused until a telemetry-backed slice lands),
`AGENT_ADDR` (this agent's own listen address — serves both the HTTP daily
brief at `/daily-brief` and the MCP endpoint at `/mcp`), and
`DAILY_BRIEF_PATH_TARGETS` (an optional JSON array overriding which process
paths the daily brief monitors; defaults to the single path the e2s-tests
bootstrap scenario seeds).

### Model-backed reasoner (ADR 0004)

`GET /flow-balance/{pathId}` can consult a real LLM behind the policy layer.
The deterministic `policy.Decide` always runs first; `LLM_MODE` decides what
the model's plan may do with its result:

| `LLM_MODE` | behaviour |
|---|---|
| `off` (default) | model never called; pre-ADR-0004 behaviour byte-for-byte |
| `shadow` | model called, plan logged and counted (`ops_agent_llm_agreement_total{agree}`), deterministic decision returned |
| `on` | a valid plan replaces action/heads/rationale; deterministic decision is the fallback on error, timeout or out-of-vocabulary output (`source=fallback` in the log line) |

The model's ONLY actuators are MCP read tools: `LLM_TOOL_ALLOWLIST`
(comma-separated `<upstream>/<tool>`, default = the five read tools the
deterministic path already uses) is invoked through the same `mcpclient`
sessions and read keys as everything else, every call schema-validated
upstream and logged as `llm.tool_call`. It answers only through a
`submit_plan` tool whose schema is the policy package's closed action
vocabulary; `policy.ValidatePlan` rejects anything else.

Env: `ANTHROPIC_API_KEY` (required unless `off`; startup fails loudly
otherwise), `LLM_MODEL` (default `claude-sonnet-4-5`), `LLM_TIMEOUT`
(default `8s`), `LLM_BASE_URL` (tests/proxies). An unrecognised `LLM_MODE`
is a startup error, never a silent `off`.

## Quality gate

```
make check       # fast pre-commit bundle: fmt-check vet build lint test
make check-all   # + arch-test (pre-push gate)
```

`lefthook install` once to activate the pre-commit/pre-push git hooks.

## Status

**T4 landed — E3 daily-brief read model + inbound HTTP + inbound MCP
adapter.** `GET /daily-brief` and the `get_daily_brief`/`list_open_exceptions`
MCP tools synthesize a daily operational brief across every configured
process path: backlog telemetry (wes), staffing gap (workforce-management),
queue depth and stuck-task diagnostics (fulfillment-execution), grouped by
facility-layout site. A path is flagged as an open exception (warning or
critical) when at least two independent signals correlate — never a single
metric alone — and every exception carries its full evidence trail. Any
single upstream context being unavailable degrades that path's brief to a
partial, typed result rather than failing the whole request.

No write path yet (this remains recommendations/read-model only). See the
sibling T2 (flow-balance conflict), T3 (stranded reservation), T5 (e2e), T6
(docs/ADR) kanban cards for what lands next.

**T6 landed — ADR finalized, governance note, and Docusaurus docs site.**
[ADR 0001](./docs/docs/adr/0001-warehouse-ops-agent-placement.md) is
Accepted and consolidates the round-1 placement draft. The
[governance note](./docs/docs/mcp/governance-note.md) records this
agent's read-only v1 write posture on top of the fleet-wide MCP
governance charter. The docs site (`docs/`, Docusaurus, same shape as the
five sibling sites) covers business context, DDD placement, the API
surface, and the ubiquitous language — including the new terms this
agent's exceptions introduce (`FlowBalanceException`,
`StrandedReservation`, `DailyBrief`).
