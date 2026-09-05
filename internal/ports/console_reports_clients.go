package ports

import (
	"context"
	"time"
)

// ConsoleReportsClients are the seven bounded-context ANALYTICS REST
// clients the console-bff fans out to for GET /console/reports/wms and
// GET /console/reports/wes. Like OrderLifecycleClients above these are
// plain REST calls against each context's own published HTTP surface --
// never a Go import of another service's packages, and never a database
// read (governance charter: no cross-service DB reads, ever).
//
// They are a THIRD port shape, deliberately distinct from both the MCP
// clients (LLM-facing, curated tools) and the order-lifecycle REST
// clients (OLTP entity lookups): these read each context's separate
// ANALYTICAL store via its *-reports reader binary, a different service,
// a different database and a different base URL from the OLTP API the
// order-lifecycle clients call.
//
// Each interface pairs the report query with that context's own
// freshness endpoint, because the two are always consumed together: a
// dashboard section renders the series AND how stale it is. Freshness is
// an annotation, not a gate -- see usecases.ConsoleReports for how a
// failing freshness call still yields an available section.
type (
	// OrderFunnelReportClient reads order-management's
	// GET /reports/funnel (+ /freshness).
	OrderFunnelReportClient interface {
		GetFunnelReport(ctx context.Context, from, to time.Time) (FunnelReport, error)
		GetFunnelFreshnessLagSeconds(ctx context.Context) (float64, error)
	}

	// FlowAccuracyReportClient reads inventory-storage's
	// GET /reports/flow-accuracy (+ /freshness).
	FlowAccuracyReportClient interface {
		GetFlowAccuracyReport(ctx context.Context, from, to time.Time) (FlowAccuracyReport, error)
		GetFlowAccuracyFreshnessLagSeconds(ctx context.Context) (float64, error)
	}

	// CatalogGrowthReportClient reads facility-layout's
	// GET /reports/catalog-growth (+ /freshness).
	CatalogGrowthReportClient interface {
		GetCatalogGrowthReport(ctx context.Context, from, to time.Time) (CatalogGrowthReport, error)
		GetCatalogGrowthFreshnessLagSeconds(ctx context.Context) (float64, error)
	}

	// PlanningThroughputReportClient reads wes-work-planning's
	// GET /reports/throughput (+ /freshness).
	PlanningThroughputReportClient interface {
		GetPlanningThroughputReport(ctx context.Context, from, to time.Time) (PlanningThroughputReport, error)
		GetPlanningThroughputFreshnessLagSeconds(ctx context.Context) (float64, error)
	}

	// FulfillmentThroughputReportClient reads fulfillment-execution's
	// GET /reports/throughput (+ /freshness). Same path as wes's, a
	// different service and a different row shape -- hence two ports.
	FulfillmentThroughputReportClient interface {
		GetFulfillmentThroughputReport(ctx context.Context, from, to time.Time) (FulfillmentThroughputReport, error)
		GetFulfillmentThroughputFreshnessLagSeconds(ctx context.Context) (float64, error)
	}

	// LaborReportClient reads workforce-management's
	// GET /reports/labor (+ /freshness).
	LaborReportClient interface {
		GetLaborReport(ctx context.Context, from, to time.Time) (LaborReport, error)
		GetLaborFreshnessLagSeconds(ctx context.Context) (float64, error)
	}

	// LaborPerformanceReportClient reads labor-performance's
	// GET /reports/performance (+ /freshness). This upstream may not
	// exist yet in a given deployment (its reports reader ships in a
	// separate PR); the use case degrades that ONE section rather than
	// treating its absence as a failure of the dashboard.
	LaborPerformanceReportClient interface {
		GetLaborPerformanceReport(ctx context.Context, from, to time.Time) (LaborPerformanceReport, error)
		GetLaborPerformanceFreshnessLagSeconds(ctx context.Context) (float64, error)
	}
)

// FunnelReport mirrors order-management's funnelReportDTO wire shape.
type FunnelReport struct {
	Rows []FunnelRow `json:"rows"`
}

type FunnelRow struct {
	PathId                   string `json:"pathId"`
	HourBucket               string `json:"hourBucket"`
	OrdersReceived           int    `json:"ordersReceived"`
	OrdersAllocated          int    `json:"ordersAllocated"`
	OrdersPartiallyAllocated int    `json:"ordersPartiallyAllocated"`
	OrdersAllocationFailed   int    `json:"ordersAllocationFailed"`
	OrdersReleased           int    `json:"ordersReleased"`
	OrdersCancelled          int    `json:"ordersCancelled"`
	LinesAllocated           int    `json:"linesAllocated"`
	LinesBackordered         int    `json:"linesBackordered"`
	LinesReleased            int    `json:"linesReleased"`
}

// FlowAccuracyReport mirrors inventory-storage's flowAccuracyReportDTO.
type FlowAccuracyReport struct {
	Rows []FlowAccuracyRow `json:"rows"`
}

type FlowAccuracyRow struct {
	SKU                   string `json:"sku"`
	BinId                 string `json:"binId"`
	HourBucket            string `json:"hourBucket"`
	ReceivedQuantity      int    `json:"receivedQuantity"`
	StowedCount           int    `json:"stowedCount"`
	PickedQuantity        int    `json:"pickedQuantity"`
	ReservationsCreated   int    `json:"reservationsCreated"`
	ReservationsExpired   int    `json:"reservationsExpired"`
	ReservationsRevoked   int    `json:"reservationsRevoked"`
	CycleCountsCompleted  int    `json:"cycleCountsCompleted"`
	DiscrepanciesDetected int    `json:"discrepanciesDetected"`
	UnlocatedCount        int    `json:"unlocatedCount"`
}

