package usecases

import (
	"context"
	"time"
)

// ExecuteWMS assembles the WMS Dashboard: order funnel (order-management),
// inventory flow accuracy (inventory-storage) and catalog growth
// (facility-layout), fetched concurrently and in that display order.
//
// It has no error return on purpose -- every failure mode is a degraded
// section, never a failed dashboard (see the package doc on
// console_reports.go).
func (uc *ConsoleReports) ExecuteWMS(ctx context.Context, from, to time.Time) DashboardResult {
	return uc.runSections(ctx, from, to, []sectionSpec{
		{
			id:            "order-funnel",
			title:         "Order Funnel",
			sourceContext: "order-management",
			chartKind:     ChartKindFunnel,
			wired:         uc.OrderFunnel != nil,
			fetch:         uc.fetchOrderFunnel,
			freshness:     uc.freshnessOrderFunnel,
		},
		{
			id:            "inventory-flow-accuracy",
			title:         "Inventory Flow Accuracy",
			sourceContext: "inventory-storage",
			chartKind:     ChartKindBar,
			wired:         uc.InventoryFlowAccuracy != nil,
			fetch:         uc.fetchInventoryFlowAccuracy,
			freshness:     uc.freshnessInventoryFlowAccuracy,
		},
		{
			id:            "catalog-growth",
			title:         "Catalog Growth",
			sourceContext: "facility-layout",
			chartKind:     ChartKindLine,
			wired:         uc.CatalogGrowth != nil,
			fetch:         uc.fetchCatalogGrowth,
			freshness:     uc.freshnessCatalogGrowth,
		},
	})
}

// fetchOrderFunnel sums every row's counters across the whole window
// into the four funnel stages. Allocated folds in partial allocations
// (an order that got some of what it asked for did pass the allocation
// stage), and the failure bucket folds cancellations together with
// allocation failures -- both are "this order left the funnel without
// being released".
func (uc *ConsoleReports) fetchOrderFunnel(ctx context.Context, from, to time.Time) ([]SeriesPoint, error) {
	report, err := uc.OrderFunnel.GetFunnelReport(ctx, from, to)
	if err != nil {
		return nil, err
	}

	var received, allocated, released, cancelledOrFailed int
	for _, row := range report.Rows {
		received += row.OrdersReceived
		allocated += row.OrdersAllocated + row.OrdersPartiallyAllocated
		released += row.OrdersReleased
		cancelledOrFailed += row.OrdersCancelled + row.OrdersAllocationFailed
	}

	return []SeriesPoint{
		{Label: "Received", Value: float64(received)},
		{Label: "Allocated", Value: float64(allocated)},
		{Label: "Released", Value: float64(released)},
		{Label: "Cancelled / Failed", Value: float64(cancelledOrFailed)},
	}, nil
}

func (uc *ConsoleReports) freshnessOrderFunnel(ctx context.Context) (float64, error) {
	return uc.OrderFunnel.GetFunnelFreshnessLagSeconds(ctx)
}

// fetchInventoryFlowAccuracy sums the four movement/accuracy counters
// across every (sku, bin, hour) row in the window.
func (uc *ConsoleReports) fetchInventoryFlowAccuracy(ctx context.Context, from, to time.Time) ([]SeriesPoint, error) {
	report, err := uc.InventoryFlowAccuracy.GetFlowAccuracyReport(ctx, from, to)
	if err != nil {
		return nil, err
	}

	var stowed, picked, discrepancies, unlocated int
	for _, row := range report.Rows {
		stowed += row.StowedCount
		picked += row.PickedQuantity
		discrepancies += row.DiscrepanciesDetected
		unlocated += row.UnlocatedCount
	}

	return []SeriesPoint{
		{Label: "Stowed", Value: float64(stowed)},
		{Label: "Picked", Value: float64(picked)},
		{Label: "Discrepancies", Value: float64(discrepancies)},
		{Label: "Unlocated", Value: float64(unlocated)},
	}, nil
}

func (uc *ConsoleReports) freshnessInventoryFlowAccuracy(ctx context.Context) (float64, error) {
	return uc.InventoryFlowAccuracy.GetFlowAccuracyFreshnessLagSeconds(ctx)
}

// fetchCatalogGrowth plots one point per distinct dayBucket, summing
// slotsRegistered across every scope that falls in the same day (a day
// can carry several scope rows; the growth line wants the site-wide
// total, not one line per scope).
func (uc *ConsoleReports) fetchCatalogGrowth(ctx context.Context, from, to time.Time) ([]SeriesPoint, error) {
	report, err := uc.CatalogGrowth.GetCatalogGrowthReport(ctx, from, to)
	if err != nil {
		return nil, err
	}

	byDay := make(map[string]float64, len(report.Rows))
	for _, row := range report.Rows {
		byDay[row.DayBucket] += float64(row.SlotsRegistered)
	}
	return bucketSeries(byDay), nil
}

func (uc *ConsoleReports) freshnessCatalogGrowth(ctx context.Context) (float64, error) {
	return uc.CatalogGrowth.GetCatalogGrowthFreshnessLagSeconds(ctx)
}
