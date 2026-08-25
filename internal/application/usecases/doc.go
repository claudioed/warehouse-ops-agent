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
// DailyBrief (dailybrief.go) is the E3 slice: it synthesizes the daily
// operational brief across every monitored site/path, tolerating any
// single upstream context being unavailable. The sibling T2 (E1
// flow-balance conflict) and T3 (E2 stranded reservation) kanban cards add
// their own use cases here independently.
package usecases
