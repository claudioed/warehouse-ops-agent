// Package policy is the decision-policy layer of warehouse-ops-agent.
//
// This is deliberately NOT a domain in the DDD sense: warehouse-ops-agent
// owns no aggregate, enforces no invariant over persisted state, and
// persists nothing of its own. What lives here instead is correlation
// policy — pure, side-effect-free rules that read facts gathered from the
// five bounded contexts' Open Host Services (via internal/ports client
// interfaces) and the telemetry-reader port, and derive a recommendation.
//
// T1 (this scaffold) intentionally adds no policy types yet: the
// correlation rules (E1 flow-balance conflict detection, E2 stranded-
// reservation detection, E3 daily brief synthesis) are separate, later
// slices (see the sibling T2/T3/T4 kanban cards). This package exists now
// so the hexagonal dependency rule — application depends only on domain —
// is enforceable by the architecture fitness tests from the very first
// commit, before any policy logic lands.
//
// Guardrail (non-negotiable, carried from PROPOSAL-agentic-warehouse-ops.md
// §5 and every T-card body): this package must never import anything from
// internal/adapters/**. Only domain, unconditionally.
package policy
