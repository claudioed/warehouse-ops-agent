// Command agent is the composition root for warehouse-ops-agent: it wires
// env config to the five outbound MCP-client adapters (one per upstream
// bounded context) and the telemetry-reader stub, then wires those into
// the DailyBrief (E3) use case and serves it over BOTH an inbound HTTP
// endpoint and this agent's own inbound MCP server (get_daily_brief,
// list_open_exceptions) — a single process, two driving adapters over the
// same use case, exactly the pattern the five bounded contexts use for
// their own HTTP+MCP pair.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	inboundhttp "github.com/claudioed/warehouse-ops-agent/internal/adapters/inbound/http"
	inboundmcp "github.com/claudioed/warehouse-ops-agent/internal/adapters/inbound/mcp"
	"github.com/claudioed/warehouse-ops-agent/internal/adapters/outbound/mcpclient"
	"github.com/claudioed/warehouse-ops-agent/internal/adapters/outbound/restclient"
	"github.com/claudioed/warehouse-ops-agent/internal/adapters/outbound/telemetry"
	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/config"
	"github.com/claudioed/warehouse-ops-agent/internal/observability"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

func main() {
	if err := run(); err != nil {
		slog.Error("warehouse-ops-agent exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := newLogger(getenv("LOG_LEVEL", "info"))
	slog.SetDefault(logger)

	rootCtx := context.Background()
	serviceName := observability.ServiceName()
	otelShutdown, err := observability.Setup(rootCtx, serviceName, observability.ServiceVersion(), observability.Endpoint())
	if err != nil {
		logger.Error("opentelemetry setup degraded", "error", err)
	}
	if otelShutdown != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otelShutdown(ctx); err != nil {
				logger.Error("opentelemetry shutdown failed", "error", err)
			}
		}()
	} else {
		logger.Warn("opentelemetry disabled; traces and metrics will not be exported")
	}

	cfg := config.Load()

	// Build every outbound MCP client. Each satisfies its internal/ports
	// interface at compile time (see the var _ assertions in
	// internal/adapters/outbound/mcpclient/*.go).
	var (
		wes ports.WesWorkPlanningClient = mcpclient.NewWesWorkPlanning(mcpclient.Config{
			Name:      "wes-work-planning",
			Endpoint:  cfg.WesWorkPlanning.Endpoint,
			BearerKey: cfg.WesWorkPlanning.ReadKey,
		})
		fe ports.FulfillmentExecutionClient = mcpclient.NewFulfillmentExecution(mcpclient.Config{
			Name:      "fulfillment-execution",
			Endpoint:  cfg.FulfillmentExecution.Endpoint,
			BearerKey: cfg.FulfillmentExecution.ReadKey,
		})
		wfm ports.WorkforceManagementClient = mcpclient.NewWorkforceManagement(mcpclient.Config{
			Name:      "workforce-management",
			Endpoint:  cfg.WorkforceManagement.Endpoint,
			BearerKey: cfg.WorkforceManagement.ReadKey,
		})
		facility ports.FacilityLayoutClient = mcpclient.NewFacilityLayout(mcpclient.Config{
			Name:      "facility-layout",
			Endpoint:  cfg.FacilityLayout.Endpoint,
			BearerKey: cfg.FacilityLayout.ReadKey,
		})
		inv ports.InventoryStorageClient = mcpclient.NewInventoryStorage(mcpclient.Config{
			Name:      "inventory-storage",
			Endpoint:  cfg.InventoryStorage.Endpoint,
			BearerKey: cfg.InventoryStorage.ReadKey,
		})
		telem ports.TelemetryReader = telemetry.NewStubReader()
	)
	_ = inv   // not used by the E3 daily brief; kept wired for T2/T3 use cases.
	_ = telem // not used by the E3 daily brief; kept wired for a future telemetry-backed slice.

	dailyBrief := &usecases.DailyBrief{
		Facility: facility,
		Wes:      wes,
		Fe:       fe,
		Wfm:      wfm,
		Targets:  toUseCaseTargets(cfg.PathTargets),
	}

	flowBalanceAdvisory := &usecases.FlowBalanceAdvisory{
		Wes: wes,
		WFM: wfm,
		FE:  fe,
	}

	// console-bff order-lifecycle: separate REST clients from the MCP
	// clients above (see internal/ports/order_lifecycle_clients.go's doc
	// comment for why these are a deliberately distinct port shape).
	var orderMgmtClient ports.OrderManagementClient = restclient.NewOrderManagement(cfg.OrderManagementRESTURL, 5*time.Second)
	orderLifecycle := &usecases.OrderLifecycle{
		OrderManagement: &orderMgmtClient,
		Inventory:       restclient.NewInventoryReservations(cfg.InventoryStorageRESTURL, 5*time.Second),
		WorkUnits:       restclient.NewWorkUnits(cfg.WesWorkPlanningRESTURL, 5*time.Second),
		Tasks:           restclient.NewTasksByOrder(cfg.FulfillmentExecutionRESTURL, 5*time.Second),
	}

	handlers := &inboundhttp.Handlers{DailyBrief: dailyBrief, FlowBalanceAdvisory: flowBalanceAdvisory, OrderLifecycle: orderLifecycle}
	router := inboundhttp.NewRouter(handlers, serviceName)

	mcpServer := inboundmcp.NewServer(inboundmcp.Deps{DailyBrief: dailyBrief, FlowBalanceAdvisory: flowBalanceAdvisory})
	mcpAuth := inboundmcp.NewStaticKeyAuth(mcpAuthKeys(cfg, logger))
	mcpHandler := inboundmcp.Handler(mcpServer, mcpAuth)

	mux := http.NewServeMux()
	mux.Handle("/", router)
	mux.Handle("/mcp", mcpHandler)

	srv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		logger.Info("warehouse-ops-agent listening",
			"addr", cfg.Addr,
			"http_routes", "/healthz, /daily-brief, /flow-balance/{pathId}",
			"mcp_route", "/mcp",
			"wes_work_planning_endpoint_configured", cfg.WesWorkPlanning.Endpoint != "",
			"fulfillment_execution_endpoint_configured", cfg.FulfillmentExecution.Endpoint != "",
			"inventory_storage_endpoint_configured", cfg.InventoryStorage.Endpoint != "",
			"workforce_management_endpoint_configured", cfg.WorkforceManagement.Endpoint != "",
			"facility_layout_endpoint_configured", cfg.FacilityLayout.Endpoint != "",
			"path_targets", len(cfg.PathTargets),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// toUseCaseTargets maps the config layer's []config.PathTarget onto the
// application layer's own []usecases.PathTarget. Kept as an explicit
// mapping (not a shared type) so internal/application never imports
// internal/config, preserving the hexagonal dependency rule enforced by
// internal/architecture/architecture_test.go.
func toUseCaseTargets(targets []config.PathTarget) []usecases.PathTarget {
	out := make([]usecases.PathTarget, 0, len(targets))
	for _, t := range targets {
		out = append(out, usecases.PathTarget{
			SiteCode:    t.SiteCode,
			PathId:      t.PathId,
			ProcessPath: t.ProcessPath,
			BuildingId:  t.BuildingId,
			ShiftId:     t.ShiftId,
		})
	}
	return out
}

// mcpAuthKeys reads this agent's OWN inbound MCP server's bearer keys from
// config. If neither is set the server still starts but rejects every
// request (fail closed) — a missing key must never mean "open to
// everyone".
func mcpAuthKeys(cfg config.Config, logger *slog.Logger) map[string]inboundmcp.Scope {
	keys := make(map[string]inboundmcp.Scope)
	if cfg.MCPReadKey != "" {
		keys[cfg.MCPReadKey] = inboundmcp.ScopeRead
	}
	if cfg.MCPReadWriteKey != "" {
		keys[cfg.MCPReadWriteKey] = inboundmcp.ScopeReadWrite
	}
	if len(keys) == 0 {
		logger.Warn("no MCP_READ_KEY or MCP_READWRITE_KEY set; MCP server will reject all requests")
	}
	return keys
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(observability.NewSlogHandler(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}),
	))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
