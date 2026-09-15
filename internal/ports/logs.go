package ports

import "context"

// LogEntry is one log line read back from the log-aggregation backend, for
// the runtime-feedback signal report (see usecases.RuntimeSignals).
type LogEntry struct {
	Labels    map[string]string `json:"labels"`
	Line      string            `json:"line"`
	Timestamp string            `json:"timestamp"` // RFC3339, as returned by the backend
}

// LogReader is the outbound port over the warehouse-infra observability
// stack's log aggregator (Loki). Like TelemetryReader, it is read-only by
// construction: there is no write method on this interface. It exists so
// the runtime-feedback signal report (Phase 5 Task 5.3 of the
// harness-coverage-expansion plan) can surface log-level anomalies
// (ERROR/FATAL lines) across the fleet's services alongside Prometheus
// metric signals, without this agent ever gaining a path back into any
// upstream bounded context's own state.
type LogReader interface {
	// QueryErrorLines returns up to limit ERROR/FATAL-level log lines
	// emitted by any pod in namespace within the last `since` window,
	// newest first. An empty result with a nil error means "no matching
	// lines" -- callers must not treat that as a failure.
	QueryErrorLines(ctx context.Context, namespace string, sinceSeconds int64, limit int) ([]LogEntry, error)
}
