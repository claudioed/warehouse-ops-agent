// Package policy is the decision-policy layer of warehouse-ops-agent.
//
// This is deliberately NOT a domain in the DDD sense: warehouse-ops-agent
// owns no aggregate, enforces no invariant over persisted state, and
// persists nothing of its own. What lives here instead is correlation
// policy — pure, side-effect-free rules that read facts gathered from the
// five bounded contexts' Open Host Services (via internal/ports client
// interfaces) and the telemetry-reader port, and derive a recommendation.
//
// T2 (see flow_balance.go) adds the first correlation rule: E1
// FlowBalanceException, which ranks a recommended action from wes-work-
// planning's rebalance recommendation, workforce-management's staffing gap,
// and fulfillment-execution's stuck-task diagnostic. The remaining
// correlation rules (E2 stranded-reservation detection, E3 daily-brief
// synthesis) are later, separate slices (see the sibling T3/T4 kanban
// cards).
//
// Guardrail (non-negotiable, carried from PROPOSAL-agentic-warehouse-ops.md
// §5 and every T-card body): this package must never import anything from
// internal/adapters/**. Only domain, unconditionally.
package policy