// CatalogGrowthReport mirrors facility-layout's catalogReportDTO. Note
// the DAY (not hour) bucket -- facility-layout's projector is the one
// day-granular report in the fleet.
type CatalogGrowthReport struct {
	Rows []CatalogGrowthRow `json:"rows"`
}

type CatalogGrowthRow struct {
	Scope                   string `json:"scope"`
	DayBucket               string `json:"dayBucket"`
	SitesRegistered         int    `json:"sitesRegistered"`
	ZonesRegistered         int    `json:"zonesRegistered"`
	AislesRegistered        int    `json:"aislesRegistered"`
	LocationTypesRegistered int    `json:"locationTypesRegistered"`
	PlacementRulesDefined   int    `json:"placementRulesDefined"`
	SlotsRegistered         int    `json:"slotsRegistered"`
	SlotsDecommissioned     int    `json:"slotsDecommissioned"`
	BulkImports             int    `json:"bulkImports"`
	ImportRowsSubmitted     int    `json:"importRowsSubmitted"`
	ImportRowsImported      int    `json:"importRowsImported"`
	ImportRowsRejected      int    `json:"importRowsRejected"`
}

// PlanningThroughputReport mirrors wes-work-planning's
// throughputReportDTO.
type PlanningThroughputReport struct {
	Rows []PlanningThroughputRow `json:"rows"`
}

type PlanningThroughputRow struct {
	PathId                   string `json:"pathId"`
	HourBucket               string `json:"hourBucket"`
	WorkReleased             int    `json:"workReleased"`
	WorkUnitCompleted        int    `json:"workUnitCompleted"`
	BacklogThresholdBreached int    `json:"backlogThresholdBreached"`
	PathThrottled            int    `json:"pathThrottled"`
	RateDeviationDetected    int    `json:"rateDeviationDetected"`
}

// FulfillmentThroughputReport mirrors fulfillment-execution's
// throughputReportDTO.
type FulfillmentThroughputReport struct {
	Rows []FulfillmentThroughputRow `json:"rows"`
}

type FulfillmentThroughputRow struct {
	TaskType                  string  `json:"taskType"`
	StationId                 string  `json:"stationId"`
	HourBucket                string  `json:"hourBucket"`
	Completions               int     `json:"completions"`
	AvgClaimToCompleteSeconds float64 `json:"avgClaimToCompleteSeconds"`
	LeaseExpiries             int     `json:"leaseExpiries"`
	WeighCheckDiverts         int     `json:"weighCheckDiverts"`
}

// LaborReport mirrors workforce-management's laborReportDTO.
type LaborReport struct {
	Rows []LaborRow `json:"rows"`
}

type LaborRow struct {
	PathId              string  `json:"pathId"`
	HourBucket          string  `json:"hourBucket"`
	ShiftsStarted       int     `json:"shiftsStarted"`
	ShiftsEnded         int     `json:"shiftsEnded"`
	Breaks              int     `json:"breaks"`
	AvgBreakSeconds     float64 `json:"avgBreakSeconds"`
	Certifications      int     `json:"certifications"`
	LaborAssigned       int     `json:"laborAssigned"`
	LaborReassigned     int     `json:"laborReassigned"`
	UnderstaffingEvents int     `json:"understaffingEvents"`
}

// LaborPerformanceReport mirrors labor-performance's
// laborPerformanceReportDTO.
//
// MeanEfficiencyPct and MeanActualSeconds are POINTERS on purpose: that
// service deliberately serialises JSON null (never a fabricated 0) when
// nothing in the bucket was scorable/measurable -- a load-bearing
// discipline documented in its ADR-0004/0006/0007. Modelling them as
// float64 here would silently launder "no data" into "0% efficient" at
// the very first decode, so the nullability is carried all the way
// through to the chart series, which SKIPS null entries rather than
// plotting a zero bar.
type LaborPerformanceReport struct {
	From       string                     `json:"from"`
	To         string                     `json:"to"`
	Rows       []LaborPerformanceRow      `json:"rows"`
	ByTaskType []LaborPerformanceTaskType `json:"byTaskType"`
	Totals     LaborPerformanceTotals     `json:"totals"`
}

type LaborPerformanceRow struct {
	TaskType          string   `json:"taskType"`
	HourBucket        string   `json:"hourBucket"`
	TasksRecorded     int      `json:"tasksRecorded"`
	TasksScored       int      `json:"tasksScored"`
	TasksUnscored     int      `json:"tasksUnscored"`
	MeanEfficiencyPct *float64 `json:"meanEfficiencyPct"`
	MeanActualSeconds *float64 `json:"meanActualSeconds"`
	StandardsDefined  int      `json:"standardsDefined"`
	StandardsRevised  int      `json:"standardsRevised"`
}

type LaborPerformanceTaskType struct {
	TaskType          string   `json:"taskType"`
	TasksRecorded     int      `json:"tasksRecorded"`
	TasksScored       int      `json:"tasksScored"`
	TasksUnscored     int      `json:"tasksUnscored"`
	MeanEfficiencyPct *float64 `json:"meanEfficiencyPct"`
	MeanActualSeconds *float64 `json:"meanActualSeconds"`
	StandardsDefined  int      `json:"standardsDefined"`
	StandardsRevised  int      `json:"standardsRevised"`
}

type LaborPerformanceTotals struct {
	TasksRecorded     int      `json:"tasksRecorded"`
	TasksScored       int      `json:"tasksScored"`
	TasksUnscored     int      `json:"tasksUnscored"`
	MeanEfficiencyPct *float64 `json:"meanEfficiencyPct"`
	MeanActualSeconds *float64 `json:"meanActualSeconds"`
}
