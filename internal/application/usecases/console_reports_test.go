package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// The fakes below are in-memory stand-ins for the seven analytics
// report ports -- no live server, mirroring the fakeOrderManagement /
// fakeWorkUnits convention in order_lifecycle_test.go. Each carries an
// optional err for the report call and a separate freshnessErr, because
// the two failure modes have deliberately different consequences: a
// failed report degrades the section, a failed freshness call only drops
// the annotation.

type fakeOrderFunnelReports struct {
	report       ports.FunnelReport
	err          error
	freshness    float64
	freshnessErr error
}

func (f *fakeOrderFunnelReports) GetFunnelReport(ctx context.Context, from, to time.Time) (ports.FunnelReport, error) {
	return f.report, f.err
}

func (f *fakeOrderFunnelReports) GetFunnelFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return f.freshness, f.freshnessErr
}

type fakeFlowAccuracyReports struct {
	report       ports.FlowAccuracyReport
	err          error
	freshness    float64
	freshnessErr error
}

func (f *fakeFlowAccuracyReports) GetFlowAccuracyReport(ctx context.Context, from, to time.Time) (ports.FlowAccuracyReport, error) {
	return f.report, f.err
}

func (f *fakeFlowAccuracyReports) GetFlowAccuracyFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return f.freshness, f.freshnessErr
}

type fakeCatalogGrowthReports struct {
	report       ports.CatalogGrowthReport
	err          error
	freshness    float64
	freshnessErr error
}

func (f *fakeCatalogGrowthReports) GetCatalogGrowthReport(ctx context.Context, from, to time.Time) (ports.CatalogGrowthReport, error) {
	return f.report, f.err
}

func (f *fakeCatalogGrowthReports) GetCatalogGrowthFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return f.freshness, f.freshnessErr
}

type fakePlanningThroughputReports struct {
	report       ports.PlanningThroughputReport
	err          error
	freshness    float64
	freshnessErr error
}

func (f *fakePlanningThroughputReports) GetPlanningThroughputReport(ctx context.Context, from, to time.Time) (ports.PlanningThroughputReport, error) {
	return f.report, f.err
}

func (f *fakePlanningThroughputReports) GetPlanningThroughputFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return f.freshness, f.freshnessErr
}

type fakeFulfillmentThroughputReports struct {
	report       ports.FulfillmentThroughputReport
	err          error
	freshness    float64
	freshnessErr error
}

func (f *fakeFulfillmentThroughputReports) GetFulfillmentThroughputReport(ctx context.Context, from, to time.Time) (ports.FulfillmentThroughputReport, error) {
	return f.report, f.err
}

func (f *fakeFulfillmentThroughputReports) GetFulfillmentThroughputFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return f.freshness, f.freshnessErr
}

type fakeLaborReports struct {
	report       ports.LaborReport
	err          error
	freshness    float64
	freshnessErr error
}

func (f *fakeLaborReports) GetLaborReport(ctx context.Context, from, to time.Time) (ports.LaborReport, error) {
	return f.report, f.err
}

func (f *fakeLaborReports) GetLaborFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return f.freshness, f.freshnessErr
}

type fakeLaborPerformanceReports struct {
	report       ports.LaborPerformanceReport
	err          error
	freshness    float64
	freshnessErr error
}

func (f *fakeLaborPerformanceReports) GetLaborPerformanceReport(ctx context.Context, from, to time.Time) (ports.LaborPerformanceReport, error) {
	return f.report, f.err
}

func (f *fakeLaborPerformanceReports) GetLaborPerformanceFreshnessLagSeconds(ctx context.Context) (float64, error) {
	return f.freshness, f.freshnessErr
}

var (
	_ ports.OrderFunnelReportClient           = (*fakeOrderFunnelReports)(nil)
	_ ports.FlowAccuracyReportClient          = (*fakeFlowAccuracyReports)(nil)
	_ ports.CatalogGrowthReportClient         = (*fakeCatalogGrowthReports)(nil)
	_ ports.PlanningThroughputReportClient    = (*fakePlanningThroughputReports)(nil)
	_ ports.FulfillmentThroughputReportClient = (*fakeFulfillmentThroughputReports)(nil)
	_ ports.LaborReportClient                 = (*fakeLaborReports)(nil)
	_ ports.LaborPerformanceReportClient      = (*fakeLaborPerformanceReports)(nil)
)

