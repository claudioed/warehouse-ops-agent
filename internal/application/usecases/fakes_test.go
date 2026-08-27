package usecases_test

import (
	"context"
	"errors"

	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// fakeFacility, fakeWes, fakeFe, fakeWfm are table-driven, in-memory fakes
// of the outbound MCP-client ports this use case orchestrates — no live
// server, per the T4 card's guardrail ("Unit-tested against FAKED MCP
// client ports").

type fakeFacility struct {
	sites ports.SitesResult
	err   error
}

func (f *fakeFacility) ListSites(ctx context.Context) (ports.SitesResult, error) {
	if f.err != nil {
		return ports.SitesResult{}, f.err
	}
	return f.sites, nil
}
func (f *fakeFacility) GetSiteLayout(ctx context.Context, siteCode string) (ports.SiteLayout, error) {
	return ports.SiteLayout{}, errors.New("not implemented in fake")
}
func (f *fakeFacility) GetZoneGrid(ctx context.Context, zoneId string) (ports.ZoneGrid, error) {
	return ports.ZoneGrid{}, errors.New("not implemented in fake")
}

type fakeWes struct {
	backlog map[string]ports.BacklogTelemetry
	err     map[string]error
}

func (f *fakeWes) GetBacklogTelemetry(ctx context.Context, pathId string) (ports.BacklogTelemetry, error) {
	if err, ok := f.err[pathId]; ok {
		return ports.BacklogTelemetry{}, err
	}
	return f.backlog[pathId], nil
}
func (f *fakeWes) GetRebalanceRecommendation(ctx context.Context, pathId string) (ports.RebalanceRecommendation, error) {
	return ports.RebalanceRecommendation{}, errors.New("not implemented in fake")
}

type fakeFe struct {
	queue map[string]ports.QueueStatus
	stuck ports.StuckTasksResult
	err   map[string]error
	// stuckErr, when non-nil, is returned by DiagnoseStuckTasks
	// unconditionally, independent of the queue-keyed err map above.
	stuckErr error
}

func (f *fakeFe) GetQueueStatus(ctx context.Context, processPath string) (ports.QueueStatus, error) {
	if err, ok := f.err[processPath]; ok {
		return ports.QueueStatus{}, err
	}
	return f.queue[processPath], nil
}
func (f *fakeFe) FindClaimableWork(ctx context.Context, processPath string) (ports.ClaimableWorkResult, error) {
	return ports.ClaimableWorkResult{}, errors.New("not implemented in fake")
}
func (f *fakeFe) DiagnoseStuckTasks(ctx context.Context, withinSeconds int) (ports.StuckTasksResult, error) {
	if f.stuckErr != nil {
		return ports.StuckTasksResult{}, f.stuckErr
	}
	return f.stuck, nil
}

type fakeWfm struct {
	gaps map[string]ports.StaffingGap
	err  map[string]error
}

func (f *fakeWfm) GetStaffingGap(ctx context.Context, buildingId, shiftId, pathId string) (ports.StaffingGap, error) {
	if err, ok := f.err[pathId]; ok {
		return ports.StaffingGap{}, err
	}
	return f.gaps[pathId], nil
}
func (f *fakeWfm) ProposePathHeads(ctx context.Context, buildingId, pathId string, charge, plannedRate float64) (ports.ProposedHeads, error) {
	return ports.ProposedHeads{}, errors.New("not implemented in fake")
}

var (
	_ ports.FacilityLayoutClient       = (*fakeFacility)(nil)
	_ ports.WesWorkPlanningClient      = (*fakeWes)(nil)
	_ ports.FulfillmentExecutionClient = (*fakeFe)(nil)
	_ ports.WorkforceManagementClient  = (*fakeWfm)(nil)
)
