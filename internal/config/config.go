// Package config loads warehouse-ops-agent's runtime configuration from the
// environment: one Streamable-HTTP endpoint + static bearer read-key pair
// per upstream bounded context, plus this agent's own listen address and
// the set of process paths the daily brief (E3) monitors. It is
// deliberately dumb (env-var reads, defaults, no validation beyond
// presence) — the composition root (cmd/agent) decides what to do with a
// missing value.
package config

import (
	"encoding/json"
	"os"
)

// UpstreamConfig is one upstream context's MCP connection info.
type UpstreamConfig struct {
	Endpoint string
	ReadKey  string
}

// PathTarget identifies one process path the daily brief monitors, binding
// together each upstream context's own naming for "the same" path (see
// internal/domain/policy.PathTarget for why those names are not guaranteed
// equal across contexts). This is deployment-time configuration: which wes
// PathId corresponds to which fulfillment-execution queue and which
// workforce-management (building, shift, path) is an operational fact
// about the fleet, not something the agent can infer.
type PathTarget struct {
	SiteCode    string `json:"siteCode"`
	PathId      string `json:"pathId"`
	ProcessPath string `json:"processPath"`
	BuildingId  string `json:"buildingId"`
	ShiftId     string `json:"shiftId"`
}

// defaultPathTargets mirrors the e2s-tests bootstrap scenario's single
// seeded path (see e2s-tests/features/bootstrap.feature), so the daily
// brief has a sensible default to synthesize against out of the box.
var defaultPathTargets = []PathTarget{
	{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK", BuildingId: "wh1", ShiftId: "shift-1"},
}

// Config is warehouse-ops-agent's full runtime configuration.
type Config struct {
	// Addr is this agent's own listen address, for the daily-brief HTTP
	// and MCP inbound adapters (T4).
	Addr string

	WesWorkPlanning      UpstreamConfig
	FulfillmentExecution UpstreamConfig
	InventoryStorage     UpstreamConfig
	WorkforceManagement  UpstreamConfig
	FacilityLayout       UpstreamConfig

	// OrderManagementRESTURL, InventoryStorageRESTURL,
	// WesWorkPlanningRESTURL, FulfillmentExecutionRESTURL are each
	// context's own plain REST base URL, used ONLY by the console-bff's
	// order-lifecycle fan-out (internal/adapters/outbound/restclient) --
	// a separate concern from the MCP upstreams above, which back the
	// daily-brief/flow-balance LLM-facing use cases. Local-dev defaults
	// mirror e2e-tests/env.sh's HTTP_PORT map exactly.
	OrderManagementRESTURL      string
	InventoryStorageRESTURL     string
	WesWorkPlanningRESTURL      string
	FulfillmentExecutionRESTURL string

	// PrometheusURL is the warehouse-infra Prometheus base URL, for the
	// telemetry-reader port's real implementation (not yet wired).
	PrometheusURL string

	// PathTargets is the set of process paths the daily brief (E3)
	// monitors. Overridable via DAILY_BRIEF_PATH_TARGETS (a JSON array
	// matching PathTarget's fields); falls back to defaultPathTargets when
	// unset or unparseable, so an operator error degrades to a working
	// default rather than an empty, useless brief.
	PathTargets []PathTarget

	// MCPReadKey/MCPReadWriteKey are this agent's OWN inbound MCP server's
	// static bearer keys (ADR-0008: no IdP), read from a Kubernetes
	// Secret. Distinct from the ReadKey fields above, which authenticate
	// THIS agent as a client of the five upstream contexts.
	MCPReadKey      string
	MCPReadWriteKey string
}

// Load reads Config from the environment. Every field defaults to an empty
// string when its env var is unset; the composition root is responsible for
// deciding whether an empty endpoint/key means "skip this client" or "fail
// closed" for its use case.
func Load() Config {
	return Config{
		Addr: getenv("AGENT_ADDR", ":8095"),

		WesWorkPlanning: UpstreamConfig{
			Endpoint: getenv("WES_WORK_PLANNING_MCP_ENDPOINT", ""),
			ReadKey:  os.Getenv("WES_WORK_PLANNING_MCP_READ_KEY"),
		},
		FulfillmentExecution: UpstreamConfig{
			Endpoint: getenv("FULFILLMENT_EXECUTION_MCP_ENDPOINT", ""),
			ReadKey:  os.Getenv("FULFILLMENT_EXECUTION_MCP_READ_KEY"),
		},
		InventoryStorage: UpstreamConfig{
			Endpoint: getenv("INVENTORY_STORAGE_MCP_ENDPOINT", ""),
			ReadKey:  os.Getenv("INVENTORY_STORAGE_MCP_READ_KEY"),
		},
		WorkforceManagement: UpstreamConfig{
			Endpoint: getenv("WORKFORCE_MANAGEMENT_MCP_ENDPOINT", ""),
			ReadKey:  os.Getenv("WORKFORCE_MANAGEMENT_MCP_READ_KEY"),
		},
		FacilityLayout: UpstreamConfig{
			Endpoint: getenv("FACILITY_LAYOUT_MCP_ENDPOINT", ""),
			ReadKey:  os.Getenv("FACILITY_LAYOUT_MCP_READ_KEY"),
		},

		OrderManagementRESTURL:      getenv("ORDER_MANAGEMENT_REST_URL", "http://localhost:8086"),
		InventoryStorageRESTURL:     getenv("INVENTORY_STORAGE_REST_URL", "http://localhost:8082"),
		WesWorkPlanningRESTURL:      getenv("WES_WORK_PLANNING_REST_URL", "http://localhost:8083"),
		FulfillmentExecutionRESTURL: getenv("FULFILLMENT_EXECUTION_REST_URL", "http://localhost:8084"),

		PrometheusURL: getenv("PROMETHEUS_URL", ""),

		PathTargets: loadPathTargets(),

		MCPReadKey:      os.Getenv("MCP_READ_KEY"),
		MCPReadWriteKey: os.Getenv("MCP_READWRITE_KEY"),
	}
}

// loadPathTargets parses DAILY_BRIEF_PATH_TARGETS as a JSON array of
// PathTarget, falling back to defaultPathTargets when the env var is unset
// or fails to parse. A malformed override must never silently produce an
// empty (and therefore useless) daily brief.
func loadPathTargets() []PathTarget {
	raw := os.Getenv("DAILY_BRIEF_PATH_TARGETS")
	if raw == "" {
		return defaultPathTargets
	}
	var targets []PathTarget
	if err := json.Unmarshal([]byte(raw), &targets); err != nil || len(targets) == 0 {
		return defaultPathTargets
	}
	return targets
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
