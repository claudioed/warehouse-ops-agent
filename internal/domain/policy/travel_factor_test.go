package policy_test

import (
	"strings"
	"testing"

	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
)

func TestCorrelateTravelFactor(t *testing.T) {
	reading := func(metres float64, estimated bool) *policy.TravelDistanceReading {
		return &policy.TravelDistanceReading{
			Source:    "facility-layout.estimate_travel_distance",
			From:      "WH1-STOR-AMB-A07-01-01-A",
			To:        "WH1-STOR-AMB-A09-03-01-A",
			MetresM:   metres,
			Estimated: estimated,
		}
	}

	t.Run("distance above threshold => significant, cites both location codes and the metres figure", func(t *testing.T) {
		got := policy.CorrelateTravelFactor(reading(85.5, false))
		if got == nil {
			t.Fatal("expected a correlation, got nil")
		}
		if got.Kind != policy.TravelFactorOutcomeSignificant {
			t.Errorf("Kind = %q, want %q", got.Kind, policy.TravelFactorOutcomeSignificant)
		}
		if !strings.Contains(got.Rationale, "85.5m") {
			t.Errorf("rationale should cite the observed distance: %q", got.Rationale)
		}
		if !strings.Contains(got.Rationale, "WH1-STOR-AMB-A07-01-01-A") || !strings.Contains(got.Rationale, "WH1-STOR-AMB-A09-03-01-A") {
			t.Errorf("rationale should cite both location codes: %q", got.Rationale)
		}
		if strings.Contains(got.Rationale, "estimated from the travel graph") {
			t.Errorf("a measured (non-estimated) reading should not carry the estimated caveat: %q", got.Rationale)
		}
	})

	t.Run("distance at threshold => negligible (boundary is inclusive of the threshold itself)", func(t *testing.T) {
		got := policy.CorrelateTravelFactor(reading(policy.TravelFactorDistanceThresholdMetres, false))
		if got == nil {
			t.Fatal("expected a correlation, got nil")
		}
		if got.Kind != policy.TravelFactorOutcomeNegligible {
			t.Errorf("Kind = %q, want %q", got.Kind, policy.TravelFactorOutcomeNegligible)
		}
	})

	t.Run("distance below threshold => negligible", func(t *testing.T) {
		got := policy.CorrelateTravelFactor(reading(12.0, false))
		if got == nil {
			t.Fatal("expected a correlation, got nil")
		}
		if got.Kind != policy.TravelFactorOutcomeNegligible {
			t.Errorf("Kind = %q, want %q", got.Kind, policy.TravelFactorOutcomeNegligible)
		}
		if !strings.Contains(got.Rationale, "unlikely to materially explain") {
			t.Errorf("negligible rationale should redirect the caller to other evidence: %q", got.Rationale)
		}
	})

	t.Run("estimated route adds the estimated caveat to the rationale", func(t *testing.T) {
		got := policy.CorrelateTravelFactor(reading(200.0, true))
		if got == nil {
			t.Fatal("expected a correlation, got nil")
		}
		if !strings.Contains(got.Rationale, "estimated from the travel graph") {
			t.Errorf("an estimated reading's rationale must say so: %q", got.Rationale)
		}
	})

	t.Run("nil reading => nil correlation, never a crash", func(t *testing.T) {
		got := policy.CorrelateTravelFactor(nil)
		if got != nil {
			t.Errorf("expected nil correlation for a nil reading, got %+v", got)
		}
	})
}
