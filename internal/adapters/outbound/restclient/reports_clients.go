package restclient

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// The seven analytics clients below back the console-bff's WMS/WES
// dashboard fan-out. Each targets its context's *-reports READER binary
// (a separate process from the OLTP API the order-lifecycle clients in
// clients.go call), so each takes its own base URL.
//
// Every upstream shares the same two-endpoint contract, verified by
// reading each repo's own reports_handler.go rather than assumed:
//
//	GET /reports/<name>?from=<RFC3339>&to=<RFC3339>  -> the report
//	GET /reports/<name>/freshness                    -> {"lagSeconds": n}
//
// from and to are REQUIRED and RFC3339 on all seven (a missing or
// malformed one is an RFC 7807 400), which is why reportsClient always
// sends both. Optional per-context filters (pathId, sku, taskType,
// scope, granularity) are deliberately not sent: the dashboards want the
// whole window, unfiltered, at each context's default granularity.

// reportsClient is the shared transport for all seven analytics clients:
// a base URL, a bounded-timeout http.Client (same 5s convention as the
// order-lifecycle clients), and the from/to query encoding every
// upstream requires.
type reportsClient struct {
	baseURL string
	client  *http.Client
}

func newReportsClient(baseURL string, timeout time.Duration) reportsClient {
	return reportsClient{baseURL: baseURL, client: newHTTPClient(timeout)}
}

// report issues the windowed report GET. from/to are normalised to UTC
// RFC3339 -- every upstream parses with time.Parse(time.RFC3339, ...),
// so an offset-carrying local timestamp would parse fine but makes the
// upstream's logs harder to correlate across the fan-out.
func (c reportsClient) report(ctx context.Context, path string, from, to time.Time, out any) error {
	q := url.Values{
		"from": {from.UTC().Format(time.RFC3339)},
		"to":   {to.UTC().Format(time.RFC3339)},
	}
	return httpGetJSON(ctx, c.client, c.baseURL, path, q, out)
}

// freshnessResponse is every context's freshness wire shape (they all
// publish the identical {"lagSeconds": <float>} body).
type freshnessResponse struct {
	LagSeconds float64 `json:"lagSeconds"`
}

func (c reportsClient) freshness(ctx context.Context, path string) (float64, error) {
	var out freshnessResponse
	if err := httpGetJSON(ctx, c.client, c.baseURL, path, nil, &out); err != nil {
		return 0, err
	}
	return out.LagSeconds, nil
}

// OrderFunnelReports implements ports.OrderFunnelReportClient against
// order-management's order-reports reader.
type OrderFunnelReports struct{ reportsClient }

func NewOrderFunnelReports(baseURL string, timeout time.Duration) *OrderFunnelReports {
	return &OrderFunnelReports{newReportsClient(baseURL, timeout)}
}

var _ ports.OrderFunnelReportClient = (*OrderFunnelReports)(nil)

func (c *OrderFunnelReports) GetFunnelReport(ctx context.Context, from, to time.Time) (ports.FunnelReport, error) {
	var out ports.FunnelReport
	if err := c.report(ctx, "/reports/funnel", from, to, &out); err != nil {
		return ports.FunnelReport{}, err
	}
	return out, nil
}

func (c *OrderFunnelReports) GetFunnelFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return c.freshness(ctx, "/reports/funnel/freshness")
}

// FlowAccuracyReports implements ports.FlowAccuracyReportClient against
// inventory-storage's inventory-reports reader.
type FlowAccuracyReports struct{ reportsClient }

func NewFlowAccuracyReports(baseURL string, timeout time.Duration) *FlowAccuracyReports {
	return &FlowAccuracyReports{newReportsClient(baseURL, timeout)}
}

var _ ports.FlowAccuracyReportClient = (*FlowAccuracyReports)(nil)

func (c *FlowAccuracyReports) GetFlowAccuracyReport(ctx context.Context, from, to time.Time) (ports.FlowAccuracyReport, error) {
	var out ports.FlowAccuracyReport
	if err := c.report(ctx, "/reports/flow-accuracy", from, to, &out); err != nil {
		return ports.FlowAccuracyReport{}, err
	}
	return out, nil
}

func (c *FlowAccuracyReports) GetFlowAccuracyFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return c.freshness(ctx, "/reports/flow-accuracy/freshness")
}

// CatalogGrowthReports implements ports.CatalogGrowthReportClient against
// facility-layout's facility-reports reader.
type CatalogGrowthReports struct{ reportsClient }

func NewCatalogGrowthReports(baseURL string, timeout time.Duration) *CatalogGrowthReports {
	return &CatalogGrowthReports{newReportsClient(baseURL, timeout)}
}

var _ ports.CatalogGrowthReportClient = (*CatalogGrowthReports)(nil)

