// Package inbound will hold warehouse-ops-agent's driving adapter(s) — most
// likely its own MCP server exposing a small number of decision-support
// tools (e.g. "daily_brief") to a host, mirroring the pattern each of the
// five bounded contexts already uses at internal/adapters/inbound/mcp/ in
// their own repos (see the mcp-go-inbound-adapter skill/template). It may
// also gain a simple scheduler-triggered CLI entrypoint for the daily-brief
// slice.
//
// T1 (this scaffold) adds no inbound adapter yet: there is no use case to
// drive. This package exists only so the hexagonal layout — and the
// architecture fitness test asserting inbound never imports outbound — is
// in place from the first commit. The concrete adapter lands with the T4
// (E3 daily-brief MCP) kanban card.
package inbound
