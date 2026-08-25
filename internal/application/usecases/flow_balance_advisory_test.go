package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// --- faked outbound MCP-client ports (no live servers) ----------------

type fbFakeWes struct {
	recommendation ports.RebalanceRecommendation
	err            error
	calledPathId   string
}

func (f *fbFakeWes) GetBacklogTelemetry(ctx context.Context, pathId string) (ports.BacklogTelemetry, error) {
	return ports.BacklogTelemetry{}, errors.New("not used by FlowBalanceAdvisory")
}

func (f *fbFakeWes) GetRebalanceRecommendation(ctx context.Context, pathId string) (ports.RebalanceRecommendation, error) {
	f.calledPathId = pathId
	if f.err != nil {
		return ports.RebalanceRecommendation{}, f.err
	}
	return f.recommendation, nil
}

type fbFakeWFM struct {
	gap                                           ports.StaffingGap
	err                                           error
	calledBuildingId, calledShiftId, calledPathId string
}

func (f *fbFakeWFM) GetStaffingGap(ctx context.Context, buildingId, shiftId, pathId string) (ports.StaffingGap, error) {
	f.calledBuildingId, f.calledShiftId, f.calledPathId = buildingId, shiftId, pathId
	if f.err != nil {
		return ports.StaffingGap{}, f.err
	}
	return f.gap, nil
}

func (f *fbFakeWFM) ProposePathHeads(ctx context.Context, buildingId, pathId string, charge, plannedRate float64) (ports.ProposedHeads, error) {
	return ports.ProposedHeads{}, errors.New("not used by FlowBalanceAdvisory")
}

type fbFakeFE struct {
	result        ports.StuckTasksResult
	err           error
	calledSeconds int
}

func (f *fbFakeFE) GetQueueStatus(ctx context.Context, processPath string) (ports.QueueStatus, error) {
	return ports.QueueStatus{}, errors.New("not used by FlowBalanceAdvisory")
}

func (f *fbFakeFE) FindClaimableWork(ctx context.Context, processPath string) (ports.ClaimableWorkResult, error) {
	return ports.ClaimableWorkResult{}, errors.New("not used by FlowBalanceAdvisory")
}

func (f *fbFakeFE) DiagnoseStuckTasks(ctx context.Context, withinSeconds int) (ports.StuckTasksResult, error) {
	f.calledSeconds = withinSeconds
	if f.err != nil {
		return ports.StuckTasksResult{}, f.err
	}
	return f.result, nil
}

var (
	_ ports.WesWorkPlanningClient      = (*fbFakeWes)(nil)
	_ ports.WorkforceManagementClient  = (*fbFakeWFM)(nil)
	_ ports.FulfillmentExecutionClient = (*fbFakeFE)(nil)
)

// --- tests --------------------------------------------------------------