func floatPtr(v float64) *float64 { return &v }

// testWindow is a fixed window so assertions never depend on wall clock.
var (
	testFrom = time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	testTo   = time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
)

func findSection(t *testing.T, result usecases.DashboardResult, id string) usecases.ReportSection {
	t.Helper()
	for _, s := range result.Sections {
		if s.Id == id {
			return s
		}
	}
	t.Fatalf("section %q not found in %+v", id, result.Sections)
	return usecases.ReportSection{}
}

func assertSeries(t *testing.T, got []usecases.SeriesPoint, want []usecases.SeriesPoint) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("series length: got %d (%+v), want %d (%+v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("series[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// --- WMS ---------------------------------------------------------------

func newWMSUseCase() *usecases.ConsoleReports {
	return &usecases.ConsoleReports{
		OrderFunnel: &fakeOrderFunnelReports{
			freshness: 12.5,
			report: ports.FunnelReport{Rows: []ports.FunnelRow{
				{PathId: "pick-zone-a", HourBucket: "2026-09-04T00:00:00Z", OrdersReceived: 100, OrdersAllocated: 70, OrdersPartiallyAllocated: 8, OrdersReleased: 60, OrdersCancelled: 3, OrdersAllocationFailed: 2},
				{PathId: "pick-zone-b", HourBucket: "2026-09-04T01:00:00Z", OrdersReceived: 20, OrdersAllocated: 20, OrdersPartiallyAllocated: 0, OrdersReleased: 20, OrdersCancelled: 0, OrdersAllocationFailed: 0},
			}},
		},
		InventoryFlowAccuracy: &fakeFlowAccuracyReports{
			freshness: 3,
			report: ports.FlowAccuracyReport{Rows: []ports.FlowAccuracyRow{
				{SKU: "SKU-1", StowedCount: 10, PickedQuantity: 40, DiscrepanciesDetected: 1, UnlocatedCount: 2},
				{SKU: "SKU-2", StowedCount: 5, PickedQuantity: 15, DiscrepanciesDetected: 0, UnlocatedCount: 1},
			}},
		},
		CatalogGrowth: &fakeCatalogGrowthReports{
			freshness: 60,
			report: ports.CatalogGrowthReport{Rows: []ports.CatalogGrowthRow{
				// Two scopes on the same day must fold into ONE point.
				{Scope: "WH1", DayBucket: "2026-09-04T00:00:00Z", SlotsRegistered: 30},
				{Scope: "WH2", DayBucket: "2026-09-04T00:00:00Z", SlotsRegistered: 12},
				// Deliberately out of order to prove the sort.
				{Scope: "WH1", DayBucket: "2026-09-03T00:00:00Z", SlotsRegistered: 7},
			}},
		},
		Now: func() time.Time { return testTo },
	}
}

func TestConsoleReports_ExecuteWMS_AllSectionsAvailable(t *testing.T) {
	result := newWMSUseCase().ExecuteWMS(context.Background(), testFrom, testTo)

	if len(result.Sections) != 3 {
		t.Fatalf("expected 3 WMS sections, got %d", len(result.Sections))
	}
	// Section order is the console's render order and must not depend on
	// which upstream answered first -- the whole point of reassembling
	// the concurrent fan-out by index.
	wantOrder := []string{"order-funnel", "inventory-flow-accuracy", "catalog-growth"}
	for i, id := range wantOrder {
		if result.Sections[i].Id != id {
			t.Fatalf("section %d: got id %q, want %q", i, result.Sections[i].Id, id)
		}
		if !result.Sections[i].Available {
			t.Fatalf("section %q should be available, error=%q", id, result.Sections[i].Error)
		}
	}

	funnel := findSection(t, result, "order-funnel")
	if funnel.ChartKind != usecases.ChartKindFunnel || funnel.SourceContext != "order-management" {
		t.Fatalf("unexpected funnel metadata: %+v", funnel)
	}
	if funnel.FreshnessLagSeconds == nil || *funnel.FreshnessLagSeconds != 12.5 {
		t.Fatalf("expected freshness 12.5, got %v", funnel.FreshnessLagSeconds)
	}
	assertSeries(t, funnel.Series, []usecases.SeriesPoint{
		{Label: "Received", Value: 120},
		{Label: "Allocated", Value: 98}, // 70+8 + 20+0
		{Label: "Released", Value: 80},
		{Label: "Cancelled / Failed", Value: 5}, // 3+2
	})

	flow := findSection(t, result, "inventory-flow-accuracy")
	if flow.ChartKind != usecases.ChartKindBar {
		t.Fatalf("expected bar chartKind, got %q", flow.ChartKind)
	}
	assertSeries(t, flow.Series, []usecases.SeriesPoint{
		{Label: "Stowed", Value: 15},
		{Label: "Picked", Value: 55},
		{Label: "Discrepancies", Value: 1},
		{Label: "Unlocated", Value: 3},
	})

	catalog := findSection(t, result, "catalog-growth")
	if catalog.ChartKind != usecases.ChartKindLine {
		t.Fatalf("expected line chartKind, got %q", catalog.ChartKind)
	}
	assertSeries(t, catalog.Series, []usecases.SeriesPoint{
		{Label: "2026-09-03T00:00:00Z", Value: 7},
		{Label: "2026-09-04T00:00:00Z", Value: 42}, // 30 + 12, same day, two scopes
	})

	if !result.From.Equal(testFrom) || !result.To.Equal(testTo) {
		t.Fatalf("window not echoed back: %v..%v", result.From, result.To)
	}
}

func TestConsoleReports_ExecuteWMS_OneUpstreamDown_DegradesOnlyThatSection(t *testing.T) {
	uc := newWMSUseCase()
	uc.InventoryFlowAccuracy = &fakeFlowAccuracyReports{err: errors.New("connection refused")}

	result := uc.ExecuteWMS(context.Background(), testFrom, testTo)

	flow := findSection(t, result, "inventory-flow-accuracy")
	if flow.Available {
		t.Fatal("expected the failing section to be unavailable")
	}
	if flow.Error == "" {
		t.Fatal("expected a human-readable error on the degraded section")
	}
	if len(flow.Series) != 0 {
		t.Fatalf("expected an empty series on the degraded section, got %+v", flow.Series)
	}
	if flow.Series == nil {
		t.Fatal("series must be an empty slice, not nil, so it marshals as [] rather than null")
	}

	// The other two must be untouched -- one dead upstream never 500s or
	// blanks the dashboard.
	for _, id := range []string{"order-funnel", "catalog-growth"} {
		if s := findSection(t, result, id); !s.Available || len(s.Series) == 0 {
			t.Fatalf("section %q should be unaffected by a sibling failure, got %+v", id, s)
		}
	}
}

func TestConsoleReports_ExecuteWMS_UnwiredClient_DegradesThatSection(t *testing.T) {
	uc := newWMSUseCase()
	uc.CatalogGrowth = nil

	result := uc.ExecuteWMS(context.Background(), testFrom, testTo)

	catalog := findSection(t, result, "catalog-growth")
	if catalog.Available || catalog.Error == "" {
		t.Fatalf("an unwired client must degrade its section, got %+v", catalog)
	}
	if s := findSection(t, result, "order-funnel"); !s.Available {
		t.Fatal("a wired sibling must still be available")
	}
}

func TestConsoleReports_FreshnessFailure_KeepsSectionAvailable(t *testing.T) {
	uc := newWMSUseCase()
	uc.OrderFunnel = &fakeOrderFunnelReports{
		report:       ports.FunnelReport{Rows: []ports.FunnelRow{{OrdersReceived: 5}}},
		freshnessErr: errors.New("freshness endpoint down"),
	}

	result := uc.ExecuteWMS(context.Background(), testFrom, testTo)

	funnel := findSection(t, result, "order-funnel")
	if !funnel.Available {
		t.Fatalf("freshness is an annotation, not a gate: section should stay available, got %+v", funnel)
	}
	if funnel.FreshnessLagSeconds != nil {
		t.Fatalf("expected nil freshness when only the freshness call failed, got %v", *funnel.FreshnessLagSeconds)
	}
	if funnel.Series[0].Value != 5 {
		t.Fatalf("the report itself must still be rendered, got %+v", funnel.Series)
	}
}

// --- WES ---------------------------------------------------------------

func newWESUseCase() *usecases.ConsoleReports {
	return &usecases.ConsoleReports{
		PlanningThroughput: &fakePlanningThroughputReports{
			freshness: 4,
			report: ports.PlanningThroughputReport{Rows: []ports.PlanningThroughputRow{
				// Same hour, two paths -> one summed point.
				{PathId: "a", HourBucket: "2026-09-04T01:00:00Z", WorkUnitCompleted: 10},
				{PathId: "b", HourBucket: "2026-09-04T01:00:00Z", WorkUnitCompleted: 5},
				{PathId: "a", HourBucket: "2026-09-04T00:00:00Z", WorkUnitCompleted: 3},
			}},
		},
		FulfillmentThroughput: &fakeFulfillmentThroughputReports{
			freshness: 9,
			report: ports.FulfillmentThroughputReport{Rows: []ports.FulfillmentThroughputRow{
				{TaskType: "PICK", StationId: "s1", Completions: 40},
				{TaskType: "PICK", StationId: "s2", Completions: 20},
				{TaskType: "PACK", StationId: "s1", Completions: 30},
			}},
		},
		Labor: &fakeLaborReports{
			freshness: 15,
			report: ports.LaborReport{Rows: []ports.LaborRow{
				{PathId: "a", ShiftsStarted: 4, LaborAssigned: 12, UnderstaffingEvents: 1},
				{PathId: "b", ShiftsStarted: 2, LaborAssigned: 6, UnderstaffingEvents: 0},
			}},
		},
		LaborPerformance: &fakeLaborPerformanceReports{
			freshness: 7,
			report: ports.LaborPerformanceReport{ByTaskType: []ports.LaborPerformanceTaskType{
				{TaskType: "PICK", MeanEfficiencyPct: floatPtr(92.5)},
				// A null mean must be SKIPPED, never charted as a 0% bar.
				{TaskType: "PACK", MeanEfficiencyPct: nil},
				{TaskType: "SLAM", MeanEfficiencyPct: floatPtr(80)},
			}},
		},
		Now: func() time.Time { return testTo },
	}
}

func TestConsoleReports_ExecuteWES_AllSectionsAvailable(t *testing.T) {
	result := newWESUseCase().ExecuteWES(context.Background(), testFrom, testTo)

	wantOrder := []string{"planning-throughput", "fulfillment-throughput", "labor-management", "labor-performance"}
	if len(result.Sections) != len(wantOrder) {
		t.Fatalf("expected %d WES sections, got %d", len(wantOrder), len(result.Sections))
	}
	for i, id := range wantOrder {
		if result.Sections[i].Id != id {
			t.Fatalf("section %d: got id %q, want %q", i, result.Sections[i].Id, id)
		}
		if !result.Sections[i].Available {
			t.Fatalf("section %q should be available, error=%q", id, result.Sections[i].Error)
		}
	}

	planning := findSection(t, result, "planning-throughput")
	if planning.ChartKind != usecases.ChartKindLine {
		t.Fatalf("expected line chartKind, got %q", planning.ChartKind)
	}
	assertSeries(t, planning.Series, []usecases.SeriesPoint{
		{Label: "2026-09-04T00:00:00Z", Value: 3},
		{Label: "2026-09-04T01:00:00Z", Value: 15}, // 10 + 5 across two paths
	})

	fulfillment := findSection(t, result, "fulfillment-throughput")
	assertSeries(t, fulfillment.Series, []usecases.SeriesPoint{
		{Label: "PACK", Value: 30},
		{Label: "PICK", Value: 60}, // 40 + 20 across two stations
	})

	labor := findSection(t, result, "labor-management")
	assertSeries(t, labor.Series, []usecases.SeriesPoint{
		{Label: "Shifts Started", Value: 6},
		{Label: "Labor Assigned", Value: 18},
		{Label: "Understaffing Events", Value: 1},
	})
}

// TestConsoleReports_ExecuteWES_NullMeanEfficiencyIsSkippedNotZeroed is
// the anti-fabrication guard. labor-performance deliberately publishes
// JSON null (never 0) for a task type with nothing scorable; charting
// that as a 0% bar would tell an operator "PACK ran at 0% efficiency"
// when the truth is "we don't know". A regression here silently
// launders missing data into a catastrophic-looking measurement.
func TestConsoleReports_ExecuteWES_NullMeanEfficiencyIsSkippedNotZeroed(t *testing.T) {
	result := newWESUseCase().ExecuteWES(context.Background(), testFrom, testTo)

	perf := findSection(t, result, "labor-performance")
	if !perf.Available {
		t.Fatalf("labor-performance should be available, error=%q", perf.Error)
	}
	assertSeries(t, perf.Series, []usecases.SeriesPoint{
		{Label: "PICK", Value: 92.5},
		{Label: "SLAM", Value: 80},
	})
	for _, p := range perf.Series {
		if p.Label == "PACK" {
			t.Fatal("a null meanEfficiencyPct must be skipped, never coerced to a 0 bar")
		}
	}
}

// TestConsoleReports_ExecuteWES_LaborPerformanceUnavailable_DegradesOnlyThatSection
// covers the concrete deployment reality that labor-performance's
// reports reader ships in a separate PR and may simply not exist yet:
// connection refused, or a 404 from a service without the route.
func TestConsoleReports_ExecuteWES_LaborPerformanceUnavailable_DegradesOnlyThatSection(t *testing.T) {
	uc := newWESUseCase()
	uc.LaborPerformance = &fakeLaborPerformanceReports{err: ports.ErrNotFound}

	result := uc.ExecuteWES(context.Background(), testFrom, testTo)

	perf := findSection(t, result, "labor-performance")
	if perf.Available {
		t.Fatal("expected labor-performance to degrade when its reader is absent")
	}
	if perf.Error != "labor-performance reports not available" {
		t.Fatalf("unexpected degradation message: %q", perf.Error)
	}
	if len(perf.Series) != 0 {
		t.Fatalf("expected an empty series, got %+v", perf.Series)
	}

	for _, id := range []string{"planning-throughput", "fulfillment-throughput", "labor-management"} {
		if s := findSection(t, result, id); !s.Available || len(s.Series) == 0 {
			t.Fatalf("section %q must survive labor-performance being down, got %+v", id, s)
		}
	}
}

// --- window resolution -------------------------------------------------

func TestConsoleReports_ResolveWindow_DefaultsToTrailing24Hours(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	uc := &usecases.ConsoleReports{Now: func() time.Time { return now }}

	from, to := uc.ResolveWindow(nil, nil)
	if !to.Equal(now) {
		t.Fatalf("expected to=now, got %v", to)
	}
	if !from.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("expected from=now-24h, got %v", from)
	}
}

func TestConsoleReports_ResolveWindow_HonoursExplicitBounds(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	uc := &usecases.ConsoleReports{Now: func() time.Time { return now }}

	explicitFrom := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	explicitTo := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	from, to := uc.ResolveWindow(&explicitFrom, &explicitTo)
	if !from.Equal(explicitFrom) || !to.Equal(explicitTo) {
		t.Fatalf("explicit window not honoured: %v..%v", from, to)
	}

	// A supplied `to` alone anchors the default 24h window to it, rather
	// than silently mixing an explicit end with a now-relative start.
	from, to = uc.ResolveWindow(nil, &explicitTo)
	if !to.Equal(explicitTo) || !from.Equal(explicitTo.Add(-24*time.Hour)) {
		t.Fatalf("expected the default window to anchor to an explicit 'to': %v..%v", from, to)
	}
}
