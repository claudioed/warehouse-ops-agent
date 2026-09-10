# CLAUDE.md — warehouse-ops-agent

`warehouse-ops-agent` is a single Go binary that is a **thin, read-side /
decision-support Customer** of the warehouse-systems fleet's five bounded
contexts (`wes-work-planning`, `fulfillment-execution`, `inventory-storage`,
`workforce-management`, `facility-layout`). It owns no aggregate, enforces
no domain invariant, and persists no state — it holds no database. It
correlates facts read from those contexts' published MCP Open Host
Services (plus, for one separate concern, plain REST) through a pure
decision-**policy** layer, with a real LLM ("reasoner") optionally
consulted behind that policy layer as of ADR 0004. This repo has **no
`apis/` directory and no OpenAPI/AsyncAPI spec by design** — see
"Role in the fleet" below.

> ⚠️ **Study project.** This repo and its five upstream services are a
> personal DDD/hexagonal-architecture learning exercise. Treat all
> "production-grade" language as illustrating the pattern being practiced,
> not an operational claim.

## Project Overview

- **Module**: `github.com/claudioed/warehouse-ops-agent`, Go 1.26.6.
- **Entrypoint**: `cmd/agent/main.go` (composition root) +
  `cmd/agent/reasoner.go` (wires the ADR-0004 LLM reasoner).
- **Listens on** `AGENT_ADDR` (default `:8095`), serving:
  - REST at `/` (chi router — `/healthz`, `/daily-brief`,
    `/flow-balance/{pathId}`, `/console/orders/{id}/lifecycle`,
    `/console/reports/wms`, `/console/reports/wes`)
  - This agent's **own** MCP server (Streamable HTTP) at `/mcp`
    (`get_daily_brief`, `list_open_exceptions`,
    `get_flow_balance_exception` — all read-only)
- **No persisted state.** Restart it and it has forgotten nothing; every
  fact it reasons over is re-derived from the five upstream MCP reads (or,
  for the console-bff fan-out, REST reads) at request time.
- **Two independent driving-use-case families in one process**:
  1. The **MCP-Customer / decision-support** path (daily brief, E1
     flow-balance correlation) — this is the "agentic" surface.
  2. The **console-bff** REST fan-out (ADR 0002/0003) backing
     `warehouse-console`'s Order Lifecycle screen and WMS/WES report
     dashboards — a separate concern, separate outbound adapter family,
     separate REST clients.

## Role in the fleet: Customer of Open Host Services via MCP

Per [ADR 0001](docs/docs/adr/0001-warehouse-ops-agent-placement.md) and the
[subdomain classification](docs/docs/ddd/subdomain-classification.md) doc,
`warehouse-ops-agent` is **deliberately not classified** alongside the
fleet's Core/Supporting/Generic bounded contexts. It is a **read-side/
decision-support mechanism**: a CQRS-style read model that spans context
boundaries, plus a policy (tactical-pattern sense) layer — not a bounded
context of its own.

Consequences that matter for anyone touching this repo:

- **No `apis/openapi.yaml` or `apis/asyncapi.yaml`, and none is ever
  expected.** This repo produces no REST/event contract for other services
  to consume against a schema; its REST surface (`docs/docs/api-surface.md`)
  is small and documented by hand in prose, kept in sync with
  `internal/adapters/inbound/http` and `internal/adapters/inbound/mcp` by
  convention, not generation. If asked to "add OpenAPI docs" here, check
  first whether the ask actually belongs to one of the five upstream
  bounded-context repos instead.
- **No cross-repo Go imports of any of the five upstream contexts, ever.**
  Enforced by `internal/architecture/architecture_test.go`'s
  `TestNoDirectDependencyOnBoundedContexts`, which fails the build the
  moment `go.mod`/`go.sum` reference any of:
  `github.com/claudioed/fulfillment-execution`,
  `github.com/claudioed/wes-work-planning`,
  `github.com/claudioed/workforce-management`,
  `github.com/claudioed/inventory-storage`,
  `github.com/claudioed/facility-layout`. Only their published MCP tool
  contracts (and, for console-bff, plain REST endpoints) are valid
  integration points.
- **Domain types are hand-mirrored, not imported.** Enums like
  `RebalanceAction`/`TaskType` in `internal/domain/policy` are local copies
  of the upstream vocabulary, validated at the MCP tool-call boundary
  rather than type-shared.
- **Zero write capability today (v1).** Every tool this agent's own MCP
  server exposes is `ReadOnlyHint: true`; there is no `AssignLabor`,
  `ReleaseNextWork`, or `RevokeReservation` method anywhere in this
  codebase to call even by mistake. See
  [governance-note.md](docs/docs/mcp/governance-note.md) for the future
  write-capable slice's two non-negotiable guardrails (an authorization
  gate + explicit human confirmation before any write executes).
- **Auth is currently fully removed, fleet-wide** (
  [ADR 0006](docs/docs/adr/0006-fleet-wide-auth-removal.md), superseding
  ADR 0005): this agent's inbound REST/MCP surfaces are unauthenticated,
  and its outbound MCP-client calls to the five upstreams carry no bearer
  key. `OIDC-AUTH-SPEC.md` and the static-key/scope plumbing were deleted,
  not disabled — do not resurrect env vars like `OIDC_ISSUER_URL`,
  `MCP_READ_KEY`, `*_MCP_READ_KEY` from old code/docs you may see referenced
  in commit history; they are gone by design.

## Architecture

Hexagonal / Ports & Adapters, same shape as the five sibling bounded-context
repos. Full package layout, the two-outbound-adapter-family split
(mcpclient vs restclient), and the ADR-0004 model-backed reasoner design
(LLM_MODE off/shadow/on, tool allowlist, fallback semantics):
`.claude/rules/architecture.md`.

