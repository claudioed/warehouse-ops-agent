package usecases

import (
	"context"
	"sort"
	"time"
)

// ExecuteWES assembles the WES Dashboard: planning throughput
// (wes-work-planning), fulfillment throughput (fulfillment-execution),
// workforce labor (workforce-management) and labor performance
// (labor-performance), fetched concurrently and in that display order.
//
// Same contract as ExecuteWMS: no error return, every failure is a
// degraded section. That matters most for the fourth section --
// labor-performance's reports reader is the fleet's newest and may not
// be deployed at all, in which case the other three panels must still
// render.
func (uc *ConsoleReports) ExecuteWES(ctx context.Context, from, to time.Time) DashboardResult {
	return uc.runSections(ctx, from, to, []sectionSpec{
		{
			id:            "planning-throughput",
			title:         "Planning Throughput",
			sourceContext: "wes-work-planning",
			chartKind:     ChartKindLine,
			wired:         uc.PlanningThroughput != nil,
			fetch:         uc.fetchPlanningThroughput,
			freshness:     uc.freshnessPlanningThroughput,
		},
		{
			id:            "fulfillment-throughput",
			title:         "Fulfillment Throughput",
			sourceContext: "fulfillment-execution",
			chartKind:     ChartKindBar,
			wired:         uc.FulfillmentThroughput != nil,
			fetch:         uc.fetchFulfillmentThroughput,
			freshness:     uc.freshnessFulfillmentThroughput,
		},
		{
			id:            "labor-management",
			title:         "Workforce Labor",
			sourceContext: "workforce-management",
			chartKind:     ChartKindBar,
			wired:         uc.Labor != nil,
			fetch:         uc.fetchLabor,
			freshness:     uc.freshnessLabor,
		},
		{
			id:            "labor-performance",
			title:         "Labor Performance (Efficiency %)",
			sourceContext: "labor-performance",
			chartKind:     ChartKindBar,
			wired:         uc.LaborPerformance != nil,
			fetch:         uc.fetchLaborPerformance,
			freshness:     uc.freshnessLaborPerformance,
		},
	})
}

// fetchPlanningThroughput plots one point per distinct hourBucket,
// summing workUnitCompleted across all pathIds in that hour.
func (uc *ConsoleReports) fetchPlanningThroughput(ctx context.Context, from, to time.Time) ([]SeriesPoint, error) {
	report, err := uc.PlanningThroughput.GetPlanningThroughputReport(ctx, from, to)
	if err != nil {
		return nil, err
	}

	byHour := make(map[string]float64, len(report.Rows))
	for _, row := range report.Rows {
		byHour[row.HourBucket] += float64(row.WorkUnitCompleted)
	}
	return bucketSeries(byHour), nil
}

func (uc *ConsoleReports) freshnessPlanningThroughput(ctx context.Context) (float64, error) {
	return uc.PlanningThroughput.GetPlanningThroughputFreshnessLagSeconds(ctx)
}

// fetchFulfillmentThroughput groups by taskType: one bar per distinct
// task type, summing completions across every station and hour.
// Task-type order is alphabetical so the bar chart is stable across
// refreshes (the upstream's row order is not a guaranteed contract).
func (uc *ConsoleReports) fetchFulfillmentThroughput(ctx context.Context, from, to time.Time) ([]SeriesPoint, error) {
	report, err := uc.FulfillmentThroughput.GetFulfillmentThroughputReport(ctx, from, to)
	if err != nil {
		return nil, err
	}

	byTaskType := make(map[string]float64, len(report.Rows))
	for _, row := range report.Rows {
		byTaskType[row.TaskType] += float64(row.Completions)
	}

	taskTypes := make([]string, 0, len(byTaskType))
	for taskType := range byTaskType {
		taskTypes = append(taskTypes, taskType)
	}
	sort.Strings(taskTypes)

	series := make([]SeriesPoint, 0, len(taskTypes))
	for _, taskType := range taskTypes {
		series = append(series, SeriesPoint{Label: taskType, Value: byTaskType[taskType]})
	}
	return series, nil
}

func (uc *ConsoleReports) freshnessFulfillmentThroughput(ctx context.Context) (float64, error) {
	return uc.FulfillmentThroughput.GetFulfillmentThroughputFreshnessLagSeconds(ctx)
}

// fetchLabor sums the three headline staffing counters across every
// (pathId, hour) row in the window.
func (uc *ConsoleReports) fetchLabor(ctx context.Context, from, to time.Time) ([]SeriesPoint, error) {
	report, err := uc.Labor.GetLaborReport(ctx, from, to)
	if err != nil {
		return nil, err
	}

	var shiftsStarted, laborAssigned, understaffing int
	for _, row := range report.Rows {
		shiftsStarted += row.ShiftsStarted
		laborAssigned += row.LaborAssigned
		understaffing += row.UnderstaffingEvents
	}

	return []SeriesPoint{
		{Label: "Shifts Started", Value: float64(shiftsStarted)},
		{Label: "Labor Assigned", Value: float64(laborAssigned)},
		{Label: "Understaffing Events", Value: float64(understaffing)},
	}, nil
}

func (uc *ConsoleReports) freshnessLabor(ctx context.Context) (float64, error) {
	return uc.Labor.GetLaborFreshnessLagSeconds(ctx)
}

// fetchLaborPerformance uses labor-performance's own byTaskType
// breakdown directly -- that service already did the mean-efficiency
// arithmetic correctly, including deciding when a mean does not exist.
//
// Entries whose meanEfficiencyPct is null are SKIPPED, never coerced to
// 0. This is the load-bearing bit: labor-performance goes out of its way
// to serialise JSON null rather than a fabricated 0 when nothing in a
// bucket was scorable (its ADR-0004/0006/0007), and plotting a 0% bar
// for "we don't know" would re-fabricate exactly the number that
// discipline exists to avoid -- an operator would read a missing
// measurement as a catastrophic one. A task type with no scorable work
// simply has no bar.
func (uc *ConsoleReports) fetchLaborPerformance(ctx context.Context, from, to time.Time) ([]SeriesPoint, error) {
	report, err := uc.LaborPerformance.GetLaborPerformanceReport(ctx, from, to)
	if err != nil {
		return nil, err
	}

	series := make([]SeriesPoint, 0, len(report.ByTaskType))
	for _, bar := range report.ByTaskType {
		if bar.MeanEfficiencyPct == nil {
			continue
		}
		series = append(series, SeriesPoint{Label: bar.TaskType, Value: *bar.MeanEfficiencyPct})
	}
	return series, nil
}

func (uc *ConsoleReports) freshnessLaborPerformance(ctx context.Context) (float64, error) {
	return uc.LaborPerformance.GetLaborPerformanceFreshnessLagSeconds(ctx)
}
