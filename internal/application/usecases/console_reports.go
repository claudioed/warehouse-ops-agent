// ConsoleReports is the console-bff's SECOND capability (after
// OrderLifecycle): it fans out to every bounded context's already-built
// analytics /reports/... endpoint and aggregates each one into a
// chart-ready dashboard section for the warehouse-console frontend.
//
// It mirrors OrderLifecycle's degrade-gracefully-per-stage philosophy
// exactly, one level up: there, a slow or down upstream degrades that
// ONE stage to nil; here, it degrades that ONE section to
// available=false with a short human error and an empty series. A dead
// upstream must never 500 the whole dashboard -- an operator looking at
// four panels should lose one panel, not the screen.
//
// It diverges from OrderLifecycle in one deliberate way: the fan-out is
// CONCURRENT. OrderLifecycle's v1 is sequential by its own documented
// choice, which is right for a 4-hop single-entity lookup where each hop
// feeds the next (tasks are keyed by the work-unit ids the previous hop
// returned). Here the 3-4 calls per dashboard are fully independent, so
// serialising them would multiply the dashboard's latency by the number
// of contexts for no benefit at all.
package usecases

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// Chart kinds. This exact three-value vocabulary is the contract with
// warehouse-console, which switches on it to pick warehouse-ui-kit's
// FunnelChart / BarChart / LineChart component. All three take a
// {label, value}[] series, which is why every section -- funnel, bar or
// line -- produces the same SeriesPoint shape.
const (
	ChartKindFunnel = "funnel"
	ChartKindBar    = "bar"
	ChartKindLine   = "line"
)

// DefaultReportWindow is the trailing window used when the caller omits
// from/to. These two BFF endpoints deliberately relax their upstreams'
// required-from/to contract: this is a dashboard a human loads without
// necessarily knowing what window to ask for.
const DefaultReportWindow = 24 * time.Hour

// SeriesPoint is one plottable point. Its JSON shape (via the inbound
// adapter's DTO) must match warehouse-ui-kit's chart `data` prop exactly.
type SeriesPoint struct {
	Label string
	Value float64
}

// ReportSection is one dashboard panel: its identity, which context it
// came from, how to draw it, and either a series or the reason there
// isn't one.
type ReportSection struct {
	Id            string
	Title         string
	SourceContext string
	ChartKind     string

	// Available reports whether the upstream answered. When false,
	// Error carries a short human-readable reason and Series is empty.
	Available bool
	Error     string

	// FreshnessLagSeconds is how far that context's projection trails
	// its event stream, from its own /reports/.../freshness endpoint.
	// Nil when the freshness call failed even though the report call
	// succeeded -- freshness is a nice-to-have annotation, never a gate
	// on Available.
	FreshnessLagSeconds *float64

	Series []SeriesPoint
}

// DashboardResult is the assembled envelope. Both dashboards share it
// identically, so the console renders either with one component.
type DashboardResult struct {
	From        time.Time
	To          time.Time
	GeneratedAt time.Time
	Sections    []ReportSection
}

// ConsoleReports holds the seven analytics clients. Every field is
// optional: an unwired client degrades its own section to unavailable
// (same never-panic-on-nil convention as OrderLifecycle's gather* helpers)
// rather than failing the dashboard or the process.
type ConsoleReports struct {
	OrderFunnel           ports.OrderFunnelReportClient
	InventoryFlowAccuracy ports.FlowAccuracyReportClient
	CatalogGrowth         ports.CatalogGrowthReportClient
	PlanningThroughput    ports.PlanningThroughputReportClient
	FulfillmentThroughput ports.FulfillmentThroughputReportClient
	Labor                 ports.LaborReportClient
	LaborPerformance      ports.LaborPerformanceReportClient

	// Logger receives a structured warning per degraded section,
	// mirroring OrderLifecycle's observability convention. Defaults to
	// slog.Default() when nil.
	Logger *slog.Logger

	// Now is injectable so tests can assert a deterministic default
	// window and generatedAt, mirroring DailyBrief's own Now field.
	// Defaults to time.Now when nil.
	Now func() time.Time
}

// ResolveWindow applies the optional-window rule: a caller-supplied
// from/to is used verbatim, and anything missing falls back to a
// DefaultReportWindow trailing window ending now. Exported so the
// inbound adapter can echo the effective window it actually queried.
func (uc *ConsoleReports) ResolveWindow(from, to *time.Time) (time.Time, time.Time) {
	now := uc.now()
	resolvedTo := now
	if to != nil {
		resolvedTo = *to
	}
	resolvedFrom := resolvedTo.Add(-DefaultReportWindow)
	if from != nil {
		resolvedFrom = *from
	}
	return resolvedFrom, resolvedTo
}

