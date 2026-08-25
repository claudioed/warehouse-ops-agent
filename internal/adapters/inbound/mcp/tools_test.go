package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

type fakeFacility struct{ sites ports.SitesResult }

func (f *fakeFacility) ListSites(ctx context.Context) (ports.SitesResult, error) { return f.sites, nil }
func (f *fakeFacility) GetSiteLayout(ctx context.Context, siteCode string) (ports.SiteLayout, error) {
	return ports.SiteLayout{}, nil
}
func (f *fakeFacility) GetZoneGrid(ctx context.Context, zoneId string) (ports.ZoneGrid, error) {
	return ports.ZoneGrid{}, nil
}

type fakeWes struct{ backlog ports.BacklogTelemetry }

func (f *fakeWes) GetBacklogTelemetry(ctx context.Context, pathId string) (ports.BacklogTelemetry, error) {
	return f.backlog, nil
}
func (f *fakeWes) GetRebalanceRecommendation(ctx context.Context, pathId string) (ports.RebalanceRecommendation, error) {
	return ports.RebalanceRecommendation{}, nil
}

type fakeFe struct {
	queue ports.QueueStatus
	stuck ports.StuckTasksResult
}

func (f *fakeFe) GetQueueStatus(ctx context.Context, processPath string) (ports.QueueStatus, error) {
	return f.queue, nil
}
func (f *fakeFe) FindClaimableWork(ctx context.Context, processPath string) (ports.ClaimableWorkResult, error) {
	return ports.ClaimableWorkResult{}, nil
}
func (f *fakeFe) DiagnoseStuckTasks(ctx context.Context, withinSeconds int) (ports.StuckTasksResult, error) {
	return f.stuck, nil
}

type fakeWfm struct{ gap ports.StaffingGap }

func (f *fakeWfm) GetStaffingGap(ctx context.Context, buildingId, shiftId, pathId string) (ports.StaffingGap, error) {
	return f.gap, nil
}
func (f *fakeWfm) ProposePathHeads(ctx context.Context, buildingId, pathId string, charge, plannedRate float64) (ports.ProposedHeads, error) {
	return ports.ProposedHeads{}, nil
}

func newTestDeps(imbalanced bool) Deps {
	if imbalanced {
		return Deps{DailyBrief: &usecases.DailyBrief{
			Facility: &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{{Code: "WH1", Name: "One"}}}},
			Wes:      &fakeWes{backlog: ports.BacklogTelemetry{PathId: "pick-zone-a", BacklogDepth: 50, WIP: 10, OverAlarmThreshold: true}},
			Fe: &fakeFe{
				queue: ports.QueueStatus{ProcessPath: "PICK", Depth: 40},
				stuck: ports.StuckTasksResult{Count: 1, Tasks: []ports.StuckTask{{TaskId: "t1", Type: "PICK"}}},
			},
			Wfm:     &fakeWfm{gap: ports.StaffingGap{PathId: "pick-zone-a", PlannedHeads: 5, ActiveHeads: 2, Understaffed: true}},
			Targets: []usecases.PathTarget{{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK"}},
			Now:     func() time.Time { return time.Unix(0, 0) },
		}}
	}
	return Deps{DailyBrief: &usecases.DailyBrief{
		Facility: &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{{Code: "WH1", Name: "One"}}}},
		Wes:      &fakeWes{backlog: ports.BacklogTelemetry{PathId: "pick-zone-a", BacklogDepth: 4, WIP: 2}},
		Fe: &fakeFe{
			queue: ports.QueueStatus{ProcessPath: "PICK", Depth: 3},
			stuck: ports.StuckTasksResult{Count: 0},
		},
		Wfm:     &fakeWfm{gap: ports.StaffingGap{PathId: "pick-zone-a", PlannedHeads: 3, ActiveHeads: 3}},
		Targets: []usecases.PathTarget{{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK"}},
		Now:     func() time.Time { return time.Unix(0, 0) },
	}}
}

func TestGetDailyBrief(t *testing.T) {
	deps := newTestDeps(true)
	out, err := deps.getDailyBrief(context.Background(), dailyBriefInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Sites) != 1 || out.Sites[0].SiteCode != "WH1" {
		t.Fatalf("unexpected sites: %+v", out.Sites)
	}
	if len(out.OpenExceptions) != 1 {
		t.Fatalf("expected 1 open exception, got %d", len(out.OpenExceptions))
	}
}

func TestListOpenExceptions_NoFilter_ReturnsAll(t *testing.T) {
	deps := newTestDeps(true)
	out, err := deps.listOpenExceptions(context.Background(), listOpenExceptionsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Count != 1 {
		t.Fatalf("count = %d, want 1", out.Count)
	}
}

func TestListOpenExceptions_HealthyBrief_ReturnsEmpty(t *testing.T) {
	deps := newTestDeps(false)
	out, err := deps.listOpenExceptions(context.Background(), listOpenExceptionsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Count != 0 {
		t.Fatalf("count = %d, want 0 for a healthy brief", out.Count)
	}
}

func TestListOpenExceptions_SeverityFilter(t *testing.T) {
	deps := newTestDeps(true) // produces exactly one CRITICAL exception (3 signals).

	tests := []struct {
		name     string
		severity string
		wantN    int
	}{
		{"critical filter includes the critical exception", "critical", 1},
		{"warning filter (>=warning severity) includes critical too", "warning", 1},
		{"info filter includes everything", "info", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := deps.listOpenExceptions(context.Background(), listOpenExceptionsInput{Severity: tc.severity})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Count != tc.wantN {
				t.Fatalf("count = %d, want %d", out.Count, tc.wantN)
			}
		})
	}
}

func TestListOpenExceptions_UnknownSeverityRejected(t *testing.T) {
	deps := newTestDeps(true)
	if _, err := deps.listOpenExceptions(context.Background(), listOpenExceptionsInput{Severity: "apocalyptic"}); err == nil {
		t.Fatal("expected an error for an unknown severity value, model input must be validated")
	}
}
