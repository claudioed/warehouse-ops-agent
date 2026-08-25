// Package telemetry is the outbound telemetry-reader adapter: it implements
// internal/ports.TelemetryReader against the warehouse-infra observability
// stack (Prometheus). T1 ships only a stub implementation — StubReader —
// that returns an empty result set for any query; it exists so the
// application layer, the composition root, and the architecture fitness
// tests all have a real (if inert) implementation to wire from the first
// commit. A real Prometheus HTTP API client (github.com/prometheus/client_golang/api)
// lands with whichever later slice first needs a concrete metric read.
package telemetry

import (
	"context"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// StubReader is a no-op ports.TelemetryReader: every query returns an empty
// result set and a nil error. It is the T1 placeholder implementation.
type StubReader struct{}

// NewStubReader builds a StubReader.
func NewStubReader() *StubReader {
	return &StubReader{}
}

var _ ports.TelemetryReader = (*StubReader)(nil)

func (s *StubReader) InstantQuery(_ context.Context, _ string) ([]ports.MetricSample, error) {
	return nil, nil
}

func (s *StubReader) RangeQuery(_ context.Context, _ string, _, _ time.Time, _ time.Duration) ([]ports.MetricSample, error) {
	return nil, nil
}