func (uc *ConsoleReports) now() time.Time {
	if uc.Now != nil {
		return uc.Now()
	}
	return time.Now()
}

func (uc *ConsoleReports) logger() *slog.Logger {
	if uc.Logger != nil {
		return uc.Logger
	}
	return slog.Default()
}

// sectionSpec is one dashboard panel's recipe: its static identity plus
// the two upstream calls that fill it. Adding an eighth context is a new
// spec plus a new aggregate function -- never a change to the fan-out
// machinery below.
type sectionSpec struct {
	id            string
	title         string
	sourceContext string
	chartKind     string

	// wired reports whether this section's client is configured at all.
	// A nil interface can't be compared usefully once boxed, so each
	// dashboard passes the check explicitly.
	wired bool

	// fetch performs the report call and aggregates it into a series.
	// Nil is never valid when wired is true.
	fetch func(ctx context.Context, from, to time.Time) ([]SeriesPoint, error)

	// freshness performs that context's freshness call. Its failure
	// degrades only the annotation, never the section.
	freshness func(ctx context.Context) (float64, error)
}

// runSections executes every spec CONCURRENTLY and assembles them back
// in declaration order (the console renders panels top-to-bottom in the
// order this slice defines, so the order must not depend on which
// upstream answered first).
//
// Each section is fully isolated: its error is captured into its own
// ReportSection and never propagated, which is the whole point -- this
// method has no error return by design.
func (uc *ConsoleReports) runSections(ctx context.Context, from, to time.Time, specs []sectionSpec) DashboardResult {
	sections := make([]ReportSection, len(specs))

	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Add(1)
		go func(i int, spec sectionSpec) {
			defer wg.Done()
			sections[i] = uc.runSection(ctx, from, to, spec)
		}(i, spec)
	}
	wg.Wait()

	return DashboardResult{
		From:        from,
		To:          to,
		GeneratedAt: uc.now(),
		Sections:    sections,
	}
}

// runSection is the per-panel degrade-gracefully rule in one place:
// unwired or erroring upstream -> available=false + a short reason + an
// empty (never nil) series; succeeded -> available=true, with freshness
// attached only if that second call also worked.
func (uc *ConsoleReports) runSection(ctx context.Context, from, to time.Time, spec sectionSpec) ReportSection {
	section := ReportSection{
		Id:            spec.id,
		Title:         spec.title,
		SourceContext: spec.sourceContext,
		ChartKind:     spec.chartKind,
		Series:        []SeriesPoint{},
	}

	if !spec.wired || spec.fetch == nil {
		section.Error = spec.sourceContext + " reports not configured"
		uc.logger().Warn("console_reports: section unwired",
			"section", spec.id, "sourceContext", spec.sourceContext)
		return section
	}

	series, err := spec.fetch(ctx, from, to)
	if err != nil {
		section.Error = spec.sourceContext + " reports not available"
		uc.logger().Warn("console_reports: section unavailable",
			"section", spec.id,
			"sourceContext", spec.sourceContext,
			"error", sanitizeForLog(err.Error()))
		return section
	}

	section.Available = true
	if series != nil {
		section.Series = series
	}

	if spec.freshness != nil {
		if lag, err := spec.freshness(ctx); err != nil {
			// Deliberately NOT a degradation: the report itself is
			// good, we just can't annotate how stale it is.
			uc.logger().Warn("console_reports: freshness unavailable",
				"section", spec.id,
				"sourceContext", spec.sourceContext,
				"error", sanitizeForLog(err.Error()))
		} else {
			section.FreshnessLagSeconds = &lag
		}
	}

	return section
}

// bucketSeries collapses a set of (bucket -> summed value) pairs into a
// series sorted ascending by bucket label. The two line charts (catalog
// growth by dayBucket, planning throughput by hourBucket) both need
// exactly this, and both upstreams emit RFC3339 UTC buckets, for which
// lexicographic order IS chronological order -- so a plain string sort
// is correct here and needs no time parsing that could itself fail on a
// malformed upstream bucket.
func bucketSeries(totals map[string]float64) []SeriesPoint {
	labels := make([]string, 0, len(totals))
	for label := range totals {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	series := make([]SeriesPoint, 0, len(labels))
	for _, label := range labels {
		series = append(series, SeriesPoint{Label: label, Value: totals[label]})
	}
	return series
}
