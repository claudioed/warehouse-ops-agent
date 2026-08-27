---
id: governance-note
title: Governance note — this agent's write posture
sidebar_label: Governance note
description: warehouse-ops-agent only ever writes through published tools; v1 is read-scope only; acting is a later slice behind read-write scope plus human confirmation.
---

# Governance note — this agent's write posture

This note records the governance decision specific to
`warehouse-ops-agent`, on top of the fleet-wide rules in the
[MCP Governance Charter](https://claudioed.github.io/fulfillment-execution/docs/mcp/governance-charter)
(`fulfillment-execution` is the charter's canonical home; every sibling
context, and this agent, follow it). Read that charter first — this page
only covers what is specific to being a cross-context *decision-support*
agent rather than a bounded context.

## The rule

**This agent writes to a bounded context only through that context's own,
already-published write MCP tool — never through a database, an internal
API, or any mechanism that bypasses that context's own invariant
enforcement.** A mistaken or hallucinated agent call is safe by
construction: the target context's existing domain layer is the backstop,
exactly as it is for a human operator calling the same tool.

Concretely, the only write tools this agent could ever call are:

| Tool | Owning context | What it would do |
|---|---|---|
| `assign_labor` | `workforce-management` | Assign heads to a path (E1's `assign_labor` recommendation) |
| `release_next_work` | `wes-work-planning` | Release the next work unit into a path (E1's `release_next_work` recommendation) |
| `revoke_reservation` | `inventory-storage` | Free a stranded reservation's stock back to usable (E2's `revoke_reservation` recommendation) |
| `complete_task` | `fulfillment-execution` | Not currently reachable from any of this agent's decision policies |

## v1 scope: read-only, recommendations-only

As shipped through T5, `warehouse-ops-agent` holds **zero write
capability**. Every MCP tool this agent's own inbound server exposes
(`get_daily_brief`, `list_open_exceptions`, `get_flow_balance_exception`)
is annotated `ReadOnlyHint: true`, and its outbound adapters
(`internal/adapters/outbound/mcpclient/`) implement only the five
contexts' **read** ports (see `internal/ports/clients.go`) — there is no
`AssignLabor`, `ReleaseNextWork`, or `RevokeReservation` method anywhere
in this codebase to call even by mistake.

This is enforced the same way the "no direct bounded-context dependency"
rule is: by what does not exist in the code, not by a runtime check that
could be bypassed. A future write-capable slice adds new outbound-client
methods and a new tool-registration entry deliberately; it cannot happen
by accident.

## The future act slice: read-write scope + human confirmation

When a write-capable slice does land, it inherits two guardrails already
decided, not deferred:

1. **A separate `:write` auth scope.** Per the charter (§7, §3), any
   write tool this agent's inbound MCP server ever exposes MUST require
   `mcp:warehouse-ops-agent:write` (or the equivalent read-write bearer
   key on this agent's own `StaticKeyAuth`), rejecting a read-only caller
   with `403` — exactly the `ScopeRead`/`ScopeReadWrite` machinery already
   wired in `internal/adapters/inbound/mcp/auth.go`, unused today only
   because no write tool exists yet to require it.
2. **Explicit human confirmation before the write executes.** A
   recommendation (`assign_labor`, `release_next_work`,
   `revoke_reservation`) surfacing from this agent's read tools is not
   itself authorization to act. The act-slice's design — a human-in-the-loop
   confirmation step between "recommended" and "executed" — is a
   commitment recorded here ahead of the implementation, per
   [ADR 0001](../adr/0001-warehouse-ops-agent-placement.md)'s consequence
   that "human-in-the-loop and least-privilege" stay preserved as this
   agent's capability grows.

## Auditability

Every tool call this agent's own MCP server handles emits an OTel span
(`mcp.tool <name>`) carrying the tool name and required scope, per the
charter's §9 auditability rule — the same instrumentation pattern the
five sibling servers use, so a call here is traceable in Jaeger alongside
every upstream call it triggers.
