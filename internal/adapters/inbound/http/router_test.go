package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	inboundhttp "github.com/claudioed/warehouse-ops-agent/internal/adapters/inbound/http"
	"github.com/claudioed/warehouse-ops-agent/internal/application/usecases"
	"github.com/claudioed/warehouse-ops-agent/internal/domain/policy"
	"github.com/claudioed/warehouse-ops-agent/internal/ports"
)

// fakeFacility, fakeWes, fakeFe, fakeWfm mirror the application-layer
// fakes (kept package-local here since Go test fakes aren't exported
// across packages).

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

func newTestDailyBrief() *usecases.DailyBrief {
	return &usecases.DailyBrief{
		Facility: &fakeFacility{sites: ports.SitesResult{Sites: []ports.SiteRef{{Code: "WH1", Name: "One"}}}},
		Wes:      &fakeWes{backlog: ports.BacklogTelemetry{PathId: "pick-zone-a", BacklogDepth: 50, WIP: 10, OverAlarmThreshold: true}},
		Fe: &fakeFe{
			queue: ports.QueueStatus{ProcessPath: "PICK", Depth: 40},
			stuck: ports.StuckTasksResult{Count: 1, Tasks: []ports.StuckTask{{TaskId: "t1", Type: "PICK"}}},
		},
		Wfm:     &fakeWfm{gap: ports.StaffingGap{PathId: "pick-zone-a", PlannedHeads: 5, ActiveHeads: 2, Understaffed: true}},
		Targets: []usecases.PathTarget{{SiteCode: "WH1", PathId: "pick-zone-a", ProcessPath: "PICK"}},
		Now:     func() time.Time { return time.Unix(0, 0) },
	}
}

func TestGetDailyBrief_Returns200WithSynthesizedBrief(t *testing.T) {
	handlers := &inboundhttp.Handlers{DailyBrief: newTestDailyBrief()}
	router := inboundhttp.NewRouter(handlers, "warehouse-ops-agent-test")

	req := httptest.NewRequest(http.MethodGet, "/daily-brief", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Sites []struct {
			SiteCode string `json:"siteCode"`
			Paths    []struct {
				PathId     string `json:"pathId"`
				Exceptions []struct {
					Severity string `json:"severity"`
				} `json:"exceptions"`
			} `json:"paths"`
		} `json:"sites"`
		OpenExceptions []struct {
			Severity string `json:"severity"`
		} `json:"openExceptions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, rec.Body.String())
	}

	if len(body.Sites) != 1 || body.Sites[0].SiteCode != "WH1" {
		t.Fatalf("unexpected sites in response: %+v", body.Sites)
	}
	if len(body.OpenExceptions) != 1 {
		t.Fatalf("expected 1 open exception (3 correlated signals), got %d", len(body.OpenExceptions))
	}
	if body.OpenExceptions[0].Severity != string(policy.SeverityCritical) {
		t.Errorf("severity = %q, want critical", body.OpenExceptions[0].Severity)
	}
}

func TestGetDailyBrief_EmptyTargets_Returns200EmptyBrief(t *testing.T) {
	uc := &usecases.DailyBrief{
		Facility: &fakeFacility{},
		Wes:      &fakeWes{},
		Fe:       &fakeFe{},
		Wfm:      &fakeWfm{},
		Now:      func() time.Time { return time.Unix(0, 0) },
	}
	handlers := &inboundhttp.Handlers{DailyBrief: uc}
	router := inboundhttp.NewRouter(handlers, "warehouse-ops-agent-test")

	req := httptest.NewRequest(http.MethodGet, "/daily-brief", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHealthz_Returns200(t *testing.T) {
	handlers := &inboundhttp.Handlers{DailyBrief: newTestDailyBrief()}
	router := inboundhttp.NewRouter(handlers, "warehouse-ops-agent-test")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// --- flow-balance ------------------------------------------------------

type fbFakeWes struct{ recommendation ports.RebalanceRecommendation }

func (f *fbFakeWes) GetBacklogTelemetry(ctx context.Context, pathId string) (ports.BacklogTelemetry, error) {
	return ports.BacklogTelemetry{}, nil
}
func (f *fbFakeWes) GetRebalanceRecommendation(ctx context.Context, pathId string) (ports.RebalanceRecommendation, error) {
	return f.recommendation, nil
}

func newTestFlowBalanceAdvisory() *usecases.FlowBalanceAdvisory {
	return &usecases.FlowBalanceAdvisory{
		Wes: &fbFakeWes{recommendation: ports.RebalanceRecommendation{
			PathId: "pick-a", Action: "ReassignLabor", BacklogDepth: 90, WIP: 30,
		}},
		WFM: &fakeWfm{gap: ports.StaffingGap{PathId: "pick-a", PlannedHeads: 10, ActiveHeads: 6, Understaffed: true}},
		FE:  &fakeFe{stuck: ports.StuckTasksResult{Count: 0}},
	}
}

func TestGetFlowBalanceException_Returns200WithDecision(t *testing.T) {
	handlers := &inboundhttp.Handlers{
		DailyBrief:          newTestDailyBrief(),
		FlowBalanceAdvisory: newTestFlowBalanceAdvisory(),
	}
	router := inboundhttp.NewRouter(handlers, "warehouse-ops-agent-test")

	req := httptest.NewRequest(http.MethodGet, "/flow-balance/pick-a?buildingId=bldg-1&shiftId=shift-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		PathId            string `json:"pathId"`
		RecommendedAction string `json:"recommendedAction"`
		ProposedHeads     int    `json:"proposedHeads"`
		Evidence          []struct {
			Source string `json:"source"`
			Detail string `json:"detail"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, rec.Body.String())
	}
	if body.RecommendedAction != "assign_labor" {
		t.Errorf("recommendedAction = %q, want assign_labor", body.RecommendedAction)
	}
	if body.ProposedHeads != 4 {
		t.Errorf("proposedHeads = %d, want 4", body.ProposedHeads)
	}
	if len(body.Evidence) != 3 {
		t.Errorf("len(evidence) = %d, want 3: %+v", len(body.Evidence), body.Evidence)
	}
}

func TestGetFlowBalanceException_NotConfigured_Returns503(t *testing.T) {
	handlers := &inboundhttp.Handlers{DailyBrief: newTestDailyBrief()}
	router := inboundhttp.NewRouter(handlers, "warehouse-ops-agent-test")

	req := httptest.NewRequest(http.MethodGet, "/flow-balance/pick-a?buildingId=bldg-1&shiftId=shift-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
