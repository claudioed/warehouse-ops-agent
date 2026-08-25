// Package config loads warehouse-ops-agent's runtime configuration from the
// environment: one Streamable-HTTP endpoint + static bearer read-key pair
// per upstream bounded context, plus this agent's own listen address. It is
// deliberately dumb (env-var reads, defaults, no validation beyond
// presence) — the composition root (cmd/agent) decides what to do with a
// missing value.
package config

import "os"

// UpstreamConfig is one upstream context's MCP connection info.
type UpstreamConfig struct {
	Endpoint string
	ReadKey  string
}

// Config is warehouse-ops-agent's full runtime configuration.
type Config struct {
	// Addr is this agent's own listen address, for whichever inbound
	// adapter a later slice adds (e.g. its own MCP server). Unused by T1.
	Addr string

	WesWorkPlanning      UpstreamConfig
	FulfillmentExecution UpstreamConfig
	InventoryStorage     UpstreamConfig
	WorkforceManagement  UpstreamConfig
	FacilityLayout       UpstreamConfig

	// PrometheusURL is the warehouse-infra Prometheus base URL, for the
	// telemetry-reader port's real implementation (not yet wired in T1).
	PrometheusURL string
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

		PrometheusURL: getenv("PROMETHEUS_URL", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
