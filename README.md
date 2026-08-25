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
`docs/adr/0001-warehouse-ops-agent-placement.md` in this repo for the
placement decision this repo embodies.

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
    inbound/                        driving adapter(s) — likely this
                                     agent's own MCP server (empty in T1)
    outbound/mcpclient/             one thin, schema-typed MCP client per
                                     upstream context, implementing the
                                     ports above over Streamable HTTP
    outbound/telemetry/             Prometheus/OTel reader port
                                     implementation (stub in T1)
  config/                           env-var configuration loader
  architecture/                     arch-go fitness tests
```

## Configuration

One Streamable-HTTP endpoint + static bearer read-key pair per upstream
context (ADR-0008: no IdP), read from the environment:

| Context | Endpoint env var | Key env var |
|---|---|---|
| wes-work-planning | `WES_WORK_PLANNING_MCP_ENDPOINT` | `WES_WORK_PLANNING_MCP_READ_KEY` |
| fulfillment-execution | `FULFILLMENT_EXECUTION_MCP_ENDPOINT` | `FULFILLMENT_EXECUTION_MCP_READ_KEY` |
| inventory-storage | `INVENTORY_STORAGE_MCP_ENDPOINT` | `INVENTORY_STORAGE_MCP_READ_KEY` |
| workforce-management | `WORKFORCE_MANAGEMENT_MCP_ENDPOINT` | `WORKFORCE_MANAGEMENT_MCP_READ_KEY` |
| facility-layout | `FACILITY_LAYOUT_MCP_ENDPOINT` | `FACILITY_LAYOUT_MCP_READ_KEY` |

Plus `PROMETHEUS_URL` (unused until a telemetry-backed slice lands) and
`AGENT_ADDR` (this agent's own listen address, unused until it grows an
inbound adapter).

## Quality gate

```
make check       # fast pre-commit bundle: fmt-check vet build lint test
make check-all   # + arch-test (pre-push gate)
```

`lefthook install` once to activate the pre-commit/pre-push git hooks.

## Status

**T1 — composition scaffold only.** No decision policy, no inbound adapter,
no write path. See the sibling T2 (flow-balance conflict), T3 (stranded
reservation), T4 (daily-brief MCP), T5 (e2e), T6 (docs/ADR) kanban cards for
what lands next.
