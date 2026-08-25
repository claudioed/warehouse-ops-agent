// Package usecases is the application layer of warehouse-ops-agent.
//
// Each use case here will, in later slices, orchestrate one decision-support
// scenario: gather facts through the internal/ports client interfaces
// (implemented by internal/adapters/outbound/mcpclient and
// internal/adapters/outbound/telemetry), hand them to an
// internal/domain/policy correlation rule, and return a recommendation. No
// use case in this package ever writes directly to a bounded context's
// storage — the only mutation path available to this module at all is
// calling one of the five contexts' own published write tools, and that
// path is deliberately not wired up in this scaffold (T1). See the T-card
// bodies for the guardrail this encodes: "writes only via existing
// published write tools (later slices)".
//
// T1 (this scaffold) adds no use case yet — this package exists so the
// hexagonal dependency rule (application depends only on domain, ports, and
// itself) is enforceable by the architecture fitness tests from the first
// commit. The concrete use cases (FlowBalanceAdvisory, StrandedReservation
// detection, DailyBrief synthesis) are the sibling T2/T3/T4 kanban cards.
package usecases
