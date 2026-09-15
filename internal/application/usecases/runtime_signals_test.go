package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// fakeTelemetry is a table-driven, in-memory fake of ports.TelemetryReader
// -- returns a canned sample set per PromQL string, or a canned error.
type fakeTelemetry struct {
	byQuery map[string][]ports.MetricSample
	err     error
}

func (f *fakeTelemetry) InstantQuery(_ context.Context, promQL string) ([]ports.MetricSample, error) {
	if f.err != nil {
		return nil, f.err
	}
	for q, samples := range f.byQuery {
		if q == promQL {
			return samples, nil
		}
	}
	return nil, nil
}

func (f *fakeTelemetry) RangeQuery(_ context.Context, _ string, _, _ time.Time, _ time.Duration) ([]ports.MetricSample, error) {
	return nil, errors.New("not implemented in fake")
}

// fakeLogs is a table-driven, in-memory fake of ports.LogReader.
type fakeLogs struct {
	entries []ports.LogEntry
	err     error
}

func (f *fakeLogs) QueryErrorLines(_ context.Context, _ string, _ int64, _ int) ([]ports.LogEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entries, nil
}

func runtimeSignalsFixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestRuntimeSignals_NormalWhenNoTraffic(t *testing.T) {
	uc := &usecases.RuntimeSignals{
		Telemetry: &fakeTelemetry{byQuery: map[string][]ports.MetricSample{}},
		Logs:      &fakeLogs{},
		Services:  []string{"order-management"},
		Namespace: "warehouse-systems",
		Now:       runtimeSignalsFixedClock(time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)),
	}

	report := uc.Execute(context.Background())

	if len(report.Services) != 1 {
		t.Fatalf("want 1 service, got %d", len(report.Services))
	}
	svc := report.Services[0]
	if svc.Severity() != policy.SeverityNormal {
		t.Errorf("want NORMAL severity with zero traffic, got %v", svc.Severity())
	}
	if len(report.UnavailableSources) != 0 {
		t.Errorf("want no unavailable sources, got %v", report.UnavailableSources)
	}
}

func TestRuntimeSignals_CriticalErrorRate(t *testing.T) {
	// 100 total requests, 10 of them 5xx => 10% error rate, above the
	// 5% CRITICAL threshold.
	uc := &usecases.RuntimeSignals{
		Telemetry: &fakeTelemetry{byQuery: map[string][]ports.MetricSample{
			`sum(increase(istio_requests_total{destination_service_name="order-management"}[10m]))`: {
				{Value: 100},
			},
			`sum(increase(istio_requests_total{destination_service_name="order-management",response_code=~"5.."}[10m]))`: {
				{Value: 10},
			},
		}},
		Logs:      &fakeLogs{},
		Services:  []string{"order-management"},
		Namespace: "warehouse-systems",
		Now:       runtimeSignalsFixedClock(time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)),
	}

	report := uc.Execute(context.Background())

	svc := report.Services[0]
	if svc.ErrorRate != 0.1 {
		t.Errorf("want error rate 0.1, got %v", svc.ErrorRate)
	}
	if svc.ErrorRateSev != policy.SeverityCritical {
		t.Errorf("want CRITICAL severity at 10%% error rate, got %v", svc.ErrorRateSev)
	}
	if svc.Severity() != policy.SeverityCritical {
		t.Errorf("want overall severity CRITICAL, got %v", svc.Severity())
	}
}

func TestRuntimeSignals_RecentErrorLogsBumpSeverityToAtLeastWarning(t *testing.T) {
	uc := &usecases.RuntimeSignals{
		Telemetry: &fakeTelemetry{byQuery: map[string][]ports.MetricSample{}},
		Logs: &fakeLogs{entries: []ports.LogEntry{
			{Labels: map[string]string{"app": "order-management"}, Line: "ERROR: something broke", Timestamp: "2026-09-14T12:00:00Z"},
		}},
		Services:  []string{"order-management"},
		Namespace: "warehouse-systems",
		Now:       runtimeSignalsFixedClock(time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)),
	}

	report := uc.Execute(context.Background())

	svc := report.Services[0]
	if svc.RecentErrorLogs != 1 {
		t.Fatalf("want 1 recent error log, got %d", svc.RecentErrorLogs)
	}
	if svc.Severity() != policy.SeverityWarning {
		t.Errorf("want overall severity WARNING when a recent error log exists, got %v", svc.Severity())
	}
}

func TestRuntimeSignals_DegradesOnSourceFailureRatherThanFailing(t *testing.T) {
	uc := &usecases.RuntimeSignals{
		Telemetry: &fakeTelemetry{err: errors.New("prometheus unreachable")},
		Logs:      &fakeLogs{err: errors.New("loki unreachable")},
		Services:  []string{"order-management"},
		Namespace: "warehouse-systems",
		Now:       runtimeSignalsFixedClock(time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)),
	}

	report := uc.Execute(context.Background())

	if len(report.Services) != 1 {
		t.Fatalf("want 1 service even on total source failure, got %d", len(report.Services))
	}
	if report.Services[0].Severity() != policy.SeverityNormal {
		t.Errorf("want severity to degrade to NORMAL (not crash/panic) on total source failure, got %v", report.Services[0].Severity())
	}
	if len(report.UnavailableSources) != 2 {
		t.Errorf("want both sources (prometheus, loki) reported unavailable, got %v", report.UnavailableSources)
	}
}

func TestRuntimeSignals_NilLogsPortReportedUnavailable(t *testing.T) {
	uc := &usecases.RuntimeSignals{
		Telemetry: &fakeTelemetry{byQuery: map[string][]ports.MetricSample{}},
		Logs:      nil,
		Services:  []string{"order-management"},
		Namespace: "warehouse-systems",
	}

	report := uc.Execute(context.Background())

	found := false
	for _, s := range report.UnavailableSources {
		if s == "loki" {
			found = true
		}
	}
	if !found {
		t.Errorf("want loki reported unavailable when Logs is nil, got %v", report.UnavailableSources)
	}
}
