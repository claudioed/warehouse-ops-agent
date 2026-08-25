// Package usecases is the application layer of warehouse-ops-agent.
//
// Each use case here orchestrates one decision-support scenario: gather
// facts through the internal/ports client interfaces (implemented by
// internal/adapters/outbound/mcpclient and
// internal/adapters/outbound/telemetry), hand them to an
// internal/domain/policy correlation rule, and return a recommendation. No
// use case in this package ever writes directly to a bounded context's
// storage — the only mutation path available to this module at all is
// calling one of the five contexts' own published write tools, and that
// path is deliberately not wired up yet. See the T-card bodies for the
// guardrail this encodes: "writes only via existing published write tools
// (later slices)".
//
// T2 (see flow_balance_advisory.go) adds the first use case,
// FlowBalanceAdvisory, correlating wes-work-planning, workforce-management,
// and fulfillment-execution readings into an E1 FlowBalanceException
// recommendation. The remaining use cases (StrandedReservation detection,
// DailyBrief synthesis) are the sibling T3/T4 kanban cards.
package usecases
