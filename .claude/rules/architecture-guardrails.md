# Rule: architecture guardrails for warehouse-ops-agent

Source of truth: `internal/architecture/architecture_test.go`,
`docs/docs/adr/0001-warehouse-ops-agent-placement.md`,
`docs/docs/adr/0004-llm-reasoner-behind-the-policy-layer.md`,
`docs/docs/mcp/governance-note.md`.

## Non-negotiable, CI-enforced

1. **Never import a Go package from any of the five upstream bounded
   contexts** (`fulfillment-execution`, `wes-work-planning`,
   `workforce-management`, `inventory-storage`, `facility-layout`). This is
   asserted by `TestNoDirectDependencyOnBoundedContexts`, which scans
   `go.mod`/`go.sum` for those module paths and fails the build if any
   appear — the check runs even if nothing today imports them, so it fails
   loudly the moment one is added. All cross-context integration must go
   through:
   - a published MCP tool call (`internal/adapters/outbound/mcpclient`), or
   - a plain REST call to the context's own OLTP/reports API
     (`internal/adapters/outbound/restclient`) — console-bff only.
2. **Hexagonal dependency direction is one-way inward**, enforced by
   `TestHexagonalDependencyRules`:
   - `internal/domain/policy` depends on nothing but itself.
   - `internal/application` depends only on `domain`, `application`, `ports`.
   - `internal/ports` depends on nothing internal.
   - `internal/adapters/inbound/**` never imports `internal/adapters/outbound/**`
     (and vice versa).
   - Nothing under `internal/**` imports `cmd/**`; only `cmd` wires every
     layer together.
   Run `go test ./internal/architecture/... -v` (or `make arch-test`) after
   touching any adapter, port, or import statement — not just before push.
3. **This repo owns no aggregate, enforces no invariant, persists no
   state.** If a change starts to need one of those three things, that is
   a signal the change belongs in a different repo (a new or existing
   bounded context), per ADR 0001's Consequences — not something to add
   here.
4. **Zero write tools today.** Do not add an `AssignLabor`,
   `ReleaseNextWork`, `RevokeReservation`, or similar mutating method to
   any outbound client, and do not register a non-read-only MCP tool on
   this agent's own inbound server, without first re-reading
   `docs/docs/mcp/governance-note.md`'s "future act slice" guardrails
   (a re-introduced authorization gate + explicit human confirmation before
   execution are both required, not optional, for that slice).
5. **LLM reasoner (ADR 0004) actuator surface is MCP read tools only,
   allow-listed.** The model must never gain a raw HTTP client, database
   handle, or shell. Any change to what the model can call must go through
   `LLM_TOOL_ALLOWLIST` and the same schema-validated `mcpclient` sessions
   the deterministic path already uses — never a new, separate client. An
   unrecognized `LLM_MODE` must remain a startup error, never silently
   degrade to `off`.
6. **Untrusted input is rejected, never defaulted.** Any enum-like field
   this agent accepts (MCP tool args, REST query params, the LLM's own
   `submit_plan` output) must reject an unknown value explicitly (see
   `listOpenExceptionsInput.Severity`'s `severityRank` pattern and
   `policy.ValidatePlan`) — do not add a "fall back to a sensible default"
   branch for an out-of-vocabulary value on any of these paths.

## Auth posture (as of ADR 0006, 2026-09-09)

Auth is fully removed fleet-wide, not just disabled. Do not reintroduce
`OIDC_ISSUER_URL`/`OIDC_CLIENT_ID`, `MCP_READ_KEY`/`MCP_READWRITE_KEY`, or
per-upstream `*_MCP_READ_KEY` env vars from old references you may find in
git history or stale docs — the implementations were deleted, not
feature-flagged off. Reintroducing identity later means redesigning from a
real requirement, per ADR 0006's Consequences.
