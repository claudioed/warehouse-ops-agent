// Package usecases is the application layer of warehouse-ops-agent.
//
// Each use case here orchestrates one decision-support scenario: gather
// facts through the internal/ports client interfaces (implemented by
// internal/adapters/outbound/mcpclient and internal/adapters/outbound/telemetry),
// hand them to an internal/domain/policy correlation rule, and return a
// recommendation or synthesized read model. No use case in this package
// ever writes directly to a bounded context's storage — the only mutation
// path available to this module at all is calling one of the five
// contexts' own published write tools, and that path is deliberately not
// wired up yet. See the T-card bodies for the guardrail this encodes:
// "writes only via existing published write tools (later slices)".
//
// FlowBalanceAdvisory (flow_balance_advisory.go, T2) correlates
// wes-work-planning, workforce-management, and fulfillment-execution
// readings into an E1 FlowBalanceException recommendation.
// StrandedReservation (stranded_reservation.go, T3) correlates
// fulfillment-execution's expired-lease signal with inventory-storage's
// usable-stock shortfall into an E2 StrandedReservationException.
// DailyBrief (dailybrief.go, T4) synthesizes the E3 daily operational brief
// across every monitored site/path, tolerating any single upstream context
// being unavailable.
package usecases