Non-negotiable, CI-enforced boundary rules (no cross-context Go imports,
zero write capability in v1, hand-mirrored domain types, current auth
status): `.claude/rules/architecture-guardrails.md`.

## Key Commands

```bash
# Build / vet / format
go build ./...
go vet ./...
gofmt -w .                 # or: make fmt

# Run standalone, pointed at the fleet's MCP servers
export WES_WORK_PLANNING_MCP_ENDPOINT=http://localhost:8091/mcp
export FULFILLMENT_EXECUTION_MCP_ENDPOINT=http://localhost:8092/mcp
export INVENTORY_STORAGE_MCP_ENDPOINT=http://localhost:8093/mcp
export WORKFORCE_MANAGEMENT_MCP_ENDPOINT=http://localhost:8094/mcp
export FACILITY_LAYOUT_MCP_ENDPOINT=http://localhost:8095/mcp
export AGENT_ADDR=:8096
go run ./cmd/agent

# Or run against the full fleet via the shared e2e harness
cd ~/warehouse-systems/e2e-tests
bash scripts/02-up-infra.sh      # Kafka + 5 Postgres instances
bash scripts/01-build.sh         # builds all binaries incl. this agent
bash scripts/03-up-services.sh   # starts every service + MCP server + this agent

# Hit the daily brief
curl -s http://localhost:8096/daily-brief | jq .

# Quality gate (mirrors CI)
make check       # fast pre-commit bundle: fmt-check vet build lint test
make check-all   # + coverage + arch-test (pre-push gate)
lefthook install # once, to activate pre-commit/pre-push git hooks

# Docs site (Docusaurus) — local dev
cd docs && npm install && npm start
```

### Configuration (env vars)

One Streamable-HTTP endpoint per upstream MCP context:

| Context | Endpoint env var |
|---|---|
| wes-work-planning | `WES_WORK_PLANNING_MCP_ENDPOINT` |
| fulfillment-execution | `FULFILLMENT_EXECUTION_MCP_ENDPOINT` |
| inventory-storage | `INVENTORY_STORAGE_MCP_ENDPOINT` |
| workforce-management | `WORKFORCE_MANAGEMENT_MCP_ENDPOINT` |
| facility-layout | `FACILITY_LAYOUT_MCP_ENDPOINT` |

Plus `AGENT_ADDR` (default `:8095`), `PROMETHEUS_URL` (unused until a
telemetry-backed slice lands), `DAILY_BRIEF_PATH_TARGETS` (optional JSON
array overriding the process paths the daily brief monitors — defaults to
the single path the e2e-tests bootstrap scenario seeds), and the
console-bff's own separate REST base URLs (`ORDER_MANAGEMENT_REST_URL`,
`INVENTORY_STORAGE_REST_URL`, `WES_WORK_PLANNING_REST_URL`,
`FULFILLMENT_EXECUTION_REST_URL`, plus seven `*_REPORTS_REST_URL` vars for
the analytics dashboards) — see `internal/config/config.go` for every
default value.

LLM reasoner (ADR 0004): `LLM_MODE` (`off`/`shadow`/`on`, default `off`),
`ANTHROPIC_API_KEY` (required unless `off`; never logged),
`LLM_MODEL` (default `claude-sonnet-4-5`), `LLM_TIMEOUT` (default `8s`),
`LLM_BASE_URL` (tests/proxies), `LLM_TOOL_ALLOWLIST` (comma-separated
`<upstream>/<tool>`, defaults to the five read tools the deterministic path
already uses).

## Testing

```bash
go test ./... -race                                  # unit tests, no DB/live MCP
go test ./internal/architecture/... -v                # arch fitness tests
make coverage                                          # coverage run + 90% gate
```

- **Coverage gate is 90%**, scoped to
  `./internal/domain/...,./internal/application/...,./internal/adapters/inbound/...`
  (see `Makefile`'s `COVERPKG` / CI's `ci.yml` `test` job).
- **No database, no live MCP servers required for unit tests** — every use
  case is tested against fakes (`internal/application/usecases/fakes_test.go`).
- **The one legitimate env-gated test in this fleet**: an
  `-tags=integration` test may hit the real Anthropic API only when
  `ANTHROPIC_API_KEY` is set locally (per ADR 0004's Consequences — hosted
  models cannot use testcontainers). Every other integration-style test in
  this fleet must use testcontainers, never an env-var skip gate; this repo
  is the documented exception, not a precedent to copy elsewhere.
- **Architecture fitness tests** (`internal/architecture/architecture_test.go`)
  are the executable form of the "no upstream Go imports" and hexagonal
  dependency rules — run them (`make arch-test` / `go test
  ./internal/architecture/... -v`) after any adapter/import change, not
  just before push.
- To exercise this agent end-to-end against the real fleet, use the shared
  `e2e-tests` harness (see Key Commands above) rather than trying to stand
  up all five upstream MCP servers by hand.

## Docs site

This repo has its own Docusaurus site under `docs/` (business context, DDD
placement/subdomain-classification, context map, API surface, governance
note, every ADR). It publishes via `.github/workflows/docs.yml` to
`https://claudioed.github.io/warehouse-ops-agent/` on push to `main`
touching `docs/**`. `docs/docs/api-surface.md` is the hand-maintained
prose doc for this repo's REST + MCP surface — there is no generated
OpenAPI page because there is no `apis/openapi.yaml` (see "Role in the
fleet" above); if that page drifts from
`internal/adapters/inbound/{http,mcp}`, the code is authoritative, not the
doc.
