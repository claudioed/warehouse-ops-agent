package policy_test

import (
	"testing"

	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
)

func TestClassifyErrorRate(t *testing.T) {
	cases := []struct {
		name string
		rate float64
		want policy.SignalSeverity
	}{
		{"zero is normal", 0, policy.SeverityNormal},
		{"just under warning threshold", 0.0099, policy.SeverityNormal},
		{"at warning threshold", 0.01, policy.SeverityWarning},
		{"between warning and critical", 0.03, policy.SeverityWarning},
		{"just under critical threshold", 0.0499, policy.SeverityWarning},
		{"at critical threshold", 0.05, policy.SeverityCritical},
		{"well above critical", 0.5, policy.SeverityCritical},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := policy.ClassifyErrorRate(c.rate)
			if got != c.want {
				t.Errorf("ClassifyErrorRate(%v) = %v, want %v", c.rate, got, c.want)
			}
		})
	}
}

func TestClassifyLatencyP99(t *testing.T) {
	cases := []struct {
		name string
		ms   float64
		want policy.SignalSeverity
	}{
		{"zero is normal", 0, policy.SeverityNormal},
		{"just under warning threshold", 999, policy.SeverityNormal},
		{"at warning threshold", 1000, policy.SeverityWarning},
		{"between warning and critical", 2000, policy.SeverityWarning},
		{"just under critical threshold", 2999, policy.SeverityWarning},
		{"at critical threshold", 3000, policy.SeverityCritical},
		{"well above critical", 10000, policy.SeverityCritical},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := policy.ClassifyLatencyP99(c.ms)
			if got != c.want {
				t.Errorf("ClassifyLatencyP99(%v) = %v, want %v", c.ms, got, c.want)
			}
		})
	}
}

func TestServiceSignal_SeverityIsTheWorstOfItsSignals(t *testing.T) {
	cases := []struct {
		name            string
		errorRateSev    policy.SignalSeverity
		latencyP99Sev   policy.SignalSeverity
		recentErrorLogs int
		want            policy.SignalSeverity
	}{
		{"all normal, no logs", policy.SeverityNormal, policy.SeverityNormal, 0, policy.SeverityNormal},
		{"error rate critical wins", policy.SeverityCritical, policy.SeverityNormal, 0, policy.SeverityCritical},
		{"latency critical wins", policy.SeverityNormal, policy.SeverityCritical, 0, policy.SeverityCritical},
		{"warning beats normal", policy.SeverityWarning, policy.SeverityNormal, 0, policy.SeverityWarning},
		{"critical beats warning", policy.SeverityCritical, policy.SeverityWarning, 0, policy.SeverityCritical},
		{"a single recent error log bumps normal to at least warning", policy.SeverityNormal, policy.SeverityNormal, 1, policy.SeverityWarning},
		{"recent error logs never downgrade an existing critical", policy.SeverityCritical, policy.SeverityNormal, 5, policy.SeverityCritical},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sig := policy.ServiceSignal{
				ErrorRateSev:    c.errorRateSev,
				LatencyP99Sev:   c.latencyP99Sev,
				RecentErrorLogs: c.recentErrorLogs,
			}
			got := sig.Severity()
			if got != c.want {
				t.Errorf("Severity() = %v, want %v", got, c.want)
			}
		})
	}
}