func (c *CatalogGrowthReports) GetCatalogGrowthReport(ctx context.Context, from, to time.Time) (ports.CatalogGrowthReport, error) {
	var out ports.CatalogGrowthReport
	if err := c.report(ctx, "/reports/catalog-growth", from, to, &out); err != nil {
		return ports.CatalogGrowthReport{}, err
	}
	return out, nil
}

func (c *CatalogGrowthReports) GetCatalogGrowthFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return c.freshness(ctx, "/reports/catalog-growth/freshness")
}

// PlanningThroughputReports implements
// ports.PlanningThroughputReportClient against wes-work-planning's
// wes-reports reader.
type PlanningThroughputReports struct{ reportsClient }

func NewPlanningThroughputReports(baseURL string, timeout time.Duration) *PlanningThroughputReports {
	return &PlanningThroughputReports{newReportsClient(baseURL, timeout)}
}

var _ ports.PlanningThroughputReportClient = (*PlanningThroughputReports)(nil)

func (c *PlanningThroughputReports) GetPlanningThroughputReport(ctx context.Context, from, to time.Time) (ports.PlanningThroughputReport, error) {
	var out ports.PlanningThroughputReport
	if err := c.report(ctx, "/reports/throughput", from, to, &out); err != nil {
		return ports.PlanningThroughputReport{}, err
	}
	return out, nil
}

func (c *PlanningThroughputReports) GetPlanningThroughputFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return c.freshness(ctx, "/reports/throughput/freshness")
}

// FulfillmentThroughputReports implements
// ports.FulfillmentThroughputReportClient against fulfillment-execution's
// fulfillment-reports reader. Same /reports/throughput path as wes's
// reader, a different service on a different base URL with a different
// row shape (taskType/stationId, not pathId).
type FulfillmentThroughputReports struct{ reportsClient }

func NewFulfillmentThroughputReports(baseURL string, timeout time.Duration) *FulfillmentThroughputReports {
	return &FulfillmentThroughputReports{newReportsClient(baseURL, timeout)}
}

var _ ports.FulfillmentThroughputReportClient = (*FulfillmentThroughputReports)(nil)

func (c *FulfillmentThroughputReports) GetFulfillmentThroughputReport(ctx context.Context, from, to time.Time) (ports.FulfillmentThroughputReport, error) {
	var out ports.FulfillmentThroughputReport
	if err := c.report(ctx, "/reports/throughput", from, to, &out); err != nil {
		return ports.FulfillmentThroughputReport{}, err
	}
	return out, nil
}

func (c *FulfillmentThroughputReports) GetFulfillmentThroughputFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return c.freshness(ctx, "/reports/throughput/freshness")
}

// LaborReports implements ports.LaborReportClient against
// workforce-management's workforce-reports reader.
type LaborReports struct{ reportsClient }

func NewLaborReports(baseURL string, timeout time.Duration) *LaborReports {
	return &LaborReports{newReportsClient(baseURL, timeout)}
}

var _ ports.LaborReportClient = (*LaborReports)(nil)

func (c *LaborReports) GetLaborReport(ctx context.Context, from, to time.Time) (ports.LaborReport, error) {
	var out ports.LaborReport
	if err := c.report(ctx, "/reports/labor", from, to, &out); err != nil {
		return ports.LaborReport{}, err
	}
	return out, nil
}

func (c *LaborReports) GetLaborFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return c.freshness(ctx, "/reports/labor/freshness")
}

// LaborPerformanceReports implements
// ports.LaborPerformanceReportClient against labor-performance's
// labor-reports reader.
//
// This upstream is the fleet's newest and may not be deployed at all in
// a given environment. That is handled where it belongs -- the use case
// degrades this one section -- rather than here: a connection refused,
// a 404 from a service without the route, and a 500 all surface as an
// ordinary error from httpGetJSON, and none of them are special-cased
// into a fabricated empty report.
type LaborPerformanceReports struct{ reportsClient }

func NewLaborPerformanceReports(baseURL string, timeout time.Duration) *LaborPerformanceReports {
	return &LaborPerformanceReports{newReportsClient(baseURL, timeout)}
}

var _ ports.LaborPerformanceReportClient = (*LaborPerformanceReports)(nil)

func (c *LaborPerformanceReports) GetLaborPerformanceReport(ctx context.Context, from, to time.Time) (ports.LaborPerformanceReport, error) {
	var out ports.LaborPerformanceReport
	if err := c.report(ctx, "/reports/performance", from, to, &out); err != nil {
		return ports.LaborPerformanceReport{}, err
	}
	return out, nil
}

func (c *LaborPerformanceReports) GetLaborPerformanceFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return c.freshness(ctx, "/reports/performance/freshness")
}
