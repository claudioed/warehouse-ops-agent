// Package inbound holds warehouse-ops-agent's driving adapters:
// internal/adapters/inbound/http (the daily-brief GET endpoint) and
// internal/adapters/inbound/mcp (this agent's own MCP server exposing
// get_daily_brief and list_open_exceptions to a host), both over the same
// DailyBrief application-layer use case — the pattern each of the five
// bounded contexts already uses for their own HTTP+MCP pair (see the
// mcp-go-inbound-adapter skill/template).
//
// This file is otherwise empty on purpose: it exists so the hexagonal
// layout — and the architecture fitness test asserting inbound never
// imports outbound — is in place, and so this package doc has one obvious
// home regardless of which inbound adapter package a reader opens first.
package inbound
