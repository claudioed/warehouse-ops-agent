package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

func TestExplainTravelFactor_SignificantDistance(t *testing.T) {
	deps := Deps{
		ExplainTravelFactor: &usecases.ExplainTravelFactor{
			Facility: &fakeFacility{travel: ports.TravelDistance{MetresM: 90.0, Estimated: false}},
		},
	}

	out, err := deps.explainTravelFactor(context.Background(), explainTravelFactorInput{
		PathId:           "pick-a",
		FromLocationCode: "WH1-STOR-AMB-A07-01-01-A",
		ToLocationCode:   "WH1-STOR-AMB-A09-03-01-A",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.MetresM != 90.0 {
		t.Errorf("MetresM = %v, want 90.0", out.MetresM)
	}
	if out.Kind != "travel_significant" {
		t.Errorf("Kind = %q, want travel_significant", out.Kind)
	}
	if out.Rationale == "" {
		t.Error("expected a non-empty Rationale")
	}
}

func TestExplainTravelFactor_UpstreamError_Propagates(t *testing.T) {
	deps := Deps{
		ExplainTravelFactor: &usecases.ExplainTravelFactor{
			Facility: &fakeFacility{err: errors.New("facility-layout unreachable")},
		},
	}

	if _, err := deps.explainTravelFactor(context.Background(), explainTravelFactorInput{
		FromLocationCode: "WH1-STOR-AMB-A07-01-01-A",
		ToLocationCode:   "WH1-STOR-AMB-A09-03-01-A",
	}); err == nil {
		t.Fatal("expected an error to propagate from the use case")
	}
}
