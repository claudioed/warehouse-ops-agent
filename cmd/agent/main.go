// Command agent is the composition root for warehouse-ops-agent: it wires
// env config to the five outbound MCP-client adapters (one per upstream
// bounded context) and the telemetry-reader stub, and will wire those into
// the read-side decision-support use cases as later slices add them.
//
// T1 (this scaffold) has no use case and no inbound adapter yet — see
// internal/application/usecases/doc.go and internal/adapters/inbound/doc.go
// for why. Running this binary today builds every outbound client and the
// telemetry stub, logs which upstream endpoints are configured, and exits;
// it exists to prove the composition wires cleanly end to end before any
// decision policy lands (T2/T3/T4).
package main

import (
	"log/slog"
	"os"

	"github.com/claudioed/warehouse-ops-agent/internal/adapters/outbound/mcpclient"
	"github.com/claudioed/warehouse-ops-agent/internal/adapters/outbound/telemetry"
	"github.com/claudioed/warehouse-ops-agent/internal/config"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	// Build every outbound MCP client. Each satisfies its internal/ports
	// interface at compile time (see the var _ assertions in
	// internal/adapters/outbound/mcpclient/*.go); wiring them here — rather
	// than only inside a use case's constructor — is what proves the
	// composition end to end for this scaffold card.
	var (
		wes ports.WesWorkPlanningClient = mcpclient.NewWesWorkPlanning(mcpclient.Config{
			Endpoint:  cfg.WesWorkPlanning.Endpoint,
			BearerKey: cfg.WesWorkPlanning.ReadKey,
		})
		fe ports.FulfillmentExecutionClient = mcpclient.NewFulfillmentExecution(mcpclient.Config{
			Endpoint:  cfg.FulfillmentExecution.Endpoint,
			BearerKey: cfg.FulfillmentExecution.ReadKey,
		})
		inv ports.InventoryStorageClient = mcpclient.NewInventoryStorage(mcpclient.Config{
			Endpoint:  cfg.InventoryStorage.Endpoint,
			BearerKey: cfg.InventoryStorage.ReadKey,
		})
		wfm ports.WorkforceManagementClient = mcpclient.NewWorkforceManagement(mcpclient.Config{
			Endpoint:  cfg.WorkforceManagement.Endpoint,
			BearerKey: cfg.WorkforceManagement.ReadKey,
		})
		facility ports.FacilityLayoutClient = mcpclient.NewFacilityLayout(mcpclient.Config{
			Endpoint:  cfg.FacilityLayout.Endpoint,
			BearerKey: cfg.FacilityLayout.ReadKey,
		})
		telem ports.TelemetryReader = telemetry.NewStubReader()
	)

	logger.Info("warehouse-ops-agent composition wired",
		"wes_work_planning_endpoint_configured", cfg.WesWorkPlanning.Endpoint != "",
		"fulfillment_execution_endpoint_configured", cfg.FulfillmentExecution.Endpoint != "",
		"inventory_storage_endpoint_configured", cfg.InventoryStorage.Endpoint != "",
		"workforce_management_endpoint_configured", cfg.WorkforceManagement.Endpoint != "",
		"facility_layout_endpoint_configured", cfg.FacilityLayout.Endpoint != "",
		"prometheus_url_configured", cfg.PrometheusURL != "",
	)

	// No use case and no inbound adapter yet (T1 scope). Reference every
	// wired client so `go vet`/the compiler confirm each satisfies its port,
	// without pretending this binary does decision-support work it doesn't
	// have yet.
	_ = wes
	_ = fe
	_ = inv
	_ = wfm
	_ = facility
	_ = telem

	logger.Info("warehouse-ops-agent T1 scaffold: no use case wired yet; exiting")
}
