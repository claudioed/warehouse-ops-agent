---
id: 0004-llm-reasoner-behind-the-policy-layer
title: "0004 — A real LLM reasoner behind the policy layer, with MCP tools as its only actuators"
sidebar_label: "0004 · LLM reasoner behind policy"
description: Why this agent gains a model-backed Reasoner port that proposes plans from the same facts the deterministic policy layer already gathers, why the model can only act through the fleet's published MCP tools, and why the deterministic rules remain the fallback and the arbiter.
---

# ADR 0004: A real LLM reasoner behind the policy layer, with MCP tools as its only actuators

- Status: Accepted — implemented for the flow-balance use case (PR #38, 2026-09-07); shadow-mode rollout pending an ANTHROPIC_API_KEY in warehouse-infra
- Date: 2026-09-07

## Context

The fleet's ecosystem assessment (2026-09-07) found that this service,
positioned in every document as the fleet's *agentic* surface, contains no
model in the loop: `internal/domain/policy` is a deterministic rules engine
(`policy.Decide`, `policy.DailyBrief`, `policy.StrandedReservation`) over
facts read from five bounded contexts' MCP tools. The seven MCP servers,
their curated intent-level tools, the governance charter and ADR-0008 in
each context were built for an AI consumer that does not yet exist. The
owner's decision: **wire a real LLM behind the policy layer, MCP tools as
its only actuators, deterministic rules kept as fallback.**

Two constraints from ADR 0001 are non-negotiable and shape the design:

1. This service owns no aggregate and enforces no new invariant. A model
   must not become a back door for either.
2. Every tool argument the agent accepts is untrusted input; unknown enum
   values are rejected, never defaulted. A model's output is *more*
   untrusted than a human operator's, not less.

## Decision

### 1. A `Reasoner` driven port, not a new domain concept

```go
// internal/ports/reasoner.go
type Reasoner interface {
    // Reason proposes a Plan for the situation described by Brief. It may
    // call the read tools listed in Brief.Tools to gather more facts.
    Reason(ctx context.Context, brief Brief) (Plan, error)
}
```

- `Brief` is assembled by the existing use case (`FlowBalanceAdvisory`,
  `DailyBrief`, …) from the signals it already gathers, plus the tool
  catalogue it is allowed to expose. It is a DTO in `ports`, not a domain
  type.
- `Plan` is a validated, **enum-typed** proposal: `RecommendedAction` uses
  the policy package's existing `ActionAssignLabor` /
  `ActionReleaseNextWork` / `FlowBalanceActionHold` vocabulary,
  `ProposedHeads` is bounded, `Rationale` is free text, `Evidence` lists
  the tool calls the model actually made. Anything outside the vocabulary
  fails `Plan.Validate()` and the whole plan is discarded.
- The domain `policy` package does not import the port. It gains one pure
  function, `policy.Arbitrate(det Decision, llm *Plan, mode Mode) Decision`,
  which is the only place the two sources meet.

### 2. The model can only act through MCP

The Anthropic adapter (`internal/adapters/outbound/llm/anthropic`) uses
the Messages API with **tool use**. The tools it offers the model are
generated 1:1 from the `mcpclient` sessions this service already holds —
`get_rebalance_recommendation`, `get_staffing_gap`,
`diagnose_stuck_tasks`, `check_availability`, `get_site_layout`, … and,
once wired, the write tools `assign_labor`, `release_next_work`,
`revoke_reservation`. The model never sees an HTTP client, a database, or
a shell. Every tool invocation:

- goes through the same `mcpclient` session with the same bearer key and
  scope the deterministic code uses (read key in `shadow`/`on` for read
  tools; the read-write key only for tools on an explicit allow-list
  `LLM_WRITE_TOOLS`, default empty);
- is validated against the tool's JSON schema *before* the MCP call and
  its enum fields re-validated *after* — the same "reject, never default"
  rule already applied to human input;
- emits one structured audit log line (`llm.tool_call` with tool, args
  hash, latency, outcome) and one OTel span under the request's trace.

### 3. The deterministic policy remains the arbiter

`LLM_MODE` selects how `policy.Arbitrate` combines the two:

| mode     | behaviour |
|----------|-----------|
| `off`    | Reasoner never called; today's behaviour, byte-for-byte. Default. |
| `shadow` | Reasoner called with a timeout; its Plan is logged and compared to the deterministic Decision (`ops_agent_llm_agreement{usecase,agree}` counter). The deterministic Decision is returned. |
| `on`     | If the Plan validates and arrived within `LLM_TIMEOUT` (default 8s), it is returned with `Source=llm`; otherwise the deterministic Decision is returned with `Source=fallback` and the reason logged. A Plan may never widen the action vocabulary or exceed `ProposedHeads` bounds. |

`shadow` is the rollout gate: the cluster runs it until the agreement
metric and the logged disagreements have been reviewed, then flips to
`on`. Rollback is `LLM_MODE=off`.

### 4. Configuration and secrets

`ANTHROPIC_API_KEY` (Kubernetes Secret, never in Terraform state or git —
supplied via `TF_VAR_anthropic_api_key` / a git-ignored tfvars),
`LLM_MODEL` (default `claude-sonnet-4-5`), `LLM_MODE`, `LLM_TIMEOUT`,
`LLM_WRITE_TOOLS`. A missing key with `LLM_MODE≠off` fails startup
loudly rather than silently degrading to `off`.

## Consequences

**Positive**
- The seven MCP servers and their governance finally have the consumer
  they were designed for, and the "agentic" claim becomes true.
- The blast radius is bounded by construction: the model's *only* effects
  are MCP tool calls, each authenticated, scoped, schema-validated, and
  audited — the same contract a human operator's REST call is held to.
- The deterministic policy is not discarded; it becomes the safety net
  and the benchmark. Disagreements are data.

**Negative / accepted**
- A hosted-model dependency on the request path of `/flow-balance` and
  `/daily-brief`. Mitigated by the timeout + fallback; those endpoints
  are decision-support, not operational hot paths.
- Cost per call. `shadow` doubles inference cost during the rollout
  window; acceptable for a study project, tracked via the
  `ops_agent_llm_calls_total` counter.
- Testing a hosted model cannot use testcontainers. Unit tests use a
  fake `Reasoner` and recorded API responses (no network in CI); a
  single `-tags=integration` test hits the real API **only** when
  `ANTHROPIC_API_KEY` is set locally. This is the one legitimate use of
  an env gate in the fleet and is documented as such.

## Alternatives considered

- **Replace the policy layer with the model.** Rejected: violates ADR 0001
  (the model would be the de-facto invariant holder), loses the benchmark,
  and makes rollback a rewrite.
- **Let the model call REST directly.** Rejected: bypasses the curated,
  scoped, intent-level MCP surface the fleet built precisely so an AI
  consumer cannot reach raw CRUD.
- **A separate "agent" service.** Rejected: this service already *is* the
  Customer of the five OHS contexts (ADR 0001); a second one would
  duplicate every MCP client and key.
- **Local model (Ollama).** Deferred: the adapter is behind a port, so a
  second implementation is additive. Hosted first, for tool-use quality.

## Prerequisite recorded here so it is not forgotten

As of this ADR **no MCP server is deployed in the cluster**: no context's
Dockerfile builds `cmd/mcp`, no chart has an MCP deployment, and this
service's `*_MCP_ENDPOINT` values are all empty (its startup log says so).
Batch 4 therefore begins with five Dockerfile+chart PRs (FE, WES, INV,
WFM, FL) and a warehouse-infra change wiring endpoints and keys — before a
single line of the Reasoner is useful.