func TestFlowBalanceAdvisory_Execute(t *testing.T) {
	t.Run("all three signals healthy => assign_labor recommendation with evidence", func(t *testing.T) {
		wes := &fbFakeWes{recommendation: ports.RebalanceRecommendation{
			PathId: "pick-a", Action: "ReassignLabor", BacklogDepth: 120, WIP: 40,
		}}
		wfm := &fbFakeWFM{gap: ports.StaffingGap{
			PathId: "pick-a", PlannedHeads: 10, ActiveHeads: 6, Understaffed: true,
		}}
		fe := &fbFakeFE{result: ports.StuckTasksResult{Count: 0}}

		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe}
		got, err := uc.Execute(context.Background(), "bldg-1", "shift-1", "pick-a")
		if err != nil {
			t.Fatalf("Execute: unexpected error: %v", err)
		}

		if got.RecommendedAction != policy.ActionAssignLabor {
			t.Errorf("RecommendedAction = %q, want %q", got.RecommendedAction, policy.ActionAssignLabor)
		}
		if got.ProposedHeads != 4 {
			t.Errorf("ProposedHeads = %d, want 4", got.ProposedHeads)
		}
		if got.Partial {
			t.Errorf("Partial = true, want false: %+v", got.MissingSignals)
		}
		if len(got.Evidence) != 3 {
			t.Errorf("len(Evidence) = %d, want 3: %+v", len(got.Evidence), got.Evidence)
		}

		// Every port was called with the right scoping arguments.
		if wes.calledPathId != "pick-a" {
			t.Errorf("wes called with pathId=%q, want pick-a", wes.calledPathId)
		}
		if wfm.calledBuildingId != "bldg-1" || wfm.calledShiftId != "shift-1" || wfm.calledPathId != "pick-a" {
			t.Errorf("wfm called with (%q,%q,%q), want (bldg-1,shift-1,pick-a)", wfm.calledBuildingId, wfm.calledShiftId, wfm.calledPathId)
		}
		if fe.calledSeconds <= 0 {
			t.Errorf("fe called with withinSeconds=%d, want > 0", fe.calledSeconds)
		}
	})

	t.Run("wes call fails => degrades to partial hold, no hard error", func(t *testing.T) {
		wes := &fbFakeWes{err: errors.New("connection refused")}
		wfm := &fbFakeWFM{gap: ports.StaffingGap{Understaffed: true}}
		fe := &fbFakeFE{}

		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe}
		got, err := uc.Execute(context.Background(), "bldg-1", "shift-1", "pick-a")
		if err != nil {
			t.Fatalf("Execute: unexpected hard error on upstream failure: %v", err)
		}
		if !got.Partial {
			t.Error("expected a Partial decision when wes is unavailable")
		}
		if got.RecommendedAction != policy.FlowBalanceActionHold {
			t.Errorf("RecommendedAction = %q, want hold", got.RecommendedAction)
		}
	})

	t.Run("wfm and fe calls fail => partial hold, wes evidence still present", func(t *testing.T) {
		wes := &fbFakeWes{recommendation: ports.RebalanceRecommendation{PathId: "pick-a", Action: "NoActionNeeded"}}
		wfm := &fbFakeWFM{err: errors.New("timeout")}
		fe := &fbFakeFE{err: errors.New("timeout")}

		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe}
		got, err := uc.Execute(context.Background(), "bldg-1", "shift-1", "pick-a")
		if err != nil {
			t.Fatalf("Execute: unexpected error: %v", err)
		}
		if !got.Partial {
			t.Error("expected Partial=true")
		}
		if len(got.Evidence) != 1 {
			t.Errorf("len(Evidence) = %d, want 1 (wes only)", len(got.Evidence))
		}
	})

	t.Run("unrecognized wes action enum is rejected outright, not defaulted", func(t *testing.T) {
		wes := &fbFakeWes{recommendation: ports.RebalanceRecommendation{PathId: "pick-a", Action: "SomethingNewAndUnknown"}}
		wfm := &fbFakeWFM{}
		fe := &fbFakeFE{}

		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe}
		_, err := uc.Execute(context.Background(), "bldg-1", "shift-1", "pick-a")
		if err == nil {
			t.Fatal("expected an error for an unrecognized RebalanceAction enum value, got nil")
		}
	})

	t.Run("nil client ports treated as unavailable, not a panic", func(t *testing.T) {
		uc := &usecases.FlowBalanceAdvisory{}
		got, err := uc.Execute(context.Background(), "bldg-1", "shift-1", "pick-a")
		if err != nil {
			t.Fatalf("Execute: unexpected error with all-nil ports: %v", err)
		}
		if !got.Partial {
			t.Error("expected Partial=true when every port is nil")
		}
		if got.RecommendedAction != policy.FlowBalanceActionHold {
			t.Errorf("RecommendedAction = %q, want hold", got.RecommendedAction)
		}
		if len(got.Evidence) != 0 {
			t.Errorf("len(Evidence) = %d, want 0", len(got.Evidence))
		}
	})

	t.Run("NoActionNeeded + healthy signals => release_next_work, not a write call", func(t *testing.T) {
		wes := &fbFakeWes{recommendation: ports.RebalanceRecommendation{PathId: "pick-a", Action: "NoActionNeeded"}}
		wfm := &fbFakeWFM{gap: ports.StaffingGap{PlannedHeads: 4, ActiveHeads: 4, Understaffed: false}}
		fe := &fbFakeFE{result: ports.StuckTasksResult{Count: 0}}

		uc := &usecases.FlowBalanceAdvisory{Wes: wes, WFM: wfm, FE: fe}
		got, err := uc.Execute(context.Background(), "bldg-1", "shift-1", "pick-a")
		if err != nil {
			t.Fatalf("Execute: unexpected error: %v", err)
		}
		if got.RecommendedAction != policy.ActionReleaseNextWork {
			t.Errorf("RecommendedAction = %q, want release_next_work", got.RecommendedAction)
		}
	})
}
