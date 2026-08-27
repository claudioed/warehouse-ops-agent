package ports

import (
	"context"
	"time"
)

// MetricSample is one instant-query result point from the telemetry-reader
// port: a scalar value with the label set that produced it (e.g.
// {"path_id": "pick-zone-a"}), at the time the sample was taken.
type MetricSample struct {
	Labels    map[string]string `json:"labels"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
}

// TelemetryReader is the outbound port over the warehouse-infra observability
// stack (Prometheus/OTel Collector). It is read-only by construction: there
// is no write method on this interface, mirroring the "read-side /
// decision-support" placement — the agent observes live operational signal
// (e.g. http.server.request.duration, queue-depth gauges, span-derived
// metrics) but never pushes telemetry of its own through this port.
//
// T1 ships this port and a stub implementation only; a real Prometheus HTTP
// API client lands with whichever later slice (T2/T3/T4) first needs a
// concrete metric.
type TelemetryReader interface {
	// InstantQuery evaluates a PromQL expression at the current time and
	// returns its result vector.
	InstantQuery(ctx context.Context, promQL string) ([]MetricSample, error)

	// RangeQuery evaluates a PromQL expression over [start, end] at the given
	// step and returns the resulting series as a flat, timestamp-ordered
	// sample list per label set.
	RangeQuery(ctx context.Context, promQL string, start, end time.Time, step time.Duration) ([]MetricSample, error)
}
