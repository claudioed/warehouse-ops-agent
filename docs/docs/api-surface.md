---
id: api-surface
title: API surface
sidebar_label: API Surface
description: The REST endpoint and MCP tools warehouse-ops-agent exposes. No OpenAPI spec yet — documented here in prose.
---

# API surface

`warehouse-ops-agent` has no `apis/openapi.yaml` yet — its surface is
small enough, and changing fast enough across T-cards, that it is
documented here in prose rather than generated. This page is kept current
by hand; if it drifts from `internal/adapters/inbound/`, the code is
authoritative.

## REST (`internal/adapters/inbound/http`)

| Method & path | What it returns |
|---|---|
| `GET /healthz` | `{"status": "ok"}` |
| `GET /daily-brief` | The full synthesized `DailyBrief`: every monitored site's paths with backlog/staffing/queue/stuck-task facts, plus ranked `openExceptions`. |
| `GET /flow-balance/{pathId}` | The E1 `FlowBalanceException` correlation for one path (503 if the use case isn't wired). |
| `GET /explain-travel-factor?pathId=&fromLocationCode=&toLocationCode=` | Calls facility-layout's `estimate_travel_distance` for the two REQUIRED, caller-supplied location codes and classifies the result (`travel_significant`/`travel_negligible`) against the ADR-0009 threshold. 400 if either location code is missing; 503 if the use case isn't wired. This agent never infers the two location codes itself — see [ADR 0009](./adr/0009-explain-travel-factor.md). |
| `GET /console/orders/{id}/lifecycle` | The **console-bff** read model (see [ADR 0002](./adr/0002-micro-frontend-console-architecture.md)): fans out to order-management, inventory-storage, wes-work-planning, and fulfillment-execution and stitches one order's cross-service lifecycle for `warehouse-console`'s Order Lifecycle screen. Each stage degrades independently — one context being unreachable never 500s the whole response. |

## MCP (`internal/adapters/inbound/mcp`)

This agent runs its **own** MCP server (Streamable HTTP, unauthenticated —
see [ADR 0006](./adr/0006-fleet-wide-auth-removal.md)) so that an agentic
host can consume its recommendations the same way it consumes any bounded
context's facts.

| Tool | What it does |
|---|---|
| `get_daily_brief` | Returns the full synthesized `DailyBrief`. |
| `list_open_exceptions` | Lists open exceptions, optionally filtered to a minimum `severity` (`info`/`warning`/`critical`). An unrecognized severity value is rejected, never silently defaulted. |
| `get_flow_balance_exception` | Correlates the E1 signals for one `pathId` (+ `buildingId`/`shiftId` for the staffing lookup) into a ranked `FlowBalanceException`. |
| `explain_travel_factor` | Calls facility-layout's `estimate_travel_distance` for two REQUIRED, caller-supplied location codes (`fromLocationCode`/`toLocationCode`) and classifies the result. The caller must already know both codes — this tool never infers or guesses them (see [ADR 0009](./adr/0009-explain-travel-factor.md)). |

All four tools are annotated read-only
(`mcp.ToolAnnotations{ReadOnlyHint: true}`). This agent has **zero write
tools** — see the [Governance note](./mcp/governance-note.md) for why that
is a v1 design choice, not an oversight.

## What is not yet exposed

The E2 StrandedReservation policy (`internal/domain/policy.Evaluate`) has
an application-layer use case
(`internal/application/usecases.stranded_reservation.go`) but is not yet
wired to either inbound adapter — it is exercised today only by its own
unit tests. Wiring it to a REST route and an MCP tool mirroring
`get_flow_balance_exception`'s shape is open follow-up work, not part of
this documentation pass.
