// Package policy is the decision-policy layer of warehouse-ops-agent.
//
// This is deliberately NOT a domain in the DDD sense: warehouse-ops-agent
// owns no aggregate, enforces no invariant over persisted state, and
// persists nothing of its own. What lives here instead is correlation
// policy — pure, side-effect-free rules that read facts gathered from the
// five bounded contexts' Open Host Services (via internal/ports client
// interfaces) and the telemetry-reader port, and derive a recommendation.
//
// flow_balance.go (T2) adds the E1 FlowBalanceException correlation rule,
// which ranks a recommended action from wes-work-planning's rebalance
// recommendation, workforce-management's staffing gap, and
// fulfillment-execution's stuck-task diagnostic.
//
// stranded_reservation.go (T3) adds the E2 StrandedReservationException
// correlation rule: fulfillment-execution's expired-lease signal correlated
// with inventory-storage's usable-stock shortfall.
//
// dailybrief.go (T4) adds the E3 daily-brief correlation rule: per-path
// facts (backlog, staffing gap, stuck tasks) are combined into a
// PathBrief and, when at least two independent signals fire together, an
// OpenException.
//
// This package exists so the hexagonal dependency rule (application
// depends only on domain) is enforceable by the architecture fitness tests.
//
// Guardrail (non-negotiable, carried from PROPOSAL-agentic-warehouse-ops.md
// §5 and every T-card body): this package must never import anything from
// internal/adapters/**. Only domain, unconditionally.
package policy
